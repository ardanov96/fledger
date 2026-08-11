// Package middleware — AuditMiddleware logs every authenticated request
// to the audit repository. The action and resource are derived from
// the request method + path. Body inspection is intentionally NOT done
// here (would duplicate validation logic); handlers should call
// audit.Record() explicitly for sensitive operations.
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/runut/fmcg-wallet/internal/domain/audit"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// AuditMiddleware writes an audit entry for each request to the wrapped
// handler. Place it AFTER AuthMiddleware so the principal is available
// in context.
//
// Note: this middleware records every request (info-level granularity).
// For finer-grained control (only certain actions), handlers should call
// audit.Repo.Record() directly.
func AuditMiddleware(repo audit.Repository, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)

			// Skip audit for health/metrics/noise endpoints
			path := r.URL.Path
			if isHealthPath(path) {
				return
			}

			// Determine principal
			principal := PrincipalFromContext(r.Context())
			actorID := ""
			actorType := audit.ActorSystem
			tenantID := ""
			if principal != nil {
				actorID = principal.UserID
				actorType = audit.ActorUser
				tenantID = principal.TenantID
			}
			if tenantID == "" {
				tenantID = "00000000-0000-0000-0000-000000000001"
			}

			// Derive action and resource from the request
			action := audit.Action(deriveAction(r.Method, path))
			resourceType, resourceID := deriveResource(path)

			// Outcome: success unless 4xx/5xx
			outcome := audit.OutcomeSuccess
			if ww.status >= 400 {
				outcome = audit.OutcomeFailure
			}

			entry := audit.Entry{
				ID:           generateID(),
				TenantID:     tenantID,
				ActorID:      actorID,
				ActorType:    actorType,
				Action:       action,
				ResourceType: resourceType,
				ResourceID:   resourceID,
				Outcome:      outcome,
				RequestID:    httpx.GetRequestID(r.Context()),
				IPAddress:    clientIP(r),
				UserAgent:    r.UserAgent(),
				Metadata: map[string]any{
					"method":      r.Method,
					"path":        path,
					"status":      ww.status,
					"duration_ms": time.Since(start).Milliseconds(),
				},
				OccurredAt: time.Now(),
			}

			// Record async-ish (best-effort, never block the request)
			if err := repo.Record(r.Context(), entry); err != nil {
				log.Warn("audit record failed",
					"error", err,
					"action", entry.Action,
					"path", path,
				)
			}
		})
	}
}

// isHealthPath returns true for paths that should not be audited.
func isHealthPath(path string) bool {
	return path == "/healthz" || path == "/readyz" || path == "/version" || path == "/metrics"
}

// deriveAction converts (method, path) to a dotted action like
// "transfer.create" or "account.get".
func deriveAction(method, path string) string {
	// Strip /v1 prefix
	path = strings.TrimPrefix(path, "/v1")
	path = strings.TrimPrefix(path, "/")

	// Split into segments
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "root." + strings.ToLower(method)
	}
	resource := parts[0]
	verb := ""
	if len(parts) >= 2 {
		// /accounts/{id} -> "account" + verb
		verb = "get"
	} else {
		switch method {
		case http.MethodGet:
			verb = "list"
		case http.MethodPost:
			verb = "create"
		case http.MethodPut, http.MethodPatch:
			verb = "update"
		case http.MethodDelete:
			verb = "delete"
		default:
			verb = strings.ToLower(method)
		}
	}
	return resource + "." + verb
}

// deriveResource extracts resource type and ID from path.
func deriveResource(path string) (rtype, rid string) {
	path = strings.TrimPrefix(path, "/v1")
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", ""
	}
	rtype = parts[0]
	if len(parts) >= 2 {
		rid = parts[1]
	}
	return rtype, rid
}

// clientIP returns the best-guess client IP.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry in the list
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// RemoteAddr is "host:port"
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// statusRecorder wraps ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// generateID returns a short pseudo-unique id for audit entries.
// In production, use a proper UUID library.
func generateID() string {
	return httpx.GenerateID()
}
