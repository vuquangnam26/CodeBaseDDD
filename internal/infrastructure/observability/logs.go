package observability

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"github.com/go-logr/stdr"
	stdlog "log"
)

// InitSigNozLogger creates an otelzap-bridged Zap logger that sends logs
// directly to SigNoz's OTel Collector via OTLP HTTP.
func InitSigNozLogger(ctx context.Context, serviceName, signozEndpoint string, existingLogger *zap.SugaredLogger) (*zap.SugaredLogger, func(), error) {
	if signozEndpoint == "" {
		if existingLogger != nil {
			existingLogger.Infow("SigNoz logs disabled: no SIGNOZ_OTLP_ENDPOINT configured")
		}
		return existingLogger, func() {}, nil
	}

	endpoint, err := normalizeOTLPEndpoint(signozEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("normalize SigNoz OTLP endpoint: %w", err)
	}

	// 1. Create OTLP HTTP exporter
	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(endpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create OTLP log exporter: %w", err)
	}

	// 2. Setup Resource with more attributes
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
			attribute.String("environment", "development"),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource: %w", err)
	}

	// 3. Configure Simple processor for "synchronous" log delivery
	// Using SimpleProcessor ensures logs are sent to SigNoz immediately
	processor := sdklog.NewSimpleProcessor(
		exporter,
	)

	// 4. Logger provider
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	// 5. Create OTEL core that emits the same zap entries to SigNoz.
	otelCore := newOTelLogCore(provider.Logger(serviceName), zapcore.DebugLevel)

	// 6. Combine with existing logger if provided
	var finalLogger *zap.Logger
	if existingLogger != nil {
		combined := zapcore.NewTee(
			existingLogger.Desugar().Core(),
			otelCore,
		)
		finalLogger = zap.New(combined, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		finalLogger = zap.New(otelCore)
	}

	cleanup := func() {
		_ = finalLogger.Sync()
		// Ensure logs are flushed before shutdown
		_ = provider.Shutdown(context.Background())
	}

	// Set OTel internal logger to standard logger to see exporter errors
	otel.SetLogger(stdr.New(stdlog.New(os.Stderr, "OTEL: ", stdlog.LstdFlags)))

	finalLogger.Sugar().Infow("SigNoz logs initialized",
		"endpoint", endpoint,
		"service_name", serviceName,
		"status", "CHECKING_CONNECTION",
	)

	return finalLogger.Sugar(), cleanup, nil
}

func normalizeOTLPEndpoint(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}

	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return "", err
		}
		if parsed.Host == "" {
			return "", fmt.Errorf("missing host in endpoint %q", endpoint)
		}
		return parsed.Host, nil
	}

	return endpoint, nil
}

type otelLogCore struct {
	logger otellog.Logger
	level  zapcore.LevelEnabler
	fields []zapcore.Field
}

func newOTelLogCore(logger otellog.Logger, level zapcore.LevelEnabler) zapcore.Core {
	return &otelLogCore{logger: logger, level: level}
}

func (c *otelLogCore) Enabled(level zapcore.Level) bool {
	return c.level.Enabled(level)
}

func (c *otelLogCore) With(fields []zapcore.Field) zapcore.Core {
	combined := append(append([]zapcore.Field{}, c.fields...), fields...)
	return &otelLogCore{logger: c.logger, level: c.level, fields: combined}
}

func (c *otelLogCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *otelLogCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	allFields := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	allFields = append(allFields, c.fields...)
	allFields = append(allFields, fields...)

	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range allFields {
		field.AddTo(encoder)
	}

	var record otellog.Record
	record.SetTimestamp(entry.Time)
	record.SetObservedTimestamp(time.Now())
	record.SetSeverity(zapLevelToOTelSeverity(entry.Level))
	record.SetSeverityText(entry.Level.String())
	record.SetBody(otellog.StringValue(entry.Message))

	attrs := make([]otellog.KeyValue, 0, len(encoder.Fields)+4)
	for key, value := range encoder.Fields {
		attrs = append(attrs, otelKeyValue(key, value))
	}
	if entry.LoggerName != "" {
		attrs = append(attrs, otellog.String("logger", entry.LoggerName))
	}
	if entry.Caller.Defined {
		attrs = append(attrs,
			otellog.String("code.file.path", entry.Caller.File),
			otellog.Int("code.line.number", entry.Caller.Line),
		)
	}
	if entry.Stack != "" {
		attrs = append(attrs, otellog.String("stacktrace", entry.Stack))
	}
	if len(attrs) > 0 {
		record.AddAttributes(attrs...)
	}

	emitCtx := context.Background()
	if spanCtx := spanContextFromFields(encoder.Fields); spanCtx.IsValid() {
		emitCtx = trace.ContextWithSpanContext(emitCtx, spanCtx)
	}

	c.logger.Emit(emitCtx, record)
	return nil
}

func (c *otelLogCore) Sync() error { return nil }

func zapLevelToOTelSeverity(level zapcore.Level) otellog.Severity {
	switch level {
	case zapcore.DebugLevel:
		return otellog.SeverityDebug
	case zapcore.InfoLevel:
		return otellog.SeverityInfo
	case zapcore.WarnLevel:
		return otellog.SeverityWarn
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel:
		return otellog.SeverityError
	case zapcore.FatalLevel:
		return otellog.SeverityFatal
	default:
		return otellog.SeverityInfo
	}
}

func otelKeyValue(key string, value interface{}) otellog.KeyValue {
	switch v := value.(type) {
	case string:
		return otellog.String(key, v)
	case bool:
		return otellog.Bool(key, v)
	case int:
		return otellog.Int(key, v)
	case int8:
		return otellog.Int64(key, int64(v))
	case int16:
		return otellog.Int64(key, int64(v))
	case int32:
		return otellog.Int64(key, int64(v))
	case int64:
		return otellog.Int64(key, v)
	case uint:
		return otellog.Int64(key, int64(v))
	case uint8:
		return otellog.Int64(key, int64(v))
	case uint16:
		return otellog.Int64(key, int64(v))
	case uint32:
		return otellog.Int64(key, int64(v))
	case uint64:
		return otellog.Int64(key, int64(v))
	case float32:
		return otellog.Float64(key, float64(v))
	case float64:
		return otellog.Float64(key, v)
	case []byte:
		return otellog.Bytes(key, v)
	case error:
		return otellog.String(key, v.Error())
	case []string:
		values := make([]otellog.Value, 0, len(v))
		for _, item := range v {
			values = append(values, otellog.StringValue(item))
		}
		return otellog.Slice(key, values...)
	case map[string]interface{}:
		kvs := make([]otellog.KeyValue, 0, len(v))
		for nestedKey, nestedValue := range v {
			kvs = append(kvs, otelKeyValue(nestedKey, nestedValue))
		}
		return otellog.Map(key, kvs...)
	default:
		return otellog.String(key, fmt.Sprint(v))
	}
}

func spanContextFromFields(fields map[string]interface{}) trace.SpanContext {
	traceIDHex, _ := fields["trace_id"].(string)
	spanIDHex, _ := fields["span_id"].(string)
	if traceIDHex == "" || spanIDHex == "" {
		return trace.SpanContext{}
	}

	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		return trace.SpanContext{}
	}
	spanID, err := trace.SpanIDFromHex(spanIDHex)
	if err != nil {
		return trace.SpanContext{}
	}

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
}
