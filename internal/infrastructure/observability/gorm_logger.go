package observability

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormZapLogger wraps zap.SugaredLogger into gorm.logger.Interface
type GormZapLogger struct {
	ZapLogger                 *zap.SugaredLogger
	LogLevel                  gormlogger.LogLevel
	SlowThreshold             time.Duration
	SkipCallerLookup          bool
	IgnoreRecordNotFoundError bool
}

// NewGormLogger creates a new GormZapLogger
func NewGormLogger(zapLogger *zap.SugaredLogger) *GormZapLogger {
	return &GormZapLogger{
		ZapLogger:                 zapLogger,
		LogLevel:                  gormlogger.Warn, // Log level for GORM
		SlowThreshold:             200 * time.Millisecond,
		SkipCallerLookup:          false,
		IgnoreRecordNotFoundError: true,
	}
}

// LogMode sets log mode
func (l *GormZapLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info prints info
func (l *GormZapLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		l.ZapLogger.Infof(msg, data...)
	}
}

// Warn prints warn messages
func (l *GormZapLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		l.ZapLogger.Warnf(msg, data...)
	}
}

// Error prints error messages
func (l *GormZapLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		l.ZapLogger.Errorf(msg, data...)
	}
}

// Trace print sql message
func (l *GormZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	
	// Extract trace information from context
	traceID := ""
	spanID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		traceID = sc.TraceID().String()
		spanID = sc.SpanID().String()
	}

	fields := []interface{}{
		"elapsed_ms", elapsed.Milliseconds(),
		"trace_id",   traceID,
		"span_id",    spanID,
	}

	switch {
	case err != nil && l.LogLevel >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		fields = append(fields, "error", err, "rows", rows, "sql", sql)
		l.ZapLogger.Errorw("database error", fields...)
	case elapsed > l.SlowThreshold && l.LogLevel >= gormlogger.Warn:
		sql, rows := fc()
		fields = append(fields, "rows", rows, "sql", sql, "slow_threshold", l.SlowThreshold)
		l.ZapLogger.Warnw("database slow query", fields...)
	case l.LogLevel == gormlogger.Info:
		sql, rows := fc()
		fields = append(fields, "rows", rows, "sql", sql)
		l.ZapLogger.Infow("database query", fields...)
	}
}
