package projection

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/domain/order"
)

const HandlerName = "OrderProjection"

// OrderProjectionHandler updates read model projections from domain events.
type OrderProjectionHandler struct {
	readStore      port.ReadModelStore
	processedStore port.ProcessedEventStore
	logger         *slog.Logger
}

func NewOrderProjectionHandler(
	readStore port.ReadModelStore,
	processedStore port.ProcessedEventStore,
	logger *slog.Logger,
) *OrderProjectionHandler {
	return &OrderProjectionHandler{
		readStore:      readStore,
		processedStore: processedStore,
		logger:         logger,
	}
}

// Handle processes an outbox event idempotently.
func (h *OrderProjectionHandler) Handle(ctx context.Context, event port.OutboxEvent) error {
	// Idempotency check: skip already-processed events.
	processed, err := h.processedStore.IsProcessed(ctx, event.ID, HandlerName)
	if err != nil {
		return fmt.Errorf("check processed event: %w", err)
	}
	if processed {
		h.logger.DebugContext(ctx, "event already processed, skipping", "event_id", event.ID, "type", event.EventType)
		return nil
	}

	switch event.EventType {
	case "OrderCreated":
		err = h.handleOrderCreated(ctx, event)
	case "OrderItemAdded":
		err = h.handleOrderItemAdded(ctx, event)
	case "OrderConfirmed":
		err = h.handleOrderConfirmed(ctx, event)
	default:
		h.logger.WarnContext(ctx, "unknown event type", "type", event.EventType)
		return nil
	}

	if err != nil {
		return err
	}

	// Mark event as processed for idempotency.
	if err := h.processedStore.MarkProcessed(ctx, event.ID, HandlerName); err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}

	return nil
}

func (h *OrderProjectionHandler) handleOrderCreated(ctx context.Context, event port.OutboxEvent) error {
	var e order.OrderCreated
	if err := json.Unmarshal(event.Payload, &e); err != nil {
		return fmt.Errorf("unmarshal OrderCreated: %w", err)
	}

	orderID, _ := uuid.Parse(e.ID)
	customerID, _ := uuid.Parse(e.CustomerID)

	return h.readStore.UpsertOrderView(ctx, port.OrderView{
		ID:          orderID,
		CustomerID:  customerID,
		Status:      e.Status,
		TotalAmount: 0,
		ItemCount:   0,
		CreatedAt:   e.OccurredAt_,
		UpdatedAt:   e.OccurredAt_,
	})
}

func (h *OrderProjectionHandler) handleOrderItemAdded(ctx context.Context, event port.OutboxEvent) error {
	var e order.OrderItemAdded
	if err := json.Unmarshal(event.Payload, &e); err != nil {
		return fmt.Errorf("unmarshal OrderItemAdded: %w", err)
	}

	orderID, _ := uuid.Parse(e.OrderID)
	itemID, _ := uuid.Parse(e.ItemID)
	totalPrice := e.UnitPrice * int64(e.Quantity)

	if err := h.readStore.UpsertOrderItemView(ctx, port.OrderItemView{
		ID:          itemID,
		OrderID:     orderID,
		ProductID:   e.ProductID,
		ProductName: e.ProductName,
		Quantity:    e.Quantity,
		UnitPrice:   e.UnitPrice,
		TotalPrice:  totalPrice,
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		return err
	}

	// Recalculate order view totals.
	_, items, err := h.readStore.GetOrderByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("get order for recalc: %w", err)
	}

	var newTotal int64
	for _, it := range items {
		newTotal += it.TotalPrice
	}

	return h.readStore.UpdateOrderStatus(ctx, orderID, "", newTotal)
}

func (h *OrderProjectionHandler) handleOrderConfirmed(ctx context.Context, event port.OutboxEvent) error {
	var e order.OrderConfirmed
	if err := json.Unmarshal(event.Payload, &e); err != nil {
		return fmt.Errorf("unmarshal OrderConfirmed: %w", err)
	}

	orderID, _ := uuid.Parse(e.OrderID)
	return h.readStore.UpdateOrderStatus(ctx, orderID, "CONFIRMED", e.TotalAmount)
}
