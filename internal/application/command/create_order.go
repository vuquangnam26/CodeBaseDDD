package command

import (
	"context"
	"fmt"

	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/domain/order"
)

// CreateOrder is the command to create a new order.
type CreateOrder struct {
	CustomerID string
}

// CreateOrderHandler handles CreateOrder commands.
type CreateOrderHandler struct {
	uow port.UnitOfWork
}

func NewCreateOrderHandler(uow port.UnitOfWork) *CreateOrderHandler {
	return &CreateOrderHandler{uow: uow}
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrder) (string, error) {
	customerID, err := order.ParseCustomerID(cmd.CustomerID)
	if err != nil {
		return "", fmt.Errorf("create order: %w", err)
	}

	var orderID string

	err = h.uow.Execute(ctx, func(ctx context.Context, tx port.UnitOfWorkTx) error {
		o, err := order.NewOrder(customerID)
		if err != nil {
			return err
		}

		// Collect events before save (save may clear them).
		events := o.Events()

		if err := tx.OrderRepo().Save(ctx, o); err != nil {
			return fmt.Errorf("save order: %w", err)
		}

		if err := tx.OutboxStore().Append(ctx, events); err != nil {
			return fmt.Errorf("append outbox events: %w", err)
		}

		orderID = o.ID().String()
		return nil
	})

	return orderID, err
}
