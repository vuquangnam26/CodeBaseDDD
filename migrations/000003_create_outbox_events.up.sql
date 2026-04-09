CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,

    CONSTRAINT chk_outbox_status CHECK (status IN ('pending', 'published', 'failed'))
);

-- For polling: pending events ordered by availability
CREATE INDEX idx_outbox_events_pending ON outbox_events (status, available_at)
    WHERE status = 'pending';

-- For preserving ordering within the same aggregate
CREATE INDEX idx_outbox_events_aggregate ON outbox_events (aggregate_id, occurred_at);

-- For monitoring / cleanup
CREATE INDEX idx_outbox_events_status ON outbox_events (status);
