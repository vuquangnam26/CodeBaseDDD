package observability

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestDatabaseHook_HookPersistsInfoLogWithoutTraceOrCorrelation(t *testing.T) {
	called := false
	hook := NewDatabaseHook(func(ctx context.Context, timestamp time.Time, level, message, loggerName, caller, traceID, correlationID string, fields map[string]interface{}) error {
		called = true
		if level != "info" {
			t.Fatalf("expected level info, got %s", level)
		}
		if message != "http request" {
			t.Fatalf("expected message http request, got %s", message)
		}
		return nil
	})

	err := hook.Hook(zapcore.Entry{Level: zapcore.InfoLevel, Message: "http request", Time: time.Now()}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected save hook to be called")
	}
}
