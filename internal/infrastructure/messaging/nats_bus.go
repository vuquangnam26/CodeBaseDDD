package messaging

import (
	"context"
	"fmt"

	"github.com/himmel/order-service/internal/application/port"
)

// NATSEventBus is a skeleton adapter for NATS.
// Replace the implementation when integrating with a real NATS server.
type NATSEventBus struct {
	url     string
	subject string
}

func NewNATSEventBus(url, subject string) *NATSEventBus {
	return &NATSEventBus{url: url, subject: subject}
}

func (b *NATSEventBus) Publish(_ context.Context, _ port.OutboxEvent) error {
	return fmt.Errorf("nats publisher not implemented")
}

func (b *NATSEventBus) Subscribe(_ string, _ port.EventHandler) {
	// TODO: implement NATS subscription
}

var _ port.EventBus = (*NATSEventBus)(nil)
