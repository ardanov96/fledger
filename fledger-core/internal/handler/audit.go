// Audit HTTP handler.
//
// Routes (mounted under /v1):
//   GET /v1/audit                     — list recent audit entries (Principal tenant scope)
//   GET /v1/audit/guc-binds           — list forensic GUC bind events (admin/finance read)
//   POST /v1/accounts/{id}/freeze     — freeze an account
//   POST /v1/accounts/{id}/close      — close an account
//
// Auth: every endpoint below is wrapped by RequirePermission middleware
// (RBAC) — tenant_id is extracted from the Principal (JWT) instead of
// trusting any X-Tenant-ID header. The previous X-Tenant-ID fallback
// (Sprint 5 scaffold) was replaced in Sprint 23 (22B hardening) so we
// no longer trust client-supplied headers — defense-in-depth.
package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/runut/fmcg-wallet/internal/domain/audit"
	"github.com/runut/fmcg-wallet/internal/middleware"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// AuditRepositoryExtension lets the handler query forensic tables beyond
// audit_logs (e.g. guc_bind_audit). Implemented by the postgres adapter.
type AuditRepositoryExtension interface {
	ListGUCBinds(ctx context.Context, tenantID string, sinceRFC3339 string, limit int) ([]GUCBindEntry, error)
}

// GUCBindEntry is one row from guc_bind_audit (mirrors the migration 000017 schema).
type GUCBindEntry struct {
	ID          int64     `json:"id"`
	TenantID    string    `json:"tenant_id"`
	UserID      string    `json:"user_id"`
	IsSalesRep  bool      `json:"is_sales_rep"`
	RequestID   string    `json:"request_id,omitempty"`
	BoundAt     string    `json:"bound_at"`
}

// =============================================================================
// Audit handler
// =============================================================================

// AuditHandlers groups audit-related routes.
type AuditHandlers struct {
	Repo audit.Repository
	// Forensic (optional — if nil, guc-bind endpoint returns 503).
	GUCRepo AuditRepositoryExtension
}

// RegisterAuditRoutes mounts the audit endpoints on the given router.
func (h *AuditHandlers) RegisterAuditRoutes(r chi.Router) {
	r.Get("/audit", h.ListAudit)
	r.Get("/audit/guc-binds", h.ListGUCBinds)
}

// ListAudit handles GET /v1/audit?limit=50
//
// Sprint 23 (hardening): tenant_id is now sourced from the JWT Principal
// via middleware.RequirePermission — never from the client-supplied
// X-Tenant-ID header (the previous fallback allowed impersonation).
func (h *AuditHandlers) ListAudit(w http.ResponseWriter, r *http.Request) {
	principal := middleware.PrincipalFromContext(r.Context())
	if principal == nil || principal.TenantID == "" {
		httpx.Error(w, r, apperrors.ErrInvalidInput)
		return
	}
	tenantID := principal.TenantID

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	entries, err := h.Repo.List(r.Context(), tenantID, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	httpx.JSON(w, http.StatusOK, entries)
}

// ListGUCBinds handles GET /v1/audit/guc-binds?limit=50&since=<RFC3339>
//
// Sprint 23 (22B.5): returns the forensic trail of Postgres GUC bind
// events recorded by tenantctx.SetTenantContext (see migration 000017).
// Operators investigate "who accessed this tenant at 14:32 UTC?" by
// filtering on request_id / user_id / tenant_id in psql or Grafana.
//
// RBAC: requires audit_log read (already enforced by mount in buildRouter).
// Returns 503 if GUCRepo is not wired (e.g. development scaffolding
// without Postgres backend).
func (h *AuditHandlers) ListGUCBinds(w http.ResponseWriter, r *http.Request) {
	if h.GUCRepo == nil {
		httpx.Error(w, r, apperrors.New("GUC_AUDIT_UNAVAILABLE", "guc bind audit not wired", 503))
		return
	}
	principal := middleware.PrincipalFromContext(r.Context())
	if principal == nil || principal.TenantID == "" {
		httpx.Error(w, r, apperrors.ErrInvalidInput)
		return
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	since := r.URL.Query().Get("since")

	rows, err := h.GUCRepo.ListGUCBinds(r.Context(), principal.TenantID, since, limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if rows == nil {
		rows = []GUCBindEntry{}
	}
	httpx.JSON(w, http.StatusOK, rows)
}

// =============================================================================
// Account state-change handlers
// =============================================================================

// FreezeAccountHandler freezes an account.
func FreezeAccountHandler(svc interface {
	Freeze(ctx context.Context, id, reason string) error
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			httpx.Error(w, r, apperrors.ErrInvalidInput)
			return
		}
		if err := svc.Freeze(r.Context(), id, ""); err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "frozen", "id": id})
	}
}

// CloseAccountHandler closes an account.
func CloseAccountHandler(svc interface {
	Close(ctx context.Context, id, reason string) error
}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			httpx.Error(w, r, apperrors.ErrInvalidInput)
			return
		}
		if err := svc.Close(r.Context(), id, ""); err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "closed", "id": id})
	}
}
