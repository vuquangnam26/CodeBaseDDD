package port

import (
	"context"
	"time"

	"github.com/google/uuid"
	domain "github.com/namcuongq/order-service/internal/domain"
)

// OutboxEvent is the persistence representation of a domain event in the outbox.
type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte // JSON
	Metadata      []byte // JSON
	OccurredAt    time.Time
	AvailableAt   time.Time
	PublishedAt   *time.Time
	Status        string
	RetryCount    int
	LastError     *string
}

// OutboxStore persists domain events for reliable publishing.
type OutboxStore interface {
	// Append inserts domain events into the outbox within the current transaction.
	Append(ctx context.Context, events []domain.DomainEvent) error

	// FetchPending retrieves a batch of pending events using FOR UPDATE SKIP LOCKED.
	FetchPending(ctx context.Context, batchSize int) ([]OutboxEvent, error)

	// MarkPublished marks an event as successfully published.
	MarkPublished(ctx context.Context, id uuid.UUID) error

	// MarkFailed records a publish failure with error detail and backoff.
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error

	// RequeueFailed resets failed events back to pending for retry.
	RequeueFailed(ctx context.Context) (int64, error)
}
