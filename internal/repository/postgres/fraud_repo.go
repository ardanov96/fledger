// fraud_repo.go - Postgres impl of fraud.Repository (Sprint 31).
//
// Same pattern as notification_repo.go (Sprint 28/29):
//   - Create runs inside caller's tx (Exec only)
//   - Reads use RunInReadTx (tenant-scoped) or RunInAdminTx (cross-tenant
//     worker scans — needed for CountRecentByAccount + HasRecipientHistory
//     which the worker invokes with no single tenant scope).
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/fraud"
)

// =============================================================================
// Repository
// =============================================================================

type FraudFlagRepository struct {
	db *DB
}

func NewFraudFlagRepository(db *DB) *FraudFlagRepository {
	return &FraudFlagRepository{db: db}
}

var _ fraud.Repository = (*FraudFlagRepository)(nil)

// =============================================================================
// Create
// =============================================================================

// =============================================================================
// Reads
// =============================================================================

// ListByAccount returns recent flags for one account (tenant-scoped via
// RunInReadTx). Defense-in-depth: app-layer WHERE clause + RLS.
func (r *FraudFlagRepository) ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID, limit int) ([]fraud.Flag, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	const q = `
SELECT id, tenant_id, account_id, transfer_id, rule_name, severity,
       status, evidence, detected_at, resolved_at
FROM fraud_flags
WHERE tenant_id = $1 AND account_id = $2
ORDER BY detected_at DESC
LIMIT $3
`
	var out []fraud.Flag
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, q, tenantID, accountID, limit)
		if err != nil {
			return fmt.Errorf("list fraud flags: %w", err)
		}
		defer rows.Close()
		out = make([]fraud.Flag, 0, limit)
		for rows.Next() {
			var f fraud.Flag
			var ruleName, severity, status, evidence string
			if err := rows.Scan(
				&f.ID, &f.TenantID, &f.AccountID, &f.TransferID,
				&ruleName, &severity, &status, &evidence,
				&f.DetectedAt, &f.ResolvedAt,
			); err != nil {
				return fmt.Errorf("scan fraud flag: %w", err)
			}
			f.RuleName = fraud.RuleName(ruleName)
			f.Severity = fraud.Severity(severity)
			f.Status = fraud.Status(status)
			if evidence != "" {
				_ = json.Unmarshal([]byte(evidence), &f.Evidence)
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return []fraud.Flag{}, nil
		}
		return nil, err
	}
	return out, nil
}

// CountRecentByAccount returns the count of transactions posted FROM
// `accountID` in the last `window` duration. Sprint 31: cross-tenant
// worker scan (the worker has no single tenant scope), so use
// RunInAdminTx to skip RLS filtering.
//
// Returns 0 if no rows match (no error).
func (r *FraudFlagRepository) CountRecentByAccount(ctx context.Context, tenantID, accountID uuid.UUID, window time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-window)
	const q = `
SELECT COUNT(*) FROM transactions
WHERE tenant_id = $1
  AND from_account_id = $2
  AND created_at >= $3
`
	var n int
	err := r.db.RunInAdminTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, tenantID, accountID, cutoff).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("count recent transfers: %w", err)
	}
	return n, nil
}

// HasRecipientHistory returns true if there exists at least one prior
// transaction from `fromAccount` to `toAccount` (regardless of recency).
// Sprint 31: cross-tenant scan via RunInAdminTx.
func (r *FraudFlagRepository) HasRecipientHistory(ctx context.Context, tenantID, fromAccount, toAccount uuid.UUID) (bool, error) {
	const q = `
SELECT EXISTS (
    SELECT 1 FROM transactions
    WHERE tenant_id = $1
      AND from_account_id = $2
      AND to_account_id = $3
)
`
	var has bool
	err := r.db.RunInAdminTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, tenantID, fromAccount, toAccount).Scan(&has)
	})
	if err != nil {
		return false, fmt.Errorf("check recipient history: %w", err)
	}
	return has, nil
}

// =============================================================================
// PersistDirect (interface method)
// =============================================================================

// PersistDirect writes a fraud flag using the pool (via RunInTxFraudDomain
// so GUC is bound from ctx). Used by the FraudScannerService from the
// worker context — no business tx wrapper needed.
//
// Sprint 42: uses ON CONFLICT (transfer_id, rule_name) DO NOTHING via
// the Create() method's INSERT statement. When NATS redelivers a
// transfer.posted event (worker crash mid-publish), the scanner
// re-runs the rule and would normally insert a duplicate row. The
// UNIQUE constraint + ON CONFLICT DO NOTHING ensures exactly one
// flag row per (transfer_id, rule_name) pair.
func (r *FraudFlagRepository) PersistDirect(ctx context.Context, f fraud.Flag) error {
	return r.db.RunInTxFraudDomain(ctx, func(tx fraud.Tx) error {
		return r.Create(ctx, tx, f)
	})
}

// Create persists a flag inside the caller's tx.
//
// Sprint 42: uses ON CONFLICT DO NOTHING. When the same
// (transfer_id, rule_name) already exists, the INSERT is silently
// ignored and no error is raised. RowsAffected() returns 0 in that case
// (vs 1 for a fresh insert), which callers can check if they care.
func (r *FraudFlagRepository) Create(ctx context.Context, tx fraud.Tx, f fraud.Flag) error {
	a, ok := tx.(*fraudTxAdapter)
	if !ok {
		return fmt.Errorf("postgres: expected *fraudTxAdapter, got %T", tx)
	}
	evidence, err := json.Marshal(f.Evidence)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	if f.DetectedAt.IsZero() {
		f.DetectedAt = time.Now().UTC()
	}
	const q = `
INSERT INTO fraud_flags (
    id, tenant_id, account_id, transfer_id, rule_name, severity,
    status, evidence, detected_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (transfer_id, rule_name) DO NOTHING
`
	if _, err := a.pgxTx.Exec(ctx, q,
		f.ID, f.TenantID, f.AccountID, f.TransferID,
		string(f.RuleName), string(f.Severity), string(f.Status),
		string(evidence), f.DetectedAt,
	); err != nil {
		return fmt.Errorf("insert fraud flag: %w", err)
	}
	return nil
}