package order

import (
	"time"

	domain "github.com/namcuongq/order-service/internal/domain"
)

const AggregateType = "Order"

// --- OrderCreated ---

type OrderCreated struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Status     string    `json:"status"`
	OccurredAt_ time.Time `json:"occurred_at"`
}

func NewOrderCreated(orderID OrderID, customerID CustomerID) OrderCreated {
	return OrderCreated{
		ID:         orderID.String(),
		CustomerID: customerID.String(),
		Status:     string(StatusDraft),
		OccurredAt_: time.Now().UTC(),
	}
}

func (e OrderCreated) EventType() string     { return "OrderCreated" }
func (e OrderCreated) AggregateID() string   { return e.ID }
func (e OrderCreated) AggregateType() string { return AggregateType }
func (e OrderCreated) OccurredAt() time.Time { return e.OccurredAt_ }

// --- OrderItemAdded ---

type OrderItemAdded struct {
	OrderID     string `json:"order_id"`
	ItemID      string `json:"item_id"`
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Quantity    int    `json:"quantity"`
	UnitPrice   int64  `json:"unit_price"`
	OccurredAt_ time.Time `json:"occurred_at"`
}

func NewOrderItemAdded(orderID OrderID, item OrderItem) OrderItemAdded {
	return OrderItemAdded{
		OrderID:     orderID.String(),
		ItemID:      item.ID().String(),
		ProductID:   item.ProductID(),
		ProductName: item.ProductName(),
		Quantity:    item.Quantity(),
		UnitPrice:   item.UnitPrice().Amount(),
		OccurredAt_: time.Now().UTC(),
	}
}

func (e OrderItemAdded) EventType() string     { return "OrderItemAdded" }
func (e OrderItemAdded) AggregateID() string   { return e.OrderID }
func (e OrderItemAdded) AggregateType() string { return AggregateType }
func (e OrderItemAdded) OccurredAt() time.Time { return e.OccurredAt_ }

// --- OrderConfirmed ---

type OrderConfirmed struct {
	OrderID     string `json:"order_id"`
	TotalAmount int64  `json:"total_amount"`
	OccurredAt_ time.Time `json:"occurred_at"`
}

func NewOrderConfirmed(orderID OrderID, totalAmount Money) OrderConfirmed {
	return OrderConfirmed{
		OrderID:     orderID.String(),
		TotalAmount: totalAmount.Amount(),
		OccurredAt_: time.Now().UTC(),
	}
}

func (e OrderConfirmed) EventType() string     { return "OrderConfirmed" }
func (e OrderConfirmed) AggregateID() string   { return e.OrderID }
func (e OrderConfirmed) AggregateType() string { return AggregateType }
func (e OrderConfirmed) OccurredAt() time.Time { return e.OccurredAt_ }

// Compile-time interface checks.
var (
	_ domain.DomainEvent = OrderCreated{}
	_ domain.DomainEvent = OrderItemAdded{}
	_ domain.DomainEvent = OrderConfirmed{}
)
