package observability

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all application Prometheus metrics.
type Metrics struct {
	OutboxPendingGauge      prometheus.Gauge
	OutboxPublishSuccess    prometheus.Counter
	OutboxPublishFailed     prometheus.Counter
	ProjectionLagSeconds    prometheus.Gauge
	HTTPRequestDuration     *prometheus.HistogramVec
}

// NewMetrics registers and returns all Prometheus metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		OutboxPendingGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "order_service",
			Name:      "outbox_pending_total",
			Help:      "Number of pending events in the outbox.",
		}),
		OutboxPublishSuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "order_service",
			Name:      "outbox_publish_success_total",
			Help:      "Total number of successfully published outbox events.",
		}),
		OutboxPublishFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "order_service",
			Name:      "outbox_publish_failed_total",
			Help:      "Total number of failed outbox publish attempts.",
		}),
		ProjectionLagSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "order_service",
			Name:      "projection_lag_seconds",
			Help:      "Projection lag in seconds.",
		}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "order_service",
			Name:      "http_request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
	}

	reg.MustRegister(
		m.OutboxPendingGauge,
		m.OutboxPublishSuccess,
		m.OutboxPublishFailed,
		m.ProjectionLagSeconds,
		m.HTTPRequestDuration,
	)

	return m
}
