//go:build !windows
// +build !windows

package usecase

// Concurrent transfer tests — verify the lock-ordering (ADR-0004) and
// idempotency behavior under concurrent load.
//
// These use the in-memory mocks (same package as transfer_service_test.go
// would be a circular import; we use usecase_test package).
//
// What we verify:
//  1. Many concurrent transfers from the same source → final balance is
//     correct (no lost updates).
//  2. Same idempotency_key across N concurrent calls → exactly 1
//     transaction recorded (no duplicate).
//  3. Both directions (A→B and B→A) under load → final balances still
//     sum to initial total (conservation of money).

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// Thread-safe test doubles (in-memory)
// =============================================================================
//
// We can't reuse mocks from transfer_service_test.go (same package),
// so we re-declare minimal thread-safe versions here.

type tsAccountRepo struct {
	mu     sync.RWMutex
	byID   map[string]*ledger.Account
	byCode map[string]*ledger.Account
}

func newTSAccountRepo() *tsAccountRepo {
	return &tsAccountRepo{
		byID:   make(map[string]*ledger.Account),
		byCode: make(map[string]*ledger.Account),
	}
}

func (m *tsAccountRepo) Create(_ context.Context, a ledger.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[a.ID] = &a
	m.byCode[a.Code] = &a
	return nil
}
func (m *tsAccountRepo) GetByID(_ context.Context, id string) (ledger.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.byID[id]
	if !ok {
		return ledger.Account{}, ledger.NotFoundForTest()
	}
	return *a, nil
}
func (m *tsAccountRepo) GetByCode(_ context.Context, code string) (ledger.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.byCode[code]
	if !ok {
		return ledger.Account{}, ledger.NotFoundForTest()
	}
	return *a, nil
}
func (m *tsAccountRepo) List(_ context.Context, _ ledger.AccountFilter) ([]ledger.Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ledger.Account, 0, len(m.byID))
	for _, a := range m.byID {
		out = append(out, *a)
	}
	return out, nil
}
func (m *tsAccountRepo) Update(_ context.Context, a ledger.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[a.ID] = &a
	m.byCode[a.Code] = &a
	return nil
}
func (m *tsAccountRepo) LockForUpdate(_ context.Context, _ ledger.Tx, id string) (ledger.Account, error) {
	return m.GetByID(context.Background(), id)
}
func (m *tsAccountRepo) UpdateBalance(_ context.Context, _ ledger.Tx, id string, newBalance money.Money) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.byID[id]
	if !ok {
		return ledger.NotFoundForTest()
	}
	a.CachedBalance = newBalance
	return nil
}

// tsTransactionRepo with idempotency tracking
type tsTransactionRepo struct {
	mu           sync.RWMutex
	transactions map[string]*ledger.Transaction
	byKey        map[string]*ledger.Transaction
	created      int64
}

func newTSTransactionRepo() *tsTransactionRepo {
	return &tsTransactionRepo{
		transactions: make(map[string]*ledger.Transaction),
		byKey:        make(map[string]*ledger.Transaction),
	}
}

func (m *tsTransactionRepo) Create(_ context.Context, _ ledger.Tx, t ledger.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byKey[t.IdempotencyKey]; exists {
		return ledger.NotFoundForTest()
	}
	m.transactions[t.ID] = &t
	m.byKey[t.IdempotencyKey] = &t
	atomic.AddInt64(&m.created, 1)
	return nil
}
func (m *tsTransactionRepo) GetByID(_ context.Context, id string, _ bool) (ledger.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transactions[id]
	if !ok {
		return ledger.Transaction{}, ledger.NotFoundForTest()
	}
	return *t, nil
}
func (m *tsTransactionRepo) GetByIdempotencyKey(_ context.Context, key string) (ledger.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.byKey[key]
	if !ok {
		return ledger.Transaction{}, ledger.NotFoundForTest()
	}
	return *t, nil
}
func (m *tsTransactionRepo) MarkPosted(_ context.Context, _ string) error  { return nil }
func (m *tsTransactionRepo) MarkFailed(_ context.Context, _ string) error  { return nil }
func (m *tsTransactionRepo) MarkReversed(_ context.Context, _ string) error { return nil }

type tsEntryRepo struct {
	mu      sync.Mutex
	entries []ledger.Entry
}

func (m *tsEntryRepo) Insert(_ context.Context, _ ledger.Tx, entries []ledger.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entries...)
	return nil
}
func (m *tsEntryRepo) ListByTransaction(_ context.Context, _ string) ([]ledger.Entry, error) {
	return nil, nil
}
func (m *tsEntryRepo) ListByAccount(_ context.Context, _ string, _ ledger.EntryFilter) ([]ledger.Entry, error) {
	return nil, nil
}
func (m *tsEntryRepo) SumForAccount(_ context.Context, _ string) (money.Money, error) {
	return money.NewFromMinor(0), nil
}

type tsTxRunner struct{}

func (tsTxRunner) ExecuteTx(_ context.Context, fn func(ledger.Tx) error) error {
	return fn(nil)
}

// =============================================================================
// Tests
// =============================================================================

