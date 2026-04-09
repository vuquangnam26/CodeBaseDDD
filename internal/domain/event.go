package domain

import "time"

// DomainEvent represents something that happened in the domain.
type DomainEvent interface {
	EventType() string
	AggregateID() string
	AggregateType() string
	OccurredAt() time.Time
}

// EventMetadata carries cross-cutting information about an event.
type EventMetadata struct {
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
}
