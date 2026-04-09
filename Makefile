.PHONY: dev dev-kafka test migrate-up migrate-down build run worker-outbox worker-projection lint

# Database URL for migrations
DATABASE_URL ?= postgres://order:order123@localhost:5432/orderdb?sslmode=disable

## dev: Start Postgres + run the app with in-memory event bus
dev:
	docker-compose up -d postgres
	@echo "Waiting for Postgres..."
	@timeout /t 3 >nul 2>&1 || sleep 3
	$(MAKE) migrate-up
	go run ./cmd/server

## dev-kafka: Start Postgres + Kafka + run the app with Kafka event bus
dev-kafka:
	docker-compose up -d postgres kafka
	@echo "Waiting for Postgres and Kafka..."
	@timeout /t 10 >nul 2>&1 || sleep 10
	$(MAKE) migrate-up
	EVENT_BUS_TYPE=kafka KAFKA_BROKERS=localhost:9092 go run ./cmd/server

## build: Build the application binary
build:
	go build -o bin/server.exe ./cmd/server

## run: Run the built binary
run: build
	./bin/server.exe

## test: Run all tests (unit)
test:
	go test -v -race -count=1 ./internal/...

## test-integration: Run integration tests (requires Docker)
test-integration:
	go test -v -race -count=1 -tags=integration ./internal/infrastructure/...

## test-e2e: Run end-to-end tests (requires Docker)
test-e2e:
	go test -v -race -count=1 -tags=e2e ./test/...

## test-all: Run all tests
test-all: test test-integration test-e2e

## migrate-up: Run database migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

## migrate-down: Roll back all database migrations
migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

## migrate-create: Create a new migration (usage: make migrate-create NAME=foo)
migrate-create:
	migrate create -ext sql -dir migrations -seq $(NAME)

## worker-outbox: Run outbox worker only (same binary, future flag support)
worker-outbox:
	go run ./cmd/server

## worker-projection: Run projection worker only (same binary, future flag support)
worker-projection:
	go run ./cmd/server

## lint: Run static analysis
lint:
	go vet ./...
	golangci-lint run ./...

## docker-up: Start ALL services (Postgres + Kafka + OTEL Collector + Jaeger + Prometheus + App + Migrations)
docker-up:
	docker-compose up --build -d
	@echo "Waiting for services to be ready..."
	@timeout /t 10 >nul 2>&1 || sleep 10
	$(MAKE) migrate-up
	@echo ""
	@echo "🎉 All services are running!"
	@echo "📊 Jaeger Dashboard: http://localhost:16686"
	@echo "📈 Prometheus Metrics: http://localhost:9090"
	@echo "🔌 API: http://localhost:8081"
	@echo "✅ App Metrics: http://localhost:8081/metrics"

## docker-up-infra: Start only infrastructure (Postgres + Kafka + OTEL Collector + Jaeger + Prometheus)
docker-up-infra:
	docker-compose up -d postgres kafka otel-collector jaeger prometheus
	@echo "✅ Infrastructure services started"
	@echo "📊 Jaeger available at: http://localhost:16686"
	@echo "📈 Prometheus available at: http://localhost:9090"

## docker-down: Stop all services
docker-down:
	docker-compose down

## jaeger-view: Open Jaeger dashboard in browser
jaeger-view:
	@echo "Opening Jaeger Dashboard at http://localhost:16686..."
	@start http://localhost:16686 || open http://localhost:16686 || echo "Please open http://localhost:16686 manually"

## prometheus-view: Open Prometheus dashboard in browser
prometheus-view:
	@echo "Opening Prometheus Dashboard at http://localhost:9090..."
	@start http://localhost:9090 || open http://localhost:9090 || echo "Please open http://localhost:9090 manually"

## docker-logs: Tail logs for all services
docker-logs:
	docker-compose logs -f

## kafka-topics: List Kafka topics
kafka-topics:
	docker exec order-kafka kafka-topics.sh --bootstrap-server localhost:9094 --list

## kafka-consume: Consume events from the order-events topic
kafka-consume:
	docker exec order-kafka kafka-console-consumer.sh --bootstrap-server localhost:9094 --topic order-events --from-beginning

## clean: Remove build artifacts
clean:
	rm -rf bin/