func TestConcurrent_TransfersFromSameSource(t *testing.T) {
	t.Parallel()
	accountRepo := newTSAccountRepo()
	txRepo := newTSTransactionRepo()
	entryRepo := &tsEntryRepo{}
	txRunner := tsTxRunner{}

	svc := newTransferServiceFromRepos(accountRepo, txRepo, entryRepo, txRunner, t)

	srcID, dstID := seedTwoAccounts(accountRepo, 10_000_000, 0, t)

	const n = 50
	const amount = 1_000

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Transfer(context.Background(), ledger.TransferInput{
				FromAccountID:  srcID,
				ToAccountID:    dstID,
				Amount:         money.NewFromMinor(amount),
				Description:    "concurrent test",
				IdempotencyKey: "concurrent-" + uuid.NewString(),
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent transfer failed: %v", err)
	}

	src, _ := accountRepo.GetByID(context.Background(), srcID)
	dst, _ := accountRepo.GetByID(context.Background(), dstID)

	expectedSrc := int64(10_000_000 - n*amount)
	expectedDst := int64(n * amount)
	assert.Equal(t, expectedSrc, src.CachedBalance.Minor(), "source balance")
	assert.Equal(t, expectedDst, dst.CachedBalance.Minor(), "destination balance")
	assert.EqualValues(t, n, txRepo.created, "exactly n transactions")
}

func TestConcurrent_SameIdempotencyKey(t *testing.T) {
	t.Parallel()
	accountRepo := newTSAccountRepo()
	txRepo := newTSTransactionRepo()
	entryRepo := &tsEntryRepo{}
	txRunner := tsTxRunner{}

	svc := newTransferServiceFromRepos(accountRepo, txRepo, entryRepo, txRunner, t)

	srcID, dstID := seedTwoAccounts(accountRepo, 10_000_000, 0, t)

	const n = 30
	key := "same-idempotency-key"

	var wg sync.WaitGroup
	var success int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Transfer(context.Background(), ledger.TransferInput{
				FromAccountID:  srcID,
				ToAccountID:    dstID,
				Amount:         money.NewFromMinor(100),
				Description:    "idempotent test",
				IdempotencyKey: key,
			})
			if err == nil {
				atomic.AddInt64(&success, 1)
			}
		}()
	}
	wg.Wait()

	// All calls should "succeed" (either by real transfer or by replay)
	assert.EqualValues(t, n, success, "all concurrent calls should return success")

	// But ONLY ONE actual transfer should have been recorded
	assert.EqualValues(t, 1, txRepo.created, "exactly one transfer recorded")

	// And balance changed by exactly 100
	src, _ := accountRepo.GetByID(context.Background(), srcID)
	assert.Equal(t, int64(10_000_000-100), src.CachedBalance.Minor())
}

func TestConcurrent_BothDirections(t *testing.T) {
	t.Parallel()
	accountRepo := newTSAccountRepo()
	txRepo := newTSTransactionRepo()
	entryRepo := &tsEntryRepo{}
	txRunner := tsTxRunner{}

	svc := newTransferServiceFromRepos(accountRepo, txRepo, entryRepo, txRunner, t)

	srcID, dstID := seedTwoAccounts(accountRepo, 1_000_000, 1_000_000, t)
	initialTotal := int64(2_000_000)

	var wg sync.WaitGroup
	const n = 20
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			from, to := srcID, dstID
			if i%2 == 0 {
				from, to = dstID, srcID
			}
			_, _ = svc.Transfer(context.Background(), ledger.TransferInput{
				FromAccountID:  from,
				ToAccountID:    to,
				Amount:         money.NewFromMinor(50),
				Description:    "bidirectional",
				IdempotencyKey: "bi-" + uuid.NewString(),
			})
		}(i)
	}
	wg.Wait()

	src, _ := accountRepo.GetByID(context.Background(), srcID)
	dst, _ := accountRepo.GetByID(context.Background(), dstID)
	total := src.CachedBalance.Minor() + dst.CachedBalance.Minor()
	assert.Equal(t, initialTotal, total, "money must be conserved (no creation/destruction)")
}

// =============================================================================
// Helpers
// =============================================================================

func seedTwoAccounts(repo *tsAccountRepo, srcBal, dstBal int64, t *testing.T) (srcID, dstID string) {
	t.Helper()
	srcID = uuid.NewString()
	dstID = uuid.NewString()
	tenantID := uuid.NewString()
	require.NoError(t, repo.Create(context.Background(), ledger.Account{
		ID: srcID, Code: "TEST-SRC", Name: "Source", Type: ledger.AccountTypeCash,
		Status: ledger.AccountStatusActive, Currency: "IDR",
		CachedBalance: money.NewFromMinor(srcBal), TenantID: tenantID,
	}))
	require.NoError(t, repo.Create(context.Background(), ledger.Account{
		ID: dstID, Code: "TEST-DST", Name: "Dest", Type: ledger.AccountTypeOutlet,
		Status: ledger.AccountStatusActive, Currency: "IDR",
		CachedBalance: money.NewFromMinor(dstBal), TenantID: tenantID,
	}))
	return srcID, dstID
}

func newTransferServiceFromRepos(
	accounts ledger.AccountRepository,
	transactions ledger.TransactionRepository,
	entries ledger.EntryRepository,
	db TxRunner,
	t *testing.T,
) *TransferService {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(discardWriterForTest{}, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewTransferService(TransferServiceDeps{
		Accounts:     accounts,
		Transactions: transactions,
		Entries:      entries,
		DB:           db,
		Logger:       logger,
	})
}

type discardWriterForTest struct{}

func (discardWriterForTest) Write(p []byte) (int, error) { return len(p), nil }

// keep time import used
var _ = time.Second
