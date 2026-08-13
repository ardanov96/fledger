//go:build !windows
// +build !windows

// ReconcilerService tests — Sprint 10 (Fase 1B).
//
// Validates the trial-balance + per-account breakdown + hash-chain check
// workflow using in-memory fakes (no real DB).
//
// Run with: go -C fmcg-wallet test ./internal/usecase/...
package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
)

// =============================================================================
// In-memory fakes
// =============================================================================

type fakeEntry struct {
	ID        string
	PeriodID  string
	AccountID string
	Type      string // "debit" | "credit"
	Amount    int64
}

type recRepo struct {
	mu          sync.Mutex
	runs        map[string]*reconciler.ReconcilerRun                 // id → run
	acctResults map[string][]*reconciler.ReconcilerAccountResult     // run_id → results
	periods     map[string]*reconciler.PeriodRef                     // id → ref
	tenants     map[string]bool                                     // distinct tenants
	entries     []fakeEntry                                          // ledger entries
}

func newRecRepo() *recRepo {
	return &recRepo{
		runs:        map[string]*reconciler.ReconcilerRun{},
		acctResults: map[string][]*reconciler.ReconcilerAccountResult{},
		periods:     map[string]*reconciler.PeriodRef{},
		tenants:     map[string]bool{},
	}
}

// =============================================================================
// Repository interface implementation
// =============================================================================

func (r *recRepo) assertTx(tx reconciler.Tx) (*recTx, error) {
	a, ok := tx.(*recTx)
	if !ok {
		return nil, errors.New("expected *recTx")
	}
	return a, nil
}

func (r *recRepo) CreateRun(_ context.Context, tx reconciler.Tx, run reconciler.ReconcilerRun) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := run
	r.runs[run.ID] = &cp
	if run.TenantID != "" {
		r.tenants[run.TenantID] = true
	}
	return nil
}

func (r *recRepo) FinishRun(_ context.Context, tx reconciler.Tx, id string,
	status reconciler.RunStatus, totalDebit, totalCredit, imbalance int64,
	hashChainOK *bool, hashChainErrors int, finishedAt time.Time,
) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	run.Status = status
	run.TotalDebit = money.NewFromMinor(totalDebit)
	run.TotalCredit = money.NewFromMinor(totalCredit)
	run.Imbalance = money.NewFromMinor(imbalance)
	run.HashChainOK = hashChainOK
	run.HashChainErrors = hashChainErrors
	t := finishedAt
	run.FinishedAt = &t
	return nil
}

func (r *recRepo) InsertAccountResult(_ context.Context, tx reconciler.Tx, result reconciler.ReconcilerAccountResult) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := result
	r.acctResults[result.RunID] = append(r.acctResults[result.RunID], &cp)
	return nil
}

func (r *recRepo) GetRun(_ context.Context, id string) (reconciler.ReconcilerRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return reconciler.ReconcilerRun{}, apperrors.ErrNotFound
	}
	return *run, nil
}

func (r *recRepo) ListRunsByPeriod(_ context.Context, periodID string, _ int) ([]reconciler.ReconcilerRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]reconciler.ReconcilerRun, 0)
	for _, run := range r.runs {
		if run.PeriodID == periodID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (r *recRepo) ListRunsByTenant(_ context.Context, tenantID string, _ int) ([]reconciler.ReconcilerRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]reconciler.ReconcilerRun, 0)
	for _, run := range r.runs {
		if run.TenantID == tenantID {
			out = append(out, *run)
		}
	}
	return out, nil
}

func (r *recRepo) ListAccountResultsByRun(_ context.Context, runID string) ([]reconciler.ReconcilerAccountResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	results := r.acctResults[runID]
	out := make([]reconciler.ReconcilerAccountResult, 0, len(results))
	for _, x := range results {
		out = append(out, *x)
	}
	return out, nil
}

