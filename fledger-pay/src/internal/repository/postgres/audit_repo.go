package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-pay/internal/domain/audit"
)

type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(p *pgxpool.Pool) *AuditRepo { return &AuditRepo{pool: p} }

// Append writes a single audit row.
func (r *AuditRepo) Append(ctx context.Context, l audit.Log) error {
	if len(l.Details) == 0 {
		l.Details = map[string]any{}
	}
	raw, _ := json.Marshal(l.Details)
	var ipArg any
	if l.IPAddress != "" {
		ipArg = l.IPAddress
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO pay_audit_logs
		  (tenant_id, actor_id, actor_role, action, resource_type, resource_id, details, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		l.TenantID, l.ActorID, l.ActorRole, string(l.Action), l.ResourceType, l.ResourceID, raw, ipArg,
	)
	if err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}