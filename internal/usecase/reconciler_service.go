// Package usecase — ReconcilerService runs the trial-balance reconciler.
//
// The reconciler (Fase 1B) periodically:
//  1. Iterates all open/closing accounting periods across all tenants
//  2. For each: SUM(debit) - SUM(credit) per period (trial balance)
//  3. Per-account breakdown (which account is off, if any)
//  4. Optional: HashChainVerifier check (tamper detection)
//  5. Records the result + any imbalance details
//
// Two trigger paths:
//   - Manual via API (operator "Run now" button)
//   - Automatic via background scheduler (ticker-based goroutine)
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// TxRunner — reconciler-flavored
// =============================================================================

// ReconcilerTxRunner is the reconciler-flavored transaction runner.
type ReconcilerTxRunner interface {
	ExecuteTx(ctx context.Context, fn func(reconciler.Tx) error) error
}

// =============================================================================
// Dependencies — narrow interfaces for testability
// =============================================================================

// LedgerProbe reads trial-balance aggregates and entries from the ledger.
// Reconciler uses this to compute the period imbalance and per-account
// breakdown without depending on the ledger usecase package directly.
type LedgerProbe interface {
	// TrialBalance returns SUM(debit), SUM(credit), and imbalance (debit-credit)
	// for the given period. All within the caller's tx.
	TrialBalance(ctx context.Context, tx reconciler.Tx, periodID string) (totalDebit, totalCredit, imbalance int64, err error)

	// AccountBalanceAtPeriod returns per-account debit/credit/signed totals.
	AccountBalanceAtPeriod(ctx context.Context, tx reconciler.Tx, accountID, periodID string) (debit, credit, signed int64, entryCount int, err error)

	// ListEntriesByPeriod returns all entries for a period (for hash chain verification).
	// Read-only — uses pool, not tx.
	ListEntriesByPeriod(ctx context.Context, periodID string) ([]ledger.Entry, error)
}

// =============================================================================
// ReconcilerService
// =============================================================================

// ReconcilerService orchestrates trial-balance reconciliation runs.
type ReconcilerService struct {
	repo   reconciler.Repository
	ledger LedgerProbe
	hasher HashChainRunner // optional; nil disables hash-chain check
	db     ReconcilerTxRunner
	log    *slog.Logger
	now    func() time.Time
}

// HashChainRunner abstracts the hash-chain verifier so reconciler can run it
// without importing the ledger usecase package directly.
type HashChainRunner interface {
	VerifyEntries(ctx context.Context, entries []ledger.Entry) []error
}

// ReconcilerServiceDeps bundles dependencies.
type ReconcilerServiceDeps struct {
	Repo    reconciler.Repository
	Ledger  LedgerProbe
	Hasher  HashChainRunner // nil → skip hash-chain check
	DB      ReconcilerTxRunner
	Logger  *slog.Logger
	NowFunc func() time.Time
}

// NewReconcilerService constructs a ReconcilerService.
func NewReconcilerService(deps ReconcilerServiceDeps) *ReconcilerService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	nowFn := deps.NowFunc
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &ReconcilerService{
		repo:   deps.Repo,
		ledger: deps.Ledger,
		hasher: deps.Hasher,
		db:     deps.DB,
		log:    log,
		now:    nowFn,
	}
}

// =============================================================================
// Use case: RunReconciliation
// =============================================================================

// RunReconciliationInput is the input for a single period reconciliation.
type RunReconciliationInput struct {
	TenantID     string
	PeriodID     string
	TriggeredBy  reconciler.TriggerSource
	RunHashCheck bool // if true and hasher != nil, also run hash-chain verifier
}

// RunReconciliationResult is the outcome of one reconciliation run.
type RunReconciliationResult struct {
	Run             reconciler.ReconcilerRun
	AccountResults  []reconciler.ReconcilerAccountResult
	HashChainErrors int
}

