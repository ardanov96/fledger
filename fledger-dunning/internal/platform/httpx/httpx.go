// Package httpx is the shared HTTP plumbing for Fledger Dunning.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		writeAppErr(w, appErr)
		return
	}
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		writeAppErr(w, apperrors.ErrInvalidInputBadRequest)
	case errors.Is(err, apperrors.ErrNotFound):
		writeAppErr(w, apperrors.ErrNotFoundResource)
	case errors.Is(err, apperrors.ErrConflict), errors.Is(err, apperrors.ErrIdempotencyConflict):
		writeAppErr(w, apperrors.ErrIdempotencyConflict409)
	case errors.Is(err, apperrors.ErrUnauthorized):
		writeAppErr(w, apperrors.ErrUnauthorizedMissing)
	case errors.Is(err, apperrors.ErrForbidden):
		writeAppErr(w, apperrors.ErrForbiddenAction)
	case errors.Is(err, apperrors.ErrUpstreamUnavailable):
		writeAppErr(w, apperrors.ErrUpstreamUnavailable503)
	case errors.Is(err, apperrors.ErrWebhookInvalid):
		writeAppErr(w, apperrors.ErrWebhookSignatureInvalid)
	default:
		writeAppErr(w, apperrors.ErrInternalDefault)
	}
}

func ErrorWithDetails(w http.ResponseWriter, r *http.Request, err error, detail map[string]any) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		appErr.WithDetail(detail)
		writeAppErr(w, appErr)
		return
	}
	base := apperrors.ErrInternalDefault
	if errors.Is(err, apperrors.ErrInvalidInput) {
		base = apperrors.ErrInvalidInputBadRequest
	}
	writeAppErr(w, base.WithDetail(detail))
}

func writeAppErr(w http.ResponseWriter, e *apperrors.AppError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(e)
}

func Logger(log *slog.Logger) func(next http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"remote", r.RemoteAddr,
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// EnvelopeOK is the standard success envelope used by all handlers.
type EnvelopeOK struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
}

// EnvelopeMessage is a status-only success envelope.
type EnvelopeMessage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Count   int    `json:"count,omitempty"`
}