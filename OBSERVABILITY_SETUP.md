# Observability Setup Guide

This document provides instructions for monitoring your Order Service using **Jaeger** (tracing) and **Prometheus** (metrics), along with **OpenTelemetry** for data collection.

## What is Jaeger?

Jaeger is an open-source distributed tracing platform that helps you:

- **Track requests** across service boundaries
- **Identify bottlenecks** in your application
- **Debug complex flows** by visualizing the full request journey
- **Monitor latency** of individual operations

## What is Prometheus?

Prometheus is an open-source metrics monitoring platform that provides:

- **Time-series metrics** collection
- **Real-time querying** with PromQL
- **Alert rules** for proactive monitoring
- **Grafana integration** for custom dashboards

## Quick Start

### 1. Start the Full Stack

```bash
cd CodeBaseDDD
make docker-up
```

This brings up:

- **PostgreSQL** (port 5432) — application database
- **Kafka** (port 9092) — event streaming
- **OpenTelemetry Collector** (port 4317/4318) — trace collection
- **Jaeger** (port 16686) — trace visualization
- **Prometheus** (port 9090) — metrics collection
- **Order Service** (port 8081) — your application

### 2. Access the Dashboards

Open your browser and navigate to:

| Service                  | URL                    |
| ------------------------ | ---------------------- |
| **Jaeger** (Tracing)     | http://localhost:16686 |
| **Prometheus** (Metrics) | http://localhost:9090  |
| **Application API**      | http://localhost:8081  |

### 3. Send API Requests to Generate Data

Create some orders to generate traces and metrics:

```bash
# Create an order
curl -X POST http://localhost:8081/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "550e8400-e29b-41d4-a716-446655440000"}'

# Record the ORDER_ID from response
ORDER_ID="<paste_order_id_here>"

# Add an item to the order
curl -X POST http://localhost:8081/orders/$ORDER_ID/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-001",
    "product_name": "Premium Widget",
    "quantity": 3,
    "unit_price": 2500
  }'

# Confirm the order
curl -X POST http://localhost:8081/orders/$ORDER_ID/confirm \
  -H "Content-Type: application/json"

# View metrics endpoint
curl http://localhost:8081/metrics
```

## Viewing Data in Jaeger

### 1. Navigate to Jaeger UI

Go to http://localhost:16686

### 2. Find Your Service

1. In the left sidebar, select **Service** dropdown
2. Choose **order-service**
3. Click **Find Traces**

### 3. Analyze Traces

Each trace shows:

- **Timeline**: When operations started and their duration
- **Spans**: Individual operations (HTTP handler, DB query, etc.)
- **Tags**: Metadata like HTTP method, status code, customer_id
- **Logs**: Events logged during execution

**Example queries:**

```
# Find slow requests (>500ms)
service.name=order-service AND duration>500ms

# Find failed requests
service.name=order-service AND type=error

# Find specific operation
service.name=order-service AND operation.name=POST /orders
```

## Viewing Data in Prometheus

### 1. Navigate to Prometheus UI

Go to http://localhost:9090

### 2. Execute Queries

In the query input field, try:

```promql
# Request rate per second
rate(http_request_duration_seconds_count[1m])

# Average request latency
rate(http_request_duration_seconds_sum[1m]) / rate(http_request_duration_seconds_count[1m])

# Error rate
rate(http_request_duration_seconds_count{status="500"}[1m])

# Outbox processing rate
rate(outbox_events_processed_total[1m])
```

### 3. View Graphs

- **Graph Mode**: See the line chart over time
- **Table Mode**: See raw values
- **Heatmap**: Visualize distribution (requires Grafana)

## Understanding the Architecture

```
┌─────────────────────────────────────────┐
│     Your Application (Order Service)    │
│                                         │
│  - Instrumented with OpenTelemetry      │
│  - Exports traces via OTLP HTTP         │
│  - Exposes metrics at /metrics          │
│                                         │
│  ├─> Sends traces to :4318              │
│  └─> Exposes metrics on :8080/metrics   │
└────┬──────────────────────────┬─────────┘
     │                          │
     │ OTLP HTTP                │ Prometheus Scrape
     │ (Port 4318)              │ (Port 8080/metrics)
     │                          │
     ▼                          ▼
┌──────────────────┐    ┌─────────────────┐
│  OTEL Collector  │    │  Prometheus     │
│                  │    │                 │
│  - Receives OTLP │    │  - Scrapes app  │
│  - Batch exports │    │  - Stores in DB │
└────────┬─────────┘    │  - PromQL API   │
         │              └────────┬────────┘
         │ gRPC (Port 14250)     │
         ▼                        │
    ┌────────────┐                │
    │   Jaeger   │                │
    │            │                │
    │ - Stores   │        ┌───────▼────────┐
    │   traces   │        │    Dashboards  │
    │ - Web UI   │        │                │
    │ :16686     │        │ - Prometheus   │
    └────────────┘        │ - Grafana      │
                          └────────────────┘
```