func (r *recRepo) ListOpenPeriods(_ context.Context, tenantID string) ([]reconciler.PeriodRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]reconciler.PeriodRef, 0)
	for _, p := range r.periods {
		if p.TenantID == tenantID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *recRepo) ListTenants(_ context.Context) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.tenants))
	for t := range r.tenants {
		out = append(out, t)
	}
	return out, nil
}

// =============================================================================
// recTx fake + TxRunner
// =============================================================================

type recTx struct {
	repo *recRepo
}

func (t *recTx) Exec(_ context.Context, _ string, _ ...any) (reconciler.CommandTag, error) {
	return fakeTag{rows: 1}, nil
}
func (t *recTx) Query(_ context.Context, _ string, _ ...any) (reconciler.Rows, error) {
	return &fakeRecRows{}, nil
}
func (t *recTx) QueryRow(_ context.Context, _ string, _ ...any) reconciler.Row {
	return &fakeRecRow{}
}

type fakeTag struct{ rows int64 }

func (f fakeTag) RowsAffected() int64 { return f.rows }

type fakeRecRows struct{}

func (r *fakeRecRows) Next() bool             { return false }
func (r *fakeRecRows) Scan(_ ...any) error    { return nil }
func (r *fakeRecRows) Err() error             { return nil }
func (r *fakeRecRows) Close()                 {}

type fakeRecRow struct{}

func (r *fakeRecRow) Scan(_ ...any) error { return nil }

type recTxRunner struct{ tx reconciler.Tx }

func (r *recTxRunner) ExecuteTx(_ context.Context, fn func(reconciler.Tx) error) error {
	return fn(r.tx)
}

// =============================================================================
// fake LedgerProbe
// =============================================================================

type recLedgerProbe struct {
	repo *recRepo
}

func (p *recLedgerProbe) TrialBalance(_ context.Context, _ reconciler.Tx, periodID string) (int64, int64, int64, error) {
	p.repo.mu.Lock()
	defer p.repo.mu.Unlock()
	var td, tc int64
	for _, e := range p.repo.entries {
		if e.PeriodID == periodID {
			if e.Type == "debit" {
				td += e.Amount
			} else {
				tc += e.Amount
			}
		}
	}
	return td, tc, td - tc, nil
}

func (p *recLedgerProbe) AccountBalanceAtPeriod(_ context.Context, _ reconciler.Tx, accountID, periodID string) (int64, int64, int64, int, error) {
	p.repo.mu.Lock()
	defer p.repo.mu.Unlock()
	var d, c int64
	cnt := 0
	for _, e := range p.repo.entries {
		if e.AccountID == accountID && e.PeriodID == periodID {
			if e.Type == "debit" {
				d += e.Amount
			} else {
				c += e.Amount
			}
			cnt++
		}
	}
	return d, c, d - c, cnt, nil
}

func (p *recLedgerProbe) ListEntriesByPeriod(_ context.Context, periodID string) ([]ledger.Entry, error) {
	p.repo.mu.Lock()
	defer p.repo.mu.Unlock()
	out := make([]ledger.Entry, 0)
	for _, e := range p.repo.entries {
		if e.PeriodID == periodID {
			out = append(out, ledger.Entry{
				ID:            e.ID,
				AccountID:     e.AccountID,
				PeriodID:      e.PeriodID,
				Type:          ledger.EntryType(e.Type),
				Amount:        money.NewFromMinor(e.Amount),
				Currency:      "IDR",
				CreatedAt:     time.Now().UTC(),
				PrevHash:      ledger.ZeroHash,
				EntryHash:     ledger.ZeroHash,
			})
		}
	}
	return out, nil
}

// =============================================================================
// fake HashChainRunner — configurable
// =============================================================================

type fakeHashChain struct {
	mu    sync.Mutex
	calls int
	errs  []error
}

func (h *fakeHashChain) VerifyEntries(_ context.Context, _ []ledger.Entry) []error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls++
	return h.errs
}

// =============================================================================
// Test fixtures
// =============================================================================

