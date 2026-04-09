-- Read model: denormalized order projection
CREATE TABLE IF NOT EXISTS order_views (
    id UUID PRIMARY KEY,
    customer_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
    total_amount BIGINT NOT NULL DEFAULT 0,
    item_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_views_customer_id ON order_views (customer_id);
CREATE INDEX idx_order_views_status ON order_views (status);
CREATE INDEX idx_order_views_created_at ON order_views (created_at);
CREATE INDEX idx_order_views_customer_status ON order_views (customer_id, status);

-- Read model: denormalized order item projection
CREATE TABLE IF NOT EXISTS order_item_views (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    product_name VARCHAR(500) NOT NULL,
    quantity INT NOT NULL,
    unit_price BIGINT NOT NULL,
    total_price BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_item_views_order_id ON order_item_views (order_id);