## Troubleshooting

### No traces appearing in Jaeger

1. **Verify the app is running and generating traffic:**

   ```bash
   curl http://localhost:8081/health/live
   curl -X POST http://localhost:8081/orders \
     -H "Content-Type: application/json" \
     -d '{"customer_id": "550e8400-e29b-41d4-a716-446655440000"}'
   ```

2. **Check OTEL Collector logs:**

   ```bash
   docker logs order-otel-collector | tail -20
   ```

3. **Verify Jaeger is healthy:**
   ```bash
   curl http://localhost:16686/
   ```

### No metrics appearing in Prometheus

1. **Check if Prometheus is scraping the app:**

   ```bash
   curl http://localhost:9090/api/v1/targets
   ```

2. **Verify app metrics endpoint:**

   ```bash
   curl http://localhost:8081/metrics | head -30
   ```

3. **Check Prometheus logs:**
   ```bash
   docker logs order-prometheus | tail -20
   ```

### Services fail to start

1. **Verify Docker is running:**

   ```bash
   docker ps
   ```

2. **Check for port conflicts:**

   ```bash
   netstat -ano | findstr :16686  # Windows
   lsof -i :16686                  # macOS/Linux
   ```

3. **View detailed logs:**
   ```bash
   docker-compose logs -f
   ```

## Advanced Configuration

### Custom Prometheus Scrape Interval

Edit `docker/prometheus.yml`:

```yaml
global:
  scrape_interval: 5s # Change from 15s to 5s for faster updates
  evaluation_interval: 5s
```

Then restart:

```bash
docker-compose restart prometheus
```

### Add Grafana for Better Dashboards

Add to `docker-compose.yaml`:

```yaml
grafana:
  image: grafana/grafana:latest
  container_name: order-grafana
  ports:
    - "3000:3000"
  environment:
    GF_SECURITY_ADMIN_PASSWORD: admin
    GF_INSTALL_PLUGINS: grafana-piechart-panel
  depends_on:
    - prometheus
```

Then start: `docker-compose up -d grafana`

### Adjust Jaeger Sampling Rate

In your Go code (modify `internal/infrastructure/observability/tracer.go`):

```go
// Sample 10% of traces to reduce overhead
sampler := sdktrace.NewParentBasedSampler(
    sdktrace.TraceIDRatioBased(0.1),
)
```

## Common Metrics to Monitor

| Metric         | Query                                                            | Meaning                    |
| -------------- | ---------------------------------------------------------------- | -------------------------- |
| Request Rate   | `rate(http_request_duration_seconds_count[1m])`                  | Requests per second        |
| P95 Latency    | `histogram_quantile(0.95, http_request_duration_seconds_bucket)` | 95th percentile latency    |
| Error Rate     | `rate(http_requests_total{status=~"5.."}[1m])`                   | Server errors per second   |
| Outbox Lag     | `outbox_events_pending_total`                                    | Events awaiting processing |
| Cache Hit Rate | `rate(cache_hits_total[1m]) / rate(cache_requests_total[1m])`    | Cache effectiveness        |

## Learn More

- **Jaeger Documentation**: https://www.jaegertracing.io/docs/
- **Prometheus Documentation**: https://prometheus.io/docs/
- **OpenTelemetry**: https://opentelemetry.io/docs/
- **PromQL Cheatsheet**: https://promlabs.com/blog/2020/07/02/choosing-a-prometheus-retention-policy/

## Cleanup

To stop all services:

```bash
make docker-down

# Or manually:
docker-compose down

# Also remove volumes (if you want a fresh start)
docker-compose down -v
```

## Makefile Shortcuts

```bash
# Start everything
make docker-up

# Open Jaeger dashboard
make jaeger-view

# Open Prometheus dashboard
make prometheus-view

# View all logs
make docker-logs

# Stop everything
make docker-down
```

---

Happy observing! 🚀

Have questions or issues? Check the troubleshooting section or review the service logs with `docker-compose logs -f`.
