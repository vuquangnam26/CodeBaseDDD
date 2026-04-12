package persistence

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// SaveHTTPLog saves an HTTP request log directly to the database.
func SaveHTTPLog(db *gorm.DB, ctx context.Context, method, path string, statusCode int, duration time.Duration, correlationID, traceID, spanID, clientIP, userAgent string) error {
	// Use background context since this is called async after request completes
	// Don't use the request context which gets cancelled
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fields := map[string]interface{}{
		"method":      method,
		"path":        path,
		"status":      statusCode,
		"duration_ms": duration.Milliseconds(),
		"client_ip":   clientIP,
		"user_agent":  userAgent,
	}

	fieldsData, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	var correlationIDPtr, traceIDPtr *string
	if correlationID != "" {
		correlationIDPtr = &correlationID
	}
	if traceID != "" {
		traceIDPtr = &traceID
	}

	log := &LogModel{
		Timestamp:     time.Now(),
		Level:         "INFO",
		Message:       "http request",
		LoggerName:    nil,
		Caller:        nil,
		TraceID:       traceIDPtr,
		CorrelationID: correlationIDPtr,
		Fields:        fieldsData,
		CreatedAt:     time.Now(),
	}

	return db.WithContext(bgCtx).Create(log).Error
}
