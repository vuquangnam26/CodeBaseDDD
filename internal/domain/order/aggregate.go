package order

import (
	"time"

	"github.com/google/uuid"
	domain "github.com/namcuongq/order-service/internal/domain"
)

// Order is the aggregate root for order management.
type Order struct {
	id         OrderID
	customerID CustomerID
	status     OrderStatus
	items      []OrderItem
	version    int
	createdAt  time.Time
	updatedAt  time.Time

	events []domain.DomainEvent
}

// NewOrder creates a new order in DRAFT status and raises OrderCreated.
func NewOrder(customerID CustomerID) (*Order, error) {
	if customerID.IsZero() {
		return nil, ErrInvalidCustomerID
	}
	now := time.Now().UTC()
	o := &Order{
		id:         NewOrderID(),
		customerID: customerID,
		status:     StatusDraft,
		items:      make([]OrderItem, 0),
		version:    1,
		createdAt:  now,
		updatedAt:  now,
	}
	o.raise(NewOrderCreated(o.id, o.customerID))
	return o, nil
}

// ReconstructOrder rebuilds an Order from persistence data (no events raised, no validation).
func ReconstructOrder(
	id uuid.UUID,
	customerID uuid.UUID,
	status string,
	version int,
	createdAt, updatedAt time.Time,
	items []OrderItem,
) *Order {
	return &Order{
		id:         OrderIDFrom(id),
		customerID: NewCustomerID(customerID),
		status:     OrderStatus(status),
		items:      items,
		version:    version,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

// AddItem adds a line item. Validates qty, price, and order modifiability.
func (o *Order) AddItem(productID, productName string, quantity int, unitPrice Money) error {
	if !o.status.IsModifiable() {
		return ErrOrderNotModifiable
	}
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if !unitPrice.IsPositive() {
		return ErrInvalidPrice
	}

	item := NewOrderItem(productID, productName, quantity, unitPrice)
	o.items = append(o.items, item)
	o.updatedAt = time.Now().UTC()

	o.raise(NewOrderItemAdded(o.id, item))
	return nil
}

// Confirm transitions the order to CONFIRMED if it has items.
func (o *Order) Confirm() error {
	if !o.status.IsModifiable() {
		return ErrOrderNotModifiable
	}
	if len(o.items) == 0 {
		return ErrNoItems
	}

	o.status = StatusConfirmed
	o.updatedAt = time.Now().UTC()

	o.raise(NewOrderConfirmed(o.id, o.TotalAmount()))
	return nil
}

// TotalAmount computes order total from items.
func (o *Order) TotalAmount() Money {
	total := NewMoney(0)
	for _, item := range o.items {
		total = total.Add(item.TotalPrice())
	}
	return total
}

// Accessors

func (o *Order) ID() OrderID          { return o.id }
func (o *Order) CustomerID() CustomerID { return o.customerID }
func (o *Order) Status() OrderStatus   { return o.status }
func (o *Order) Items() []OrderItem    { return o.items }
func (o *Order) Version() int          { return o.version }
func (o *Order) CreatedAt() time.Time  { return o.createdAt }
func (o *Order) UpdatedAt() time.Time  { return o.updatedAt }

// IncrementVersion bumps version after a successful save (called by repo).
func (o *Order) IncrementVersion() { o.version++ }

// Events returns collected domain events and clears them.
func (o *Order) Events() []domain.DomainEvent {
	events := o.events
	o.events = nil
	return events
}

func (o *Order) raise(event domain.DomainEvent) {
	o.events = append(o.events, event)
}
