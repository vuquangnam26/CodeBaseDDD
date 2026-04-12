package persistence

import (
	"time"

	"github.com/google/uuid"
)

// --- Write-side GORM models ---

// OrderModel maps to the "orders" table.
type OrderModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	CustomerID  uuid.UUID `gorm:"type:uuid;not null;index"`
	Status      string    `gorm:"type:varchar(20);not null;default:'DRAFT';index"`
	TotalAmount int64     `gorm:"not null;default:0"`
	Version     int       `gorm:"not null;default:1"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
	UpdatedAt   time.Time `gorm:"not null;default:now()"`
}

func (OrderModel) TableName() string { return "orders" }

// OrderItemModel maps to the "order_items" table.
type OrderItemModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrderID     uuid.UUID `gorm:"type:uuid;not null;index"`
	ProductID   string    `gorm:"type:varchar(255);not null"`
	ProductName string    `gorm:"type:varchar(500);not null"`
	Quantity    int       `gorm:"not null"`
	UnitPrice   int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null;default:now()"`
}

func (OrderItemModel) TableName() string { return "order_items" }

// --- Outbox model ---

// OutboxEventModel maps to the "outbox_events" table.
type OutboxEventModel struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey"`
	AggregateType string     `gorm:"type:varchar(100);not null"`
	AggregateID   string     `gorm:"type:varchar(100);not null"`
	EventType     string     `gorm:"type:varchar(100);not null"`
	Payload       []byte     `gorm:"type:jsonb;not null"`
	Metadata      []byte     `gorm:"type:jsonb;not null;default:'{}'"`
	OccurredAt    time.Time  `gorm:"not null;default:now()"`
	AvailableAt   time.Time  `gorm:"not null;default:now()"`
	PublishedAt   *time.Time `gorm:""`
	Status        string     `gorm:"type:varchar(20);not null;default:'pending'"`
	RetryCount    int        `gorm:"not null;default:0"`
	LastError     *string    `gorm:"type:text"`
}

func (OutboxEventModel) TableName() string { return "outbox_events" }

// --- Idempotency / Processed events models ---

type IdempotencyKeyModel struct {
	Key            string    `gorm:"type:varchar(255);primaryKey"`
	ResponseStatus int       `gorm:"not null"`
	ResponseBody   []byte    `gorm:"type:jsonb"`
	CreatedAt      time.Time `gorm:"not null;default:now()"`
	ExpiresAt      time.Time `gorm:"not null;index"`
}

func (IdempotencyKeyModel) TableName() string { return "idempotency_keys" }

type ProcessedEventModel struct {
	EventID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	HandlerName string    `gorm:"type:varchar(100);not null;primaryKey"`
	ProcessedAt time.Time `gorm:"not null;default:now()"`
}

func (ProcessedEventModel) TableName() string { return "processed_events" }

// --- Logging and Metrics models ---

// LogModel maps to the "logs" table
type LogModel struct {
	ID            int64     `gorm:"primaryKey;autoIncrement"`
	Timestamp     time.Time `gorm:"not null;index;default:now()"`
	Level         string    `gorm:"type:varchar(10);not null;index"`
	Message       string    `gorm:"type:text;not null"`
	LoggerName    *string   `gorm:"type:varchar(255)"`
	Caller        *string   `gorm:"type:varchar(255)"`
	TraceID       *string   `gorm:"type:varchar(32);index"`
	CorrelationID *string   `gorm:"type:varchar(36);index"`
	Fields        []byte    `gorm:"type:jsonb"`
	CreatedAt     time.Time `gorm:"not null;index:,type:desc;default:now()"`
}

func (LogModel) TableName() string { return "logs" }

// MetricModel maps to the "metrics" table
type MetricModel struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	Timestamp  time.Time `gorm:"not null;index;default:now()"`
	MetricName string    `gorm:"type:varchar(255);not null;index"`
	MetricType string    `gorm:"type:varchar(20);not null"`
	Value      float64   `gorm:"not null"`
	Labels     []byte    `gorm:"type:jsonb"`
	CreatedAt  time.Time `gorm:"not null;index:,type:desc;default:now()"`
}

func (MetricModel) TableName() string { return "metrics" }
