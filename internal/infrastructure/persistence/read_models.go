package persistence

import (
	"time"

	"github.com/google/uuid"
)

// --- Read-side GORM models ---

type OrderViewModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	CustomerID  uuid.UUID `gorm:"type:uuid;not null;index"`
	Status      string    `gorm:"type:varchar(20);not null;default:'DRAFT';index"`
	TotalAmount int64     `gorm:"not null;default:0"`
	ItemCount   int       `gorm:"not null;default:0"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
	UpdatedAt   time.Time `gorm:"not null;default:now()"`
}

func (OrderViewModel) TableName() string { return "order_views" }

type OrderItemViewModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrderID     uuid.UUID `gorm:"type:uuid;not null;index"`
	ProductID   string    `gorm:"type:varchar(255);not null"`
	ProductName string    `gorm:"type:varchar(500);not null"`
	Quantity    int       `gorm:"not null"`
	UnitPrice   int64     `gorm:"not null"`
	TotalPrice  int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
}

func (OrderItemViewModel) TableName() string { return "order_item_views" }
