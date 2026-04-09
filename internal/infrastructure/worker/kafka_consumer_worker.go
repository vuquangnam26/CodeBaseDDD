package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/infrastructure/messaging"
)

// KafkaConsumerWorker reads messages from Kafka and dispatches them
// to registered event handlers via the KafkaEventBus.
type KafkaConsumerWorker struct {
	reader *kafkago.Reader
	bus    *messaging.KafkaEventBus
	logger *slog.Logger
}

func NewKafkaConsumerWorker(
	reader *kafkago.Reader,
	bus *messaging.KafkaEventBus,
	logger *slog.Logger,
) *KafkaConsumerWorker {
	return &KafkaConsumerWorker{
		reader: reader,
		bus:    bus,
		logger: logger,
	}
}

// Run starts the Kafka consumer loop. It commits offsets only after
// successful processing (at-least-once delivery).
func (w *KafkaConsumerWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "kafka consumer worker started",
		"topic", w.reader.Config().Topic,
		"group", w.reader.Config().GroupID,
	)

	for {
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "kafka consumer worker shutting down")
			if err := w.reader.Close(); err != nil {
				w.logger.ErrorContext(ctx, "failed to close kafka reader", "error", err)
			}
			return
		default:
		}

		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, shutting down.
			}
			w.logger.ErrorContext(ctx, "failed to fetch kafka message", "error", err)
			time.Sleep(1 * time.Second) // Backoff on transient errors.
			continue
		}

		event, err := w.parseMessage(msg)
		if err != nil {
			w.logger.ErrorContext(ctx, "failed to parse kafka message",
				"error", err,
				"partition", msg.Partition,
				"offset", msg.Offset,
			)
			// Commit to skip malformed messages (dead-letter in production).
			_ = w.reader.CommitMessages(ctx, msg)
			continue
		}

		w.logger.DebugContext(ctx, "kafka: received event",
			"event_id", event.ID,
			"event_type", event.EventType,
			"aggregate_id", event.AggregateID,
			"partition", msg.Partition,
			"offset", msg.Offset,
		)

		if err := w.bus.HandleMessage(ctx, *event); err != nil {
			w.logger.ErrorContext(ctx, "failed to handle kafka message",
				"event_id", event.ID,
				"event_type", event.EventType,
				"error", err,
			)
			// Don't commit — message will be redelivered (at-least-once).
			time.Sleep(1 * time.Second)
			continue
		}

		// Commit offset only after successful processing.
		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			w.logger.ErrorContext(ctx, "failed to commit kafka offset",
				"error", err,
				"partition", msg.Partition,
				"offset", msg.Offset,
			)
		}
	}
}

// parseMessage converts a raw Kafka message into a port.OutboxEvent.
func (w *KafkaConsumerWorker) parseMessage(msg kafkago.Message) (*port.OutboxEvent, error) {
	var km messaging.KafkaMessage
	if err := json.Unmarshal(msg.Value, &km); err != nil {
		return nil, err
	}

	eventID, err := uuid.Parse(km.EventID)
	if err != nil {
		return nil, err
	}

	return &port.OutboxEvent{
		ID:            eventID,
		AggregateType: km.AggregateType,
		AggregateID:   km.AggregateID,
		EventType:     km.EventType,
		Payload:       km.Payload,
		Metadata:      km.Metadata,
		OccurredAt:    km.OccurredAt,
		Status:        "published",
	}, nil
}
