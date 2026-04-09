package order

import "github.com/google/uuid"

// OrderItem is a child entity of Order.
type OrderItem struct {
	id          uuid.UUID
	productID   string
	productName string
	quantity    int
	unitPrice   Money
}

func NewOrderItem(productID, productName string, quantity int, unitPrice Money) OrderItem {
	return OrderItem{
		id:          uuid.Must(uuid.NewV7()),
		productID:   productID,
		productName: productName,
		quantity:    quantity,
		unitPrice:   unitPrice,
	}
}

// ReconstructOrderItem rebuilds an OrderItem from persistence (no validation).
func ReconstructOrderItem(id uuid.UUID, productID, productName string, quantity int, unitPrice int64) OrderItem {
	return OrderItem{
		id:          id,
		productID:   productID,
		productName: productName,
		quantity:    quantity,
		unitPrice:   NewMoney(unitPrice),
	}
}

func (i OrderItem) ID() uuid.UUID       { return i.id }
func (i OrderItem) ProductID() string    { return i.productID }
func (i OrderItem) ProductName() string  { return i.productName }
func (i OrderItem) Quantity() int        { return i.quantity }
func (i OrderItem) UnitPrice() Money     { return i.unitPrice }
func (i OrderItem) TotalPrice() Money    { return i.unitPrice.Multiply(i.quantity) }
