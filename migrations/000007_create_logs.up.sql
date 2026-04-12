CREATE TABLE IF NOT EXISTS logs (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    level VARCHAR(10) NOT NULL,
    message TEXT NOT NULL,
    logger_name VARCHAR(255),
    caller VARCHAR(255),
    trace_id VARCHAR(32),
    correlation_id VARCHAR(36),
    fields JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_logs_timestamp ON logs (timestamp DESC);
CREATE INDEX idx_logs_level ON logs (level);
CREATE INDEX idx_logs_correlation_id ON logs (correlation_id);
CREATE INDEX idx_logs_trace_id ON logs (trace_id);
CREATE INDEX idx_logs_created_at ON logs (created_at DESC);
