package observability

import (
	"context"

	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
)

// InitLogs sets up OpenTelemetry logs with a simple processor.
// Logs are currently handled through the standard slog implementation.
// When OTel Logs exporter is stable, we can add OTLP export here.
func InitLogs(ctx context.Context, serviceName, otlpEndpoint string, logger *zap.SugaredLogger) (func(context.Context) error, error) {
	if otlpEndpoint == "" {
		logger.Infow("logs export disabled: no OTLP endpoint configured")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create a log provider with a simple processor.
	// The OTEL collector will receive logs via slog output that includes trace context.
	provider := log.NewLoggerProvider(
		log.WithResource(res),
	)

	global.SetLoggerProvider(provider)

	logger.Info("logs handling initialized",
		"note", "logs are exported via slog with trace context in structured JSON",
		"endpoint", otlpEndpoint,
	)
	return provider.Shutdown, nil
}
