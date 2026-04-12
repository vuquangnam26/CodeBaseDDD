package command_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/himmel/order-service/internal/application/command"
	"github.com/himmel/order-service/internal/application/port"
	"github.com/himmel/order-service/internal/domain"
	"github.com/himmel/order-service/internal/domain/order"
)

// --- Mock implementations ---

type mockUnitOfWork struct {
	executeFn func(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error
}

func (m *mockUnitOfWork) Execute(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
	return m.executeFn(ctx, fn)
}

type mockUoWTx struct {
	orderRepo   order.Repository
	outboxStore port.OutboxStore
}

func (m *mockUoWTx) OrderRepo() order.Repository { return m.orderRepo }
func (m *mockUoWTx) OutboxStore() port.OutboxStore { return m.outboxStore }

type mockOrderRepo struct {
	orders map[string]*order.Order
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{orders: make(map[string]*order.Order)}
}

func (r *mockOrderRepo) Save(_ context.Context, o *order.Order) error {
	r.orders[o.ID().String()] = o
	o.IncrementVersion()
	return nil
}

func (r *mockOrderRepo) FindByID(_ context.Context, id order.OrderID) (*order.Order, error) {
	o, ok := r.orders[id.String()]
	if !ok {
		return nil, order.ErrOrderNotFound
	}
	return o, nil
}

type mockOutboxStore struct {
	events []domain.DomainEvent
}

func (s *mockOutboxStore) Append(_ context.Context, events []domain.DomainEvent) error {
	s.events = append(s.events, events...)
	return nil
}

func (s *mockOutboxStore) FetchPending(_ context.Context, _ int) ([]port.OutboxEvent, error) {
	return nil, nil
}
func (s *mockOutboxStore) MarkPublished(_ context.Context, _ uuid.UUID) error { return nil }
func (s *mockOutboxStore) MarkFailed(_ context.Context, _ uuid.UUID, _ string) error { return nil }
func (s *mockOutboxStore) RequeueFailed(_ context.Context) (int64, error) { return 0, nil }

// --- Tests ---

func TestCreateOrderHandler_Success(t *testing.T) {
	repo := newMockOrderRepo()
	outbox := &mockOutboxStore{}

	uow := &mockUnitOfWork{
		executeFn: func(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
			return fn(ctx, &mockUoWTx{orderRepo: repo, outboxStore: outbox})
		},
	}

	handler := command.NewCreateOrderHandler(uow)
	orderID, err := handler.Handle(context.Background(), command.CreateOrder{
		CustomerID: uuid.New().String(),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, orderID)
	assert.Len(t, repo.orders, 1)
	assert.Len(t, outbox.events, 1)
	assert.Equal(t, "OrderCreated", outbox.events[0].EventType())
}

func TestCreateOrderHandler_InvalidCustomerID(t *testing.T) {
	uow := &mockUnitOfWork{
		executeFn: func(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
			return fn(ctx, &mockUoWTx{orderRepo: newMockOrderRepo(), outboxStore: &mockOutboxStore{}})
		},
	}

	handler := command.NewCreateOrderHandler(uow)
	_, err := handler.Handle(context.Background(), command.CreateOrder{
		CustomerID: "not-a-uuid",
	})

	assert.Error(t, err)
}

func TestAddItemHandler_Success(t *testing.T) {
	repo := newMockOrderRepo()
	outbox := &mockOutboxStore{}

	// Pre-create an order.
	customerID := order.NewCustomerID(uuid.New())
	o, _ := order.NewOrder(customerID)
	_ = o.Events()
	repo.orders[o.ID().String()] = o

	uow := &mockUnitOfWork{
		executeFn: func(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
			return fn(ctx, &mockUoWTx{orderRepo: repo, outboxStore: outbox})
		},
	}

	handler := command.NewAddItemHandler(uow)
	err := handler.Handle(context.Background(), command.AddItemToOrder{
		OrderID:     o.ID().String(),
		ProductID:   "PROD-1",
		ProductName: "Widget",
		Quantity:    5,
		UnitPrice:   1000,
	})

	require.NoError(t, err)
	assert.Len(t, outbox.events, 1)
	assert.Equal(t, "OrderItemAdded", outbox.events[0].EventType())
}

func TestConfirmOrderHandler_Success(t *testing.T) {
	repo := newMockOrderRepo()
	outbox := &mockOutboxStore{}

	customerID := order.NewCustomerID(uuid.New())
	o, _ := order.NewOrder(customerID)
	_ = o.Events()
	_ = o.AddItem("PROD-1", "Widget", 1, order.NewMoney(500))
	_ = o.Events()
	repo.orders[o.ID().String()] = o

	uow := &mockUnitOfWork{
		executeFn: func(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
			return fn(ctx, &mockUoWTx{orderRepo: repo, outboxStore: outbox})
		},
	}

	handler := command.NewConfirmOrderHandler(uow)
	err := handler.Handle(context.Background(), command.ConfirmOrder{
		OrderID: o.ID().String(),
	})

	require.NoError(t, err)
	assert.Len(t, outbox.events, 1)
	assert.Equal(t, "OrderConfirmed", outbox.events[0].EventType())
}

func TestConfirmOrderHandler_NoItems(t *testing.T) {
	repo := newMockOrderRepo()
	outbox := &mockOutboxStore{}

	customerID := order.NewCustomerID(uuid.New())
	o, _ := order.NewOrder(customerID)
	_ = o.Events()
	repo.orders[o.ID().String()] = o

	uow := &mockUnitOfWork{
		executeFn: func(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
			return fn(ctx, &mockUoWTx{orderRepo: repo, outboxStore: outbox})
		},
	}

	handler := command.NewConfirmOrderHandler(uow)
	err := handler.Handle(context.Background(), command.ConfirmOrder{
		OrderID: o.ID().String(),
	})

	assert.ErrorIs(t, err, order.ErrNoItems)
}
