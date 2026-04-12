package worker

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/himmel/order-service/internal/application/port"
)

// OutboxWorker polls the outbox table and publishes events via the EventBus.
type OutboxWorker struct {
	outboxStore  port.OutboxStore
	publisher    port.Publisher
	logger       *zap.SugaredLogger
	batchSize    int
	pollInterval time.Duration

	// Metrics
	pendingGauge   prometheus.Gauge
	successCounter prometheus.Counter
	failedCounter  prometheus.Counter
}

func NewOutboxWorker(
	outboxStore port.OutboxStore,
	publisher port.Publisher,
	logger *zap.SugaredLogger,
	batchSize int,
	pollInterval time.Duration,
	pendingGauge prometheus.Gauge,
	successCounter prometheus.Counter,
	failedCounter prometheus.Counter,
) *OutboxWorker {
	return &OutboxWorker{
		outboxStore:    outboxStore,
		publisher:      publisher,
		logger:         logger,
		batchSize:      batchSize,
		pollInterval:   pollInterval,
		pendingGauge:   pendingGauge,
		successCounter: successCounter,
		failedCounter:  failedCounter,
	}
}

// Run starts the polling loop. It stops when ctx is cancelled.
func (w *OutboxWorker) Run(ctx context.Context) {
	w.logger.Infow("outbox worker started",
		"batch_size", w.batchSize,
		"poll_interval", w.pollInterval,
	)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Infow("outbox worker shutting down")
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	events, err := w.outboxStore.FetchPending(ctx, w.batchSize)
	if err != nil {
		w.logger.Errorw("failed to fetch pending events", "error", err)
		return
	}

	if len(events) == 0 {
		return
	}

	w.pendingGauge.Set(float64(len(events)))
	w.logger.Debugw("processing outbox batch", "count", len(events))

	for _, event := range events {
		if err := w.publisher.Publish(ctx, event); err != nil {
			w.logger.Errorw("failed to publish event",
				"event_id", event.ID,
				"event_type", event.EventType,
				"error", err,
			)
			if markErr := w.outboxStore.MarkFailed(ctx, event.ID, err.Error()); markErr != nil {
				w.logger.Errorw("failed to mark event as failed",
					"event_id", event.ID,
					"error", markErr,
				)
			}
			w.failedCounter.Inc()
			continue
		}

		if err := w.outboxStore.MarkPublished(ctx, event.ID); err != nil {
			w.logger.Errorw("failed to mark event as published",
				"event_id", event.ID,
				"error", err,
			)
			continue
		}

		w.successCounter.Inc()
		w.logger.Debugw("event published",
			"event_id", event.ID,
			"event_type", event.EventType,
		)
	}
}
