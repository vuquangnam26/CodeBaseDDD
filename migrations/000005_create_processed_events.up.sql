-- Consumer idempotency: tracks which events have been processed
CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    handler_name VARCHAR(100) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_processed_events_handler ON processed_events (handler_name);
