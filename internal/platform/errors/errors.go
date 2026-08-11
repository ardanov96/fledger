// Package errors defines typed errors used across the application.
//
// Using sentinel errors allows:
//  1. Easy comparison with errors.Is()
//  2. Consistent HTTP status code mapping in middleware
//  3. Self-documenting error taxonomy
//
// Convention:
//
//	<Domain><Reason>
//
// Example: ErrInsufficientBalance, ErrAccountNotFound
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is a typed application error with an associated HTTP status and
// a stable error code for client consumption.
type AppError struct {
	Code       string // stable code, e.g. "INSUFFICIENT_BALANCE"
	Message    string // human-readable, safe to expose to client
	HTTPStatus int    // HTTP status code
	Wrapped    error  // underlying cause (optional)
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error.
func (e *AppError) Unwrap() error {
	return e.Wrapped
}

// Is compares by code, allowing errors.Is(err, ErrInsufficientBalance) to work.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// New constructs an AppError from a sentinel.
func New(code, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap wraps an underlying error with a typed AppError.
func Wrap(err error, code, message string, httpStatus int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Wrapped:    err,
	}
}

// =============================================================================
// Ledger domain
// =============================================================================

var (
	ErrInsufficientBalance = New("INSUFFICIENT_BALANCE", "Insufficient balance for this operation", http.StatusUnprocessableEntity)
	ErrAccountNotFound     = New("ACCOUNT_NOT_FOUND", "Account not found", http.StatusNotFound)
	ErrAccountFrozen       = New("ACCOUNT_FROZEN", "Account is frozen", http.StatusForbidden)
	ErrAccountClosed       = New("ACCOUNT_CLOSED", "Account is closed", http.StatusForbidden)
	ErrInvalidAmount       = New("INVALID_AMOUNT", "Invalid money amount", http.StatusBadRequest)
	ErrCurrencyMismatch    = New("CURRENCY_MISMATCH", "Currency mismatch", http.StatusBadRequest)
	ErrDoubleEntryViolation = New("DOUBLE_ENTRY_VIOLATION", "Debits must equal credits", http.StatusUnprocessableEntity)
)

// =============================================================================
// Invoice & Receivables domain
// =============================================================================

var (
	ErrInvoiceNotFound      = New("INVOICE_NOT_FOUND", "Invoice not found", http.StatusNotFound)
	ErrInvoiceAlreadyPaid   = New("INVOICE_ALREADY_PAID", "Invoice is already fully paid", http.StatusConflict)
	ErrInvoiceOverpaid      = New("INVOICE_OVERPAID", "Payment exceeds invoice balance", http.StatusUnprocessableEntity)
	ErrPaymentAllocationMismatch = New("PAYMENT_ALLOCATION_MISMATCH", "Payment allocation does not match payment amount", http.StatusUnprocessableEntity)
	ErrPeriodClosed         = New("PERIOD_CLOSED", "Accounting period is closed; entries frozen", http.StatusForbidden)
	ErrCreditLimitExceeded  = New("CREDIT_LIMIT_EXCEEDED", "Credit limit exceeded for customer", http.StatusUnprocessableEntity)
)

// =============================================================================
// Auth & Access Control
// =============================================================================

var (
	ErrUnauthorized           = New("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
	ErrForbidden              = New("FORBIDDEN", "Insufficient permissions", http.StatusForbidden)
	ErrInvalidCredentials     = New("INVALID_CREDENTIALS", "Invalid username or password", http.StatusUnauthorized)
	ErrTokenExpired           = New("TOKEN_EXPIRED", "Token has expired", http.StatusUnauthorized)
	ErrTokenInvalid           = New("TOKEN_INVALID", "Token is invalid", http.StatusUnauthorized)
	ErrSessionNotFound        = New("SESSION_NOT_FOUND", "Session not found or expired", http.StatusUnauthorized)
	ErrAccountLocked          = New("ACCOUNT_LOCKED", "Account locked due to too many failed attempts", http.StatusTooManyRequests)
)

// =============================================================================
// Idempotency & Concurrency
// =============================================================================

var (
	ErrIdempotencyKeyMissing = New("IDEMPOTENCY_KEY_MISSING", "Idempotency-Key header is required", http.StatusBadRequest)
	ErrIdempotencyConflict   = New("IDEMPOTENCY_CONFLICT", "Idempotency key reused with different payload", http.StatusConflict)
	ErrConcurrentModification = New("CONCURRENT_MODIFICATION", "Resource was modified concurrently; please retry", http.StatusConflict)
)

// =============================================================================
// Validation
// =============================================================================

var (
	ErrValidationFailed = New("VALIDATION_FAILED", "Request validation failed", http.StatusBadRequest)
	ErrInvalidInput     = New("INVALID_INPUT", "Invalid input", http.StatusBadRequest)
)

// =============================================================================
// Generic / Infrastructure
// =============================================================================

var (
	ErrInternal         = New("INTERNAL_ERROR", "Internal server error", http.StatusInternalServerError)
	ErrNotFound         = New("NOT_FOUND", "Resource not found", http.StatusNotFound)
	ErrAlreadyExists    = New("ALREADY_EXISTS", "Resource already exists", http.StatusConflict)
	ErrRateLimited      = New("RATE_LIMITED", "Too many requests", http.StatusTooManyRequests)
	ErrServiceUnavailable = New("SERVICE_UNAVAILABLE", "Service temporarily unavailable", http.StatusServiceUnavailable)
	ErrTimeout          = New("TIMEOUT", "Request timed out", http.StatusGatewayTimeout)
)

// =============================================================================
// Helpers
// =============================================================================

// AsAppError extracts an *AppError from a wrapped error chain, or returns
// a generic internal error if not found.
func AsAppError(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return ErrInternal
}
