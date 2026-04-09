# Order Service — CQRS + Transactional Outbox

A production-grade Go backend demonstrating **CQRS**, **Transactional Outbox**, and **event-driven projection** for Order Management.

## Architecture

```
┌──────────────┐     ┌─────────────────────┐     ┌───────────────┐
│  HTTP Client │────▶│    REST Handlers     │────▶│   Command     │
│              │     │  (chi + middleware)  │     │   Handlers    │
└──────────────┘     └─────────────────────┘     └───────┬───────┘
                                                         │
                            ┌────────────────────────────┤
                            │  UnitOfWork (single TX)    │
                            │                            │
                     ┌──────▼──────┐          ┌──────────▼──────────┐
                     │ Write Repo  │          │   Outbox Store      │
                     │ (GORM/PG)   │          │   (outbox_events)   │
                     └─────────────┘          └──────────┬──────────┘
                                                         │
                                              ┌──────────▼──────────┐
                                              │   Outbox Worker     │
                                              │  (poll + publish)   │
                                              └──────────┬──────────┘
                                                         │
                                              ┌──────────▼──────────┐
                                              │   Event Bus         │
                                              │  (in-memory/Kafka)  │
                                              └──────────┬──────────┘
                                                         │
                                              ┌──────────▼──────────┐
                                              │  Projection Handler │
                                              │  (idempotent)       │
                                              └──────────┬──────────┘
                                                         │
                                              ┌──────────▼──────────┐
                                              │  Read Model Store   │
                                              │  (order_views, etc) │
                                              └─────────────────────┘
```

## Tech Stack

| Component   | Technology               |
| ----------- | ------------------------ |
| Language    | Go 1.22+                 |
| HTTP Router | chi/v5                   |
| ORM (write) | GORM                     |
| Database    | PostgreSQL 15            |
| Migrations  | golang-migrate           |
| Validation  | go-playground/validator  |
| LINQ        | go-linq/v3               |
| Metrics     | Prometheus               |
| Tracing     | OpenTelemetry            |
| Logging     | slog (stdlib)            |
| Testing     | testify + testcontainers |
| Config      | Environment variables    |

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `golang-migrate` CLI (`go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)

### 1. Start PostgreSQL

```bash
docker-compose up -d postgres
```

### 2. Run Migrations

```bash
make migrate-up
```

Or manually:

```bash
migrate -path migrations -database "postgres://order:order123@localhost:5432/orderdb?sslmode=disable" up
```

### 3. Run the Application

```bash
make dev
```

Or manually:

```bash
go run ./cmd/server
```

The server starts on `http://localhost:8080`.

For Docker + Kafka + observability stack (app, postgres, kafka, otel collector):

```bash
make docker-up
```

### 4. Verify

```bash
curl http://localhost:8080/health/live
# {"status":"alive"}

curl http://localhost:8080/health/ready
# {"status":"ready"}
```

## API Endpoints

| Method | Path                    | Description                  |
| ------ | ----------------------- | ---------------------------- |
| POST   | `/orders`               | Create a new order           |
| POST   | `/orders/{id}/items`    | Add item to order            |
| POST   | `/orders/{id}/confirm`  | Confirm order                |
| GET    | `/orders/{id}`          | Get order by ID              |
| GET    | `/orders`               | List orders (filtered)       |
| GET    | `/health/live`          | Liveness probe               |
| GET    | `/health/ready`         | Readiness probe              |
| GET    | `/metrics`              | Prometheus metrics           |
| POST   | `/admin/outbox/requeue` | Requeue failed outbox events |
| GET    | `/swagger/index.html`   | Swagger UI Documentation     |

## Curl Examples

### Create an Order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "550e8400-e29b-41d4-a716-446655440000"}'

# Response (201):
# {"order_id": "01905a7b-..."}
```

### Add Item to Order

```bash
curl -X POST http://localhost:8080/orders/{ORDER_ID}/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-001",
    "product_name": "Premium Widget",
    "quantity": 3,
    "unit_price": 2500
  }'

# Response (200):
# {"message": "item added"}
```

### Add Another Item

```bash
curl -X POST http://localhost:8080/orders/{ORDER_ID}/items \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD-002",
    "product_name": "Deluxe Gadget",
    "quantity": 1,
    "unit_price": 5000
  }'
```

### Confirm Order

```bash
curl -X POST http://localhost:8080/orders/{ORDER_ID}/confirm \
  -H "Content-Type: application/json"

# Response (200):
# {"message": "order confirmed"}
```

### Get Order (Read Model)

```bash
curl http://localhost:8080/orders/{ORDER_ID}

