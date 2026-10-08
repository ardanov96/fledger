// Package httpx provides HTTP transport-layer helpers: response envelopes,
// error mapping, and recovery middleware.
//
// All API responses use a consistent envelope:
//
//	Success: { "data": ..., "meta": { "request_id": "...", "timestamp": "..." } }
//	Error:   { "error": { "code": "...", "message": "..." }, "meta": {...} }
//
// This makes the API predictable for clients and easier to test.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// Response is the standard success envelope.
type Response struct {
	Data any    `json:"data"`
	Meta *Meta  `json:"meta,omitempty"`
}

// ErrorBody is the error envelope.
type ErrorBody struct {
	Error *ErrorDetail `json:"error"`
	Meta  *Meta        `json:"meta,omitempty"`
}

// ErrorDetail contains the structured error info for clients.
type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Meta contains request-level metadata (request_id, timestamp, pagination).
type Meta struct {
	RequestID    string `json:"request_id,omitempty"`
	TraceID      string `json:"trace_id,omitempty"`
	Timestamp    string `json:"timestamp"`
	ServerVersion string `json:"server_version,omitempty"`
}

// JSON writes a JSON response with the given status code and payload.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Data: data})
}

// JSONWithMeta writes a JSON response with metadata.
func JSONWithMeta(w http.ResponseWriter, status int, data any, meta *Meta) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Data: data, Meta: meta})
}

// Error writes a structured error response. It maps the error to an HTTP
// status code using the typed error catalog.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	ae := apperrors.AsAppError(err)

	status := ae.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}

	// In production, do NOT leak wrapped error details to the client.
	// Log the full error server-side.
	if status >= 500 {
		log := slog.Default()
		log.Error("internal server error",
			"error", err.Error(),
			"path", r.URL.Path,
			"method", r.Method,
			"request_id", GetRequestID(r.Context()),
		)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{
		Error: &ErrorDetail{
			Code:    ae.Code,
			Message: ae.Message,
		},
		Meta: &Meta{
			RequestID: GetRequestID(r.Context()),
			TraceID:   GetTraceID(r.Context()),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}

// ErrorWithDetails includes a details map (e.g. validation errors).
func ErrorWithDetails(w http.ResponseWriter, r *http.Request, err error, details map[string]any) {
	ae := apperrors.AsAppError(err)
	status := ae.HTTPStatus
	if status == 0 {
		status = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{
		Error: &ErrorDetail{
			Code:    ae.Code,
			Message: ae.Message,
			Details: details,
		},
		Meta: &Meta{
			RequestID: GetRequestID(r.Context()),
			TraceID:   GetTraceID(r.Context()),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		},
	})
}

// NewMeta creates a Meta for a request.
func NewMeta(requestID, traceID, version string) *Meta {
	return &Meta{
		RequestID:     requestID,
		TraceID:       traceID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		ServerVersion: version,
	}
}

// Sentinel for graceful 404
var ErrNotFoundHTTP = errors.New("http: not found")
