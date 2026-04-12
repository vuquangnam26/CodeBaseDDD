package persistence

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/himmel/order-service/internal/application/port"
)

// GormIdempotencyStore implements port.IdempotencyService using PostgreSQL.
type GormIdempotencyStore struct {
	db  *gorm.DB
	ttl time.Duration
}

func NewGormIdempotencyStore(db *gorm.DB, ttl time.Duration) *GormIdempotencyStore {
	return &GormIdempotencyStore{db: db, ttl: ttl}
}

func (s *GormIdempotencyStore) Check(ctx context.Context, key string) (*port.IdempotencyResult, error) {
	var model IdempotencyKeyModel
	err := s.db.WithContext(ctx).Where("key = ? AND expires_at > ?", key, time.Now().UTC()).First(&model).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found = new request
		}
		return nil, err
	}

	return &port.IdempotencyResult{
		StatusCode:   model.ResponseStatus,
		ResponseBody: model.ResponseBody,
	}, nil
}

func (s *GormIdempotencyStore) Store(ctx context.Context, key string, result port.IdempotencyResult) error {
	model := IdempotencyKeyModel{
		Key:            key,
		ResponseStatus: result.StatusCode,
		ResponseBody:   result.ResponseBody,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(s.ttl),
	}

	return s.db.WithContext(ctx).
		Where("key = ?", key).
		Assign(model).
		FirstOrCreate(&model).Error
}
