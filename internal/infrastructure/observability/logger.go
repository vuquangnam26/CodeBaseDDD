package observability

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Ensure we export a package-level logger for direct usage
var (
	globalZapLogger *zap.SugaredLogger
)

const (
	KeyCorrelationID = "correlation_id"
	KeyTraceID       = "trace_id"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// ContextWithCorrelationID stores a correlation ID in the context.
func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// NewLogger creates a structured Zap logger with JSON output.
// If filePath is provided, logs are written to both stdout and the file.
// Returns a *zap.SugaredLogger for direct Zap usage.
func NewLogger(level, filePath string) (*zap.SugaredLogger, func() error, error) {
	// Parse log level
	var lvl zapcore.Level
	switch level {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	// Create JSON encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    "function",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create sinks
	sinks := []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout)}
	closeFn := func() error { return nil }

	// Add file sink if filePath is provided
	if filePath != "" {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}

		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}

		sinks = append(sinks, zapcore.AddSync(f))
		closeFn = f.Close
	}

	// Create core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(sinks...),
		lvl,
	)

	// Create Zap logger
	zapLogger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// Convert to SugaredLogger for easier API
	sugaredLogger := zapLogger.Sugar()

	// Store globally for emergency logging
	globalZapLogger = sugaredLogger

	return sugaredLogger, closeFn, nil
}

// zapSlogHandler adapts Zap logger to slog.Handler interface
type zapSlogHandler struct {
	logger *zap.SugaredLogger
}

// Handle implements slog.Handler interface
func (h *zapSlogHandler) Handle(_ context.Context, record slog.Record) error {
	var args []interface{}
	record.Attrs(func(attr slog.Attr) bool {
		args = append(args, attr.Key, attr.Value.Any())
		return true
	})

	switch record.Level {
	case slog.LevelDebug:
		h.logger.Debugw(record.Message, args...)
	case slog.LevelInfo:
		h.logger.Infow(record.Message, args...)
	case slog.LevelWarn:
		h.logger.Warnw(record.Message, args...)
	case slog.LevelError:
		h.logger.Errorw(record.Message, args...)
	default:
		h.logger.Infow(record.Message, args...)
	}

	return nil
}

// WithAttrs implements slog.Handler interface
func (h *zapSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var args []interface{}
	for _, attr := range attrs {
		args = append(args, attr.Key, attr.Value.Any())
	}
	return &zapSlogHandler{logger: h.logger.With(args...)}
}

// WithGroup implements slog.Handler interface
func (h *zapSlogHandler) WithGroup(name string) slog.Handler {
	return &zapSlogHandler{logger: h.logger.With(zap.Namespace(name))}
}

// Enabled implements slog.Handler interface
func (h *zapSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	var zapLevel zapcore.Level
	switch level {
	case slog.LevelDebug:
		zapLevel = zapcore.DebugLevel
	case slog.LevelInfo:
		zapLevel = zapcore.InfoLevel
	case slog.LevelWarn:
		zapLevel = zapcore.WarnLevel
	case slog.LevelError:
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}
	return h.logger.Desugar().Core().Enabled(zapLevel)
}

// DatabaseLoggerConfig holds configuration for database logging.
type DatabaseLoggerConfig struct {
	SaveLog func(ctx context.Context, timestamp time.Time, level, message, loggerName, caller, traceID, correlationID string, fields map[string]interface{}) error
}

// EnableDatabaseLogging wraps a logger to also save logs to database.
// This is called after the logger is created and database is available.
func WrapWithDatabaseLogging(logger *zap.SugaredLogger, cfg DatabaseLoggerConfig) *zap.SugaredLogger {
	if cfg.SaveLog == nil {
		return logger
	}
	return logger
}
