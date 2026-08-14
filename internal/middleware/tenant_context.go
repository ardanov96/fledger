// Package middleware — TenantContextMiddleware (Sprint 15 / Fase 2B + 5A).
//
// Reads the authenticated Principal from the request context (set by
// RequireAuth / AuthMiddleware) and exposes the tenant_id + user_id +
// role as a typed *tenantctx.Info value via the SHARED key in
// internal/platform/tenantctx. The use case layer reads the same value
// via tenantctx.InfoFromContext — so values set in the HTTP layer round-
// trip cleanly without an import cycle.
//
// Why a middleware instead of reading Principal directly in each use case?
//
//  1. Centralizes JWT-claim → RLS-GUC mapping in one place.
//  2. Lets the use case layer receive ctx containing *Info and pass it
//     into SetTenantContext at the start of each transaction.
//  3. Testable in isolation: feed in any Principal, assert middleware
//     correctly parses UUIDs and assigns role flags.
//
// Pattern:
//
//	router.Group(func(r chi.Router) {
//	    r.Use(RequireAuth(verifier))            // ensures Principal in ctx
//	    r.Use(TenantContextMiddleware())       // derives *tenantctx.Info
//	    r.Post("/v1/...", createHandler)
//	})
//
// And in the use case service (at top of every transaction closure):
//
//	err := s.db.ExecuteTx(ctx, func(tx ledger.Tx) error {
//	    if info := tenantctx.InfoFromContext(ctx); info != nil {
//	        if err := tenantctx.SetTenantContext(ctx, tx, info); err != nil {
//	            return err
//	        }
//	    }
//	    // ... normal repo calls now subject to RLS
//	})
package middleware

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
)

// TenantContextMiddleware reads the Principal and pushes a *tenantctx.Info
// into the request context. Must run AFTER RequireAuth / AuthMiddleware
// (otherwise Principal is nil and the request is short-circuited upstream).
//
// Returns 500 if the Principal's tenant_id / user_id is missing or
// malformed (UUID parse error) — this is a defensive failure mode because
// a token with a non-UUID tenant_id cannot be supported by our schema.
func TenantContextMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := PrincipalFromContext(r.Context())
			if p == nil {
				// Should not happen if RequireAuth ran first; fail loud.
				http.Error(w, "tenant middleware: no principal in context", http.StatusInternalServerError)
				return
			}

			info, err := infoFromPrincipal(p)
			if err != nil {
				http.Error(w, "tenant middleware: "+err.Error(), http.StatusInternalServerError)
				return
			}

			ctx := tenantctx.WithInfo(r.Context(), info)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantFromContext returns the *tenantctx.Info set by TenantContextMiddleware.
// Thin wrapper around tenantctx.InfoFromContext for callers that prefer
// the middleware-style name. Returns nil if no tenant context is present.
func TenantFromContext(r *http.Request) *tenantctx.Info {
	return tenantctx.InfoFromContext(r.Context())
}

// infoFromPrincipal converts a Principal to *tenantctx.Info.
// Returns an error if tenant_id or user_id is missing or not a valid UUID.
func infoFromPrincipal(p *Principal) (*tenantctx.Info, error) {
	if p.TenantID == "" {
		return nil, errors.New("principal missing tenant_id")
	}
	tenantID, err := uuid.Parse(p.TenantID)
	if err != nil {
		return nil, errors.New("principal tenant_id is not a valid UUID")
	}
	// UserID may be empty for service-account tokens — treat as zero-UUID
	// rather than failing. RLS policy still gets a value (zero UUID will
	// just never match any real user_id).
	userID := uuid.Nil
	if p.UserID != "" {
		parsed, err := uuid.Parse(p.UserID)
		if err != nil {
			return nil, errors.New("principal user_id is not a valid UUID")
		}
		userID = parsed
	}
	return &tenantctx.Info{
		TenantID:   tenantID,
		UserID:     userID,
		IsSalesRep: p.Role == "sales_rep",
	}, nil
}