const (
	recTenant1   = "00000000-0000-0000-0000-000000000001"
	recPeriod1   = "11111111-1111-1111-1111-111111111111"
	recAccountHQ = "22222222-2222-2222-2222-222222222222"
)

func newReconcilerSvc(t *testing.T) (*ReconcilerService, *recRepo, *fakeHashChain) {
	t.Helper()
	repo := newRecRepo()
	repo.periods[recPeriod1] = &reconciler.PeriodRef{ID: recPeriod1, TenantID: recTenant1}
	repo.tenants[recTenant1] = true

	txr := &recTxRunner{tx: &recTx{repo: repo}}
	probe := &recLedgerProbe{repo: repo}
	hasher := &fakeHashChain{}
	svc := NewReconcilerService(ReconcilerServiceDeps{
		Repo:   repo,
		Ledger: probe,
		Hasher: hasher,
		DB:     txr,
		Logger: slog.Default(),
	})
	return svc, repo, hasher
}

func seedBalanced(recRepo *recRepo) {
	recRepo.entries = append(recRepo.entries,
		fakeEntry{ID: "e1", PeriodID: recPeriod1, AccountID: recAccountHQ, Type: "debit", Amount: 10000},
		fakeEntry{ID: "e2", PeriodID: recPeriod1, AccountID: "33333333-3333-3333-3333-333333333333", Type: "credit", Amount: 10000},
	)
}

func seedImbalanced(recRepo *recRepo) {
	recRepo.entries = append(recRepo.entries,
		fakeEntry{ID: "e1", PeriodID: recPeriod1, AccountID: recAccountHQ, Type: "debit", Amount: 10000},
		fakeEntry{ID: "e2", PeriodID: recPeriod1, AccountID: "33333333-3333-3333-3333-333333333333", Type: "credit", Amount: 5000},
	)
}

func seedMultiAccount(recRepo *recRepo) {
	// 3 accounts: HQ, outlet1, outlet2
	// Transfer HQ → outlet1 (10000): debit HQ 10000, credit outlet1 10000
	// Transfer outlet1 → outlet2 (3000): debit outlet1 3000, credit outlet2 3000
	// Total: debit 13000, credit 13000 — balanced
	recRepo.entries = append(recRepo.entries,
		fakeEntry{ID: "e1", PeriodID: recPeriod1, AccountID: recAccountHQ, Type: "debit", Amount: 10000},
		fakeEntry{ID: "e2", PeriodID: recPeriod1, AccountID: "33333333-3333-3333-3333-333333333333", Type: "credit", Amount: 10000},
		fakeEntry{ID: "e3", PeriodID: recPeriod1, AccountID: "33333333-3333-3333-3333-333333333333", Type: "debit", Amount: 3000},
		fakeEntry{ID: "e4", PeriodID: recPeriod1, AccountID: "44444444-4444-4444-4444-444444444444", Type: "credit", Amount: 3000},
	)
}

// =============================================================================
// ReconcilerService tests
// =============================================================================

func TestReconcilerService_BalancedPeriod_StatusBalanced(t *testing.T) {
	t.Parallel()
	svc, repo, hasher := newReconcilerSvc(t)
	seedBalanced(repo)

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerManual,
		RunHashCheck: false,
	})
	require.NoError(t, err)
	assert.Equal(t, reconciler.RunStatusBalanced, res.Run.Status)
	assert.Equal(t, int64(10000), res.Run.TotalDebit.Minor())
	assert.Equal(t, int64(10000), res.Run.TotalCredit.Minor())
	assert.Equal(t, int64(0), res.Run.Imbalance.Minor())
	assert.Nil(t, res.Run.HashChainOK, "hash check was disabled")
	assert.Equal(t, 0, res.Run.HashChainErrors)
	assert.Equal(t, 0, hasher.calls, "hasher should not be called when RunHashCheck=false")

	// 2 account result rows.
	assert.Len(t, res.AccountResults, 2)
}

