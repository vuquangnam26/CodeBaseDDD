package order

import (
	"fmt"

	"github.com/google/uuid"
)

// OrderID uniquely identifies an order.
type OrderID struct {
	value uuid.UUID
}

func NewOrderID() OrderID            { return OrderID{value: uuid.Must(uuid.NewV7())} }
func OrderIDFrom(id uuid.UUID) OrderID { return OrderID{value: id} }
func ParseOrderID(s string) (OrderID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return OrderID{}, fmt.Errorf("invalid order id %q: %w", s, err)
	}
	return OrderID{value: id}, nil
}
func (o OrderID) String() string  { return o.value.String() }
func (o OrderID) UUID() uuid.UUID { return o.value }
func (o OrderID) IsZero() bool    { return o.value == uuid.Nil }

// CustomerID uniquely identifies a customer.
type CustomerID struct {
	value uuid.UUID
}

func NewCustomerID(id uuid.UUID) CustomerID { return CustomerID{value: id} }
func ParseCustomerID(s string) (CustomerID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return CustomerID{}, fmt.Errorf("invalid customer id %q: %w", s, err)
	}
	return CustomerID{value: id}, nil
}
func (c CustomerID) String() string  { return c.value.String() }
func (c CustomerID) UUID() uuid.UUID { return c.value }
func (c CustomerID) IsZero() bool    { return c.value == uuid.Nil }

// Money represents a monetary value in the smallest currency unit (cents).
type Money struct {
	amount int64
}

func NewMoney(cents int64) Money { return Money{amount: cents} }
func (m Money) Amount() int64   { return m.amount }
func (m Money) Add(other Money) Money {
	return Money{amount: m.amount + other.amount}
}
func (m Money) Multiply(qty int) Money {
	return Money{amount: m.amount * int64(qty)}
}
func (m Money) IsPositive() bool { return m.amount > 0 }

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusDraft     OrderStatus = "DRAFT"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusCancelled OrderStatus = "CANCELLED"
)

func (s OrderStatus) String() string { return string(s) }
func (s OrderStatus) IsModifiable() bool {
	return s == StatusDraft
}
