# SigNoz Cloud Integration Guide

This guide shows how to connect your Order Service to **SigNoz Cloud** for distributed tracing, metrics, and logs.

## Prerequisites

- A SigNoz Cloud account (sign up at https://signoz.io/teams/)
- Your ingestion key from SigNoz Cloud
- Docker & Docker Compose (for containerized deployment)

## Configuration Steps

### Step 1: Get Your SigNoz Cloud Credentials

1. Log in to your SigNoz Cloud dashboard
2. Navigate to **Settings** → **API Keys**
3. Copy your **Ingestion Key**
4. Note your SigNoz region (e.g., `in2` for India, `us` for USA)

### Step 2: Set Environment Variables

Update your deployment configuration with the following environment variables:

```bash
# For SigNoz Cloud (in2 region)
OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.in2.signoz.cloud:443"
OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<your-ingestion-key>"
OTEL_SERVICE_NAME="order-service"

# Or for US region:
OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.us.signoz.cloud:443"
OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<your-ingestion-key>"
```

### Step 3: Running with Docker

#### Option A: Update docker-compose.yaml

Add these environment variables to your service definition:

```yaml
services:
  app:
    build: ./docker
    ports:
      - "8081:8080"
    environment:
      OTEL_EXPORTER_OTLP_ENDPOINT: "https://ingest.in2.signoz.cloud:443"
      OTEL_EXPORTER_OTLP_HEADERS: "signoz-ingestion-key=YOUR_INGESTION_KEY"
      OTEL_SERVICE_NAME: "order-service"
      # ... other environment variables
    depends_on:
      - postgres
```

#### Option B: Use docker run with environment variables

```bash
docker build -t order-service:latest ./docker

docker run \
  -e OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.in2.signoz.cloud:443" \
  -e OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<your-ingestion-key>" \
  -e OTEL_SERVICE_NAME="order-service" \
  -e DB_HOST="postgres" \
  -e DB_USER="order" \
  -e DB_PASSWORD="order123" \
  -e DB_NAME="orderdb" \
  -p 8081:8080 \
  order-service:latest
```

### Step 4: Running Locally (Development)

For local development without containerization:

```bash
# Set environment variables before running
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.in2.signoz.cloud:443"
export OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<your-ingestion-key>"
export OTEL_SERVICE_NAME="order-service"
export DB_HOST="localhost"

# Start Postgres
docker-compose up -d postgres

# Run migrations
make migrate-up

# Start the application
go run ./cmd/server
```

## Instrumentation Overview

### Tracing

Your application automatically instruments:

- **HTTP Requests** — All incoming API calls
- **Database Operations** — GORM queries to PostgreSQL
- **Message Processing** — Event publishing and consumption
- **Background Workers** — Outbox processing and projections

### Metrics

The application exports Prometheus metrics:

- HTTP request duration and count
- Outbox pending events gauge
- Custom business metrics (orders created, confirmed, etc.)

### Logs

Application logs are structured with:

- Trace context (trace ID, span ID) for correlation
- Log level (info, warn, error)
- Contextual attributes (user ID, order ID, etc.)

## Verifying the Integration

### 1. Generate Some Traffic

```bash
# Create an order
curl -X POST http://localhost:8081/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "550e8400-e29b-41d4-a716-446655440000",
    "items": [
      {"product_id": "prod-1", "quantity": 2}
    ]
  }'

# Record the ORDER_ID from response

# Confirm the order
curl -X POST http://localhost:8081/orders/{ORDER_ID}/confirm \
  -H "Content-Type: application/json"

# Check other endpoints
curl http://localhost:8081/health/live
curl http://localhost:8081/orders
```

### 2. View in SigNoz Cloud

1. Open your SigNoz Cloud dashboard
2. Go to **Services** tab
3. Look for **order-service** in the services list
4. Click on it to see:
   - Recent traces (with latency, status, errors)
   - Request distribution across operations
   - Error rates and flame graphs

### 3. Explore Traces

Each trace shows:

- **Service Name** — order-service
- **Operation** — HTTP method and path (e.g., POST /orders)
- **Latency** — Total request duration
- **Status** — Success or error
- **Spans** — Individual operations within the request:
  - HTTP handler
  - Database queries
  - Message publishing
  - Business logic execution

## Common SigNoz Cloud Regions

| Region      | Endpoint                            | Ingestion Key Prefix |
| ----------- | ----------------------------------- | -------------------- |
| India (in2) | https://ingest.in2.signoz.cloud:443 | `in2_xxx`            |
| USA (us)    | https://ingest.us.signoz.cloud:443  | `us_xxx`             |
| EU (eu)     | https://ingest.eu.signoz.cloud:443  | `eu_xxx`             |

## Environment Variables Reference

| Variable                      | Example                             | Purpose                        |
| ----------------------------- | ----------------------------------- | ------------------------------ |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | https://ingest.in2.signoz.cloud:443 | SigNoz Cloud endpoint          |
| `OTEL_EXPORTER_OTLP_HEADERS`  | signoz-ingestion-key=xyz            | Authentication header          |
| `OTEL_SERVICE_NAME`           | order-service                       | Service identifier in SigNoz   |
| `DB_HOST`                     | localhost                           | Database host                  |
| `DB_USER`                     | order                               | Database user                  |
| `DB_PASSWORD`                 | order123                            | Database password              |
| `DB_NAME`                     | orderdb                             | Database name                  |
| `EVENT_BUS_TYPE`              | inmemory \| kafka                   | Event messaging backend        |
| `KAFKA_BROKERS`               | localhost:9092                      | Kafka brokers (if using Kafka) |
| `LOG_LEVEL`                   | info \| debug                       | Application log level          |

## Troubleshooting

### No Traces Appearing

1. **Verify endpoint and key:**

   ```bash
   curl -H "signoz-ingestion-key=<your-key>" \
        https://ingest.in2.signoz.cloud:443
   ```

2. **Check application logs:**

   ```bash
   # If using docker
   docker logs <container-id> | grep -i "tracing\|otel"
   ```

3. **Verify service name:**
   - In SigNoz, look for `OTEL_SERVICE_NAME` value in the services list
   - If missing, the traces may be sent to a different service name

### Connection Timeout

1. **Check network accessibility:**
   - Ensure your network allows HTTPS on port 443
   - Check if corporate firewall/proxy requires special configuration

2. **Verify TLS certificates:**
   - The application uses secure HTTPS by default
   - If behind a proxy, ensure proxy doesn't intercept TLS

### High Latency or Dropped Traces

1. **Enable sampling** (optional, in code):

   ```go
   // In observability/tracing.go
   sampler := sdktrace.NewParentBasedSampler(
       sdktrace.TraceIDRatioBased(0.1), // Sample 10% of traces
   )
   tp := sdktrace.NewTracerProvider(
       sdktrace.WithSampler(sampler),
       // ... other options
   )
   ```

2. **Check payload size:**
   - Large traces with many spans may be rejected
   - Consider enabling batch processor tuning

## Advanced: Custom Instrumentation

To add custom spans in your application code:

```go
import "go.opentelemetry.io/otel"

// Get tracer
tracer := otel.Tracer("order-service")

// Create custom span
ctx, span := tracer.Start(ctx, "custom-operation")
defer span.End()

// Add attributes
span.SetAttribute("customer_id", customerID)
span.SetAttribute("order_total", totalAmount)

// Code being traced...
```

## Cost Optimization

SigNoz Cloud charges based on:

- **Spans ingested** (traces)
- **Metrics** (time-series data)
- **Logs** (GB stored)

### Tips to Reduce Costs

1. **Sampling** — Only trace a percentage of requests:

   ```go
   // Sample 10% of traffic
   sdktrace.TraceIDRatioBased(0.1)
   ```

2. **Filter endpoints** — Exclude health checks:

   ```go
   // In middleware, skip /health/* endpoints
   ```

3. **Reduce log verbosity** — Set `LOG_LEVEL=info` instead of `debug`

4. **Metric retention** — Configure in SigNoz Cloud settings

## Next Steps

- 📊 [Create custom dashboards](https://signoz.io/docs/dashboards/) in SigNoz
- 🚨 [Set up alerts](https://signoz.io/docs/alerts-management/) for anomalies
- 📈 [Explore advanced queries](https://signoz.io/docs/query-builder/) in SigNoz
- 🔍 [Learn trace correlation](https://signoz.io/docs/logs-pipelines/) with logs

## Reference

- [SigNoz Documentation](https://signoz.io/docs/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [OTEL Protocol Specification](https://opentelemetry.io/docs/specs/otel/protocol/)

---

For SigNoz local deployment, see [SIGNOZ_SETUP.md](SIGNOZ_SETUP.md).
