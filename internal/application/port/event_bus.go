package port

import "context"

// EventBus abstracts message publishing and subscribing.
// Implementations can be in-memory, Kafka, NATS, RabbitMQ, etc.
type EventBus interface {
	Publisher
	Subscriber
}

// Publisher publishes outbox events to downstream consumers.
type Publisher interface {
	Publish(ctx context.Context, event OutboxEvent) error
}

// Subscriber registers handlers for specific event types.
type Subscriber interface {
	Subscribe(eventType string, handler EventHandler)
}

// EventHandler processes a single event. Implementations must be idempotent.
type EventHandler func(ctx context.Context, event OutboxEvent) error
