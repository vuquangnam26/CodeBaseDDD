package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
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

	// 1. Create OTLP HTTP exporter
	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(signozEndpoint),
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

	// 3. Configure Batch processor
	processor := log.NewBatchProcessor(
		exporter,
	)

	// 4. Logger provider
	provider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)

	// 5. Create otelzap core
	otelCore := otelzap.NewCore(serviceName, otelzap.WithLoggerProvider(provider))

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
		"endpoint", signozEndpoint,
		"service_name", serviceName,
		"status", "CHECKING_CONNECTION",
	)

	return finalLogger.Sugar(), cleanup, nil
}
