package port

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OrderView is the read-model projection of an order.
type OrderView struct {
	ID          uuid.UUID
	CustomerID  uuid.UUID
	Status      string
	TotalAmount int64
	ItemCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrderItemView is the read-model projection of an order item.
type OrderItemView struct {
	ID          uuid.UUID
	OrderID     uuid.UUID
	ProductID   string
	ProductName string
	Quantity    int
	UnitPrice   int64
	TotalPrice  int64
	CreatedAt   time.Time
}

// ListOrdersFilter defines query parameters for listing orders.
type ListOrdersFilter struct {
	CustomerID *uuid.UUID
	Status     *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	SortBy     string // "created_at", "total_amount", "status"
	SortDir    string // "asc", "desc"
	Page       int
	PageSize   int
}

// PaginatedOrders wraps paginated results.
type PaginatedOrders struct {
	Orders     []OrderView
	TotalCount int64
	Page       int
	PageSize   int
}

// ReadModelStore provides read-side query operations.
type ReadModelStore interface {
	// GetOrderByID retrieves a single order view with its items.
	GetOrderByID(ctx context.Context, id uuid.UUID) (*OrderView, []OrderItemView, error)

	// ListOrders returns paginated, filtered order views.
	ListOrders(ctx context.Context, filter ListOrdersFilter) (*PaginatedOrders, error)

	// UpsertOrderView creates or updates an order projection.
	UpsertOrderView(ctx context.Context, view OrderView) error

	// UpsertOrderItemView creates or updates an item projection.
	UpsertOrderItemView(ctx context.Context, view OrderItemView) error

	// UpdateOrderStatus updates status and total in the projection.
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string, totalAmount int64) error
}

// ProcessedEventStore tracks which events have been handled (consumer idempotency).
type ProcessedEventStore interface {
	IsProcessed(ctx context.Context, eventID uuid.UUID, handlerName string) (bool, error)
	MarkProcessed(ctx context.Context, eventID uuid.UUID, handlerName string) error
}
