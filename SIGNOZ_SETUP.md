# SigNoz Integration Guide

This document provides instructions for monitoring your Order Service using **SigNoz**, an open-source observability platform.

> **Cloud Users:** If you want to use SigNoz Cloud instead of local deployment, see [SIGNOZ_CLOUD_SETUP.md](SIGNOZ_CLOUD_SETUP.md) for cloud-specific configuration.

## What is SigNoz?

SigNoz is a unified observability platform that provides:

- **Distributed Tracing** — Track requests across services
- **Metrics Monitoring** — Monitor performance over time
- **Log Management** — Centralized log collection and inspection
- **Alerts** — Set up alerts based on metrics and anomalies

## Quick Start

### 1. Start the Full Stack

```bash
docker-compose up -d
```

This brings up:

- **PostgreSQL** (port 5432) — application database
- **Kafka** (port 9092) — event streaming
- **ClickHouse** (port 9000) — time-series database for SigNoz
- **OpenTelemetry Collector** — data collection and forwarding
- **SigNoz Query Service** (port 8080) — API for telemetry data
- **SigNoz Frontend** (port 3301) — web dashboard
- **Order Service** (port 8081) — your application

### 2. Access SigNoz Dashboard

Open your browser and navigate to:

```
http://localhost:3301
```

### 3. Send API Requests to Generate Data

Create some orders to generate traces and metrics:

```bash
# Create an order
curl -X POST http://localhost:8081/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id": "550e8400-e29b-41d4-a716-446655440000"}'

# Confirm the order
curl -X POST http://localhost:8081/orders/{ORDER_ID}/confirm \
  -H "Content-Type: application/json"

# View metrics
curl http://localhost:8081/metrics
```

## Viewing Data in SigNoz

### Traces

1. Go to **Services** tab
2. Look for **order-service**
3. Click on it to see recent traces
4. Filter by:
   - **Operation** (e.g., POST /orders)
   - **Status** (error, success)
   - **Latency** (duration)

### Metrics

1. Go to **Metrics** tab
2. Select metrics like:
   - HTTP request duration
   - Error rates
   - Request count

### Logs

1. Go to **Logs** tab
2. Search logs by:
   - **Service**: order-service
   - **Level**: info, error, warn
   - **Message content**

## Architecture Overview

```
┌─────────────────────┐
│  Your Application   │
│  (Order Service)    │
│                     │
│  - OTLP Exporter    │──┐
│  - JSON Log Files   │  │
└─────────────────────┘  │
                         │
                   ┌─────▼──────────────┐
                   │  OTEL Collector    │
                   │                    │
                   │  - Receives OTLP   │
                   │  - Collects logs   │
                   │  - Forwards data   │
                   └─────┬──────────────┘
                         │
        ┌────────────────┴────────────────┬──────────────┐
        │                                 │              │
   ┌────▼─────────┐            ┌──────────▼────┐  ┌─────▼──────┐
   │  ClickHouse  │            │  Query Service│  │  Frontend  │
   │              │            │                │  │            │
   │ Time-Series  │◄───────────│  API (REST)    │  │  Dashboard │
   │ Database     │            │  Port: 8080    │  │  Port:3301 │
   └──────────────┘            └────────────────┘  └────────────┘
```

## Key Environment Variables

| Variable                      | Value                          | Purpose                           |
| ----------------------------- | ------------------------------ | --------------------------------- |
| `OTEL_SERVICE_NAME`           | order-service                  | Identifies your service in SigNoz |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | otel-collector:4318            | Where to send telemetry           |
| `LOG_FILE_PATH`               | /var/log/order-service/app.log | Application logs                  |

## Verifying the Integration

### Check if Data is Flowing

1. Open SigNoz Frontend: http://localhost:3301
2. Go to **Services** tab
3. You should see **order-service** listed
4. Click on it to view recent traces

### Check Container Logs

```bash
# View OpenTelemetry Collector logs
docker logs order-otel-collector

# View SigNoz Query Service logs
docker logs signoz-query-service

# View ClickHouse logs
docker logs signoz-clickhouse
```

### Verify Port Connectivity

```bash
# Check if OTEL Collector is up
curl http://localhost:4318/status

# Check if SigNoz Query Service is up
curl http://localhost:8080/health

# Check if SigNoz Frontend is up
curl http://localhost:3301
```

## Troubleshooting

### No Data Showing in SigNoz

**Problem:** Traces/metrics not appearing in the dashboard.

**Solution:**

1. Verify the application is running:
   ```bash
   curl http://localhost:8081/health/live
   ```
2. Check OTEL Collector logs for errors:
   ```bash
   docker logs order-otel-collector | tail -20
   ```
3. Ensure ClickHouse is healthy:
   ```bash
   docker logs signoz-clickhouse | tail -20
   ```
4. Generate some traffic:
   ```bash
   curl -X POST http://localhost:8081/orders \
     -H "Content-Type: application/json" \
     -d '{"customer_id": "550e8400-e29b-41d4-a716-446655440000"}'
   ```

### OTEL Collector Connection Refused

**Problem:** Application can't connect to OTEL Collector.

**Solution:**

1. Ensure all containers are running:
   ```bash
   docker-compose ps
   ```
2. Check network connectivity:
   ```bash
   docker exec order-service ping order-otel-collector
   ```
3. Verify endpoint in environment variable:
   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4318
   ```

### High Memory Usage in ClickHouse

**Problem:** ClickHouse container consuming too much memory.

**Solution:**

- ClickHouse keeps recent data (TTL can be configured)
- Consider setting retention policy:
  ```yaml
  # In docker-compose.yaml
  environment:
    CLICKHOUSE_RETENTION_DAYS: 7 # Keep 7 days of data
  ```

## Advanced Configuration

### Custom Dashboards

You can create custom dashboards in SigNoz to monitor specific metrics:

1. Go to **Dashboards** tab
2. Click **Create Dashboard**
3. Add panels with:
   - Service latency
   - Error rates
   - Custom business metrics

### Alert Configuration

Set up alerts for production:

1. Go to **Alerts** tab
2. Create an alert rule:
   - **Condition**: Error rate > 5%
   - **Duration**: 5 minutes
   - **Action**: Send notification (Slack, email, webhook)

### Retention & Storage

Configure how long data is retained:

1. Edit `docker-compose.yaml`
2. Adjust ClickHouse retention:
   ```yaml
   clickhouse:
     environment:
       TTL_DAYS: 30 # Keep 30 days of data
   ```

## Performance Tips

1. **Sample High Traffic**: Use sampling to reduce data volume:

   ```go
   // In your app configuration
   sampler := sdktrace.NewParentBasedSampler(
       sdktrace.TraceIDRatioBased(0.1), // 10% sampling
   )
   ```

2. **Batch Exports**: Configure batch processor in OTEL Collector:

   ```yaml
   processors:
     batch:
       send_batch_size: 1024
       timeout: 10s
   ```

3. **Enable Compression**: Compress telemetry in transit:
   ```yaml
   exporters:
     otlp:
       compression: gzip
   ```

## Learn More

- **SigNoz Docs**: https://signoz.io/docs/
- **OpenTelemetry**: https://opentelemetry.io/
- **OTEL Collector Config**: https://opentelemetry.io/docs/collector/configuration/

## Cleanup

To stop all services:

```bash
docker-compose down

# Also remove volumes (if you want fresh start)
docker-compose down -v
```

---

Enjoy monitoring your order service! 🚀
