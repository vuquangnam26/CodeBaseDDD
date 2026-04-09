package order_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/namcuongq/order-service/internal/domain/order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCustomerID() order.CustomerID {
	return order.NewCustomerID(uuid.New())
}

func TestNewOrder_Success(t *testing.T) {
	o, err := order.NewOrder(validCustomerID())
	require.NoError(t, err)
	assert.False(t, o.ID().IsZero())
	assert.Equal(t, order.StatusDraft, o.Status())
	assert.Empty(t, o.Items())
	assert.Equal(t, 1, o.Version())

	events := o.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "OrderCreated", events[0].EventType())
}

func TestNewOrder_InvalidCustomerID(t *testing.T) {
	_, err := order.NewOrder(order.CustomerID{})
	assert.ErrorIs(t, err, order.ErrInvalidCustomerID)
}

func TestOrder_AddItem_Success(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events() // clear creation event

	err := o.AddItem("PROD-1", "Widget", 3, order.NewMoney(1000))
	require.NoError(t, err)
	assert.Len(t, o.Items(), 1)
	assert.Equal(t, order.NewMoney(3000), o.TotalAmount())

	events := o.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "OrderItemAdded", events[0].EventType())
}

func TestOrder_AddItem_InvalidQuantity(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	err := o.AddItem("PROD-1", "Widget", 0, order.NewMoney(1000))
	assert.ErrorIs(t, err, order.ErrInvalidQuantity)

	err = o.AddItem("PROD-1", "Widget", -1, order.NewMoney(1000))
	assert.ErrorIs(t, err, order.ErrInvalidQuantity)
}

func TestOrder_AddItem_InvalidPrice(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	err := o.AddItem("PROD-1", "Widget", 1, order.NewMoney(0))
	assert.ErrorIs(t, err, order.ErrInvalidPrice)

	err = o.AddItem("PROD-1", "Widget", 1, order.NewMoney(-100))
	assert.ErrorIs(t, err, order.ErrInvalidPrice)
}

func TestOrder_AddItem_NotModifiable(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	_ = o.AddItem("PROD-1", "Widget", 1, order.NewMoney(500))
	_ = o.Events()

	_ = o.Confirm()
	_ = o.Events()

	err := o.AddItem("PROD-2", "Gadget", 1, order.NewMoney(1000))
	assert.ErrorIs(t, err, order.ErrOrderNotModifiable)
}

func TestOrder_Confirm_Success(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	_ = o.AddItem("PROD-1", "Widget", 2, order.NewMoney(500))
	_ = o.Events()

	err := o.Confirm()
	require.NoError(t, err)
	assert.Equal(t, order.StatusConfirmed, o.Status())

	events := o.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "OrderConfirmed", events[0].EventType())
}

func TestOrder_Confirm_NoItems(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	err := o.Confirm()
	assert.ErrorIs(t, err, order.ErrNoItems)
}

func TestOrder_Confirm_AlreadyConfirmed(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	_ = o.AddItem("PROD-1", "Widget", 1, order.NewMoney(500))
	_ = o.Events()
	_ = o.Confirm()
	_ = o.Events()

	err := o.Confirm()
	assert.ErrorIs(t, err, order.ErrOrderNotModifiable)
}

func TestOrder_TotalAmount_MultipleItems(t *testing.T) {
	o, _ := order.NewOrder(validCustomerID())
	_ = o.Events()

	_ = o.AddItem("PROD-1", "Widget", 2, order.NewMoney(500))   // 1000
	_ = o.AddItem("PROD-2", "Gadget", 3, order.NewMoney(1500)) // 4500
	_ = o.Events()

	assert.Equal(t, order.NewMoney(5500), o.TotalAmount())
}

func TestMoney_Operations(t *testing.T) {
	m1 := order.NewMoney(100)
	m2 := order.NewMoney(200)

	assert.Equal(t, order.NewMoney(300), m1.Add(m2))
	assert.Equal(t, order.NewMoney(500), m1.Multiply(5))
	assert.True(t, m1.IsPositive())
	assert.False(t, order.NewMoney(0).IsPositive())
	assert.False(t, order.NewMoney(-1).IsPositive())
}

func TestOrderStatus_IsModifiable(t *testing.T) {
	assert.True(t, order.StatusDraft.IsModifiable())
	assert.False(t, order.StatusConfirmed.IsModifiable())
	assert.False(t, order.StatusCancelled.IsModifiable())
}
