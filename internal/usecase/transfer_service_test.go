package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// MOCKS
// =============================================================================

type mockAccountRepo struct {
	mu       sync.Mutex
	accounts map[string]*ledger.Account
}

func newMockAccountRepo() *mockAccountRepo {
	return &mockAccountRepo{accounts: make(map[string]*ledger.Account)}
}

func (m *mockAccountRepo) Create(_ context.Context, a ledger.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.ID] = &a
	return nil
}

func (m *mockAccountRepo) GetByID(_ context.Context, id string) (ledger.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return ledger.Account{}, apperrors.ErrNotFound
	}
	return *a, nil
}

func (m *mockAccountRepo) GetByCode(_ context.Context, code string) (ledger.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.accounts {
		if a.Code == code {
			return *a, nil
		}
	}
	return ledger.Account{}, apperrors.ErrNotFound
}

func (m *mockAccountRepo) List(_ context.Context, _ ledger.AccountFilter) ([]ledger.Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ledger.Account, 0, len(m.accounts))
	for _, a := range m.accounts {
		out = append(out, *a)
	}
	return out, nil
}

func (m *mockAccountRepo) Update(_ context.Context, a ledger.Account) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accounts[a.ID] = &a
	return nil
}

func (m *mockAccountRepo) LockForUpdate(_ context.Context, _ ledger.Tx, id string) (ledger.Account, error) {
	return m.GetByID(context.Background(), id)
}

func (m *mockAccountRepo) UpdateBalance(_ context.Context, _ ledger.Tx, id string, balance money.Money) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accounts[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	a.CachedBalance = balance
	return nil
}

type mockTransactionRepo struct {
	mu           sync.Mutex
	transactions map[string]*ledger.Transaction
	byKey        map[string]*ledger.Transaction
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{
		transactions: make(map[string]*ledger.Transaction),
		byKey:        make(map[string]*ledger.Transaction),
	}
}

func (m *mockTransactionRepo) Create(_ context.Context, _ ledger.Tx, t ledger.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byKey[t.IdempotencyKey]; exists {
		return apperrors.ErrNotFound
	}
	m.transactions[t.ID] = &t
	m.byKey[t.IdempotencyKey] = &t
	return nil
}

func (m *mockTransactionRepo) GetByID(_ context.Context, id string, _ bool) (ledger.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transactions[id]
	if !ok {
		return ledger.Transaction{}, apperrors.ErrNotFound
	}
	return *t, nil
}

func (m *mockTransactionRepo) GetByIdempotencyKey(_ context.Context, key string) (ledger.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byKey[key]
	if !ok {
		return ledger.Transaction{}, apperrors.ErrNotFound
	}
	return *t, nil
}

func (m *mockTransactionRepo) MarkPosted(_ context.Context, _ string) error  { return nil }
func (m *mockTransactionRepo) MarkFailed(_ context.Context, _ string) error  { return nil }
func (m *mockTransactionRepo) MarkReversed(_ context.Context, _ string) error { return nil }

type mockEntryRepo struct {
	mu      sync.Mutex
	entries []ledger.Entry
}

func newMockEntryRepo() *mockEntryRepo { return &mockEntryRepo{} }

func (m *mockEntryRepo) Insert(_ context.Context, _ ledger.Tx, entries []ledger.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entries...)
	return nil
}

func (m *mockEntryRepo) ListByTransaction(_ context.Context, _ string) ([]ledger.Entry, error) {
	return nil, nil
}
func (m *mockEntryRepo) ListByAccount(_ context.Context, _ string, _ ledger.EntryFilter) ([]ledger.Entry, error) {
	return nil, nil
}
func (m *mockEntryRepo) SumForAccount(_ context.Context, _ string) (money.Money, error) {
	return money.NewFromMinor(0), nil
}

type mockTxRunner struct{}

func (m *mockTxRunner) ExecuteTx(_ context.Context, fn func(ledger.Tx) error) error {
	return fn(nil)
}

// =============================================================================
// Setup
// =============================================================================

