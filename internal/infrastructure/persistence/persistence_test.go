//go:build integration

package persistence_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/domain/order"
	"github.com/namcuongq/order-service/internal/infrastructure/persistence"
)

func setupPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")

	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=testdb sslmode=disable",
		host, port.Port())

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	// Run migrations inline (simplified for tests).
	sqlDB, _ := db.DB()
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY, customer_id UUID NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
			total_amount BIGINT NOT NULL DEFAULT 0, version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id UUID PRIMARY KEY, order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			product_id VARCHAR(255) NOT NULL, product_name VARCHAR(500) NOT NULL,
			quantity INT NOT NULL, unit_price BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS outbox_events (
			id UUID PRIMARY KEY, aggregate_type VARCHAR(100) NOT NULL, aggregate_id VARCHAR(100) NOT NULL,
			event_type VARCHAR(100) NOT NULL, payload JSONB NOT NULL, metadata JSONB NOT NULL DEFAULT '{}',
			occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			published_at TIMESTAMPTZ, status VARCHAR(20) NOT NULL DEFAULT 'pending',
			retry_count INT NOT NULL DEFAULT 0, last_error TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS order_views (
			id UUID PRIMARY KEY, customer_id UUID NOT NULL, status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
			total_amount BIGINT NOT NULL DEFAULT 0, item_count INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS order_item_views (
			id UUID PRIMARY KEY, order_id UUID NOT NULL, product_id VARCHAR(255) NOT NULL,
			product_name VARCHAR(500) NOT NULL, quantity INT NOT NULL, unit_price BIGINT NOT NULL,
			total_price BIGINT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS processed_events (
			event_id UUID NOT NULL, handler_name VARCHAR(100) NOT NULL,
			processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY(event_id, handler_name)
		)`,
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			key VARCHAR(255) PRIMARY KEY, response_status INT NOT NULL,
			response_body JSONB, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL
		)`,
	}
	for _, m := range migrations {
		_, err := sqlDB.Exec(m)
		require.NoError(t, err)
	}

	return db
}

func TestGormOrderRepository_SaveAndFind(t *testing.T) {
	db := setupPostgres(t)
	repo := persistence.NewGormOrderRepository(db)
	ctx := context.Background()

	o, err := order.NewOrder(order.NewCustomerID(uuid.New()))
	require.NoError(t, err)
	_ = o.Events()

	err = repo.Save(ctx, o)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, o.ID())
	require.NoError(t, err)
	assert.Equal(t, o.ID().String(), found.ID().String())
	assert.Equal(t, o.CustomerID().String(), found.CustomerID().String())
	assert.Equal(t, order.StatusDraft, found.Status())
}

func TestGormOrderRepository_OptimisticConcurrency(t *testing.T) {
	db := setupPostgres(t)
	repo := persistence.NewGormOrderRepository(db)
	ctx := context.Background()

	o, _ := order.NewOrder(order.NewCustomerID(uuid.New()))
	_ = o.Events()
	err := repo.Save(ctx, o)
	require.NoError(t, err)

	// Load two copies.
	o1, _ := repo.FindByID(ctx, o.ID())
	o2, _ := repo.FindByID(ctx, o.ID())

	// Save first copy.
	_ = o1.AddItem("P1", "Widget", 1, order.NewMoney(100))
	_ = o1.Events()
	err = repo.Save(ctx, o1)
	require.NoError(t, err)

	// Save second copy — should conflict.
	_ = o2.AddItem("P2", "Gadget", 1, order.NewMoney(200))
	_ = o2.Events()
	err = repo.Save(ctx, o2)
	assert.ErrorIs(t, err, order.ErrConcurrencyConflict)
}

func TestUnitOfWork_TransactionalOutbox(t *testing.T) {
	db := setupPostgres(t)
	uow := persistence.NewGormUnitOfWork(db)
	ctx := context.Background()

	var savedOrderID string

	err := uow.Execute(ctx, func(ctx context.Context, tx port.UnitOfWorkTx) error {
		o, err := order.NewOrder(order.NewCustomerID(uuid.New()))
		if err != nil {
			return err
		}
		events := o.Events()

		if err := tx.OrderRepo().Save(ctx, o); err != nil {
			return err
		}
		if err := tx.OutboxStore().Append(ctx, events); err != nil {
			return err
		}
		savedOrderID = o.ID().String()
		return nil
	})

	require.NoError(t, err)
	assert.NotEmpty(t, savedOrderID)

	// Verify outbox has the event.
	outboxStore := persistence.NewGormOutboxStore(db)
	events, err := outboxStore.FetchPending(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "OrderCreated", events[0].EventType)
}

func TestProcessedEventStore_Idempotency(t *testing.T) {
	db := setupPostgres(t)
	store := persistence.NewGormProcessedEventStore(db)
	ctx := context.Background()

	eventID := uuid.New()
	handler := "TestHandler"

	processed, err := store.IsProcessed(ctx, eventID, handler)
	require.NoError(t, err)
	assert.False(t, processed)

	err = store.MarkProcessed(ctx, eventID, handler)
	require.NoError(t, err)

	processed, err = store.IsProcessed(ctx, eventID, handler)
	require.NoError(t, err)
	assert.True(t, processed)

	// Marking again should be idempotent (no error).
	err = store.MarkProcessed(ctx, eventID, handler)
	require.NoError(t, err)
}
