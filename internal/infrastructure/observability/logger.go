package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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

// NewLogger creates a structured slog.Logger with JSON output.
// If filePath is provided, logs are written to both stdout and the file.
func NewLogger(level, filePath string) (*slog.Logger, func() error, error) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	writer := io.Writer(os.Stdout)
	closeFn := func() error { return nil }

	if filePath != "" {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, nil, err
		}

		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, err
		}

		writer = io.MultiWriter(os.Stdout, f)
		closeFn = f.Close
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     lvl,
		AddSource: true,
	})

	return slog.New(handler), closeFn, nil
}
