package port

import "context"

// IdempotencyResult stores the cached response for a previously-processed request.
type IdempotencyResult struct {
	StatusCode   int
	ResponseBody []byte
}

// IdempotencyService provides HTTP-level idempotency.
type IdempotencyService interface {
	// Check returns the cached result if the key was already processed, or nil if new.
	Check(ctx context.Context, key string) (*IdempotencyResult, error)

	// Store saves a result for a given idempotency key with a TTL.
	Store(ctx context.Context, key string, result IdempotencyResult) error
}
