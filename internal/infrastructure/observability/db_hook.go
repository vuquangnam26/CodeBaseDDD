package observability

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DatabaseHook sends logs to database.
type DatabaseHook struct {
	SaveLog func(ctx context.Context, timestamp time.Time, level, message, loggerName, caller, traceID, correlationID string, fields map[string]interface{}) error
	mu      sync.Mutex
}

// NewDatabaseHook creates a new database hook for logs.
func NewDatabaseHook(saveFn func(ctx context.Context, timestamp time.Time, level, message, loggerName, caller, traceID, correlationID string, fields map[string]interface{}) error) *DatabaseHook {
	return &DatabaseHook{
		SaveLog: saveFn,
	}
}

// LogEntry represents a structured log entry for database storage.
type LogEntry struct {
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	Caller        string                 `json:"caller,omitempty"`
	TraceID       string                 `json:"trace_id,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
	Timestamp     int64                  `json:"timestamp"`
}

// Hook is compatible with Zap's CheckedEntry structure.
// This allows use as a hook for captured entries.
func (h *DatabaseHook) Hook(entry zapcore.Entry, fields []zapcore.Field) error {
	if h.SaveLog == nil {
		return nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(encoder)
	}
	fieldsMap := encoder.Fields

	// Extract caller info if available
	caller := entry.Caller.String()
	if caller == "" {
		caller = ""
	}

	// Note: You might extract trace_id and correlation_id from context or fields
	// For now, we'll pass empty strings - they should be added as fields by the application
	traceID := ""
	correlationID := ""
	if val, ok := fieldsMap["trace_id"]; ok {
		if str, ok := val.(string); ok {
			traceID = str
		}
	}
	if val, ok := fieldsMap["correlation_id"]; ok {
		if str, ok := val.(string); ok {
			correlationID = str
		}
	}

	return h.SaveLog(context.Background(), entry.Time, entry.Level.String(), entry.Message, entry.LoggerName, caller, traceID, correlationID, fieldsMap)
}

// LoggerWithDatabasePersistence creates a logger that persists logs to database.
func LoggerWithDatabasePersistence(baseLogger *zap.SugaredLogger, hooks ...*DatabaseHook) *zap.Logger {
	// Convert sugared logger back to normal logger to add hooks
	unsugared := baseLogger.Desugar()

	// Create a core that wraps the existing core with database hook
	wrappedCore := &hookedCore{
		core:  unsugared.Core(),
		hooks: hooks,
	}

	return zap.New(wrappedCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

// hookedCore wraps a zapcore.Core and calls hooks on each write.
type hookedCore struct {
	core  zapcore.Core
	hooks []*DatabaseHook
}

// Enabled delegates to the wrapped core.
func (c *hookedCore) Enabled(lvl zapcore.Level) bool {
	return c.core.Enabled(lvl)
}

// With delegates to the wrapped core.
func (c *hookedCore) With(fields []zapcore.Field) zapcore.Core {
	return &hookedCore{
		core:  c.core.With(fields),
		hooks: c.hooks,
	}
}

// Check delegates to the wrapped core and returns a checked entry with hooks.
func (c *hookedCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

// Write writes the entry and calls all registered hooks.
func (c *hookedCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if err := c.core.Write(entry, fields); err != nil {
		return err
	}

	// Call all hooks
	for _, hook := range c.hooks {
		if hook != nil {
			if err := hook.Hook(entry, fields); err != nil {
				// Don't fail on hook error, just log it silently
				_ = err
			}
		}
	}

	return nil
}

// Sync delegates to the wrapped core.
func (c *hookedCore) Sync() error {
	return c.core.Sync()
}
