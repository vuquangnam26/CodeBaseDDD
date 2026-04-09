FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /server .
COPY migrations ./migrations

# Set OpenTelemetry environment variables
# For local OTEL Collector (Docker Compose):
ENV OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector:4318"
ENV OTEL_SERVICE_NAME="order-service"

# For SigNoz Cloud (uncomment and set your values):
# ENV OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.in2.signoz.cloud:443"
# ENV OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<your-ingestion-key>"
# ENV OTEL_SERVICE_NAME="order-service"

EXPOSE 8080

ENTRYPOINT ["./server"]
