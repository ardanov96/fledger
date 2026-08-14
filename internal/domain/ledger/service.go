package ledger

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// TransferInput is the data needed to perform a transfer between two accounts.
type TransferInput struct {
	FromAccountID  string
	ToAccountID    string
	Amount         money.Money
	Description    string
	IdempotencyKey string
	InitiatorID    string
	RefType        string
	RefID          string
	Metadata       map[string]any

	// Cross-currency fields (Sprint 12 / Fase 1D). All optional for
	// same-currency transfers.
	//
	// ExpectedFxRateID — if set, the transfer is locked to this FX rate.
	//                       If nil and currencies differ, server looks up the
	//                       latest active rate at transfer time.
	// ExpectedRateLockAt — if set, validates against server-side lock time
	//                       (within tolerance window, e.g. 60s).
	ExpectedFxRateID    *uuid.UUID
	ExpectedRateLockAt  *time.Time
}

// TransferService orchestrates the double-entry transfer use case.
//
// The implementation (Fase 0) must:
//  1. Open a DB transaction
//  2. Lock both accounts (SELECT ... FOR UPDATE, ordered by ID to avoid deadlocks)
//  3. Verify sufficient balance on the source
//  4. Insert transaction header (status: pending)
//  5. Insert 2 entries: debit source, credit destination
//  6. Update cached balances
//  7. Mark transaction as posted
//  8. Commit
//
// All within ONE transaction. On error, rollback.
//
// See ADR-0004-locking-strategy for the deadlock-avoidance ordering.
type TransferService interface {
	Transfer(ctx context.Context, input TransferInput) (Transaction, error)
}

// BalanceService exposes balance read operations.
//
// Balance is computed two ways:
//   - Cached: read from accounts.cached_balance (fast, may be briefly stale)
//   - Authoritative: SUM(entries.signed_amount) for the account
//
// In production, use cached for display, authoritative for reconciler/audit.
type BalanceService interface {
	// GetCached returns the cached balance for an account.
	GetCached(ctx context.Context, accountID string) (money.Money, error)
	// GetAuthoritative returns the balance computed from entries.
	GetAuthoritative(ctx context.Context, accountID string) (money.Money, error)
	// Reconcile compares cached vs authoritative; returns the delta.
	// Should be zero in a healthy system. Used by Fase 1 reconciler.
	Reconcile(ctx context.Context, accountID string) (delta money.Money, err error)
}

// AccountService exposes account CRUD operations to the HTTP layer.
type AccountService interface {
	Create(ctx context.Context, account Account) (Account, error)
	GetByID(ctx context.Context, id string) (Account, error)
	GetByCode(ctx context.Context, code string) (Account, error)
	List(ctx context.Context, filter AccountFilter) ([]Account, error)
	Freeze(ctx context.Context, id, reason string) error
	Close(ctx context.Context, id, reason string) error
}
