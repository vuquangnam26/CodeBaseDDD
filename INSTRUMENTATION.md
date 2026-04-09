# OpenTelemetry Instrumentation Guide

This document describes the OpenTelemetry instrumentation in the Order Service.

## Overview

The Order Service includes comprehensive observability instrumentation for:

- **Distributed Tracing** — Track requests across service boundaries
- **Metrics** — Monitor application performance
- **Log Correlation** — Link logs to trace IDs

## Automatic Instrumentation

### HTTP Server (net/http)

**Component:** `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`

**What's traced:**

- POST /orders — Order creation
- POST /orders/{id}/add-item — Adding items to order
- POST /orders/{id}/confirm — Order confirmation
- GET /orders — List all orders
- GET /orders/{id} — Get order details
- GET /health/live — Liveness probe
- GET /health/ready — Readiness probe
- POST /admin/outbox/requeue — Admin operations

**Span attributes:**

- `http.method` — GET, POST, etc.
- `http.url` — Full request URL
- `http.status_code` — Response status
- `http.client_ip` — Client IP address
- `http.user_agent` — User agent
- `http.request_content_length` — Request size

**Location:** [docker/Dockerfile](docker/Dockerfile#L16) - HTTP server wrapped with `otelhttp.NewHandler()`

### Database (GORM)

**Component:** `gorm.io/plugin/opentelemetry/tracing`

**What's traced:**

- SELECT queries — Fetching orders and items
- INSERT queries — Creating orders and items
- UPDATE queries — Updating order status
- DELETE queries — Soft deletes
- Connection pool operations

**Span attributes:**

- `db.system` — postgres
- `db.name` — orderdb
- `db.operation` — SELECT, INSERT, UPDATE, DELETE
- `db.sql.table` — Table name (orders, order_items)
- `db.statement` — SQL query (sanitized)
- `db.rows_affected` — Number of affected rows

**Location:** [internal/bootstrap/app.go](internal/bootstrap/app.go#L60-L62) - GORM plugin enabled

### Context Propagation

**Component:** `go.opentelemetry.io/otel/propagation`

**What's propagated:**

- W3C Trace Context (`traceparent` header)
- W3C Baggage (`baggage` header)

**Use case:** When the service calls external APIs or services, trace context is automatically propagated in HTTP headers.

## Manual Instrumentation

Example: Creating custom spans for business logic

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

// Get tracer
tracer := otel.Tracer("order-service")

// Create span
ctx, span := tracer.Start(ctx, "process-order")
defer span.End()

// Add attributes
span.SetAttribute("order_id", orderID)
span.SetAttribute("customer_id", customerID)
span.SetAttribute("total_amount", amount)

// Code being traced...
```

## Trace Context Correlation

Traces are automatically correlated across:

- **HTTP requests** — Each incoming request gets a unique trace ID
- **Database operations** — Queries within a request inherit the trace context
- **Background workers** — Outbox and projection workers include trace context
- **Event messages** — Kafka events carry trace context for downstream services

### Trace ID Flow

```
HTTP Request (POST /orders)
    └── Trace ID: 4bf92f3577b34da6a3ce929d0e0e4736
        ├── [Span] HTTP Handler (1ms)
        ├── [Span] CreateOrder Command Handler (8ms)
        │   ├── [Span] Begin Transaction (0.5ms)
        │   ├── [Span] Insert Order (2ms)
        │   ├── [Span] Insert OrderItems (3ms)
        │   ├── [Span] Publish OutboxEvent (1.5ms)
        │   └── [Span] Commit Transaction (1ms)
        ├── [Span] Write Outbox to PostgreSQL (2ms)
        └── HTTP Response (200 OK)

(Background Worker)
    Outbox Worker processes pending events
    └── Trace ID: 4bf92f3577b34da6a3ce929d0e0e4736 (correlated!)
        ├── [Span] ProcessOutbox (5ms)
        │   ├── [Span] Query Pending Events (1ms)
        │   ├── [Span] Publish to Kafka (3ms)
        │   └── [Span] Mark as Published (1ms)
```

## Configuration

### Environment Variables

| Variable                      | Default       | Purpose                                                   |
| ----------------------------- | ------------- | --------------------------------------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | (empty)       | OTLP receiver endpoint (e.g., http://otel-collector:4318) |
| `OTEL_SERVICE_NAME`           | order-service | Service name in observability platform                    |
| `LOG_LEVEL`                   | info          | Structured log levels                                     |
| `LOG_FILE_PATH`               | (empty)       | Optional file path for logs                               |

### Configuration Code

Location: [internal/bootstrap/config.go](internal/bootstrap/config.go#L58-L61)

```go
type TracingConfig struct {
    OTLPEndpoint string  // OTEL_EXPORTER_OTLP_ENDPOINT
    ServiceName  string  // OTEL_SERVICE_NAME
}
```

### Initialization

Location: [internal/bootstrap/app.go](internal/bootstrap/app.go#L71-L80)

```go
// Initialize tracer with OTLP exporter
shutdown, err := observability.InitTracer(
    ctx,
    cfg.Tracing.ServiceName,
    cfg.Tracing.OTLPEndpoint,
    logger,
)
defer shutdown(ctx) // Critical for flushing pending traces
```

## Metrics

### Built-in Metrics

The application exports these Prometheus metrics:

**HTTP Metrics:**

- `http_request_duration_seconds` — Request latency (histogram)
- `http_requests_total` — Request count (counter)

**Outbox Metrics:**

- `outbox_pending_events` — Gauge of pending events
- `outbox_publish_success_total` — Successfully published
- `outbox_publish_failed_total` — Failed publish attempts

**Go Runtime:**

- `go_goroutines` — Active goroutine count
- `go_threads` — Active OS threads
- `process_resident_memory_bytes` — Memory usage

**View metrics:**

```bash
curl http://localhost:8081/metrics
```

## Logging

### Structured Logging

All logs are structured with:

```json
{
  "time": "2025-04-09T10:30:45Z",
  "level": "INFO",
  "msg": "order created",
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "customer_id": "customer-123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "5f50a1b4c2e1d3f9"
}
```

### Log Levels

| Level | Use Case                     | Example                                           |
| ----- | ---------------------------- | ------------------------------------------------- |
| DEBUG | Development, detailed flow   | "starting database connection"                    |
| INFO  | Key events                   | "order created", "service started"                |
| WARN  | Recoverable issues           | "tracing init failed, continuing without tracing" |
| ERROR | Failures requiring attention | "database connection failed"                      |

### Configuration

```bash
# Set log level
export LOG_LEVEL=info     # info, debug, warn, error

# Optional: write to file
export LOG_FILE_PATH=/var/log/order-service/app.log
```

## Deployment Configurations

### Docker (Local OTEL Collector)

Environment in docker-compose.yaml:

```yaml
services:
  app:
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector:4318"
      OTEL_SERVICE_NAME: "order-service"
```

### Docker (SigNoz Cloud)

Update [docker/Dockerfile](docker/Dockerfile) at runtime:

```bash
docker run \
  -e OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.in2.signoz.cloud:443" \
  -e OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<key>" \
  -e OTEL_SERVICE_NAME="order-service" \
  order-service:latest
```

### Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: order-service-config
data:
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector:4318"
  OTEL_SERVICE_NAME: "order-service"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-service
spec:
  template:
    spec:
      containers:
        - name: order-service
          envFrom:
            - configMapRef:
                name: order-service-config
```

## Sampling (Optional)

For high-traffic scenarios, you can sample traces to reduce costs:

```go
// In internal/infrastructure/observability/tracing.go
sampler := sdktrace.NewParentBasedSampler(
    sdktrace.TraceIDRatioBased(0.1), // Sample 10% of traces
)

tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sampler),
    sdktrace.WithBatcher(exporter),
    sdktrace.WithResource(res),
)
```

## Debugging

### Enable Debug Logging

```bash
export LOG_LEVEL=debug
go run ./cmd/server
```

Output will include detailed trace initialization and sampling decisions.

### Check Trace Exporter Status

```go
// In code
logger.Info("tracing initialized", "endpoint", otlpEndpoint)
```

If you see "tracing disabled: no OTLP endpoint configured", the endpoint is not set.

### Verify Data Flow

1. **Check if spans are generated:**

   ```bash
   curl http://localhost:8081/orders
   curl http://localhost:8081/metrics | grep http_requests_total
   ```

2. **Check OTEL Collector logs:**

   ```bash
   docker logs order-otel-collector | grep -i "span\|metric"
   ```

3. **Check SigNoz dashboard:**
   - Go to Services → order-service
   - Recent traces should appear within seconds

## Performance Impact

Instrumentation adds minimal overhead:

- **Startup:** +100-200ms (tracer initialization)
- **Per-request:** +1-5ms (span creation, context propagation)
- **Memory:** +10-20MB (SDK buffers, batching)

For most applications, this overhead is negligible compared to I/O operations.

## Troubleshooting

### Traces not appearing

1. Verify endpoint is reachable (check logs for errors)
2. Ensure OTEL_EXPORTER_OTLP_ENDPOINT is set
3. Restart application after changing environment
4. Check SigNoz/Collector logs for ingestion errors

### High memory usage

- Check trace batching configuration
- Reduce sampling percentage if too many traces
- Monitor ClickHouse retention policies

### Slow application startup

- Increase OTEL Collector connection timeout
- Verify network connectivity to OTLP endpoint
- Set graceful degradation (non-blocking tracer init)

## Next Steps

1. **View traces:** Open SigNoz/Jaeger dashboard
2. **Create dashboards:** Monitor key metrics
3. **Set up alerts:** Get notified of errors
4. **Add custom spans:** Instrument business logic
5. **Integrate with APM:** Connect to error tracking, profiling

## References

- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [OTEL Instrumentation Index](https://opentelemetry.io/ecosystem/registry/?language=go)
- [Best Practices](https://opentelemetry.io/docs/guide/trace-instrumentation/)

---

**Layout:** `internal/infrastructure/observability/`

- `tracing.go` — Tracer initialization
- `logs.go` — Log exporter setup
- `metrics.go` — Prometheus metrics
- `logger.go` — Structured logger
