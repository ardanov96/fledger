package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-fleet/internal/domain/outbox"
)

// OutboxRepo persists durable outbox rows used to push POD-driven invoice
// events to Fledger Core (Sprint 4 / deliverable 4.4).
type OutboxRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepo(p *pgxpool.Pool) *OutboxRepo { return &OutboxRepo{pool: p} }

const outboxColumns = `id, tenant_id, aggregate_type, aggregate_id, event_type,
	subject, payload, status, attempts, COALESCE(last_error,''), next_attempt_at,
	sent_at, created_at, updated_at`

func scanOutbox(row pgx.Row) (outbox.Event, error) {
	var e outbox.Event
	var status string
	if err := row.Scan(
		&e.ID, &e.TenantID, &e.AggregateType, &e.AggregateID, &e.EventType,
		&e.Subject, &e.Payload, &status, &e.Attempts, &e.LastError,
		&e.NextAttemptAt, &e.SentAt, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return outbox.Event{}, err
	}
	e.Status = outbox.Status(status)
	return e, nil
}

// Append inserts a PENDING outbox row.
func (r *OutboxRepo) Append(ctx context.Context, e outbox.Event) (outbox.Event, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO fleet_outbox
		  (tenant_id, aggregate_type, aggregate_id, event_type, subject, payload, status, next_attempt_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'PENDING', NOW())
		RETURNING `+outboxColumns,
		e.TenantID, e.AggregateType, e.AggregateID, e.EventType, e.Subject, e.Payload,
	)
	return scanOutbox(row)
}

// FetchDue selects the next batch of due rows with FOR UPDATE SKIP LOCKED,
// so multiple workers can drain concurrently without colliding.
func (r *OutboxRepo) FetchDue(ctx context.Context, limit int) ([]outbox.Event, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT `+outboxColumns+`
		  FROM fleet_outbox
		 WHERE status = 'PENDING' AND next_attempt_at <= NOW()
		 ORDER BY next_attempt_at
		 LIMIT $1
		   FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch due: %w", err)
	}
	defer rows.Close()
	out := make([]outbox.Event, 0, limit)
	ids := make([]string, 0, limit)
	for rows.Next() {
		e, err := scanOutbox(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
		ids = append(ids, e.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Mark as in-flight by bumping attempts (status stays PENDING; the worker
	// decides next-attempt backoff). This also locks it via SKIP LOCKED above.
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE fleet_outbox SET attempts = attempts + 1, updated_at = NOW() WHERE id = ANY($1::uuid[])`,
			ids,
		); err != nil {
			return nil, fmt.Errorf("bump attempts: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

// MarkSent promotes a row to SENT with sent_at.
func (r *OutboxRepo) MarkSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE fleet_outbox SET status='SENT', sent_at=NOW(), updated_at=NOW() WHERE id=$1`,
		id,
	)
	return err
}

// MarkFailed keeps the row PENDING but schedules the next attempt. If maxAttempts
// is reached the row is flipped to FAILED.
func (r *OutboxRepo) MarkFailed(ctx context.Context, id, errMsg string, attempts, maxAttempts int, backoff time.Duration) error {
	if attempts >= maxAttempts {
		_, err := r.pool.Exec(ctx,
			`UPDATE fleet_outbox SET status='FAILED', last_error=$2, updated_at=NOW() WHERE id=$1`,
			id, errMsg,
		)
		return err
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE fleet_outbox SET last_error=$2, next_attempt_at=NOW() + $3::interval, updated_at=NOW() WHERE id=$1`,
		id, errMsg, fmt.Sprintf("%d milliseconds", backoff.Milliseconds()),
	)
	return err
}

// Count returns rows grouped by status — handy for ops dashboards.
type OutboxCounts struct {
	Pending int `json:"pending"`
	Sent    int `json:"sent"`
	Failed  int `json:"failed"`
}

func (r *OutboxRepo) Count(ctx context.Context, tenantID string) (OutboxCounts, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='PENDING'),
			COUNT(*) FILTER (WHERE status='SENT'),
			COUNT(*) FILTER (WHERE status='FAILED')
		  FROM fleet_outbox WHERE tenant_id = $1`, tenantID)
	var c OutboxCounts
	if err := row.Scan(&c.Pending, &c.Sent, &c.Failed); err != nil {
		return OutboxCounts{}, err
	}
	return c, nil
}