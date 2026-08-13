//go:build !windows
// +build !windows

// PeriodService tests — in-memory fakes (repo + TxRunner + accounts + entries)
// to validate Sprint 9 correctness without a real DB.
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

	"github.com/runut/fmcg-wallet/internal/domain/period"
)

// =============================================================================
// In-memory fakes
// =============================================================================

type fakeAccount struct {
	ID           string
	TenantID     string
	Currency     string
	DebitTotal   int64 // sum of debit entries
	CreditTotal  int64 // sum of credit entries
	EntryCounts  int   // entries posted to this account in the period
}

type fakeEntry struct {
	ID        string
	PeriodID  string
	AccountID string
	Type      string // "debit" | "credit"
	Amount    int64
}

type periodRepo struct {
	mu        sync.Mutex
	periods   map[string]*period.Period                  // id → period
	requests  map[string]*period.CloseRequest            // id → request
	snapshots map[string][]*period.PeriodSnapshot        // period_id → snapshots
	accounts  map[string]*fakeAccount                    // id → account
	entries   []fakeEntry                                // append-only ledger entries
}

func newPeriodRepo() *periodRepo {
	return &periodRepo{
		periods:   map[string]*period.Period{},
		requests:  map[string]*period.CloseRequest{},
		snapshots: map[string][]*period.PeriodSnapshot{},
		accounts:  map[string]*fakeAccount{},
	}
}

// =============================================================================
// Repository interface implementation
// =============================================================================

func (r *periodRepo) assertTx(tx period.Tx) (*periodTx, error) {
	a, ok := tx.(*periodTx)
	if !ok {
		return nil, errors.New("expected *periodTx")
	}
	return a, nil
}

func (r *periodRepo) LockPeriod(_ context.Context, tx period.Tx, periodID string) (period.Period, error) {
	if _, err := r.assertTx(tx); err != nil {
		return period.Period{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.periods[periodID]
	if !ok {
		return period.Period{}, apperrors.ErrNotFound
	}
	cp := *p
	return cp, nil
}

func (r *periodRepo) UpdatePeriodStatus(_ context.Context, tx period.Tx, periodID string, status period.PeriodStatus) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.periods[periodID]
	if !ok {
		return apperrors.ErrNotFound
	}
	p.Status = status
	return nil
}

func (r *periodRepo) InsertCloseRequest(_ context.Context, tx period.Tx, req period.CloseRequest) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Idempotent check: one pending per (period, requester)
	for _, existing := range r.requests {
		if existing.PeriodID == req.PeriodID &&
			existing.RequesterID == req.RequesterID &&
			existing.Status == period.CloseRequestPending {
			return apperrors.ErrAlreadyExists
		}
	}
	cp := req
	r.requests[req.ID] = &cp
	return nil
}

func (r *periodRepo) GetCloseRequest(_ context.Context, id string) (period.CloseRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[id]
	if !ok {
		return period.CloseRequest{}, apperrors.ErrNotFound
	}
	return *req, nil
}

func (r *periodRepo) LockCloseRequest(_ context.Context, tx period.Tx, id string) (period.CloseRequest, error) {
	if _, err := r.assertTx(tx); err != nil {
		return period.CloseRequest{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[id]
	if !ok {
		return period.CloseRequest{}, apperrors.ErrNotFound
	}
	return *req, nil
}

func (r *periodRepo) DecideCloseRequest(_ context.Context, tx period.Tx, id string,
	status period.CloseRequestStatus, approverID string,
	totalDebit, totalCredit, imbalance int64,
	rejectionReason string, decidedAt time.Time,
) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	req.Status = status
	req.ApproverID = approverID
	req.TotalDebit = money.NewFromMinor(totalDebit)
	req.TotalCredit = money.NewFromMinor(totalCredit)
	req.Imbalance = money.NewFromMinor(imbalance)
	req.RejectionReason = rejectionReason
	t := decidedAt
	req.DecidedAt = &t
	req.TrialBalanceOK = (status == period.CloseRequestApproved)
	return nil
}

func (r *periodRepo) InsertSnapshot(_ context.Context, tx period.Tx, snap period.PeriodSnapshot) error {
	if _, err := r.assertTx(tx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// UNIQUE (request_id, account_id) check
	for _, existing := range r.snapshots[snap.PeriodID] {
		if existing.RequestID == snap.RequestID && existing.AccountID == snap.AccountID {
			return apperrors.ErrAlreadyExists
		}
	}
	cp := snap
	r.snapshots[snap.PeriodID] = append(r.snapshots[snap.PeriodID], &cp)
	return nil
}

func (r *periodRepo) ListSnapshotsByPeriod(_ context.Context, periodID string) ([]period.PeriodSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]period.PeriodSnapshot, 0, len(r.snapshots[periodID]))
	for _, s := range r.snapshots[periodID] {
		out = append(out, *s)
	}
	return out, nil
}

