package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/namcuongq/order-service/internal/application/port"
)

// InMemoryEventBus is a channel-based event bus for development and testing.
type InMemoryEventBus struct {
	mu       sync.RWMutex
	handlers map[string][]port.EventHandler
	logger   *slog.Logger
}

func NewInMemoryEventBus(logger *slog.Logger) *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[string][]port.EventHandler),
		logger:   logger,
	}
}

func (b *InMemoryEventBus) Publish(ctx context.Context, event port.OutboxEvent) error {
	b.mu.RLock()
	handlers, ok := b.handlers[event.EventType]
	b.mu.RUnlock()

	if !ok || len(handlers) == 0 {
		b.logger.DebugContext(ctx, "no handlers for event type", "type", event.EventType)
		return nil
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("handler error for %s: %w", event.EventType, err)
		}
	}

	return nil
}

func (b *InMemoryEventBus) Subscribe(eventType string, handler port.EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Compile-time check.
var _ port.EventBus = (*InMemoryEventBus)(nil)
