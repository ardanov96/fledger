package handler

import (
	"context"
	"net/http"

	"github.com/fledger/fledger-fleet/internal/middleware"
)

// principalFromCtx is a small wrapper used by tenantFrom so it can live in
// handler/helpers.go without creating an import cycle with middleware.
func principalFromCtx(ctx context.Context) *middleware.Principal {
	return middleware.PrincipalFromContext(ctx)
}

// actorIDFrom returns the authenticated principal's UserID (driver or sales
// rep, depending on the caller's role) for the audit-trail rows (§7.3).
// Falls back to the X-Actor-Id header so the dev login / E2E script can pass
// it through.
func actorIDFrom(r *http.Request) string {
	if p := principalFromCtx(r.Context()); p != nil && p.UserID != "" {
		return p.UserID
	}
	return r.Header.Get("X-Actor-Id")
}