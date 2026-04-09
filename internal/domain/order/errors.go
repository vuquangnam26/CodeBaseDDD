package order

import "errors"

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrItemNotFound       = errors.New("order item not found")
	ErrOrderNotModifiable = errors.New("order is not in a modifiable state")
	ErrNoItems            = errors.New("order must have at least one item to confirm")
	ErrInvalidQuantity    = errors.New("quantity must be greater than zero")
	ErrInvalidPrice       = errors.New("unit price must be greater than zero")
	ErrConcurrencyConflict = errors.New("order has been modified by another process")
	ErrInvalidCustomerID  = errors.New("customer id is required")
)
