package command

import (
	"context"
	"fmt"

	"github.com/himmel/order-service/internal/application/port"
	"github.com/himmel/order-service/internal/domain/order"
)

// AddItemToOrder is the command to add an item to an existing order.
type AddItemToOrder struct {
	OrderID     string
	ProductID   string
	ProductName string
	Quantity    int
	UnitPrice   int64
}

// AddItemHandler handles AddItemToOrder commands.
type AddItemHandler struct {
	uow port.UnitOfWork
}

func NewAddItemHandler(uow port.UnitOfWork) *AddItemHandler {
	return &AddItemHandler{uow: uow}
}

func (h *AddItemHandler) Handle(ctx context.Context, cmd AddItemToOrder) error {
	orderID, err := order.ParseOrderID(cmd.OrderID)
	if err != nil {
		return fmt.Errorf("add item: %w", err)
	}

	return h.uow.Execute(ctx, func(ctx context.Context, tx port.UnitOfWorkTx) error {
		o, err := tx.OrderRepo().FindByID(ctx, orderID)
		if err != nil {
			return err
		}

		if err := o.AddItem(cmd.ProductID, cmd.ProductName, cmd.Quantity, order.NewMoney(cmd.UnitPrice)); err != nil {
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