func (r *periodRepo) ListRequestsByPeriod(_ context.Context, periodID string) ([]period.CloseRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]period.CloseRequest, 0)
	for _, req := range r.requests {
		if req.PeriodID == periodID {
			out = append(out, *req)
		}
	}
	return out, nil
}

func (r *periodRepo) ComputeTrialBalance(_ context.Context, tx period.Tx, periodID string) (int64, int64, int64, error) {
	if _, err := r.assertTx(tx); err != nil {
		return 0, 0, 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var td, tc int64
	for _, e := range r.entries {
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

func (r *periodRepo) CountEntriesForPeriod(_ context.Context, tx period.Tx, periodID string) (int, error) {
	if _, err := r.assertTx(tx); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	c := 0
	for _, e := range r.entries {
		if e.PeriodID == periodID {
			c++
		}
	}
	return c, nil
}

func (r *periodRepo) ListAccountsByTenant(_ context.Context, tenantID string) ([]period.AccountRef, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]period.AccountRef, 0)
	for _, a := range r.accounts {
		if a.TenantID == tenantID {
			out = append(out, period.AccountRef{ID: a.ID, Currency: a.Currency})
		}
	}
	return out, nil
}

func (r *periodRepo) ComputeAccountBalanceAtPeriod(_ context.Context, tx period.Tx, accountID, periodID string) (int64, int, error) {
	if _, err := r.assertTx(tx); err != nil {
		return 0, 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var bal int64
	var cnt int
	for _, e := range r.entries {
		if e.AccountID == accountID && e.PeriodID == periodID {
			if e.Type == "debit" {
				bal += e.Amount
			} else {
				bal -= e.Amount
			}
			cnt++
		}
	}
	return bal, cnt, nil
}

// =============================================================================
// period.Tx fake + TxRunner
// =============================================================================

type periodTx struct{ repo *periodRepo }

func (t *periodTx) Exec(_ context.Context, _ string, _ ...any) (period.CommandTag, error) {
	return fakeTag{rows: 1}, nil
}
func (t *periodTx) Query(_ context.Context, _ string, _ ...any) (period.Rows, error) {
	return nil, errors.New("not used")
}
func (t *periodTx) QueryRow(_ context.Context, _ string, _ ...any) period.Row {
	return nil
}

type fakeTag struct{ rows int64 }

func (f fakeTag) RowsAffected() int64 { return f.rows }

type fakeTxRunner struct{ tx period.Tx }

func (r *fakeTxRunner) ExecuteTx(_ context.Context, fn func(period.Tx) error) error {
	return fn(r.tx)
}

// =============================================================================
// Test fixtures
// =============================================================================

const testTenant = "00000000-0000-0000-0000-000000000001"
const testPeriod1 = "11111111-1111-1111-1111-111111111111"

func newSvc(t *testing.T) (*PeriodService, *periodRepo) {
	t.Helper()
	repo := newPeriodRepo()
	// Default: an "open" period
	repo.periods[testPeriod1] = &period.Period{
		ID:          testPeriod1,
		TenantID:    testTenant,
		PeriodStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC),
		Status:      period.PeriodStatusOpen,
	}
	// Two accounts for tenant
	repo.accounts["acct-hq"] = &fakeAccount{ID: "acct-hq", TenantID: testTenant, Currency: "IDR"}
	repo.accounts["acct-outlet"] = &fakeAccount{ID: "acct-outlet", TenantID: testTenant, Currency: "IDR"}
	txr := &fakeTxRunner{tx: &periodTx{repo: repo}}
	svc := NewPeriodService(PeriodServiceDeps{
		Repo:   repo,
		DB:     txr,
		Logger: slog.Default(),
	})
	return svc, repo
}

// seedBalancedEntries posts a balanced pair of entries for the test period:
// debit acct-hq 10000, credit acct-outlet 10000.
func seedBalancedEntries(repo *periodRepo) {
	repo.entries = append(repo.entries,
		fakeEntry{ID: "e1", PeriodID: testPeriod1, AccountID: "acct-hq", Type: "debit", Amount: 10000},
		fakeEntry{ID: "e2", PeriodID: testPeriod1, AccountID: "acct-outlet", Type: "credit", Amount: 10000},
	)
}

// seedImbalancedEntries posts unbalanced entries: debit 10000, credit 5000.
func seedImbalancedEntries(repo *periodRepo) {
	repo.entries = append(repo.entries,
		fakeEntry{ID: "e1", PeriodID: testPeriod1, AccountID: "acct-hq", Type: "debit", Amount: 10000},
		fakeEntry{ID: "e2", PeriodID: testPeriod1, AccountID: "acct-outlet", Type: "credit", Amount: 5000},
	)
}

const testRequester = "00000000-0000-0000-0000-000000000aaa"
const testApprover = "00000000-0000-0000-0000-000000000bbb"

// =============================================================================
// RequestClose tests
// =============================================================================

func TestPeriodService_RequestClose_Success(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)

	req, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)
	assert.Equal(t, period.CloseRequestPending, req.Status)
	assert.Equal(t, period.PeriodStatusClosing, repo.periods[testPeriod1].Status)
}