func TestReconcilerService_ImbalancedPeriod_StatusImbalanced(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newReconcilerSvc(t)
	seedImbalanced(repo)

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerManual,
		RunHashCheck: false,
	})
	require.NoError(t, err)
	assert.Equal(t, reconciler.RunStatusImbalanced, res.Run.Status)
	assert.Equal(t, int64(10000), res.Run.TotalDebit.Minor())
	assert.Equal(t, int64(5000), res.Run.TotalCredit.Minor())
	assert.Equal(t, int64(5000), res.Run.Imbalance.Minor())
}

func TestReconcilerService_HashChainCheck_OK_RemainsBalanced(t *testing.T) {
	t.Parallel()
	svc, repo, hasher := newReconcilerSvc(t)
	seedBalanced(repo)
	hasher.errs = nil // clean chain

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerAPI,
		RunHashCheck: true,
	})
	require.NoError(t, err)
	assert.Equal(t, reconciler.RunStatusBalanced, res.Run.Status)
	require.NotNil(t, res.Run.HashChainOK)
	assert.True(t, *res.Run.HashChainOK)
	assert.Equal(t, 0, res.Run.HashChainErrors)
	assert.Equal(t, 1, hasher.calls, "hasher should be called exactly once")
}

func TestReconcilerService_HashChainCheck_Tampered_StatusTampered(t *testing.T) {
	t.Parallel()
	svc, repo, hasher := newReconcilerSvc(t)
	seedBalanced(repo)
	hasher.errs = []error{errors.New("entry e1: tamper detected")}

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerScheduler,
		RunHashCheck: true,
	})
	require.NoError(t, err)
	// Even when trial balance is OK, hash chain failure → tampered status.
	assert.Equal(t, reconciler.RunStatusTampered, res.Run.Status)
	require.NotNil(t, res.Run.HashChainOK)
	assert.False(t, *res.Run.HashChainOK)
	assert.Equal(t, 1, res.Run.HashChainErrors)
}

func TestReconcilerService_InvalidTenantID_Fails(t *testing.T) {
	t.Parallel()
	svc, _, _ := newReconcilerSvc(t)

	_, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:    "",
		PeriodID:    recPeriod1,
		TriggeredBy: reconciler.TriggerManual,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

func TestReconcilerService_InvalidPeriodID_Fails(t *testing.T) {
	t.Parallel()
	svc, _, _ := newReconcilerSvc(t)

	_, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:    recTenant1,
		PeriodID:    "not-a-uuid",
		TriggeredBy: reconciler.TriggerManual,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

func TestReconcilerService_MultiAccount_PerAccountBreakdown(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newReconcilerSvc(t)
	seedMultiAccount(repo)

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerManual,
		RunHashCheck: false,
	})
	require.NoError(t, err)
	assert.Equal(t, reconciler.RunStatusBalanced, res.Run.Status)

	// 3 distinct accounts → 3 result rows.
	require.Len(t, res.AccountResults, 3)

	// Build map for assertion.
	byAcct := map[string]reconciler.ReconcilerAccountResult{}
	for _, r := range res.AccountResults {
		byAcct[r.AccountID] = r
	}

	// HQ: debit 10000, credit 0, signed +10000, 1 entry.
	hq := byAcct[recAccountHQ]
	assert.Equal(t, int64(10000), hq.DebitMinor)
	assert.Equal(t, int64(0), hq.CreditMinor)
	assert.Equal(t, int64(10000), hq.SignedBalance)
	assert.Equal(t, 1, hq.EntryCount)

	// outlet1: debit 3000, credit 10000, signed -7000, 2 entries.
	o1 := byAcct["33333333-3333-3333-3333-333333333333"]
	assert.Equal(t, int64(3000), o1.DebitMinor)
	assert.Equal(t, int64(10000), o1.CreditMinor)
	assert.Equal(t, int64(-7000), o1.SignedBalance)
	assert.Equal(t, 2, o1.EntryCount)

	// outlet2: debit 0, credit 3000, signed -3000, 1 entry.
	o2 := byAcct["44444444-4444-4444-4444-444444444444"]
	assert.Equal(t, int64(0), o2.DebitMinor)
	assert.Equal(t, int64(3000), o2.CreditMinor)
	assert.Equal(t, int64(-3000), o2.SignedBalance)
	assert.Equal(t, 1, o2.EntryCount)
}

