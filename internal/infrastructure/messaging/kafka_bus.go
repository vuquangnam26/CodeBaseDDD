package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/himmel/order-service/internal/application/port"
)

// KafkaEventBus implements port.EventBus using Apache Kafka.
//
// Publishing: writes messages to a Kafka topic. The aggregate_id is used as the
// message key so that events for the same aggregate land in the same partition,
// preserving ordering within an aggregate.
//
// Subscribing: registers handlers that the KafkaConsumerWorker dispatches to
// when it reads messages from the topic.
type KafkaEventBus struct {
	writer *kafka.Writer
	logger *zap.SugaredLogger

	mu       sync.RWMutex
	handlers map[string][]port.EventHandler
}

// KafkaConfig holds Kafka connection settings.
type KafkaConfig struct {
	Brokers       []string
	Topic         string
	ConsumerGroup string
	// Writer tuning
	BatchSize    int
	BatchTimeout time.Duration
	// Consumer tuning
	MinBytes int
	MaxBytes int
	MaxWait  time.Duration
}

// DefaultKafkaConfig returns sensible defaults.
func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Brokers:       []string{"localhost:9092"},
		Topic:         "order-events",
		ConsumerGroup: "order-projection",
		BatchSize:     100,
		BatchTimeout:  10 * time.Millisecond,
		MinBytes:      1,        // 1 byte
		MaxBytes:      10 << 20, // 10 MB
		MaxWait:       500 * time.Millisecond,
	}
}

// NewKafkaEventBus creates a Kafka-backed event bus.
func NewKafkaEventBus(cfg KafkaConfig, logger *zap.SugaredLogger) *KafkaEventBus {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.Hash{}, // Partition by key (aggregate_id)
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		RequiredAcks: kafka.RequireAll, // Wait for all ISR replicas
		Async:        false,            // Synchronous writes for reliability
		Logger:       kafka.LoggerFunc(func(msg string, args ...interface{}) { logger.Debug(fmt.Sprintf(msg, args...)) }),
		ErrorLogger:  kafka.LoggerFunc(func(msg string, args ...interface{}) { logger.Error(fmt.Sprintf(msg, args...)) }),
	}

	return &KafkaEventBus{
		writer:   w,
		logger:   logger,
		handlers: make(map[string][]port.EventHandler),
	}
}

// KafkaMessage is the JSON envelope written to Kafka.
type KafkaMessage struct {
	EventID       string          `json:"event_id"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     string          `json:"event_type"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      json.RawMessage `json:"metadata"`
	OccurredAt    time.Time       `json:"occurred_at"`
}

// Publish sends an outbox event to Kafka.
func (b *KafkaEventBus) Publish(ctx context.Context, event port.OutboxEvent) error {
	msg := KafkaMessage{
		EventID:       event.ID.String(),
		AggregateType: event.AggregateType,
		AggregateID:   event.AggregateID,
		EventType:     event.EventType,
		Payload:       event.Payload,
		Metadata:      event.Metadata,
		OccurredAt:    event.OccurredAt,
	}

	value, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal kafka message: %w", err)
	}

	err = b.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID), // Same aggregate → same partition → ordered
		Value: value,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "aggregate_type", Value: []byte(event.AggregateType)},
			{Key: "event_id", Value: []byte(event.ID.String())},
		},
	})
	if err != nil {
		return fmt.Errorf("write kafka message: %w", err)
	}

	b.logger.Debugw("kafka: published event",
		"event_id", event.ID,
		"event_type", event.EventType,
		"aggregate_id", event.AggregateID,
	)

	// Also dispatch to local handlers (for same-process projections).
	b.dispatchLocal(ctx, event)

	return nil
}

// Subscribe registers a handler for a given event type.
// Handlers are called by KafkaConsumerWorker when messages are read.
func (b *KafkaEventBus) Subscribe(eventType string, handler port.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// HandleMessage is called by KafkaConsumerWorker for each consumed message.
func (b *KafkaEventBus) HandleMessage(ctx context.Context, event port.OutboxEvent) error {
	b.mu.RLock()
	handlers, ok := b.handlers[event.EventType]
	b.mu.RUnlock()

	if !ok || len(handlers) == 0 {
		b.logger.Debugw("kafka: no handlers for event type", "type", event.EventType)
		return nil
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("kafka handler error for %s: %w", event.EventType, err)
		}
	}

	return nil
}

// dispatchLocal runs local handlers synchronously (used when publish + consume in same process).
func (b *KafkaEventBus) dispatchLocal(ctx context.Context, event port.OutboxEvent) {
	b.mu.RLock()
	handlers, ok := b.handlers[event.EventType]
	b.mu.RUnlock()

	if !ok || len(handlers) == 0 {
		return
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			b.logger.Errorw("kafka: local handler error",
				"event_type", event.EventType,
				"error", err,
			)
		}
	}
}

// Close shuts down the Kafka writer.
func (b *KafkaEventBus) Close() error {
	return b.writer.Close()
}

// NewKafkaReader creates a kafka.Reader configured for the consumer group.
func NewKafkaReader(cfg KafkaConfig) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cfg.Brokers,
		Topic:    cfg.Topic,
		GroupID:  cfg.ConsumerGroup,
		MinBytes: cfg.MinBytes,
		MaxBytes: cfg.MaxBytes,
		MaxWait:  cfg.MaxWait,
	})
}

// Compile-time interface check.
var _ port.EventBus = (*KafkaEventBus)(nil)
