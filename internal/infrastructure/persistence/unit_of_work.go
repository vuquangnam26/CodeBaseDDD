package persistence

import (
	"context"

	"gorm.io/gorm"

	"github.com/namcuongq/order-service/internal/application/port"
	"github.com/namcuongq/order-service/internal/domain/order"
)

// GormUnitOfWork implements port.UnitOfWork using GORM transactions.
type GormUnitOfWork struct {
	db *gorm.DB
}

func NewGormUnitOfWork(db *gorm.DB) *GormUnitOfWork {
	return &GormUnitOfWork{db: db}
}

func (u *GormUnitOfWork) Execute(ctx context.Context, fn func(ctx context.Context, tx port.UnitOfWorkTx) error) error {
	return u.db.WithContext(ctx).Transaction(func(gormTx *gorm.DB) error {
		tx := &gormUnitOfWorkTx{
			orderRepo:  NewGormOrderRepository(gormTx),
			outboxStore: NewGormOutboxStore(gormTx),
		}
		return fn(ctx, tx)
	})
}

// gormUnitOfWorkTx provides scoped repositories within a transaction.
type gormUnitOfWorkTx struct {
	orderRepo  *GormOrderRepository
	outboxStore *GormOutboxStore
}

func (t *gormUnitOfWorkTx) OrderRepo() order.Repository {
	return t.orderRepo
}

func (t *gormUnitOfWorkTx) OutboxStore() port.OutboxStore {
	return t.outboxStore
}
