package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"

	"github.com/namcuongq/order-service/internal/application/command"
	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/application/projection"
	"github.com/namcuongq/order-service/internal/application/query"
	"github.com/namcuongq/order-service/internal/infrastructure/messaging"
	"github.com/namcuongq/order-service/internal/infrastructure/observability"
	"github.com/namcuongq/order-service/internal/infrastructure/persistence"
	"github.com/namcuongq/order-service/internal/infrastructure/worker"
	httphandler "github.com/namcuongq/order-service/internal/interfaces/http"
)

// Run is the main application lifecycle.
func Run() error {
	cfg := LoadConfig()
	logger, closeLogFile, err := observability.NewLogger(cfg.Log.Level, cfg.Log.FilePath)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer func() {
		if closeErr := closeLogFile(); closeErr != nil {
			logger.Error("failed to close log file", "error", closeErr)
		}
	}()
	slog.SetDefault(logger)

	logger.Info("starting order-service",
		"port", cfg.Server.Port,
		"event_bus", cfg.EventBus.Type,
	)

	// --- Database ---
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	// Add OpenTelemetry tracing to GORM
	if err := db.Use(gormtracing.NewPlugin()); err != nil {
		return fmt.Errorf("setup gorm tracing: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// --- Tracing ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown, err := observability.InitTracer(ctx, cfg.Tracing.ServiceName, cfg.Tracing.OTLPEndpoint, logger)
	if err != nil {
		logger.Warn("tracing init failed", "error", err)
	}
	defer shutdown(ctx)

	// --- Logs Export ---
	logsShutdown, err := observability.InitLogs(ctx, cfg.Tracing.ServiceName, cfg.Tracing.OTLPEndpoint, logger)
	if err != nil {
		logger.Warn("logs export init failed", "error", err)
	}
	defer logsShutdown(ctx)

	// --- Metrics ---
	promRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = promRegistry
	prometheus.DefaultGatherer = promRegistry

	promRegistry.MustRegister(prometheus.NewGoCollector())
	promRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	metrics := observability.NewMetrics(promRegistry)

	// --- Persistence ---
	uow := persistence.NewGormUnitOfWork(db)
	outboxStore := persistence.NewGormOutboxStore(db)
	readModelStore := persistence.NewGormReadModelStore(db)
	processedEventStore := persistence.NewGormProcessedEventStore(db)

	// --- Event Bus (pluggable: inmemory or kafka) ---
	var eventBus port.EventBus
	var kafkaBus *messaging.KafkaEventBus // Keep reference for close + consumer wiring.

	switch cfg.EventBus.Type {
	case "kafka":
		kafkaCfg := messaging.KafkaConfig{
			Brokers:       cfg.Kafka.Brokers,
			Topic:         cfg.Kafka.Topic,
			ConsumerGroup: cfg.Kafka.ConsumerGroup,
			BatchSize:     cfg.Kafka.BatchSize,
			BatchTimeout:  cfg.Kafka.BatchTimeout,
			MinBytes:      1,
			MaxBytes:      10 << 20,
			MaxWait:       500 * time.Millisecond,
		}
		kafkaBus = messaging.NewKafkaEventBus(kafkaCfg, logger)
		eventBus = kafkaBus
		logger.Info("event bus: kafka",
			"brokers", cfg.Kafka.Brokers,
			"topic", cfg.Kafka.Topic,
			"consumer_group", cfg.Kafka.ConsumerGroup,
		)
	default:
		eventBus = messaging.NewInMemoryEventBus(logger)
		logger.Info("event bus: inmemory")
	}

	// --- Projection ---
	projHandler := projection.NewOrderProjectionHandler(readModelStore, processedEventStore, logger)
	projWorker := worker.NewProjectionWorker(eventBus, projHandler, logger)
	projWorker.Setup()

	// --- Outbox Worker ---
	outboxWorker := worker.NewOutboxWorker(
		outboxStore,
		eventBus,
		logger,
		cfg.Outbox.BatchSize,
		cfg.Outbox.PollInterval,
		metrics.OutboxPendingGauge,
		metrics.OutboxPublishSuccess,
		metrics.OutboxPublishFailed,
	)

	// --- Command & Query Handlers ---
	createOrderH := command.NewCreateOrderHandler(uow)
	addItemH := command.NewAddItemHandler(uow)
	confirmOrderH := command.NewConfirmOrderHandler(uow)
	getOrderH := query.NewGetOrderHandler(readModelStore)
	listOrdersH := query.NewListOrdersHandler(readModelStore)

	// --- HTTP Router ---
	r := chi.NewRouter()

	// Middleware stack.
	r.Use(httphandler.RecoveryMiddleware(logger))
	r.Use(httphandler.RequestIDMiddleware)
	r.Use(httphandler.LoggingMiddleware(logger))
	r.Use(httphandler.MetricsMiddleware(metrics.HTTPRequestDuration))

	// Health & metrics endpoints.
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := sqlDB.PingContext(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready", "error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})
	r.Handle("/metrics", promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}))

	// API routes.
	handler := httphandler.NewHandler(createOrderH, addItemH, confirmOrderH, getOrderH, listOrdersH, logger)
	handler.RegisterRoutes(r)

	// Admin: requeue failed events.
	r.Post("/admin/outbox/requeue", func(w http.ResponseWriter, r *http.Request) {
		count, err := outboxStore.RequeueFailed(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int64{"requeued": count})
	})

	// --- Start Workers ---
	go outboxWorker.Run(ctx)

	// Start Kafka consumer worker if using Kafka.
	if kafkaBus != nil {
		kafkaCfg := messaging.KafkaConfig{
			Brokers:       cfg.Kafka.Brokers,
			Topic:         cfg.Kafka.Topic,
			ConsumerGroup: cfg.Kafka.ConsumerGroup,
			MinBytes:      1,
			MaxBytes:      10 << 20,
			MaxWait:       500 * time.Millisecond,
		}
		reader := messaging.NewKafkaReader(kafkaCfg)
		kafkaConsumer := worker.NewKafkaConsumerWorker(reader, kafkaBus, logger)
		go kafkaConsumer.Run(ctx)
	}

	// --- HTTP Server ---
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: otelhttp.NewHandler(r, "http.server"),
	}

	// Graceful shutdown.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Shutdown sequence.
	cancel() // Stop workers.

	// Close Kafka writer if used.
	if kafkaBus != nil {
		if err := kafkaBus.Close(); err != nil {
			logger.Error("failed to close kafka writer", "error", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}
