package persistence

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// LogStore provides database persistence for logs.
type LogStore struct {
	db *gorm.DB
}

// NewLogStore creates a new LogStore.
func NewLogStore(db *gorm.DB) *LogStore {
	return &LogStore{db: db}
}

// SaveLog saves a log entry to the database.
func (s *LogStore) SaveLog(ctx context.Context, level, message, loggerName, caller, traceID, correlationID string, fields map[string]interface{}) error {
	var fieldsData []byte
	var err error
	if fields != nil {
		fieldsData, err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}

	var loggerNamePtr, callerPtr, traceIDPtr, correlationIDPtr *string
	if loggerName != "" {
		loggerNamePtr = &loggerName
	}
	if caller != "" {
		callerPtr = &caller
	}
	if traceID != "" {
		traceIDPtr = &traceID
	}
	if correlationID != "" {
		correlationIDPtr = &correlationID
	}

	log := &LogModel{
		Timestamp:     time.Now(),
		Level:         level,
		Message:       message,
		LoggerName:    loggerNamePtr,
		Caller:        callerPtr,
		TraceID:       traceIDPtr,
		CorrelationID: correlationIDPtr,
		Fields:        fieldsData,
		CreatedAt:     time.Now(),
	}

	return s.db.WithContext(ctx).Create(log).Error
}

// QueryLogs queries logs with filters.
func (s *LogStore) QueryLogs(ctx context.Context, level string, correlationID string, limit int, offset int) ([]LogModel, error) {
	var logs []LogModel
	query := s.db.WithContext(ctx)

	if level != "" {
		query = query.Where("level = ?", level)
	}
	if correlationID != "" {
		query = query.Where("correlation_id = ?", correlationID)
	}

	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error

	return logs, err
}

// GetLogsByTraceID retrieves all logs for a specific trace ID.
func (s *LogStore) GetLogsByTraceID(ctx context.Context, traceID string) ([]LogModel, error) {
	var logs []LogModel
	err := s.db.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("created_at ASC").
		Find(&logs).Error
	return logs, err
}
