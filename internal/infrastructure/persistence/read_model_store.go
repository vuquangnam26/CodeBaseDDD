package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/himmel/order-service/internal/application/port"
	domainOrder "github.com/himmel/order-service/internal/domain/order"
)

// GormReadModelStore implements port.ReadModelStore.
type GormReadModelStore struct {
	db *gorm.DB
}

func NewGormReadModelStore(db *gorm.DB) *GormReadModelStore {
	return &GormReadModelStore{db: db}
}

func (s *GormReadModelStore) GetOrderByID(ctx context.Context, id uuid.UUID) (*port.OrderView, []port.OrderItemView, error) {
	var model OrderViewModel
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, domainOrder.ErrOrderNotFound
		}
		return nil, nil, err
	}

	var itemModels []OrderItemViewModel
	if err := s.db.WithContext(ctx).Where("order_id = ?", id).Find(&itemModels).Error; err != nil {
		return nil, nil, err
	}

	orderView := &port.OrderView{
		ID:          model.ID,
		CustomerID:  model.CustomerID,
		Status:      model.Status,
		TotalAmount: model.TotalAmount,
		ItemCount:   model.ItemCount,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}

	items := make([]port.OrderItemView, 0, len(itemModels))
	for _, im := range itemModels {
		items = append(items, port.OrderItemView{
			ID:          im.ID,
			OrderID:     im.OrderID,
			ProductID:   im.ProductID,
			ProductName: im.ProductName,
			Quantity:    im.Quantity,
			UnitPrice:   im.UnitPrice,
			TotalPrice:  im.TotalPrice,
			CreatedAt:   im.CreatedAt,
		})
	}

	return orderView, items, nil
}

func (s *GormReadModelStore) ListOrders(ctx context.Context, filter port.ListOrdersFilter) (*port.PaginatedOrders, error) {
	query := s.db.WithContext(ctx).Model(&OrderViewModel{})

	if filter.CustomerID != nil {
		query = query.Where("customer_id = ?", *filter.CustomerID)
	}
	if filter.Status != nil && *filter.Status != "" {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, err
	}

	// Sorting
	orderClause := "created_at DESC"
	if filter.SortBy != "" {
		dir := "ASC"
		if filter.SortDir == "desc" {
			dir = "DESC"
		}
		// Whitelist allowed sort columns to prevent SQL injection.
		switch filter.SortBy {
		case "created_at", "total_amount", "status", "updated_at":
			orderClause = filter.SortBy + " " + dir
		}
	}
	query = query.Order(orderClause)

	// Pagination
	offset := (filter.Page - 1) * filter.PageSize
	query = query.Offset(offset).Limit(filter.PageSize)

	var models []OrderViewModel
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	views := make([]port.OrderView, 0, len(models))
	for _, m := range models {
		views = append(views, port.OrderView{
			ID:          m.ID,
			CustomerID:  m.CustomerID,
			Status:      m.Status,
			TotalAmount: m.TotalAmount,
			ItemCount:   m.ItemCount,
			CreatedAt:   m.CreatedAt,
			UpdatedAt:   m.UpdatedAt,
		})
	}

	return &port.PaginatedOrders{
		Orders:     views,
		TotalCount: totalCount,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	}, nil
}

func (s *GormReadModelStore) UpsertOrderView(ctx context.Context, view port.OrderView) error {
	model := OrderViewModel{
		ID:          view.ID,
		CustomerID:  view.CustomerID,
		Status:      view.Status,
		TotalAmount: view.TotalAmount,
		ItemCount:   view.ItemCount,
		CreatedAt:   view.CreatedAt,
		UpdatedAt:   view.UpdatedAt,
	}

	return s.db.WithContext(ctx).
		Where("id = ?", model.ID).
		Assign(model).
		FirstOrCreate(&model).Error
}

func (s *GormReadModelStore) UpsertOrderItemView(ctx context.Context, view port.OrderItemView) error {
	model := OrderItemViewModel{
		ID:          view.ID,
		OrderID:     view.OrderID,
		ProductID:   view.ProductID,
		ProductName: view.ProductName,
		Quantity:    view.Quantity,
		UnitPrice:   view.UnitPrice,
		TotalPrice:  view.TotalPrice,
		CreatedAt:   view.CreatedAt,
	}

	return s.db.WithContext(ctx).
		Where("id = ?", model.ID).
		Assign(model).
		FirstOrCreate(&model).Error
}

func (s *GormReadModelStore) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string, totalAmount int64) error {
	updates := map[string]interface{}{
		"updated_at": gorm.Expr("NOW()"),
	}

	if status != "" {
		updates["status"] = status
	}
	if totalAmount > 0 {
		updates["total_amount"] = totalAmount
	}

	// Also update the item count.
	var itemCount int64
	s.db.WithContext(ctx).Model(&OrderItemViewModel{}).Where("order_id = ?", orderID).Count(&itemCount)
	updates["item_count"] = itemCount

	return s.db.WithContext(ctx).
		Model(&OrderViewModel{}).
		Where("id = ?", orderID).
		Updates(updates).Error
}
