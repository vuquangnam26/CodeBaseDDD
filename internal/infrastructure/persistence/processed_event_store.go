package persistence

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GormProcessedEventStore implements port.ProcessedEventStore.
type GormProcessedEventStore struct {
	db *gorm.DB
}

func NewGormProcessedEventStore(db *gorm.DB) *GormProcessedEventStore {
	return &GormProcessedEventStore{db: db}
}

func (s *GormProcessedEventStore) IsProcessed(ctx context.Context, eventID uuid.UUID, handlerName string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&ProcessedEventModel{}).
		Where("event_id = ? AND handler_name = ?", eventID, handlerName).
		Count(&count).Error
	return count > 0, err
}

func (s *GormProcessedEventStore) MarkProcessed(ctx context.Context, eventID uuid.UUID, handlerName string) error {
	model := ProcessedEventModel{
		EventID:     eventID,
		HandlerName: handlerName,
		ProcessedAt: time.Now().UTC(),
	}
	// Use ON CONFLICT DO NOTHING for idempotency.
	return s.db.WithContext(ctx).
		Where("event_id = ? AND handler_name = ?", eventID, handlerName).
		FirstOrCreate(&model).Error
}
