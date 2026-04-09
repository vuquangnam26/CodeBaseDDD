package command

import (
	"context"
	"fmt"

	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/domain/order"
)

// ConfirmOrder is the command to confirm an order.
type ConfirmOrder struct {
	OrderID string
}

// ConfirmOrderHandler handles ConfirmOrder commands.
type ConfirmOrderHandler struct {
	uow port.UnitOfWork
}

func NewConfirmOrderHandler(uow port.UnitOfWork) *ConfirmOrderHandler {
	return &ConfirmOrderHandler{uow: uow}
}

func (h *ConfirmOrderHandler) Handle(ctx context.Context, cmd ConfirmOrder) error {
	orderID, err := order.ParseOrderID(cmd.OrderID)
	if err != nil {
		return fmt.Errorf("confirm order: %w", err)
	}

	return h.uow.Execute(ctx, func(ctx context.Context, tx port.UnitOfWorkTx) error {
		o, err := tx.OrderRepo().FindByID(ctx, orderID)
		if err != nil {
			return err
		}

		if err := o.Confirm(); err != nil {
			return err
		}

		events := o.Events()

		if err := tx.OrderRepo().Save(ctx, o); err != nil {
			return fmt.Errorf("save order: %w", err)
		}

		if err := tx.OutboxStore().Append(ctx, events); err != nil {
			return fmt.Errorf("append outbox events: %w", err)
		}

		return nil
	})
}
