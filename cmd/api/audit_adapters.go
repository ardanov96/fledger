// Package main — audit_adapters.go (Sprint 23 / 22B.5)
//
// Bridges postgres.AuditRepository (which knows about the guc_bind_audit
// table) and handler.AuditRepositoryExtension (which has the response
// shape). Decouples the persistence layer from the HTTP layer — both can
// evolve without leaking types.
package main

import (
	"context"

	"github.com/runut/fmcg-wallet/internal/handler"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
)

// auditGUCAdapter implements handler.AuditRepositoryExtension by translating
// the persistence-layer GUCBindRow struct to the HTTP-layer GUCBindEntry.
type auditGUCAdapter struct {
	repo *postgres.AuditRepository
}

func newAuditGUCAdapter(repo *postgres.AuditRepository) *auditGUCAdapter {
	return &auditGUCAdapter{repo: repo}
}

// ListGUCBinds satisfies handler.AuditRepositoryExtension.
func (a *auditGUCAdapter) ListGUCBinds(ctx context.Context, tenantID, sinceRFC3339 string, limit int) ([]handler.GUCBindEntry, error) {
	rows, err := a.repo.ListGUCBinds(ctx, tenantID, sinceRFC3339, limit)
	if err != nil {
		return nil, err
	}
	out := make([]handler.GUCBindEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, handler.GUCBindEntry{
			ID:         r.ID,
			TenantID:   r.TenantID,
			UserID:     r.UserID,
			IsSalesRep: r.IsSalesRep,
			RequestID:  r.RequestID,
			BoundAt:    r.BoundAt.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		})
	}
	return out, nil
}

// Compile-time check.
var _ handler.AuditRepositoryExtension = (*auditGUCAdapter)(nil)
