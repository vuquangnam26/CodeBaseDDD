package dto

import "time"

// --- Request DTOs ---

type CreateOrderRequest struct {
	CustomerID string `json:"customer_id" validate:"required,uuid"`
}

type AddItemRequest struct {
	ProductID   string  `json:"product_id" validate:"required"`
	ProductName string  `json:"product_name" validate:"required"`
	Quantity    int     `json:"quantity" validate:"required,gt=0"`
	UnitPrice   int64   `json:"unit_price" validate:"required,gt=0"`
}

// --- Response DTOs ---

type OrderResponse struct {
	ID          string              `json:"id"`
	CustomerID  string              `json:"customer_id"`
	Status      string              `json:"status"`
	TotalAmount int64               `json:"total_amount"`
	ItemCount   int                 `json:"item_count"`
	Items       []OrderItemResponse `json:"items,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type OrderItemResponse struct {
	ID          string `json:"id"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	TotalPrice  int64  `json:"total_price"`
}

type PaginatedOrdersResponse struct {
	Orders     []OrderResponse `json:"orders"`
	TotalCount int64           `json:"total_count"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
}

// --- Error DTO ---

type ErrorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

// --- Command Results ---

type CreateOrderResult struct {
	OrderID string `json:"order_id"`
}

type AddItemResult struct {
	ItemID string `json:"item_id"`
}
