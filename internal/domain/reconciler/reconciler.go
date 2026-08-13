// Package reconciler defines the trial-balance reconciliation workflow (Fase 1B).
//
// This package is interface-only; the Postgres implementation lives in
// internal/repository/postgres. Domain has zero dependencies on infrastructure
// (no SQL, no HTTP) so it can be unit-tested without spinning up services.
//
// Design notes:
//
//   - A ReconcilerRun captures one execution of the reconciler for one period.
//     status='balanced' = SUM(debit) == SUM(credit); status='imbalanced' otherwise.
//     status='tampered' = hash chain verifier found mismatch.
//   - ReconcilerAccountResult holds per-account breakdown so operators can
//     identify WHICH account is off (one account with off-balance, others fine).
//   - Repository methods are split: read-only (Get, List) don't need tx;
//     mutating ops (CreateRun, FinishRun, InsertAccountResult) take tx so
//     callers can compose run start + finish + results in one atomic tx.
package reconciler

import (
	"context"
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// Status enums
// =============================================================================

// RunStatus mirrors the reconciler_runs.status column.
type RunStatus string

const (
	RunStatusRunning    RunStatus = "running"
	RunStatusBalanced   RunStatus = "balanced"
	RunStatusImbalanced RunStatus = "imbalanced"
	RunStatusTampered   RunStatus = "tampered"
	RunStatusError      RunStatus = "error"
)

// TriggerSource mirrors the reconciler_runs.triggered_by column.
type TriggerSource string

const (
	TriggerScheduler TriggerSource = "scheduler"
	TriggerManual    TriggerSource = "manual"
	TriggerAPI       TriggerSource = "api"
)

// =============================================================================
// Entities
// =============================================================================

// ReconcilerRun is one row of reconciler run history.
type ReconcilerRun struct {
	ID              string
	TenantID        string
	PeriodID        string
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          RunStatus
	TotalDebit      money.Money
	TotalCredit     money.Money
	Imbalance       money.Money
	HashChainOK     *bool
	HashChainErrors int
	TriggeredBy     TriggerSource
	Metadata        map[string]any
}

// ReconcilerAccountResult is one row of per-account breakdown.
type ReconcilerAccountResult struct {
	ID            string
	RunID         string
	PeriodID      string
	AccountID     string
	DebitMinor    int64
	CreditMinor   int64
	SignedBalance int64
	EntryCount    int
	Currency      string
}

// =============================================================================
// Repository interface
// =============================================================================

// Repository defines persistence operations for reconciler runs.
type Repository interface {
	// CreateRun inserts a new 'running' run. Caller's tx.
	CreateRun(ctx context.Context, tx Tx, run ReconcilerRun) error

	// FinishRun updates status + totals + finished_at. Caller's tx.
	FinishRun(ctx context.Context, tx Tx, id string, status RunStatus, totalDebit, totalCredit, imbalance int64, hashChainOK *bool, hashChainErrors int, finishedAt time.Time) error

	// InsertAccountResult writes one per-account row. Caller's tx.
	InsertAccountResult(ctx context.Context, tx Tx, result ReconcilerAccountResult) error

	// GetRun reads a run by id (no lock).
	GetRun(ctx context.Context, id string) (ReconcilerRun, error)

	// ListRunsByPeriod returns all runs for a period (audit trail).
	ListRunsByPeriod(ctx context.Context, periodID string, limit int) ([]ReconcilerRun, error)

	// ListRunsByTenant returns recent runs for a tenant (dashboard).
	ListRunsByTenant(ctx context.Context, tenantID string, limit int) ([]ReconcilerRun, error)

	// ListAccountResultsByRun returns per-account breakdown for a run.
	ListAccountResultsByRun(ctx context.Context, runID string) ([]ReconcilerAccountResult, error)

	// ListOpenPeriods returns all periods in 'open' or 'closing' status for a tenant.
	// Reconciler iterates these periodically.
	ListOpenPeriods(ctx context.Context, tenantID string) ([]PeriodRef, error)

	// ListTenants returns all distinct tenant_ids (for scheduler).
	ListTenants(ctx context.Context) ([]string, error)
}

// PeriodRef is a minimal reference to a period (used by reconciler).
type PeriodRef struct {
	ID       string
	TenantID string
}

// =============================================================================
// Transaction abstraction
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
	Close()
}
