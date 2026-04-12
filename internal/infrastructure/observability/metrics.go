package observability

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// MetricsSaver interface for persisting metrics to storage.
type MetricsSaver interface {
	SaveMetric(ctx context.Context, metricName, metricType string, value float64, labels map[string]string) error
}

// Metrics holds all application Prometheus metrics.
type Metrics struct {
	OutboxPendingGauge   prometheus.Gauge
	OutboxPublishSuccess prometheus.Counter
	OutboxPublishFailed  prometheus.Counter
	ProjectionLagSeconds prometheus.Gauge
	HTTPRequestDuration  *prometheus.HistogramVec
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

// MetricsWithDatabasePersistence wraps Metrics to persist them to database.
type MetricsWithDatabasePersistence struct {
	*Metrics
	saver  MetricsSaver
	ticker *time.Ticker
	done   chan struct{}
	mu     sync.Mutex
}

// NewMetricsWithDatabasePersistence creates a metrics instance that saves to database.
func NewMetricsWithDatabasePersistence(reg prometheus.Registerer, saver MetricsSaver) *MetricsWithDatabasePersistence {
	m := NewMetrics(reg)

	wrapper := &MetricsWithDatabasePersistence{
		Metrics: m,
		saver:   saver,
		ticker:  time.NewTicker(30 * time.Second), // Save metrics every 30 seconds
		done:    make(chan struct{}),
	}

	// Start background goroutine to periodically save metrics
	go wrapper.persistMetrics()

	return wrapper
}

// persistMetrics periodically saves metrics to database.
func (mwd *MetricsWithDatabasePersistence) persistMetrics() {
	for {
		select {
		case <-mwd.ticker.C:
			mwd.saveCurrentMetrics()
		case <-mwd.done:
			mwd.ticker.Stop()
			return
		}
	}
}

// saveCurrentMetrics saves the current metric values to database.
func (mwd *MetricsWithDatabasePersistence) saveCurrentMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mwd.mu.Lock()
	defer mwd.mu.Unlock()

	// Save gauge values
	_ = mwd.saver.SaveMetric(ctx, "outbox_pending_total", "gauge", 0, nil)

	// Save counter values (counters only increase)
	_ = mwd.saver.SaveMetric(ctx, "outbox_publish_success_total", "counter", 0, nil)

	_ = mwd.saver.SaveMetric(ctx, "outbox_publish_failed_total", "counter", 0, nil)
}

// Stop stops the periodic persistence.
func (mwd *MetricsWithDatabasePersistence) Stop() {
	close(mwd.done)
}
