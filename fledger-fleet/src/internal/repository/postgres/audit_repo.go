package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditRepo persists status-history rows for the audit trail (§7.3).
//
// `actor_id` is a free-form string (user UUID, driver UUID, or a service
// identifier like "service:fledger-fleet") — column is TEXT, not UUID, so the
// repository does not validate the value.
type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(p *pgxpool.Pool) *AuditRepo { return &AuditRepo{pool: p} }

// Append inserts a single status-change row. Pass `fromStatus=""` for the
// first INSERT (DO creation).
func (r *AuditRepo) Append(ctx context.Context, tenantID, doID, fromStatus, toStatus, actorID, actorType, note string) error {
	var actorArg any
	if actorID != "" {
		actorArg = actorID
	}
	var fromArg any
	if fromStatus != "" {
		fromArg = fromStatus
	}
	var noteArg any
	if note != "" {
		noteArg = note
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO fleet_do_status_history
		  (tenant_id, do_id, from_status, to_status, actor_id, actor_type, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, doID, fromArg, toStatus, actorArg, actorType, noteArg,
	)
	if err != nil {
		return fmt.Errorf("append audit row: %w", err)
	}
	return nil
}

// SetActor updates the denormalised actor_id on the DO itself.
func (r *AuditRepo) SetActor(ctx context.Context, tenantID, doID, actorID string) error {
	var actorArg any
	if actorID != "" {
		actorArg = actorID
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE fleet_delivery_orders SET actor_id = $3, updated_at = NOW() WHERE tenant_id = $1 AND id = $2`,
		tenantID, doID, actorArg,
	)
	return err
}

// ListByDO returns the audit trail for one DO, oldest-first.
func (r *AuditRepo) ListByDO(ctx context.Context, doID string) ([]AuditRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT do_id, COALESCE(from_status,''), to_status,
		       COALESCE(actor_id,''), actor_type, COALESCE(note,''), created_at
		  FROM fleet_do_status_history
		 WHERE do_id = $1
		 ORDER BY created_at ASC`, doID)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()
	out := make([]AuditRow, 0, 4)
	for rows.Next() {
		var a AuditRow
		if err := rows.Scan(&a.DOID, &a.FromStatus, &a.ToStatus, &a.ActorID, &a.ActorType, &a.Note, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AuditRow is one row of the status-history.
type AuditRow struct {
	DOID       string
	FromStatus string
	ToStatus   string
	ActorID    string
	ActorType  string
	Note       string
	CreatedAt  interface{}
}