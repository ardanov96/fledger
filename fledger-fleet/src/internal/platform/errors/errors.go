// Package errors provides typed errors and helpers shared by the platform
// HTTP layer (handlers, middleware) and the domain/use-case layers.
//
// Mirrors the pattern used by fledger-core so the two remain easy to compare.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors — handlers should wrap with errors.Join to attach detail.
var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrValidationFailed   = errors.New("validation failed")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInternal           = errors.New("internal error")
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
)

// AppError is a typed error that knows its HTTP status, public code, and an
// optional detail map. It implements `error` and unwraps to the wrapped error
// so errors.Is still works.
type AppError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Detail  map[string]any `json:"detail,omitempty"`
	Cause   error          `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error so errors.Is/As work transitively.
func (e *AppError) Unwrap() error { return e.Cause }

// New constructs an AppError with the given status, code, and message.
func New(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// Wrap wraps the given cause with the same metadata.
func Wrap(status int, code, message string, cause error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Cause: cause}
}

// WithDetail attaches a detail map for client-side diagnostics.
func (e *AppError) WithDetail(d map[string]any) *AppError {
	e.Detail = d
	return e
}

// Pre-baked common errors.
var (
	ErrInvalidInputBadRequest  = New(http.StatusBadRequest, "invalid_input", "request payload is invalid")
	ErrNotFoundResource        = New(http.StatusNotFound, "not_found", "resource not found")
	ErrUnauthorizedMissing     = New(http.StatusUnauthorized, "unauthorized", "missing or invalid Authorization header")
	ErrForbiddenAction         = New(http.StatusForbidden, "forbidden", "action not permitted")
	ErrInternalDefault         = New(http.StatusInternalServerError, "internal_error", "internal server error")
	ErrUpstreamUnavailable503  = New(http.StatusServiceUnavailable, "upstream_unavailable", "Fledger Core unavailable; request queued for retry")
	ErrIdempotencyConflict409  = New(http.StatusConflict, "idempotency_conflict", "idempotency key reused with different payload")
)