// aging_snapshot_repo.go — Postgres impl of AgingSnapshotRepository.
// Sprint 25 / Fase 4D.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
)

// =============================================================================
// Repository
// =============================================================================

type AgingSnapshotRepository struct {
	db *DB
}

func NewAgingSnapshotRepository(db *DB) *AgingSnapshotRepository {
	return &AgingSnapshotRepository{db: db}
}

var _ invoice.AgingSnapshotRepository = (*AgingSnapshotRepository)(nil)

// ----- UpsertAgingSnapshots -----

// UpsertAgingSnapshots atomically replaces all rows for the given run.
//
// Strategy (single transaction):
//  1. DELETE FROM aging_snapshots WHERE snapshot_run_id = $1 (idempotent)
//  2. INSERT ... (one row per snapshot in the batch)
//  3. The new rows are queryable immediately on commit
//
// We choose DELETE + INSERT (not MERGE/UPSERT) for simplicity and
// predictable cardinality: at most len(snapshots) rows after the call.
// ON CONFLICT is unnecessary because step 1 guarantees no collisions.
func (r *AgingSnapshotRepository) UpsertAgingSnapshots(ctx context.Context, runID string, snapshots []invoice.AgingSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return fmt.Errorf("upsert aging snapshots: invalid run_id %q: %w", runID, err)
	}

	return r.db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		// 1. DELETE any leftover rows from this run (idempotent re-runs)
		if _, err := pgxTx.Exec(ctx,
			`DELETE FROM aging_snapshots WHERE snapshot_run_id = $1`, runUUID,
		); err != nil {
			return fmt.Errorf("delete previous snapshots: %w", err)
		}

		// 2. Bulk INSERT (one round-trip per batch of 1000 rows for speed)
		const batchSize = 1000
		for i := 0; i < len(snapshots); i += batchSize {
			end := i + batchSize
			if end > len(snapshots) {
				end = len(snapshots)
			}
			batch := snapshots[i:end]

			// Build VALUES (...) with positional args
			values := make([]any, 0, len(batch)*6)
			placeholders := make([]byte, 0, len(batch)*40)
			for j, s := range batch {
				if j > 0 {
					placeholders = append(placeholders, ',')
				}
				placeholders = append(placeholders, []byte(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d)",
					j*7+1, j*7+2, j*7+3, j*7+4, j*7+5, j*7+6, j*7+7))...)
				tid, terr := uuid.Parse(s.TenantID)
				if terr != nil {
					return fmt.Errorf("invalid tenant_id in snapshot: %w", terr)
				}
				cid, cerr := uuid.Parse(s.CustomerID)
				if cerr != nil {
					return fmt.Errorf("invalid customer_id in snapshot: %w", cerr)
				}
				values = append(values,
					tid, cid, string(s.Bucket),
					s.Count, s.OutstandingMinor,
					s.SnapshotAt, runUUID,
				)
			}

			q := "INSERT INTO aging_snapshots (tenant_id, customer_id, bucket, invoice_count, outstanding_minor, snapshot_at, snapshot_run_id) VALUES " + string(placeholders)
			if _, err := pgxTx.Exec(ctx, q, values...); err != nil {
				return fmt.Errorf("bulk insert aging snapshots: %w", err)
			}
		}
		return nil
	})
}

// ----- GetAgingSnapshot -----

func (r *AgingSnapshotRepository) GetAgingSnapshot(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, error) {
	var (
		q    string
		args []any
	)
	if customerID != "" {
		q = `
SELECT bucket, invoice_count, outstanding_minor
FROM aging_snapshots
WHERE tenant_id = $1 AND customer_id = $2
ORDER BY bucket
`
		args = []any{tenantID, customerID}
	} else {
		q = `
SELECT bucket, invoice_count, outstanding_minor
FROM aging_snapshots
WHERE tenant_id = $1
ORDER BY bucket
`
		args = []any{tenantID}
	}

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get aging snapshot: %w", err)
	}
	defer rows.Close()

	out := make([]invoice.AgingSummary, 0, 6)
	for rows.Next() {
		var s invoice.AgingSummary
		if err := rows.Scan(&s.Bucket, &s.Count, &s.OutstandingMinor); err != nil {
			return nil, fmt.Errorf("scan aging snapshot: %w", err)
		}
		s.TenantID = tenantID
		s.CustomerID = customerID
		out = append(out, s)
	}
	return out, rows.Err()
}

// ----- ListCustomersWithOutstanding -----

func (r *AgingSnapshotRepository) ListCustomersWithOutstanding(ctx context.Context) ([]invoice.CustomerRef, error) {
	const q = `
SELECT DISTINCT tenant_id, customer_id
FROM invoices
WHERE status IN ('open', 'partial', 'overdue')
ORDER BY tenant_id, customer_id
`
	rows, err := r.db.Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list customers with outstanding: %w", err)
	}
	defer rows.Close()

	out := make([]invoice.CustomerRef, 0, 64)
	for rows.Next() {
		var c invoice.CustomerRef
		var tid, cid uuid.UUID
		if err := rows.Scan(&tid, &cid); err != nil {
			return nil, fmt.Errorf("scan customer ref: %w", err)
		}
		c.TenantID = tid.String()
		c.CustomerID = cid.String()
		out = append(out, c)
	}
	return out, rows.Err()
}

// ----- StartRun / FinishRun -----

func (r *AgingSnapshotRepository) StartRun(ctx context.Context) (string, error) {
	const q = `INSERT INTO aging_snapshot_runs DEFAULT VALUES RETURNING id`
	var id uuid.UUID
	if err := r.db.Pool.QueryRow(ctx, q).Scan(&id); err != nil {
		return "", fmt.Errorf("start aging run: %w", err)
	}
	return id.String(), nil
}

func (r *AgingSnapshotRepository) FinishRun(
	ctx context.Context, runID string, status invoice.RunStatus,
	tenants, customers, rows int, durationMs int64, errMsg string,
) error {
	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return fmt.Errorf("finish aging run: invalid run_id %q: %w", runID, err)
	}
	const q = `
UPDATE aging_snapshot_runs
SET finished_at       = now(),
    tenants_processed = $2,
    customers_processed = $3,
    rows_written      = $4,
    duration_ms       = $5,
    status            = $6,
    error             = NULLIF($7, '')
WHERE id = $1
`
	_, err = r.db.Pool.Exec(ctx, q,
		runUUID, tenants, customers, rows, durationMs,
		string(status), errMsg,
	)
	if err != nil {
		return fmt.Errorf("finish aging run: %w", err)
	}
	return nil
}

// ----- LatestRun -----

func (r *AgingSnapshotRepository) LatestRun(ctx context.Context) (*invoice.SnapshotRun, error) {
	const q = `
SELECT id, started_at, finished_at, tenants_processed, customers_processed,
       rows_written, duration_ms, status, COALESCE(error, '')
FROM aging_snapshot_runs
ORDER BY started_at DESC
LIMIT 1
`
	var (
		run         invoice.SnapshotRun
		finishedAt  *time.Time
		status      string
	)
	var id uuid.UUID
	if err := r.db.Pool.QueryRow(ctx, q).Scan(
		&id, &run.StartedAt, &finishedAt,
		&run.TenantsProcessed, &run.CustomersProcessed,
		&run.RowsWritten, &run.DurationMs,
		&status, &run.Error,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("latest aging run: %w", err)
	}
	run.ID = id.String()
	run.FinishedAt = finishedAt
	run.Status = invoice.RunStatus(status)
	return &run, nil
}
