package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	gormtracing "gorm.io/plugin/opentelemetry/tracing"

	"github.com/himmel/order-service/internal/application/command"
	"github.com/himmel/order-service/internal/application/port"
	"github.com/himmel/order-service/internal/application/projection"
	"github.com/himmel/order-service/internal/application/query"
	"github.com/himmel/order-service/internal/infrastructure/messaging"
	"github.com/himmel/order-service/internal/infrastructure/observability"
	"github.com/himmel/order-service/internal/infrastructure/persistence"
	"github.com/himmel/order-service/internal/infrastructure/worker"
	httphandler "github.com/himmel/order-service/internal/interfaces/http"
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
			logger.Errorw("failed to close log file", "error", closeErr)
		}
	}()

	logger.Infow("starting order-service",
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

	// --- Persistence (for early initialization) ---
	logStore := persistence.NewLogStore(db)
	metricStore := persistence.NewMetricStore(db)

	// Setup logger with database persistence
	dbHook := observability.NewDatabaseHook(logStore.SaveLog)
	loggerWithDB := observability.LoggerWithDatabasePersistence(logger, dbHook).Sugar()

	// --- Tracing ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdown, err := observability.InitTracer(ctx, cfg.Tracing.ServiceName, cfg.Tracing.OTLPEndpoint, loggerWithDB)
	if err != nil {
		loggerWithDB.Warnw("tracing init failed", "error", err)
	}
	defer shutdown(ctx)

	// --- SigNoz Logs ---
	newLogger, signozCleanup, err := observability.InitSigNozLogger(ctx, cfg.Tracing.ServiceName, cfg.SigNoz.OTLPEndpoint, loggerWithDB)
	if err != nil {
		loggerWithDB.Warnw("SigNoz logs init failed, continuing without SigNoz", "error", err)
	} else {
		loggerWithDB = newLogger
	}
	defer signozCleanup()
	
	// Update GORM logger to use our Zap logger (which now includes DB persistence and SigNoz)
	db.Logger = observability.NewGormLogger(loggerWithDB).LogMode(gormlogger.Info)

	// --- Metrics ---
	promRegistry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = promRegistry
	prometheus.DefaultGatherer = promRegistry

	promRegistry.MustRegister(prometheus.NewGoCollector())
	promRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// --- Persistence ---
	uow := persistence.NewGormUnitOfWork(db)
	outboxStore := persistence.NewGormOutboxStore(db)
	readModelStore := persistence.NewGormReadModelStore(db)
	processedEventStore := persistence.NewGormProcessedEventStore(db)

	// Initialize metrics with database persistence
	metrics := observability.NewMetricsWithDatabasePersistence(promRegistry, metricStore)
	defer metrics.Stop()

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
		kafkaBus = messaging.NewKafkaEventBus(kafkaCfg, loggerWithDB)
		eventBus = kafkaBus
		loggerWithDB.Infow("event bus: kafka",
			"brokers", cfg.Kafka.Brokers,
			"topic", cfg.Kafka.Topic,
			"consumer_group", cfg.Kafka.ConsumerGroup,
		)
	default:
		eventBus = messaging.NewInMemoryEventBus(loggerWithDB)
		loggerWithDB.Infow("event bus: inmemory")
	}

	// --- Projection ---
	projHandler := projection.NewOrderProjectionHandler(readModelStore, processedEventStore, loggerWithDB)
	projWorker := worker.NewProjectionWorker(eventBus, projHandler, loggerWithDB)
	projWorker.Setup()

	// --- Outbox Worker ---
	outboxWorker := worker.NewOutboxWorker(
		outboxStore,
		eventBus,
		loggerWithDB,
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
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Middleware stack
	r.Use(httphandler.GinRecoveryMiddleware(loggerWithDB))
	r.Use(httphandler.GinRequestIDMiddleware())
	loggerWithDB.Infow("DEBUG: Registering logging middleware with database", "db_not_nil", db != nil)
	r.Use(httphandler.GinLoggingMiddlewareWithDB(loggerWithDB, db))
	r.Use(httphandler.GinMetricsMiddleware(metrics.HTTPRequestDuration))

	// Health & metrics endpoints
	r.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	r.GET("/health/ready", func(c *gin.Context) {
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			loggerWithDB.Warnw("health check failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapF(promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}).ServeHTTP))

	// API routes
	handler := httphandler.NewHandler(createOrderH, addItemH, confirmOrderH, getOrderH, listOrdersH, loggerWithDB)
	handler.RegisterGinRoutes(r)

	// Admin: requeue failed events
	r.POST("/admin/outbox/requeue", func(c *gin.Context) {
		count, err := outboxStore.RequeueFailed(c.Request.Context())
		if err != nil {
			loggerWithDB.Errorw("failed to requeue events", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		loggerWithDB.Infow("events requeued successfully", "count", count)
		c.JSON(http.StatusOK, gin.H{"requeued": count})
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
		kafkaConsumer := worker.NewKafkaConsumerWorker(reader, kafkaBus, loggerWithDB)
		go kafkaConsumer.Run(ctx)
	}

	// --- HTTP Server ---
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: otelhttp.NewHandler(r, "http.server"),
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		if cfg.Server.HTTPS.Enabled {
			loggerWithDB.Infow("https server listening",
				"addr", srv.Addr,
				"cert_file", cfg.Server.HTTPS.CertFile,
			)
			if err := srv.ListenAndServeTLS(cfg.Server.HTTPS.CertFile, cfg.Server.HTTPS.KeyFile); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		} else {
			loggerWithDB.Infow("http server listening", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		loggerWithDB.Infow("received shutdown signal", "signal", sig)
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	// Shutdown sequence
	cancel() // Stop workers

	// Close Kafka writer if used
	if kafkaBus != nil {
		if err := kafkaBus.Close(); err != nil {
			loggerWithDB.Errorw("failed to close kafka writer", "error", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	loggerWithDB.Infow("server stopped gracefully")
	return nil
}