# Response (200):
# {
#   "id": "...",
#   "customer_id": "550e8400-...",
#   "status": "CONFIRMED",
#   "total_amount": 12500,
#   "item_count": 2,
#   "items": [
#     {"id":"...","product_id":"PROD-001","product_name":"Premium Widget","quantity":3,"unit_price":2500,"total_price":7500},
#     {"id":"...","product_id":"PROD-002","product_name":"Deluxe Gadget","quantity":1,"unit_price":5000,"total_price":5000}
#   ],
#   "created_at": "2026-04-07T00:00:00Z",
#   "updated_at": "2026-04-07T00:00:00Z"
# }
```

### List Orders (with filters)

```bash
# All orders
curl "http://localhost:8080/orders"

# Filter by status
curl "http://localhost:8080/orders?status=CONFIRMED"

# Filter by customer
curl "http://localhost:8080/orders?customer_id=550e8400-e29b-41d4-a716-446655440000"

# Pagination + sorting
curl "http://localhost:8080/orders?page=1&size=10&sort=created_at&dir=desc"

# Combine filters
curl "http://localhost:8080/orders?status=DRAFT&customer_id=...&page=1&size=20"
```

### Requeue Failed Outbox Events

```bash
curl -X POST http://localhost:8080/admin/outbox/requeue

# Response: {"requeued": 3}
```

### Error Response Format

```json
{
  "code": "ORDER_NOT_FOUND",
  "message": "order not found",
  "details": null,
  "trace_id": "abc123..."
}
```

## Sequence Diagrams

### Command → Transaction → Outbox

```
Client          HTTP Handler      Command Handler     UoW (DB Tx)       Outbox Table
  │                 │                   │                  │                  │
  │  POST /orders   │                   │                  │                  │
  │────────────────>│                   │                  │                  │
  │                 │  CreateOrder cmd   │                  │                  │
  │                 │──────────────────>│                  │                  │
  │                 │                   │   Begin Tx        │                  │
  │                 │                   │─────────────────>│                  │
  │                 │                   │   Save Order      │                  │
  │                 │                   │─────────────────>│                  │
  │                 │                   │   Insert Outbox   │                  │
  │                 │                   │─────────────────>│────────────────>│
  │                 │                   │   Commit Tx       │                  │
  │                 │                   │─────────────────>│                  │
  │                 │   201 + OrderDTO  │                  │                  │
  │<────────────────│<─────────────────│                  │                  │
```

### Outbox Worker → Bus → Projection

```
Outbox Worker    Outbox Table     EventBus      Projection Handler   Read Model DB
     │                │               │                │                  │
     │  Poll pending  │               │                │                  │
     │───────────────>│               │                │                  │
     │   events[]     │               │                │                  │
     │<───────────────│               │                │                  │
     │                │               │                │                  │
     │  Publish event │               │                │                  │
     │───────────────────────────────>│                │                  │
     │                │               │  Deliver event │                  │
     │                │               │───────────────>│                  │
     │                │               │                │  Check processed │
     │                │               │                │─────────────────>│
     │                │               │                │  Upsert view     │
     │                │               │                │─────────────────>│
     │                │               │                │  Mark processed  │
     │                │               │                │─────────────────>│
     │  Mark published│               │                │                  │
     │───────────────>│               │                │                  │