func TestPeriodService_RequestClose_PeriodNotOpen_Fails(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)
	repo.periods[testPeriod1].Status = period.PeriodStatusClosed

	_, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
	// Period status unchanged.
	assert.Equal(t, period.PeriodStatusClosed, repo.periods[testPeriod1].Status)
}

func TestPeriodService_RequestClose_DuplicatePending_Fails(t *testing.T) {
	t.Parallel()
	svc, _ := newSvc(t)
	_, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)

	_, err = svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrAlreadyExists))
}

// =============================================================================
// ApproveClose tests
// =============================================================================

func TestPeriodService_ApproveClose_TrialBalanceOK_ClosesWithSnapshots(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)
	seedBalancedEntries(repo)

	// Step 1: request close.
	req, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)

	// Step 2: approve.
	approved, err := svc.ApproveClose(context.Background(), ApproveCloseInput{
		RequestID:  req.ID,
		ApproverID: testApprover,
	})
	require.NoError(t, err)
	assert.Equal(t, period.CloseRequestApproved, approved.Status)
	assert.True(t, approved.TrialBalanceOK)
	assert.Equal(t, int64(10000), approved.TotalDebit.Minor())
	assert.Equal(t, int64(10000), approved.TotalCredit.Minor())
	assert.Equal(t, int64(0), approved.Imbalance.Minor())

	// Period status flipped to closed.
	assert.Equal(t, period.PeriodStatusClosed, repo.periods[testPeriod1].Status)

	// 2 snapshots (one per account that had entries).
	snaps, err := svc.ListSnapshotsByPeriod(context.Background(), testPeriod1)
	require.NoError(t, err)
	assert.Len(t, snaps, 2)

	// Verify balance signs (debit-positive).
	for _, s := range snaps {
		switch s.AccountID {
		case "acct-hq":
			assert.Equal(t, int64(10000), s.BalanceMinor, "debit account should be positive")
		case "acct-outlet":
			assert.Equal(t, int64(-10000), s.BalanceMinor, "credit account should be negative")
		}
		assert.Equal(t, "IDR", s.Currency)
		assert.Equal(t, 1, s.EntryCount)
	}
}

