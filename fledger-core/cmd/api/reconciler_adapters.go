// reconciler_adapters.go — adapters to wire ReconcilerService → handler.ReconcilerAPI.
//
// Four pieces:
//  1. reconcilerTxAdapter — wraps DB.RunInTxReconcilerDomain → usecase.ReconcilerTxRunner
//  2. ledgerProbeAdapter — implements usecase.LedgerProbe using Postgres repos
//  3. hashChainAdapter    — adapts usecase.Verifier.Verify → usecase.HashChainRunner.VerifyEntries
//  4. reconcilerAPIAdapter — adapts usecase.ReconcilerService → handler.ReconcilerAPI
//     (translates input types from handler package to usecase package)
package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
	"github.com/runut/fmcg-wallet/internal/handler"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// errNotPgxTx is a local alias for postgres.ErrNotReconcilerTx, kept here
// for adapter-local error matching.
var errNotPgxTx = postgres.ErrNotReconcilerTx

// =============================================================================
// reconcilerTxAdapter
// =============================================================================

type reconcilerTxAdapter struct {
	db *postgres.DB
}

func (a *reconcilerTxAdapter) ExecuteTx(ctx context.Context, fn func(reconciler.Tx) error) error {
	return a.db.RunInTxReconcilerDomain(ctx, fn)
}

// =============================================================================
// ledgerProbeAdapter — implements usecase.LedgerProbe for the reconciler
// =============================================================================

// ledgerProbeAdapter exposes only the trial-balance + entry-listing queries
// the reconciler needs, via the existing PeriodRepository + EntryRepository.
//
// This avoids importing ledger usecase package from reconciler (cycle prevention)
// while reusing existing repository code.
type ledgerProbeAdapter struct {
	periodRepo *postgres.PeriodRepository
	entryRepo  *postgres.EntryRepository
}

// TrialBalance computes SUM(debit) - SUM(credit) for a period.
// For trial balance we delegate to PeriodRepository.ComputeTrialBalance (already implemented).
func (a *ledgerProbeAdapter) TrialBalance(ctx context.Context, tx reconciler.Tx, periodID string) (int64, int64, int64, error) {
	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0)
FROM ledger_entries
WHERE period_id = $1
`
	pgxTx, err := postgres.UnwrapPgxTxFromReconciler(tx)
	if err != nil {
		return 0, 0, 0, err
	}
	var td, tc int64
	if err := pgxTx.QueryRow(ctx, q, periodID).Scan(&td, &tc); err != nil {
		return 0, 0, 0, fmt.Errorf("trial balance query: %w", err)
	}
	return td, tc, td - tc, nil
}

func (a *ledgerProbeAdapter) AccountBalanceAtPeriod(ctx context.Context, tx reconciler.Tx, accountID, periodID string) (int64, int64, int64, int, error) {
	const q = `
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit'  THEN amount ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0),
    COUNT(*)
FROM ledger_entries
WHERE account_id = $1 AND period_id = $2
`
	pgxTx, err := postgres.UnwrapPgxTxFromReconciler(tx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	var d, c int64
	var cnt int
	if err := pgxTx.QueryRow(ctx, q, accountID, periodID).Scan(&d, &c, &cnt); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("account balance query: %w", err)
	}
	return d, c, d - c, cnt, nil
}

func (a *ledgerProbeAdapter) ListEntriesByPeriod(ctx context.Context, periodID string) ([]ledger.Entry, error) {
	// Read-only, uses pool not tx. EntryRepository exposes entries via DB layer.
	return a.entryRepo.ListByPeriod(ctx, periodID)
}

// =============================================================================
// hashChainAdapter — bridges usecase.Verifier.Verify → usecase.HashChainRunner
// =============================================================================

// hashChainAdapter adapts the existing HashChainVerifier (Sprint 8) to the
// narrower HashChainRunner interface expected by ReconcilerService.
//
// This avoids an import cycle (reconciler can't import hashchain_verifier.go
// directly because both live in internal/usecase).
type hashChainAdapter struct {
	verifier *usecase.Verifier
}

func (a *hashChainAdapter) VerifyEntries(ctx context.Context, entries []ledger.Entry) []error {
	return a.verifier.Verify(ctx, entries)
}

// =============================================================================
// reconcilerAPIAdapter — adapts usecase.ReconcilerService → handler.ReconcilerAPI
// =============================================================================

type reconcilerAPIAdapter struct {
	svc *usecase.ReconcilerService
}

func (a *reconcilerAPIAdapter) RunReconciliation(ctx context.Context, in handler.ReconcilerRunInput) (handler.ReconcilerRunResult, error) {
	res, err := a.svc.RunReconciliation(ctx, usecase.RunReconciliationInput{
		TenantID:     in.TenantID,
		PeriodID:     in.PeriodID,
		TriggeredBy:  in.TriggeredBy,
		RunHashCheck: in.RunHashCheck,
	})
	if err != nil {
		return handler.ReconcilerRunResult{}, err
	}
	return handler.ReconcilerRunResult{
		Run:             res.Run,
		AccountResults:  res.AccountResults,
		HashChainErrors: res.HashChainErrors,
	}, nil
}

func (a *reconcilerAPIAdapter) GetRun(ctx context.Context, id string) (reconciler.ReconcilerRun, error) {
	return a.svc.GetRun(ctx, id)
}

func (a *reconcilerAPIAdapter) ListRunsByTenant(ctx context.Context, tenantID string, limit int) ([]reconciler.ReconcilerRun, error) {
	return a.svc.ListRunsByTenant(ctx, tenantID, limit)
}

func (a *reconcilerAPIAdapter) ListAccountResultsByRun(ctx context.Context, runID string) ([]reconciler.ReconcilerAccountResult, error) {
	return a.svc.ListAccountResultsByRun(ctx, runID)
}

// Compile-time guards.
var (
	_ handler.ReconcilerAPI      = (*reconcilerAPIAdapter)(nil)
	_ usecase.LedgerProbe        = (*ledgerProbeAdapter)(nil)
	_ usecase.ReconcilerTxRunner = (*reconcilerTxAdapter)(nil)
	_ usecase.HashChainRunner    = (*hashChainAdapter)(nil)
)

// Ensure errNotPgxTx keeps a reference (for future debug logs) — silences
// "declared but not used" if a future refactor removes the local var.
var _ = errors.Is
