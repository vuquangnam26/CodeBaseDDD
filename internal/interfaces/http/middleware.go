package http

import (
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/himmel/order-service/internal/infrastructure/observability"
	"github.com/himmel/order-service/internal/infrastructure/persistence"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

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
		c.Next()

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
		reqPath := c.Request.URL.Path
		resStatus := c.Writer.Status()
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		reqCtx := c.Request.Context()

		// Log to console
		logger.Infow("http request",
			"method", reqMethod,
			"path", reqPath,
			"status", resStatus,
			"duration_ms", duration.Milliseconds(),
			"correlation_id", correlationID,
			"trace_id", traceID,
			"span_id", spanID,
			"client_ip", clientIP,
			"user_agent", userAgent,
		)

		// Save to database asynchronously if db is provided
		if db != nil {
			go func() {
				err := persistence.SaveHTTPLog(db, reqCtx, reqMethod, reqPath,
					resStatus, duration, correlationID, traceID, spanID, clientIP, userAgent)
				if err != nil {
					logger.Errorw("failed to save http log to database", "error", err, "method", reqMethod, "path", reqPath)
				} else {
					logger.Infow("successfully saved http log to database", "method", reqMethod, "path", reqPath)
				}
			}()
		}
	}
}

// GinMetricsMiddleware records HTTP request duration for Gin.
func GinMetricsMiddleware(histogram *prometheus.HistogramVec) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start).Seconds()
		histogram.WithLabelValues(
			c.Request.Method,
			c.Request.URL.Path,
			strconv.Itoa(c.Writer.Status()),
		).Observe(duration)
	}
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