func setupTransferTest(t *testing.T, srcBalance, dstBalance int64) (*TransferService, *mockAccountRepo, string, string) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	accountRepo := newMockAccountRepo()
	txRepo := newMockTransactionRepo()
	entryRepo := newMockEntryRepo()
	txRunner := &mockTxRunner{}

	svc := NewTransferService(TransferServiceDeps{
		Accounts:     accountRepo,
		Transactions: txRepo,
		Entries:      entryRepo,
		DB:           txRunner,
		Logger:       logger,
	})

	srcID := uuid.NewString()
	dstID := uuid.NewString()
	tenantID := uuid.NewString()

	src := ledger.Account{
		ID: srcID, Code: "TEST-SRC", Name: "Source",
		Type: ledger.AccountTypeCash, Status: ledger.AccountStatusActive,
		Currency: money.IDR, CachedBalance: money.NewFromMinor(srcBalance),
		TenantID: tenantID,
	}
	dst := ledger.Account{
		ID: dstID, Code: "TEST-DST", Name: "Dest",
		Type: ledger.AccountTypeOutlet, Status: ledger.AccountStatusActive,
		Currency: money.IDR, CachedBalance: money.NewFromMinor(dstBalance),
		TenantID: tenantID,
	}

	_ = accountRepo.Create(context.Background(), src)
	_ = accountRepo.Create(context.Background(), dst)

	return svc, accountRepo, srcID, dstID
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// =============================================================================
// Tests
// =============================================================================

func TestTransferService_HappyPath(t *testing.T) {
	t.Parallel()
	svc, repo, srcID, dstID := setupTransferTest(t, 100_000, 50_000)

	result, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(30_000),
		Description:    "Test transfer",
		IdempotencyKey: "key-1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, ledger.TransactionStatusPosted, result.Status)
	assert.Len(t, result.Entries, 2)

	src, _ := repo.GetByID(context.Background(), srcID)
	dst, _ := repo.GetByID(context.Background(), dstID)
	assert.Equal(t, money.NewFromMinor(70_000), src.CachedBalance)
	assert.Equal(t, money.NewFromMinor(80_000), dst.CachedBalance)
}

func TestTransferService_InsufficientBalance(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 1_000, 0)

	_, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(50_000),
		IdempotencyKey: "key-insufficient",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInsufficientBalance))
}

func TestTransferService_SameAccount(t *testing.T) {
	t.Parallel()
	svc, _, srcID, _ := setupTransferTest(t, 100_000, 0)

	_, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID: srcID, ToAccountID: srcID,
		Amount:         money.NewFromMinor(100),
		IdempotencyKey: "key-same",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

func TestTransferService_ZeroAmount(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 100_000, 0)

	_, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(0),
		IdempotencyKey: "key-zero",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
}

func TestTransferService_IdempotentReplay(t *testing.T) {
	t.Parallel()
	svc, repo, srcID, dstID := setupTransferTest(t, 100_000, 0)
	key := "idempotent-key"

	input := ledger.TransferInput{
		FromAccountID: srcID, ToAccountID: dstID,
		Amount: money.NewFromMinor(10_000), IdempotencyKey: key,
	}

	first, err := svc.Transfer(context.Background(), input)
	require.NoError(t, err)

	second, err := svc.Transfer(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID, "idempotent replay must return same transaction ID")

	src, _ := repo.GetByID(context.Background(), srcID)
	assert.Equal(t, money.NewFromMinor(90_000), src.CachedBalance, "balance debited only once")
}

func TestProperty_TransferConservation(t *testing.T) {
	t.Parallel()
	svc, repo, srcID, dstID := setupTransferTest(t, 1_000_000, 500_000)
	initialTotal := int64(1_500_000)

	transfers := []struct {
		from, to string
		amount   int64
	}{
		{srcID, dstID, 50_000},
		{dstID, srcID, 30_000},
		{srcID, dstID, 100_000},
		{dstID, srcID, 75_000},
		{srcID, dstID, 25_000},
	}

	for i, tr := range transfers {
		_, err := svc.Transfer(context.Background(), ledger.TransferInput{
			FromAccountID:  tr.from,
			ToAccountID:    tr.to,
			Amount:         money.NewFromMinor(tr.amount),
			Description:    "Property test",
			IdempotencyKey: "prop-" + time.Now().Format("150405") + "-" + uuid.NewString(),
		})
		require.NoError(t, err, "transfer %d failed", i)
	}

	src, _ := repo.GetByID(context.Background(), srcID)
	dst, _ := repo.GetByID(context.Background(), dstID)
	total := src.CachedBalance.Minor() + dst.CachedBalance.Minor()

	assert.Equal(t, initialTotal, total, "total balance must be conserved")
}
