package port

import (
	"context"

	"github.com/namcuongq/order-service/internal/domain/order"
)

// UnitOfWork manages a transactional boundary that coordinates
// aggregate persistence and outbox event insertion.
type UnitOfWork interface {
	// Execute runs fn inside a database transaction. If fn returns an error
	// the transaction is rolled back; otherwise it is committed.
	Execute(ctx context.Context, fn func(ctx context.Context, tx UnitOfWorkTx) error) error
}

// UnitOfWorkTx provides access to repositories scoped to the current transaction.
type UnitOfWorkTx interface {
	OrderRepo() order.Repository
	OutboxStore() OutboxStore
}
