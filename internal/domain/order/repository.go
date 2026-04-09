package order

import "context"

// Repository defines write-side persistence for Order aggregates.
type Repository interface {
	// Save persists a new or updated order. Returns ErrConcurrencyConflict on version mismatch.
	Save(ctx context.Context, order *Order) error

	// FindByID loads an order by its ID. Returns ErrOrderNotFound if missing.
	FindByID(ctx context.Context, id OrderID) (*Order, error)
}
