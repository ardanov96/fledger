package errors

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("conflict")
	ErrValidationFailed   = errors.New("validation failed")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInternal           = errors.New("internal error")
	ErrUpstreamUnavailable = errors.New("upstream unavailable")
	ErrSignatureInvalid   = errors.New("signature invalid")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
)

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

func (e *AppError) Unwrap() error { return e.Cause }

func New(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func Wrap(status int, code, message string, cause error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Cause: cause}
}

func (e *AppError) WithDetail(d map[string]any) *AppError {
	e.Detail = d
	return e
}

var (
	ErrInvalidInputBadRequest  = New(http.StatusBadRequest, "invalid_input", "request payload is invalid")
	ErrNotFoundResource        = New(http.StatusNotFound, "not_found", "resource not found")
	ErrUnauthorizedMissing     = New(http.StatusUnauthorized, "unauthorized", "missing or invalid Authorization header")
	ErrForbiddenAction         = New(http.StatusForbidden, "forbidden", "action not permitted")
	ErrInternalDefault         = New(http.StatusInternalServerError, "internal_error", "internal server error")
	ErrUpstreamUnavailable503  = New(http.StatusServiceUnavailable, "upstream_unavailable", "Fledger Core unavailable; request queued for retry")
	ErrSignatureInvalid401     = New(http.StatusUnauthorized, "signature_invalid", "HMAC signature verification failed")
	ErrIdempotencyConflict409  = New(http.StatusConflict, "idempotency_conflict", "idempotency key reused with different payload")
)