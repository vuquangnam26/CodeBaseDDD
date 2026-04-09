package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/namcuongq/order-service/internal/application/dto"
	"github.com/namcuongq/order-service/internal/domain/order"
)

// mapAndWriteError converts domain/application errors to standardized HTTP responses.
func mapAndWriteError(w http.ResponseWriter, r *http.Request, err error) {
	code, status, msg := mapError(err)
	writeError(w, r, status, code, msg, nil)
}

func mapError(err error) (code string, status int, msg string) {
	switch {
	case errors.Is(err, order.ErrOrderNotFound):
		return "ORDER_NOT_FOUND", http.StatusNotFound, err.Error()
	case errors.Is(err, order.ErrItemNotFound):
		return "ITEM_NOT_FOUND", http.StatusNotFound, err.Error()
	case errors.Is(err, order.ErrOrderNotModifiable):
		return "ORDER_NOT_MODIFIABLE", http.StatusConflict, err.Error()
	case errors.Is(err, order.ErrNoItems):
		return "NO_ITEMS", http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, order.ErrInvalidQuantity):
		return "INVALID_QUANTITY", http.StatusBadRequest, err.Error()
	case errors.Is(err, order.ErrInvalidPrice):
		return "INVALID_PRICE", http.StatusBadRequest, err.Error()
	case errors.Is(err, order.ErrConcurrencyConflict):
		return "CONCURRENCY_CONFLICT", http.StatusConflict, err.Error()
	case errors.Is(err, order.ErrInvalidCustomerID):
		return "INVALID_CUSTOMER_ID", http.StatusBadRequest, err.Error()
	default:
		return "INTERNAL_ERROR", http.StatusInternalServerError, "An unexpected error occurred"
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, msg string, details interface{}) {
	traceID := ""
	if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(dto.ErrorResponse{
		Code:    code,
		Message: msg,
		Details: details,
		TraceID: traceID,
	})
}
