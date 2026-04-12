package worker

import (
	"go.uber.org/zap"

	"github.com/himmel/order-service/internal/application/port"
	"github.com/himmel/order-service/internal/application/projection"
)

// ProjectionWorker subscribes to events and runs the projection handler.
// In the in-memory bus case, events are processed synchronously within the
// outbox worker's Publish call, so this worker just wires the subscriptions.
type ProjectionWorker struct {
	bus        port.EventBus
	projection *projection.OrderProjectionHandler
	logger     *zap.SugaredLogger
}

func NewProjectionWorker(
	bus port.EventBus,
	proj *projection.OrderProjectionHandler,
	logger *zap.SugaredLogger,
) *ProjectionWorker {
	return &ProjectionWorker{
		bus:        bus,
		projection: proj,
		logger:     logger,
	}
}

// Setup registers event handlers on the bus.
func (w *ProjectionWorker) Setup() {
	w.logger.Infow("projection worker: registering event handlers")

	w.bus.Subscribe("OrderCreated", w.projection.Handle)
	w.bus.Subscribe("OrderItemAdded", w.projection.Handle)
	w.bus.Subscribe("OrderConfirmed", w.projection.Handle)
}
