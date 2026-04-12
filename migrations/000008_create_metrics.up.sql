CREATE TABLE IF NOT EXISTS metrics (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metric_name VARCHAR(255) NOT NULL,
    metric_type VARCHAR(20) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    labels JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_metrics_timestamp ON metrics (timestamp DESC);
CREATE INDEX idx_metrics_name ON metrics (metric_name);
CREATE INDEX idx_metrics_created_at ON metrics (created_at DESC);
