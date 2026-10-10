package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/fledger/fledger-dunning/internal/platform/errors"
	"github.com/fledger/fledger-dunning/internal/domain"
)

type QueueRepo struct {
	pool *pgxpool.Pool
}

func NewQueueRepo(p *pgxpool.Pool) *QueueRepo { return &QueueRepo{pool: p} }

const qColumns = `id, tenant_id, invoice_id, invoice_number, store_id, phone_number,
	stage, due_date, amount_due_minor, payment_link_url, message_body, status,
	scheduled_at, sent_at, COALESCE(failure_reason,''), retry_count, created_at, updated_at`

func scanQueue(row pgx.Row) (domain.QueueItem, error) {
	var q domain.QueueItem
	var stage, status string
	if err := row.Scan(
		&q.ID, &q.TenantID, &q.InvoiceID, &q.InvoiceNumber, &q.StoreID, &q.PhoneNumber,
		&stage, &q.DueDate, &q.AmountDueMinor, &q.PaymentLinkURL, &q.MessageBody, &status,
		&q.ScheduledAt, &q.SentAt, &q.FailureReason, &q.RetryCount, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		return domain.QueueItem{}, err
	}
	q.Stage = domain.DunningStage(stage)
	q.Status = domain.DunningStatus(status)
	return q, nil
}

// Insert creates a new queue row. Idempotent via the (tenant_id, invoice_id,
// stage) UNIQUE constraint.
func (r *QueueRepo) Insert(ctx context.Context, q domain.QueueItem) (domain.QueueItem, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO dunning_queues
		  (tenant_id, invoice_id, invoice_number, store_id, phone_number, stage,
		   due_date, amount_due_minor, payment_link_url, message_body, status, scheduled_at)
		VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,'QUEUED',$11)
		ON CONFLICT (tenant_id, invoice_id, stage) DO UPDATE
		  SET message_body = EXCLUDED.message_body,
		      scheduled_at = EXCLUDED.scheduled_at,
		      status = 'QUEUED',
		      updated_at = NOW()
		RETURNING `+qColumns,
		q.TenantID, q.InvoiceID, q.InvoiceNumber, q.StoreID, q.PhoneNumber,
		string(q.Stage), q.DueDate, q.AmountDueMinor, q.PaymentLinkURL, q.MessageBody,
		q.ScheduledAt,
	)
	out, err := scanQueue(row)
	if err != nil {
		return domain.QueueItem{}, fmt.Errorf("insert queue: %w", err)
	}
	return out, nil
}

// ListByTenant returns rows optionally filtered by status.
func (r *QueueRepo) ListByTenant(ctx context.Context, tenantID, status string, limit int) ([]domain.QueueItem, error) {
	if limit <= 0 {
		limit = 100
	}
	var (
		rows pgx.Rows
		err error
	)
	if status == "" {
		rows, err = r.pool.Query(ctx, `SELECT `+qColumns+` FROM dunning_queues WHERE tenant_id = $1::uuid ORDER BY scheduled_at DESC LIMIT $2`, tenantID, limit)
	} else {
		rows, err = r.pool.Query(ctx, `SELECT `+qColumns+` FROM dunning_queues WHERE tenant_id = $1::uuid AND status = $2 ORDER BY scheduled_at DESC LIMIT $3`, tenantID, status, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.QueueItem, 0, 8)
	for rows.Next() {
		v, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// FetchDue atomically selects rows that are QUEUED with scheduled_at <= now
// and flips them to PROCESSING. Other workers using FOR UPDATE SKIP LOCKED
// will not see them.
func (r *QueueRepo) FetchDue(ctx context.Context, limit int) ([]domain.QueueItem, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT `+qColumns+`
		  FROM dunning_queues
		 WHERE status = 'QUEUED' AND scheduled_at <= NOW()
		 ORDER BY scheduled_at
		 LIMIT $1
		   FOR UPDATE SKIP LOCKED`, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch due: %w", err)
	}
	defer rows.Close()
	out := make([]domain.QueueItem, 0, limit)
	ids := make([]string, 0, limit)
	for rows.Next() {
		q, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
		ids = append(ids, q.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE dunning_queues SET status='PROCESSING', retry_count=retry_count+1, updated_at=NOW() WHERE id = ANY($1::uuid[])`,
			ids); err != nil {
			return nil, fmt.Errorf("mark processing: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

// MarkSent promotes a row to SENT.
func (r *QueueRepo) MarkSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE dunning_queues SET status='SENT', sent_at=NOW(), updated_at=NOW() WHERE id=$1::uuid`, id)
	return err
}

// MarkFailed schedules a retry; flips to FAILED when retry_count >= max.
func (r *QueueRepo) MarkFailed(ctx context.Context, id, errMsg string, nextRetry time.Duration) error {
	nextAt := time.Now().UTC().Add(nextRetry)
	_, err := r.pool.Exec(ctx, `
		UPDATE dunning_queues
		   SET status = CASE
		           WHEN retry_count >= max_retries THEN 'FAILED'::varchar
		           ELSE 'QUEUED'::varchar
		       END,
		       failure_reason = $2,
		       scheduled_at = $3::timestamptz,
		       updated_at = NOW()
		 WHERE id = $1::uuid`, id, errMsg, nextAt)
	return err
}

// CancelByInvoice marks every QUEUED row for an invoice as CANCELLED_BY_PAYMENT
// inside a single transaction. Returns the number of rows cancelled.
func (r *QueueRepo) CancelByInvoice(ctx context.Context, tenantID, invoiceID string) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `
		UPDATE dunning_queues
		   SET status = 'CANCELLED_BY_PAYMENT', updated_at = NOW()
		 WHERE tenant_id = $1::uuid AND invoice_id = $2 AND status = 'QUEUED'`,
		tenantID, invoiceID)
	if err != nil {
		return 0, fmt.Errorf("cancel: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// GetByID returns one row.
func (r *QueueRepo) GetByID(ctx context.Context, tenantID, id string) (domain.QueueItem, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+qColumns+` FROM dunning_queues WHERE tenant_id = $1::uuid AND id = $2::uuid`, tenantID, id)
	q, err := scanQueue(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.QueueItem{}, fmt.Errorf("%w: queue %s", apperrors.ErrNotFound, id)
		}
		return domain.QueueItem{}, err
	}
	return q, nil
}

// CountByStatus returns dashboard counts.
type QueueCounts struct {
	Queued    int `json:"queued"`
	Sent      int `json:"sent"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

func (r *QueueRepo) CountByStatus(ctx context.Context, tenantID string) (QueueCounts, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status='QUEUED'),
		  COUNT(*) FILTER (WHERE status='SENT'),
		  COUNT(*) FILTER (WHERE status='FAILED'),
		  COUNT(*) FILTER (WHERE status='CANCELLED_BY_PAYMENT')
		  FROM dunning_queues WHERE tenant_id = $1::uuid`, tenantID)
	var c QueueCounts
	if err := row.Scan(&c.Queued, &c.Sent, &c.Failed, &c.Cancelled); err != nil {
		return QueueCounts{}, err
	}
	return c, nil
}