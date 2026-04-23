package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestGinLoggingMiddlewareWithDB_LogsRequestAndResponseBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	logger := zap.New(core).Sugar()

	r := gin.New()
	r.Use(GinRequestIDMiddleware())
	r.Use(GinLoggingMiddlewareWithDB(logger, nil))
	r.POST("/orders", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/orders?page=1", strings.NewReader(`{"customer_id":"cust-1"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, resp.Code)
	}

	entries := logs.FilterMessage("http request").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	ctx := entries[0].ContextMap()
	if got := ctx["request_body"]; got != `{"customer_id":"cust-1"}` {
		t.Fatalf("expected request body to be logged, got %#v", got)
	}
	if got := ctx["response_body"]; got != `{"ok":true}` {
		t.Fatalf("expected response body to be logged, got %#v", got)
	}
	if got := ctx["query"]; got != "page=1" {
		t.Fatalf("expected query to be logged, got %#v", got)
	}
	if _, ok := ctx["duration_ms"]; !ok {
		t.Fatal("expected duration_ms to be logged")
	}
	if got := ctx["status"]; got != int64(http.StatusCreated) {
		t.Fatalf("expected status to be logged, got %#v", got)
	}
}

func TestFormatBodyForLog_OmitsBinaryBody(t *testing.T) {
	got := formatBodyForLog("application/octet-stream", []byte{0x00, 0x01, 0x02})
	if got != "<omitted: non-text body>" {
		t.Fatalf("expected binary body to be omitted, got %q", got)
	}
}
