package http

import (
	"bytes"
	"io"
	"net/http"
	"mime"
	"strings"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/himmel/order-service/internal/infrastructure/observability"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const maxLoggedBodyBytes = 4096

// RequestIDMiddleware injects a unique correlation ID into each request context.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		ctx := observability.ContextWithCorrelationID(r.Context(), correlationID)
		w.Header().Set("X-Correlation-ID", correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs each HTTP request with structured fields.
func LoggingMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			traceID := ""
			spanID := ""
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				traceID = sc.TraceID().String()
				spanID = sc.SpanID().String()
			}

			logger.Infow("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.statusCode,
				"duration_ms", time.Since(start).Milliseconds(),
				"correlation_id", observability.CorrelationIDFromContext(r.Context()),
				"trace_id", traceID,
				"span_id", spanID,
			)
		})
	}
}

// MetricsMiddleware records HTTP request duration.
func MetricsMiddleware(histogram *prometheus.HistogramVec) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			histogram.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(ww.statusCode)).Observe(duration)
		})
	}
}

// RecoveryMiddleware catches panics and returns 500.
func RecoveryMiddleware(logger *zap.SugaredLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Errorw("panic recovered",
						"error", rec,
						"stack", string(debug.Stack()),
					)
					writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", nil)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// ============ Gin Middleware ============

// GinRequestIDMiddleware injects a unique correlation ID into each Gin request context.
func GinRequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		ctx := observability.ContextWithCorrelationID(c.Request.Context(), correlationID)
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			span.SetAttributes(attribute.String("correlation_id", correlationID))
		}
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Correlation-ID", correlationID)
		c.Next()
	}
}

// GinLoggingMiddleware logs each HTTP request with structured fields for Gin.
func GinLoggingMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return GinLoggingMiddlewareWithDB(logger, nil)
}

// GinLoggingMiddlewareWithDB logs each HTTP request and saves to database.
func GinLoggingMiddlewareWithDB(logger *zap.SugaredLogger, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestBody, requestBodyReadErr := readAndRestoreRequestBody(c.Request)
		writer := &bodyLogWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()

		if shouldSkipRequestTelemetry(c.Request.URL.Path) {
			return
		}

		traceID := ""
		spanID := ""
		if sc := trace.SpanContextFromContext(c.Request.Context()); sc.IsValid() {
			traceID = sc.TraceID().String()
			spanID = sc.SpanID().String()
		}

		duration := time.Since(start)
		correlationID := observability.CorrelationIDFromContext(c.Request.Context())

		// Extract variables to prevent data races when c is recycled
		reqMethod := c.Request.Method
		reqPath := c.FullPath()
		if reqPath == "" {
			reqPath = c.Request.URL.Path
		}
		resStatus := c.Writer.Status()
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		requestQuery := c.Request.URL.RawQuery
		requestBodyLog := formatBodyForLog(c.ContentType(), requestBody)
		responseBodyLog := formatBodyForLog(c.Writer.Header().Get("Content-Type"), writer.body.Bytes())

		// Log the request. The logger already handles both DB persistence and SigNoz integration.
		logger.Infow("http request",
			"method", reqMethod,
			"path", reqPath,
			"query", requestQuery,
			"status", resStatus,
			"duration_ms", duration.Milliseconds(),
			"correlation_id", correlationID,
			"trace_id", traceID,
			"span_id", spanID,
			"client_ip", clientIP,
			"user_agent", userAgent,
			"request_body", requestBodyLog,
			"response_body", responseBodyLog,
		)

		if requestBodyReadErr != nil {
			logger.Warnw("failed to read request body for logging",
				"method", reqMethod,
				"path", reqPath,
				"error", requestBodyReadErr,
			)
		}
	}
}

// GinMetricsMiddleware records HTTP request duration for Gin.
func GinMetricsMiddleware(histogram *prometheus.HistogramVec) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if shouldSkipRequestTelemetry(c.Request.URL.Path) {
			return
		}

		duration := time.Since(start).Seconds()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		histogram.WithLabelValues(
			c.Request.Method,
			path,
			strconv.Itoa(c.Writer.Status()),
		).Observe(duration)
	}
}

func shouldSkipRequestTelemetry(path string) bool {
	return path == "/metrics" || strings.HasPrefix(path, "/health/") || strings.HasPrefix(path, "/swagger/")
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *bodyLogWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func readAndRestoreRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func formatBodyForLog(contentType string, body []byte) string {
	if len(body) == 0 {
		return ""
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		switch {
		case strings.HasPrefix(mediaType, "text/"):
		case mediaType == "application/json", mediaType == "application/xml", mediaType == "application/x-www-form-urlencoded":
		default:
			return "<omitted: non-text body>"
		}
	}

	if len(body) > maxLoggedBodyBytes {
		return string(body[:maxLoggedBodyBytes]) + "...(truncated)"
	}

	return string(body)
}

// GinRecoveryMiddleware catches panics and returns 500 for Gin.
func GinRecoveryMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Errorw("panic recovered",
					"error", rec,
					"stack", string(debug.Stack()),
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
					"code":  "INTERNAL_ERROR",
				})
			}
		}()
		c.Next()
	}
}