// RunReconciliation runs the full trial-balance check for one period.
func (s *ReconcilerService) RunReconciliation(ctx context.Context, in RunReconciliationInput) (RunReconciliationResult, error) {
	if in.TenantID == "" {
		return RunReconciliationResult{}, fmt.Errorf("%w: tenant_id required", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.PeriodID); err != nil {
		return RunReconciliationResult{}, fmt.Errorf("%w: invalid period_id", apperrors.ErrInvalidInput)
	}
	if in.TriggeredBy == "" {
		in.TriggeredBy = reconciler.TriggerManual
	}

	now := s.now()
	runID := uuid.NewString()
	run := reconciler.ReconcilerRun{
		ID:          runID,
		TenantID:    in.TenantID,
		PeriodID:    in.PeriodID,
		StartedAt:   now,
		Status:      reconciler.RunStatusRunning,
		TriggeredBy: in.TriggeredBy,
		Metadata:    map[string]any{"run_id": runID},
	}

	var result RunReconciliationResult

	err := s.db.ExecuteTx(ctx, func(tx reconciler.Tx) error {
		// 1. Insert run row.
		if err := s.repo.CreateRun(ctx, tx, run); err != nil {
			return fmt.Errorf("create run: %w", err)
		}

		// 2. Trial balance.
		td, tc, imb, err := s.ledger.TrialBalance(ctx, tx, in.PeriodID)
		if err != nil {
			return s.markError(ctx, tx, runID, fmt.Errorf("trial balance: %w", err))
		}

		// 3. Per-account breakdown.
		accountIDs, err := s.listAccountsInPeriod(ctx, tx, in.PeriodID)
		if err != nil {
			return s.markError(ctx, tx, runID, fmt.Errorf("list accounts in period: %w", err))
		}

		var accountResults []reconciler.ReconcilerAccountResult
		for _, accountID := range accountIDs {
			debit, credit, signed, entryCount, err := s.ledger.AccountBalanceAtPeriod(ctx, tx, accountID, in.PeriodID)
			if err != nil {
				return s.markError(ctx, tx, runID, fmt.Errorf("account balance %s: %w", accountID, err))
			}
			if entryCount == 0 {
				continue
			}
			ar := reconciler.ReconcilerAccountResult{
				ID:            uuid.NewString(),
				RunID:         runID,
				PeriodID:      in.PeriodID,
				AccountID:     accountID,
				DebitMinor:    debit,
				CreditMinor:   credit,
				SignedBalance: signed,
				EntryCount:    entryCount,
				Currency:      "IDR", // MVP: single currency
			}
			if err := s.repo.InsertAccountResult(ctx, tx, ar); err != nil {
				return s.markError(ctx, tx, runID, fmt.Errorf("insert account result: %w", err))
			}
			accountResults = append(accountResults, ar)
		}

		// 4. Hash chain check (optional).
		hashChainOK := true
		hashChainErrors := 0
		hashChainChecked := in.RunHashCheck && s.hasher != nil
		if hashChainChecked {
			entries, err := s.ledger.ListEntriesByPeriod(ctx, in.PeriodID)
			if err != nil {
				return s.markError(ctx, tx, runID, fmt.Errorf("list entries: %w", err))
			}
			errs := s.hasher.VerifyEntries(ctx, entries)
			hashChainErrors = len(errs)
			hashChainOK = hashChainErrors == 0
		}

		// 5. Determine final status.
		status := reconciler.RunStatusBalanced
		if imb != 0 {
			status = reconciler.RunStatusImbalanced
		}
		if hashChainChecked && !hashChainOK {
			status = reconciler.RunStatusTampered
		}

		// 6. Update run.
		var hashChainOKPtr *bool
		if hashChainChecked {
			ok := hashChainOK
			hashChainOKPtr = &ok
		}
		if err := s.repo.FinishRun(ctx, tx, runID, status, td, tc, imb,
			hashChainOKPtr, hashChainErrors, s.now()); err != nil {
			return fmt.Errorf("finish run: %w", err)
		}

		// Build result.
		finishedAt := s.now()
		run.Status = status
		run.TotalDebit = money.NewFromMinor(td)
		run.TotalCredit = money.NewFromMinor(tc)
		run.Imbalance = money.NewFromMinor(imb)
		run.FinishedAt = &finishedAt
		if hashChainChecked {
			ok := hashChainOK
			run.HashChainOK = &ok
			run.HashChainErrors = hashChainErrors
		}

		result = RunReconciliationResult{
			Run:             run,
			AccountResults:  accountResults,
			HashChainErrors: hashChainErrors,
		}
		return nil
	})

	if err != nil {
		s.log.Warn("reconciliation failed",
			"run_id", runID,
			"period_id", in.PeriodID,
			"error", err.Error(),
		)
		return RunReconciliationResult{}, err
	}

	s.log.Info("reconciliation completed",
		"run_id", runID,
		"period_id", in.PeriodID,
		"status", result.Run.Status,
		"total_debit", result.Run.TotalDebit.Minor(),
		"total_credit", result.Run.TotalCredit.Minor(),
		"imbalance", result.Run.Imbalance.Minor(),
		"hash_chain_errors", result.HashChainErrors,
	)
	return result, nil
}

// listAccountsInPeriod returns distinct account_ids from ledger_entries for the period.
// Uses Tx.Query directly (read-only inside the caller's tx).
func (s *ReconcilerService) listAccountsInPeriod(ctx context.Context, tx reconciler.Tx, periodID string) ([]string, error) {
	const q = `SELECT DISTINCT account_id FROM ledger_entries WHERE period_id = $1 ORDER BY account_id`
	rows, err := tx.Query(ctx, q, periodID)
	if err != nil {
		return nil, fmt.Errorf("list accounts in period: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0, 16)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan account_id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// markError marks the run as 'error' status (best-effort).
func (s *ReconcilerService) markError(ctx context.Context, tx reconciler.Tx, runID string, cause error) error {
	_ = s.repo.FinishRun(ctx, tx, runID, reconciler.RunStatusError, 0, 0, 0, nil, 0, s.now())
	return cause
}

// =============================================================================
// Use case: RunAllForTenant — for scheduler
// =============================================================================

// RunAllForTenant runs reconciliation for all open/closing periods of a tenant.
func (s *ReconcilerService) RunAllForTenant(ctx context.Context, tenantID string, runHashCheck bool) ([]string, error) {
	periods, err := s.repo.ListOpenPeriods(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list open periods for tenant %s: %w", tenantID, err)
	}
	runIDs := make([]string, 0, len(periods))
	for _, p := range periods {
		res, err := s.RunReconciliation(ctx, RunReconciliationInput{
			TenantID:     tenantID,
			PeriodID:     p.ID,
			TriggeredBy:  reconciler.TriggerScheduler,
			RunHashCheck: runHashCheck,
		})
		if err != nil {
			s.log.Warn("scheduled run failed",
				"tenant_id", tenantID,
				"period_id", p.ID,
				"error", err.Error(),
			)
			continue
		}
		runIDs = append(runIDs, res.Run.ID)
	}
	return runIDs, nil
}

// RunAllForAllTenants iterates all tenants. For scheduler entry-point.
func (s *ReconcilerService) RunAllForAllTenants(ctx context.Context, runHashCheck bool) (map[string][]string, error) {
	tenants, err := s.repo.ListTenants(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	result := make(map[string][]string, len(tenants))
	for _, t := range tenants {
		ids, _ := s.RunAllForTenant(ctx, t, runHashCheck)
		result[t] = ids
	}
	return result, nil
}

// =============================================================================
// Query helpers
// =============================================================================

// GetRun returns one run by id (read-only).
func (s *ReconcilerService) GetRun(ctx context.Context, id string) (reconciler.ReconcilerRun, error) {
	return s.repo.GetRun(ctx, id)
}

// ListRunsByTenant returns recent runs for a tenant.
func (s *ReconcilerService) ListRunsByTenant(ctx context.Context, tenantID string, limit int) ([]reconciler.ReconcilerRun, error) {
	return s.repo.ListRunsByTenant(ctx, tenantID, limit)
}

// ListAccountResultsByRun returns per-account breakdown.
func (s *ReconcilerService) ListAccountResultsByRun(ctx context.Context, runID string) ([]reconciler.ReconcilerAccountResult, error) {
	return s.repo.ListAccountResultsByRun(ctx, runID)
}
