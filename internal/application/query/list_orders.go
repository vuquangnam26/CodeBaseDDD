package query

import (
	"context"
	"time"

	"github.com/ahmetb/go-linq/v3"
	"github.com/google/uuid"
	"github.com/himmel/order-service/internal/application/dto"
	"github.com/himmel/order-service/internal/application/port"
)

// ListOrders is the query to list orders with filters, sorting, and pagination.
type ListOrders struct {
	CustomerID  *string
	Status      *string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
	SortBy      string
	SortDir     string
	Page        int
	PageSize    int
}

// ListOrdersHandler handles ListOrders queries.
type ListOrdersHandler struct {
	readStore port.ReadModelStore
}

func NewListOrdersHandler(readStore port.ReadModelStore) *ListOrdersHandler {
	return &ListOrdersHandler{readStore: readStore}
}

func (h *ListOrdersHandler) Handle(ctx context.Context, q ListOrders) (*dto.PaginatedOrdersResponse, error) {
	filter := port.ListOrdersFilter{
		Status:      q.Status,
		CreatedFrom: q.CreatedFrom,
		CreatedTo:   q.CreatedTo,
		SortBy:      q.SortBy,
		SortDir:     q.SortDir,
		Page:        q.Page,
		PageSize:    q.PageSize,
	}

	if q.CustomerID != nil {
		id, err := uuid.Parse(*q.CustomerID)
		if err == nil {
			filter.CustomerID = &id
		}
	}

	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	result, err := h.readStore.ListOrders(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Use go-linq for in-memory post-processing (e.g., compute summary stats).
	var orders []dto.OrderResponse
	linq.From(result.Orders).SelectT(func(ov port.OrderView) dto.OrderResponse {
		return dto.OrderResponse{
			ID:          ov.ID.String(),
			CustomerID:  ov.CustomerID.String(),
			Status:      ov.Status,
			TotalAmount: ov.TotalAmount,
			ItemCount:   ov.ItemCount,
			CreatedAt:   ov.CreatedAt,
			UpdatedAt:   ov.UpdatedAt,
		}
	}).ToSlice(&orders)

	return &dto.PaginatedOrdersResponse{
		Orders:     orders,
		TotalCount: result.TotalCount,
		Page:       result.Page,
		PageSize:   result.PageSize,
	}, nil
}