```

## Makefile Targets

| Target                   | Description                             |
| ------------------------ | --------------------------------------- |
| `make dev`               | Start Postgres + run app locally        |
| `make build`             | Build binary                            |
| `make test`              | Run unit tests                          |
| `make test-integration`  | Run integration tests (Docker required) |
| `make test-e2e`          | Run E2E tests (app must be running)     |
| `make migrate-up`        | Apply all migrations                    |
| `make migrate-down`      | Rollback all migrations                 |
| `make docker-up`         | Start all via docker-compose            |
| `make docker-down`       | Stop all services                       |
| `make worker-outbox`     | Run outbox worker                       |
| `make worker-projection` | Run projection worker                   |

## Configuration

All config via environment variables (see `.env.example`):

| Variable                      | Default             | Description                                    |
| ----------------------------- | ------------------- | ---------------------------------------------- |
| `SERVER_PORT`                 | 8080                | HTTP server port                               |
| `DB_HOST`                     | localhost           | PostgreSQL host                                |
| `DB_PORT`                     | 5432                | PostgreSQL port                                |
| `DB_USER`                     | order               | Database user                                  |
| `DB_PASSWORD`                 | order123            | Database password                              |
| `DB_NAME`                     | orderdb             | Database name                                  |
| `OUTBOX_BATCH_SIZE`           | 50                  | Events per poll batch                          |
| `OUTBOX_POLL_INTERVAL`        | 1s                  | Poll interval                                  |
| `LOG_LEVEL`                   | info                | Log level (debug/info/warn/error)              |
| `LOG_FILE_PATH`               | ""                  | Optional log file path for collector ingestion |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | otel-collector:4318 | OTLP HTTP endpoint for traces                  |

### Observability Stack (Docker Compose)

**Start the full stack (app + PostgreSQL + Kafka + OpenTelemetry + Jaeger + Prometheus):**

```bash
make docker-up
# or
docker-compose up -d
```

**Access the services:**

| Service            | URL                           | Purpose                         |
| ------------------ | ----------------------------- | ------------------------------- |
| Order API          | http://localhost:8081         | REST API (port 8081 in compose) |
| Jaeger Tracing     | http://localhost:16686        | Distributed tracing dashboard   |
| Prometheus Metrics | http://localhost:9090         | Metrics collection and querying |
| App Metrics        | http://localhost:8081/metrics | Prometheus-format app metrics   |

**How it works:**

1. `app` sends traces to `otel-collector:4318` (OTLP HTTP)
2. `otel-collector` forwards traces to `jaeger:14250` (gRPC)
3. `app` exposes metrics at `/metrics` endpoint (Prometheus format)
4. `prometheus` scrapes metrics from the app every 15 seconds
5. `jaeger` visualizes distributed traces in web UI
6. `prometheus` provides time-series metrics querying

**In Jaeger Dashboard:**

- Filter traces by `service.name=order-service`
- Search by operation (e.g., `POST /orders`)
- View end-to-end request latency and dependencies
- Click on spans to see detailed timing information

**In Prometheus Dashboard:**

- View real-time metrics from your application
- Query metrics using PromQL
- Set up alerts based on metric thresholds

## Trade-offs: CQRS + Outbox

### Why use it:

- **Separate read/write optimization** — read model denormalized for fast queries
- **Reliable event publishing** — outbox ensures events aren't lost even if broker is down
- **Loose coupling** — write and read sides evolve independently
- **Scalability** — read side can be scaled, cached, or backed by different stores

### When NOT to use it:

- **Simple CRUD** apps — overhead doesn't justify the benefit
- **Strong read-after-write consistency** — read model is eventually consistent
- **Small teams** — complexity cost is high for small teams
- **Low traffic** — polling overhead is unnecessary

## Production Hardening Checklist

| Area                         | Status | Notes                                   |
| ---------------------------- | ------ | --------------------------------------- |
| Exponential backoff + jitter | ✅     | On outbox publish failures              |
| Dead-letter (max retries)    | ✅     | Events marked "failed" after 10 retries |
| Requeue endpoint             | ✅     | POST /admin/outbox/requeue              |
| Idempotent consumers         | ✅     | processed_events table                  |
| Optimistic concurrency       | ✅     | Version field on aggregate              |
| Graceful shutdown            | ✅     | HTTP drain + worker context cancel      |
| Structured logging           | ✅     | slog with correlation_id                |
| Prometheus metrics           | ✅     | outbox/projection/http metrics          |
| OpenTelemetry tracing        | ✅     | Configurable OTLP endpoint              |
| Health endpoints             | ✅     | /health/live + /health/ready            |
| Circuit breaker              | 🔲     | Add sony/gobreaker for external calls   |
| Rate limiting                | 🔲     | Add chi rate-limit middleware           |
| AuthN/AuthZ                  | 🔲     | Add JWT/OAuth2 middleware               |
| Outbox archiving             | 🔲     | Cron to move old published events       |
| Outbox partitioning          | 🔲     | Partition by occurred_at for scale      |
| Backup strategy              | 🔲     | pg_dump + WAL archiving                 |
| Security headers             | 🔲     | Add helmet/security middleware          |

## Event Ordering Strategy

Events within the same aggregate are ordered by `occurred_at` in the outbox. The outbox worker processes events in `occurred_at ASC` order. Since the in-memory bus processes events synchronously, ordering is preserved within a single worker.

For multi-worker or multi-partition setups (e.g., Kafka), ensure events for the same aggregate go to the same partition by using `aggregate_id` as the partition key.

## Testing

```bash
# Unit tests (domain + handlers)
go test -v ./internal/domain/... ./internal/application/...

# Integration tests (requires Docker)
go test -v -tags=integration ./internal/infrastructure/...

# E2E tests (requires running app)
go test -v -tags=e2e ./test/...
```
