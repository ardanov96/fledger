package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-order/internal/domain/outbox"
)

type OutboxRepo struct {
	pool *pgxpool.Pool
}

func NewOutboxRepo(p *pgxpool.Pool) *OutboxRepo { return &OutboxRepo{pool: p} }

const obColumns = `id, tenant_id, event_type, aggregate_id, payload, status,
	retry_count, max_retries, COALESCE(last_error,''), next_retry_at, sent_at, created_at`

func scanOB(row pgx.Row) (outbox.Event, error) {
	var e outbox.Event
	var status string
	var raw []byte
	if err := row.Scan(
		&e.ID, &e.TenantID, &e.EventType, &e.AggregateID, &raw, &status,
		&e.RetryCount, &e.MaxRetries, &e.LastError, &e.NextRetryAt, &e.SentAt, &e.CreatedAt,
	); err != nil {
		return outbox.Event{}, err
	}
	e.Status = outbox.Status(status)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &e.Payload)
	}
	return e, nil
}

// Append inserts a PENDING outbox row.
func (r *OutboxRepo) Append(ctx context.Context, e outbox.Event) (outbox.Event, error) {
	if len(e.Payload) == 0 {
		e.Payload = map[string]any{}
	}
	raw, _ := json.Marshal(e.Payload)
	if e.MaxRetries <= 0 {
		e.MaxRetries = 10
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO order_outbox
		  (tenant_id, event_type, aggregate_id, payload, status, max_retries, next_retry_at)
		VALUES ($1, $2, $3::uuid, $4, 'PENDING', $5, NOW())
		RETURNING `+obColumns,
		e.TenantID, string(e.EventType), e.AggregateID, raw, e.MaxRetries,
	)
	return scanOB(row)
}

// FetchDue returns up to `limit` due rows, locked FOR UPDATE SKIP LOCKED.
func (r *OutboxRepo) FetchDue(ctx context.Context, limit int) ([]outbox.Event, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT `+obColumns+`
		  FROM order_outbox
		 WHERE status IN ('PENDING', 'FAILED')
		   AND next_retry_at <= NOW()
		 ORDER BY next_retry_at
		 LIMIT $1
		   FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]outbox.Event, 0, limit)
	ids := make([]string, 0, limit)
	for rows.Next() {
		e, err := scanOB(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
		ids = append(ids, e.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE order_outbox SET status='PROCESSING', retry_count=retry_count+1 WHERE id = ANY($1::uuid[])`,
			ids); err != nil {
			return nil, fmt.Errorf("mark processing: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

func (r *OutboxRepo) MarkSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE order_outbox SET status='SENT', sent_at=NOW() WHERE id=$1`, id)
	return err
}

func (r *OutboxRepo) MarkFailed(ctx context.Context, id, errMsg string, nextRetry time.Duration) error {
	nextAt := time.Now().UTC().Add(nextRetry)
	row := r.pool.QueryRow(ctx, `
		UPDATE order_outbox
		   SET status = CASE
		           WHEN retry_count >= max_retries THEN 'FAILED'::varchar
		           ELSE 'PENDING'::varchar
		       END,
		       last_error = $2,
		       next_retry_at = CASE
		           WHEN retry_count >= max_retries THEN next_retry_at
		           ELSE $3::timestamptz
		       END
		 WHERE id = $1
		 RETURNING status`, id, errMsg, nextAt)
	var s string
	if err := row.Scan(&s); err != nil && err != pgx.ErrNoRows {
		return err
	}
	return nil
}

type OutboxCounts struct {
	Pending    int `json:"pending"`
	Processing int `json:"processing"`
	Sent       int `json:"sent"`
	Failed     int `json:"failed"`
}

func (r *OutboxRepo) Count(ctx context.Context, tenantID string) (OutboxCounts, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='PENDING'),
			COUNT(*) FILTER (WHERE status='PROCESSING'),
			COUNT(*) FILTER (WHERE status='SENT'),
			COUNT(*) FILTER (WHERE status='FAILED')
		  FROM order_outbox WHERE tenant_id=$1`, tenantID)
	var c OutboxCounts
	if err := row.Scan(&c.Pending, &c.Processing, &c.Sent, &c.Failed); err != nil {
		return OutboxCounts{}, err
	}
	return c, nil
}