// Audit HTTP handler.
//
// Routes (mounted under /v1):
//   GET /v1/audit                     — list recent audit entries
//   GET /v1/audit/{id}                — fetch one entry (TODO)
//   POST /v1/accounts/{id}/freeze     — freeze an account
//   POST /v1/accounts/{id}/close      — close an account
//
// Auth: in this scaffold the audit endpoint is open. In production,
// wrap it with middleware.RequireRole("admin") once JWT auth lands.
package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/runut/fmcg-wallet/internal/domain/audit"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// =============================================================================
// Audit handler
// =============================================================================

// AuditHandlers groups audit-related routes.
type AuditHandlers struct {
	Repo audit.Repository
}

// RegisterAuditRoutes mounts the audit endpoints on the given router.
func (h *AuditHandlers) RegisterAuditRoutes(r chi.Router) {
	r.Get("/audit", h.ListAudit)
}

// ListAudit handles GET /v1/audit?limit=50
func (h *AuditHandlers) ListAudit(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000001"
	}

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
