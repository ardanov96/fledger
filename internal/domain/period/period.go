// Package period defines the accounting-period close workflow domain.
//
// This package is interface-only; the Postgres implementation lives in
// internal/repository/postgres. Domain has zero dependencies on infrastructure
// (no SQL, no HTTP) so it can be unit-tested without spinning up services.
//
// Design notes:
//
//   - A CloseRequest follows a two-step approval workflow:
//     pending → approved (closes period + freezes snapshot)
//              → rejected (period stays open)
//              → cancelled (requester撤回 sebelum approval)
//   - A PeriodSnapshot is a frozen, immutable record of per-account balances
//     at the moment the close was approved. Corrections to closed periods
//     must be done via "opening balance entry" in a new period, never by
//     editing the snapshot.
//   - TrialBalanceResult is the snapshot at the moment of approval; used
//     for audit (proves the books balanced when closed).
package period

import (
	"context"
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// Status enums
// =============================================================================

// PeriodStatus mirrors the accounting_periods.status column.
type PeriodStatus string

const (
	PeriodStatusOpen    PeriodStatus = "open"
	PeriodStatusClosing PeriodStatus = "closing"
	PeriodStatusClosed  PeriodStatus = "closed"
)

// CloseRequestStatus mirrors the period_close_requests.status column.
type CloseRequestStatus string

const (
	CloseRequestPending   CloseRequestStatus = "pending"
	CloseRequestApproved  CloseRequestStatus = "approved"
	CloseRequestRejected  CloseRequestStatus = "rejected"
	CloseRequestCancelled CloseRequestStatus = "cancelled"
)

// =============================================================================
// Entities
// =============================================================================

// Period mirrors the accounting_periods table (minimal fields used here).
type Period struct {
	ID          string
	TenantID    string
	PeriodStart time.Time
	PeriodEnd   time.Time
	Status      PeriodStatus
}

// CloseRequest is one close workflow event for one period.
type CloseRequest struct {
	ID              string
	TenantID        string
	PeriodID        string
	RequesterID     string
	ApproverID      string // empty until decided
	Status          CloseRequestStatus
	TrialBalanceOK  bool
	TotalDebit      money.Money
	TotalCredit     money.Money
	Imbalance       money.Money
	RejectionReason string
	RequestedAt     time.Time
	DecidedAt       *time.Time // nil until decided
	Metadata        map[string]any
}

// PeriodSnapshot is one row in period_snapshots — frozen balance of one
// account at the moment the close request was approved.
type PeriodSnapshot struct {
	ID            string
	TenantID      string
	PeriodID      string
	RequestID     string
	AccountID     string
	BalanceMinor  int64     // signed: debit positive, credit negative
	Currency      string
	EntryCount    int
	SnapshotAt    time.Time
	Metadata      map[string]any
}

// =============================================================================
// Repository interface
// =============================================================================

// Repository defines persistence operations for the period-close workflow.
//
// Methods that mutate period status (RequestClose / Approve / Reject) accept a
// Tx so the caller can compose them with other writes in one atomic transaction
// (e.g. snapshot rows inserted in the same tx as status='closed').
type Repository interface {
	// LockPeriod reads + SELECT FOR UPDATE on the period row. Callers MUST
	// call this inside their own tx to serialize concurrent close attempts.
	LockPeriod(ctx context.Context, tx Tx, periodID string) (Period, error)

	// UpdatePeriodStatus mutates accounting_periods.status. Caller's tx.
	UpdatePeriodStatus(ctx context.Context, tx Tx, periodID string, status PeriodStatus) error

	// InsertCloseRequest writes a new pending request. Caller's tx.
	InsertCloseRequest(ctx context.Context, tx Tx, req CloseRequest) error

	// GetCloseRequest reads a request by id (no lock; for read-only ops).
	GetCloseRequest(ctx context.Context, id string) (CloseRequest, error)

	// LockCloseRequest reads + SELECT FOR UPDATE on the request. Caller's tx.
	LockCloseRequest(ctx context.Context, tx Tx, id string) (CloseRequest, error)

	// DecideCloseRequest updates request status + approver + trial balance fields.
	// Caller's tx.
	DecideCloseRequest(ctx context.Context, tx Tx, id string, status CloseRequestStatus, approverID string, totalDebit, totalCredit, imbalance int64, rejectionReason string, decidedAt time.Time) error

	// InsertSnapshot writes one snapshot row. Caller's tx.
	InsertSnapshot(ctx context.Context, tx Tx, snap PeriodSnapshot) error

	// ListSnapshotsByPeriod returns all snapshots for a period (for balance-sheet query).
	ListSnapshotsByPeriod(ctx context.Context, periodID string) ([]PeriodSnapshot, error)

	// ListRequestsByPeriod returns all close requests for a period (audit trail).
	ListRequestsByPeriod(ctx context.Context, periodID string) ([]CloseRequest, error)

	// ComputeTrialBalance queries SUM(debit)-SUM(credit) over ledger_entries
	// for the given period (filtered by period_id). Returns totals in minor units.
	// Caller's tx — so it's consistent with the snapshot read.
	ComputeTrialBalance(ctx context.Context, tx Tx, periodID string) (totalDebit, totalCredit, imbalance int64, err error)

	// CountEntriesForPeriod returns the number of entries in the period (audit context).
	CountEntriesForPeriod(ctx context.Context, tx Tx, periodID string) (int, error)

	// ListAccountsByTenant returns all accounts for a tenant (used to seed snapshots).
	// Read-only; no tx required.
	ListAccountsByTenant(ctx context.Context, tenantID string) ([]AccountRef, error)

	// ComputeAccountBalanceAtPeriod computes signed balance for an account up to (and including)
	// the period. Caller's tx.
	ComputeAccountBalanceAtPeriod(ctx context.Context, tx Tx, accountID, periodID string) (balanceMinor int64, entryCount int, err error)
}

// AccountRef is a minimal reference to an account (used when generating snapshots).
type AccountRef struct {
	ID       string
	Currency string
}

// =============================================================================
// Transaction abstraction (mirrors invoice.Tx — separate type so test mocks can differ)
// =============================================================================

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

type CommandTag interface {
	RowsAffected() int64
}

type Row interface {
	Scan(dest ...any) error
}

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}
