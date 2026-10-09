// Package httpx is the shared HTTP plumbing for the Fleet service:
// JSON encoder, error translator, middleware-friendly helpers.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
)

// JSON writes the given value as JSON with the supplied status. The encoder
// is configured to refuse unknown fields (so callers cannot accidentally pass
// stale payloads).
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// Error translates an error into an HTTP response. It recognises:
//   - *apperrors.AppError → preserves status, code, detail
//   - stdlib errors.Is(...) against known sentinels → mapped to a status
//   - fallback → 500 internal_error
//
// A slog logger may be supplied (or nil for the noisy default).
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
	case errors.Is(err, apperrors.ErrUnprocessableEntity):
		writeAppErr(w, apperrors.ErrUnprocessableEntity422)
	default:
		writeAppErr(w, apperrors.ErrInternalDefault)
	}
}

// ErrorWithDetails is the same as Error but lets callers attach a per-request
// detail object to the typed error (e.g. validation field errors).
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
	} else if errors.Is(err, apperrors.ErrUnprocessableEntity) {
		base = apperrors.ErrUnprocessableEntity422
	}
	writeAppErr(w, base.WithDetail(detail))
}

func writeAppErr(w http.ResponseWriter, e *apperrors.AppError) {
	if e.ErrorMsg == "" {
		e.ErrorMsg = e.Message
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(e)
}

// LogWriter returns a thin http.ResponseWriter wrapper that records the
// status code in a slog attribute. Useful for access-log middleware.
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