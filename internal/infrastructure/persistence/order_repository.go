package persistence

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/himmel/order-service/internal/domain/order"
)

// GormOrderRepository implements order.Repository using GORM.
type GormOrderRepository struct {
	db *gorm.DB
}

func NewGormOrderRepository(db *gorm.DB) *GormOrderRepository {
	return &GormOrderRepository{db: db}
}

func (r *GormOrderRepository) Save(ctx context.Context, o *order.Order) error {
	model := ToOrderModel(o)
	itemModels := ToOrderItemModels(o)

	// Optimistic concurrency: only update if version matches.
	result := r.db.WithContext(ctx).
		Where("id = ? AND version = ?", model.ID, model.Version).
		Save(&model)

	if result.Error != nil {
		return result.Error
	}

	// If no rows were updated and this isn't a new order, it's a concurrency conflict.
	if result.RowsAffected == 0 {
		// Check if it's a new order (insert).
		var count int64
		r.db.WithContext(ctx).Model(&OrderModel{}).Where("id = ?", model.ID).Count(&count)
		if count > 0 {
			return order.ErrConcurrencyConflict
		}
		// New order: create it.
		if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
			return err
		}
	}

	// Delete existing items and re-insert all (simplest approach for child entities).
	if err := r.db.WithContext(ctx).Where("order_id = ?", model.ID).Delete(&OrderItemModel{}).Error; err != nil {
		return err
	}
	if len(itemModels) > 0 {
		if err := r.db.WithContext(ctx).Create(&itemModels).Error; err != nil {
			return err
		}
	}

	// Bump version in the domain object after successful save.
	o.IncrementVersion()
	return nil
}

func (r *GormOrderRepository) FindByID(ctx context.Context, id order.OrderID) (*order.Order, error) {
	var model OrderModel
	if err := r.db.WithContext(ctx).Where("id = ?", id.UUID()).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, order.ErrOrderNotFound
		}
		return nil, err
	}

	var itemModels []OrderItemModel
	if err := r.db.WithContext(ctx).Where("order_id = ?", model.ID).Find(&itemModels).Error; err != nil {
		return nil, err
	}

	return ToDomainOrder(model, itemModels), nil
}