func TestPeriodService_ApproveClose_TrialBalanceImbalanced_Rejected(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)
	seedImbalancedEntries(repo)

	req, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)

	_, err = svc.ApproveClose(context.Background(), ApproveCloseInput{
		RequestID:  req.ID,
		ApproverID: testApprover,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrDoubleEntryViolation),
		"expected ErrDoubleEntryViolation, got: %v", err)

	// Request should be marked rejected (with reason).
	got, err := svc.GetRequest(context.Background(), req.ID)
	require.NoError(t, err)
	assert.Equal(t, period.CloseRequestRejected, got.Status)
	assert.Contains(t, got.RejectionReason, "trial balance imbalance")

	// Period status remains 'closing' (operator must investigate).
	assert.Equal(t, period.PeriodStatusClosing, repo.periods[testPeriod1].Status)

	// No snapshots generated.
	snaps, _ := svc.ListSnapshotsByPeriod(context.Background(), testPeriod1)
	assert.Len(t, snaps, 0)
}

func TestPeriodService_ApproveClose_NotPending_Fails(t *testing.T) {
	t.Parallel()
	svc, _ := newSvc(t)

	_, err := svc.ApproveClose(context.Background(), ApproveCloseInput{
		RequestID:  "00000000-0000-0000-0000-000000000ccc",
		ApproverID: testApprover,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

// =============================================================================
// RejectClose tests
// =============================================================================

func TestPeriodService_RejectClose_ReopensPeriod(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)

	req, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)

	rejected, err := svc.RejectClose(context.Background(), RejectCloseInput{
		RequestID:       req.ID,
		ApproverID:      testApprover,
		RejectionReason: "missing approval attachment",
	})
	require.NoError(t, err)
	assert.Equal(t, period.CloseRequestRejected, rejected.Status)
	assert.Equal(t, "missing approval attachment", rejected.RejectionReason)

	// Period status flipped back to 'open'.
	assert.Equal(t, period.PeriodStatusOpen, repo.periods[testPeriod1].Status)
}

func TestPeriodService_RejectClose_EmptyReason_Fails(t *testing.T) {
	t.Parallel()
	svc, _ := newSvc(t)

	req, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)

	_, err = svc.RejectClose(context.Background(), RejectCloseInput{
		RequestID:       req.ID,
		ApproverID:      testApprover,
		RejectionReason: "",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

// =============================================================================
// Reopen tests
// =============================================================================

func TestPeriodService_Reopen_ClosedPeriod_Success(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)
	seedBalancedEntries(repo)

	req, err := svc.RequestClose(context.Background(), RequestCloseInput{
		TenantID:    testTenant,
		PeriodID:    testPeriod1,
		RequesterID: testRequester,
	})
	require.NoError(t, err)
	_, err = svc.ApproveClose(context.Background(), ApproveCloseInput{
		RequestID:  req.ID,
		ApproverID: testApprover,
	})
	require.NoError(t, err)
	require.Equal(t, period.PeriodStatusClosed, repo.periods[testPeriod1].Status)

	// Snapshots remain after reopen (audit trail).
	preSnapshots, _ := svc.ListSnapshotsByPeriod(context.Background(), testPeriod1)
	require.Len(t, preSnapshots, 2)

	// Reopen.
	out, err := svc.Reopen(context.Background(), ReopenInput{
		PeriodID: testPeriod1,
		AdminID:  testApprover,
		Reason:   "found missing entry, need to correct",
	})
	require.NoError(t, err)
	assert.Equal(t, period.PeriodStatusOpen, out.Status)

	// Snapshots should still be present (audit trail).
	postSnapshots, _ := svc.ListSnapshotsByPeriod(context.Background(), testPeriod1)
	assert.Len(t, postSnapshots, 2, "snapshots should NOT be deleted on reopen")
}

func TestPeriodService_Reopen_OpenPeriod_Fails(t *testing.T) {
	t.Parallel()
	svc, _ := newSvc(t)
	_, err := svc.Reopen(context.Background(), ReopenInput{
		PeriodID: testPeriod1,
		AdminID:  testApprover,
		Reason:   "test",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

func TestPeriodService_Reopen_EmptyReason_Fails(t *testing.T) {
	t.Parallel()
	svc, repo := newSvc(t)
	repo.periods[testPeriod1].Status = period.PeriodStatusClosed

	_, err := svc.Reopen(context.Background(), ReopenInput{
		PeriodID: testPeriod1,
		AdminID:  testApprover,
		Reason:   "",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}