func TestReconcilerService_EmptyPeriod_BalancedZero(t *testing.T) {
	t.Parallel()
	svc, _, _ := newReconcilerSvc(t)
	// No entries seeded.

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerManual,
		RunHashCheck: false,
	})
	require.NoError(t, err)
	assert.Equal(t, reconciler.RunStatusBalanced, res.Run.Status)
	assert.Equal(t, int64(0), res.Run.TotalDebit.Minor())
	assert.Equal(t, int64(0), res.Run.TotalCredit.Minor())
	assert.Equal(t, int64(0), res.Run.Imbalance.Minor())
	assert.Len(t, res.AccountResults, 0)
}

func TestReconcilerService_RunAllForTenant_IteratesOpenPeriods(t *testing.T) {
	t.Parallel()
	svc, repo, _ := newReconcilerSvc(t)
	// Add second period for same tenant.
	const period2 = "55555555-5555-5555-5555-555555555555"
	repo.periods[period2] = &reconciler.PeriodRef{ID: period2, TenantID: recTenant1}
	// Period1 has balanced entries; period2 has imbalanced.
	seedBalanced(repo)
	repo.entries = append(repo.entries,
		fakeEntry{ID: "e3", PeriodID: period2, AccountID: "a", Type: "debit", Amount: 1000},
		fakeEntry{ID: "e4", PeriodID: period2, AccountID: "b", Type: "credit", Amount: 999},
	)

	runIDs, err := svc.RunAllForTenant(context.Background(), recTenant1, false)
	require.NoError(t, err)
	assert.Len(t, runIDs, 2, "should run for both open periods")

	// Verify both runs are persisted.
	repo.mu.Lock()
	defer repo.mu.Unlock()
	statuses := map[string]reconciler.RunStatus{}
	for _, run := range repo.runs {
		statuses[run.PeriodID] = run.Status
	}
	assert.Equal(t, reconciler.RunStatusBalanced, statuses[recPeriod1])
	assert.Equal(t, reconciler.RunStatusImbalanced, statuses[period2])
}

func TestReconcilerService_HashChainCheck_Imbalance_StillTampered(t *testing.T) {
	t.Parallel()
	// When BOTH imbalance AND hash chain failure happen, status should be tampered
	// (tamper is a more severe condition than imbalance).
	svc, repo, hasher := newReconcilerSvc(t)
	seedImbalanced(repo)
	hasher.errs = []error{errors.New("prev_hash mismatch")}

	res, err := svc.RunReconciliation(context.Background(), RunReconciliationInput{
		TenantID:     recTenant1,
		PeriodID:     recPeriod1,
		TriggeredBy:  reconciler.TriggerManual,
		RunHashCheck: true,
	})
	require.NoError(t, err)
	assert.Equal(t, reconciler.RunStatusTampered, res.Run.Status)
	assert.Equal(t, int64(5000), res.Run.Imbalance.Minor())
	assert.Equal(t, 1, res.Run.HashChainErrors)
}

func TestReconcilerService_GetRun_NotFound(t *testing.T) {
	t.Parallel()
	svc, _, _ := newReconcilerSvc(t)

	_, err := svc.GetRun(context.Background(), "00000000-0000-0000-0000-000000000xxx")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestReconcilerService_ListAccountResultsByRun_Empty(t *testing.T) {
	t.Parallel()
	svc, _, _ := newReconcilerSvc(t)

	out, err := svc.ListAccountResultsByRun(context.Background(), "00000000-0000-0000-0000-000000000xxx")
	require.NoError(t, err)
	assert.Empty(t, out)
}
