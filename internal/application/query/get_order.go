package query

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/himmel/order-service/internal/application/dto"
	"github.com/himmel/order-service/internal/application/port"
)

// GetOrderByID is the query to retrieve a single order with its items.
type GetOrderByID struct {
	OrderID string
}

// GetOrderHandler handles GetOrderByID queries.
type GetOrderHandler struct {
	readStore port.ReadModelStore
}

func NewGetOrderHandler(readStore port.ReadModelStore) *GetOrderHandler {
	return &GetOrderHandler{readStore: readStore}
}

func (h *GetOrderHandler) Handle(ctx context.Context, q GetOrderByID) (*dto.OrderResponse, error) {
	id, err := uuid.Parse(q.OrderID)
	if err != nil {
		return nil, fmt.Errorf("invalid order id: %w", err)
	}

	orderView, itemViews, err := h.readStore.GetOrderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	items := make([]dto.OrderItemResponse, 0, len(itemViews))
	for _, iv := range itemViews {
		items = append(items, dto.OrderItemResponse{
			ID:          iv.ID.String(),
			ProductID:   iv.ProductID,
			ProductName: iv.ProductName,
			Quantity:    iv.Quantity,
			UnitPrice:   iv.UnitPrice,
			TotalPrice:  iv.TotalPrice,
		})
	}

	return &dto.OrderResponse{
		ID:          orderView.ID.String(),
		CustomerID:  orderView.CustomerID.String(),
		Status:      orderView.Status,
		TotalAmount: orderView.TotalAmount,
		ItemCount:   orderView.ItemCount,
		Items:       items,
		CreatedAt:   orderView.CreatedAt,
		UpdatedAt:   orderView.UpdatedAt,
	}, nil
}
