package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	domain "github.com/namcuongq/order-service/internal/domain"
	"github.com/namcuongq/order-service/internal/application/port"
)

const (
	maxRetries      = 10
	baseBackoffSecs = 2
	maxBackoffSecs  = 300 // 5 minutes
)

// GormOutboxStore implements port.OutboxStore using GORM / PostgreSQL.
type GormOutboxStore struct {
	db *gorm.DB
}

func NewGormOutboxStore(db *gorm.DB) *GormOutboxStore {
	return &GormOutboxStore{db: db}
}

func (s *GormOutboxStore) Append(ctx context.Context, events []domain.DomainEvent) error {
	models := make([]OutboxEventModel, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
		metadata, _ := json.Marshal(domain.EventMetadata{})

		now := time.Now().UTC()
		models = append(models, OutboxEventModel{
			ID:            uuid.Must(uuid.NewV7()),
			AggregateType: e.AggregateType(),
			AggregateID:   e.AggregateID(),
			EventType:     e.EventType(),
			Payload:       payload,
			Metadata:      metadata,
			OccurredAt:    e.OccurredAt(),
			AvailableAt:   now,
			Status:        "pending",
			RetryCount:    0,
		})
	}

	if len(models) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Create(&models).Error
}

func (s *GormOutboxStore) FetchPending(ctx context.Context, batchSize int) ([]port.OutboxEvent, error) {
	var models []OutboxEventModel

	// Use raw SQL for FOR UPDATE SKIP LOCKED.
	err := s.db.WithContext(ctx).Raw(`
		SELECT * FROM outbox_events
		WHERE status = 'pending' AND available_at <= NOW()
		ORDER BY occurred_at ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, batchSize).Scan(&models).Error

	if err != nil {
		return nil, err
	}

	events := make([]port.OutboxEvent, 0, len(models))
	for _, m := range models {
		events = append(events, port.OutboxEvent{
			ID:            m.ID,
			AggregateType: m.AggregateType,
			AggregateID:   m.AggregateID,
			EventType:     m.EventType,
			Payload:       m.Payload,
			Metadata:      m.Metadata,
			OccurredAt:    m.OccurredAt,
			AvailableAt:   m.AvailableAt,
			PublishedAt:   m.PublishedAt,
			Status:        m.Status,
			RetryCount:    m.RetryCount,
			LastError:     m.LastError,
		})
	}

	return events, nil
}

func (s *GormOutboxStore) MarkPublished(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).
		Model(&OutboxEventModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       "published",
			"published_at": now,
		}).Error
}

func (s *GormOutboxStore) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	var model OutboxEventModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return err
	}

	model.RetryCount++
	model.LastError = &errMsg

	if model.RetryCount >= maxRetries {
		model.Status = "failed"
	} else {
		// Exponential backoff: 2^retry * baseSecs, capped at maxBackoffSecs.
		backoff := math.Min(
			math.Pow(2, float64(model.RetryCount))*float64(baseBackoffSecs),
			float64(maxBackoffSecs),
		)
		model.AvailableAt = time.Now().UTC().Add(time.Duration(backoff) * time.Second)
	}

	return s.db.WithContext(ctx).Save(&model).Error
}

func (s *GormOutboxStore) RequeueFailed(ctx context.Context) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&OutboxEventModel{}).
		Where("status = 'failed'").
		Updates(map[string]interface{}{
			"status":       "pending",
			"retry_count":  0,
			"available_at": time.Now().UTC(),
			"last_error":   nil,
		})
	return result.RowsAffected, result.Error
}
