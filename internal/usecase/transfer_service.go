// Package usecase contains the application orchestration layer.
//
// TransferService is the canonical example of how a use case wires together
// domain logic, repository transactions, and locking to produce a correct
// and concurrency-safe financial operation.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/outbox"
	"github.com/runut/fmcg-wallet/internal/platform/money"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// TransferService implements ledger.TransferService.
//
// Algorithm (single DB transaction):
//  1. Resolve accounts (must exist, both active)
//  2. If cross-currency, lookup FX rate via CurrencyLookup and convert amount
//  3. Order account IDs lexicographically and lock BOTH with SELECT FOR UPDATE
//     (prevents deadlocks; see ADR-0004-locking-strategy)
//  4. Check sufficient balance on the source
//  5. Find or create the current accounting period (single open period for MVP)
//  6. Insert transaction header (status: pending) with idempotency_key + fx_rate_id
//  7. Insert 2 entries (debit source, credit destination) — asymmetric amounts
//     if cross-currency, each entry in its own account currency
//  8. Update cached_balance on both accounts
//  9. Mark transaction as posted
// 10. Commit
//
// On any error, rollback.
//
// Idempotency: if a transaction with the same idempotency_key already exists
// (and was posted), we return it instead of creating a new one.
//
// Cross-currency invariant (Sprint 12 / Fase 1D):
//   - SUM(debit.minor) == SUM(credit.minor) within a single currency scope.
//   - Tiap entry tetap dalam currency akun-nya; cross-currency tidak
//     memaksa "total debit == total credit" di angka absolut, tapi
//     konversi dilakukan via FX rate yang di-snapshot ke transaction.
type TransferService struct {
	accounts      ledger.AccountRepository
	transactions  ledger.TransactionRepository
	entries       ledger.EntryRepository
	db            TxRunner
	currencyLk    CurrencyLookup // Sprint 12 — nil if same-currency only
	period        PeriodResolver // Sprint 23.2 — replaces hardcoded ensureOpenPeriod stub
	outbox        OutboxWriter   // Sprint 24 / Fase 4A — transactional outbox hook
	log           *slog.Logger
}

// TxRunner is the minimal interface we need from the DB to run a transaction.
// Defined here so the service can be unit-tested with a mock DB.
type TxRunner interface {
	ExecuteTx(ctx context.Context, fn func(ledger.Tx) error) error
}

// CurrencyLookup is the narrow interface used by TransferService to resolve
// FX rates for cross-currency transfers (Sprint 12). Implemented by an
// adapter over currency.Service. May be nil if multi-currency is disabled.
type CurrencyLookup interface {
	GetLatestFxRate(ctx context.Context, tenantID uuid.UUID, from, to string, at time.Time) (currency.FxRate, error)
	GetCurrency(ctx context.Context, code string) (currency.Currency, error)
}

// PeriodResolver resolves the current open accounting period for a tenant.
// Implemented by an adapter over period.Service (see cmd/api/period_adapters.go).
//
// Sprint 23.2: this replaces the previous ensureOpenPeriod stub that returned
// a hardcoded seed UUID. The resolver runs OUTSIDE the ledger tx — race vs
// concurrent period close is acceptable because migration 000008's trigger
// blocks inserts pointing to a closed period.
type PeriodResolver interface {
	GetOrCreateOpenPeriod(ctx context.Context, tenantID string, now time.Time) (periodID string, err error)
}

// OutboxWriter abstracts the outbox insert so the use case layer stays
// decoupled from the postgres package. Implemented in cmd/api by an adapter
// that extracts the pgx.Tx from the ledger.Tx and wraps it as outbox.Tx.
//
// Sprint 24 / Fase 4A: used to write the `transfer.posted` event in the
// SAME tx as the ledger writes. If the transfer tx commits, the event is
// durable and will be published by the OutboxPublisherWorker. If it rolls
// back, the event row is never written — no orphan events.
type OutboxWriter interface {
	AppendTransferPosted(ctx context.Context, tx ledger.Tx, e outbox.Event) error
}

// TransferServiceDeps bundles all dependencies for TransferService.
type TransferServiceDeps struct {
	Accounts      ledger.AccountRepository
	Transactions  ledger.TransactionRepository
	Entries       ledger.EntryRepository
	DB            TxRunner
	CurrencyLk    CurrencyLookup   // optional; nil disables cross-currency
	Period        PeriodResolver   // optional; if nil, falls back to stubUuidPeriodResolver
	Outbox        OutboxWriter     // optional; if nil, falls back to noopOutboxWriter
	Logger        *slog.Logger
}

// NewTransferService creates a new transfer service.
func NewTransferService(deps TransferServiceDeps) *TransferService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	resolver := deps.Period
	if resolver == nil {
		resolver = stubUuidPeriodResolver{}
	}
	outboxW := deps.Outbox
	if outboxW == nil {
		outboxW = noopOutboxWriter{}
	}
	return &TransferService{
		accounts:     deps.Accounts,
		transactions: deps.Transactions,
		entries:      deps.Entries,
		db:           deps.DB,
		currencyLk:   deps.CurrencyLk,
		period:       resolver,
		outbox:       outboxW,
		log:          log,
	}
}

// stubUuidPeriodResolver is the fallback used when no PeriodResolver is wired.
// Kept for backward compat with unit tests that don't set up the period repo.
// Returns a deterministic-but-unique UUID so the seed-UUID hardcode is no
// longer the only behavior.
type stubUuidPeriodResolver struct{}

func (stubUuidPeriodResolver) GetOrCreateOpenPeriod(_ context.Context, _ string, _ time.Time) (string, error) {
	return uuid.NewString(), nil
}

// noopOutboxWriter is the fallback used when no OutboxWriter is wired.
// Used by unit tests that don't care about the outbox side-effect. Returns
// nil so the transfer succeeds normally.
type noopOutboxWriter struct{}

func (noopOutboxWriter) AppendTransferPosted(_ context.Context, _ ledger.Tx, _ outbox.Event) error {
	return nil
}

// Transfer performs a double-entry transfer between two accounts.
func (s *TransferService) Transfer(ctx context.Context, input ledger.TransferInput) (ledger.Transaction, error) {
	// 0. Validate input
	if err := s.validateInput(input); err != nil {
		return ledger.Transaction{}, err
	}

	// 1. Idempotency check (outside the tx — fast path).
	existing, err := s.transactions.GetByIdempotencyKey(ctx, input.IdempotencyKey)
	if err == nil {
		s.log.Info("transfer: idempotent replay",
			"idempotency_key", input.IdempotencyKey,
			"transaction_id", existing.ID,
		)
		return existing, nil
	}
	if !errors.Is(err, apperrors.ErrNotFound) && !errors.Is(err, apperrors.ErrIdempotencyConflict) {
		return ledger.Transaction{}, fmt.Errorf("check idempotency: %w", err)
	}
	if errors.Is(err, apperrors.ErrIdempotencyConflict) {
		return ledger.Transaction{}, fmt.Errorf("check idempotency: %w", apperrors.ErrIdempotencyConflict)
	}
	// else: not found — proceed to create new

	// 2. Resolve accounts
	srcBefore, err := s.accounts.GetByID(ctx, input.FromAccountID)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("load source account: %w", err)
	}
	dstBefore, err := s.accounts.GetByID(ctx, input.ToAccountID)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("load dest account: %w", err)
	}

	if srcBefore.Status != ledger.AccountStatusActive {
		return ledger.Transaction{}, apperrors.ErrAccountFrozen
	}
	if dstBefore.Status != ledger.AccountStatusActive {
		return ledger.Transaction{}, apperrors.ErrAccountFrozen
	}

	// 2b. Cross-currency path (Sprint 12 / Fase 1D).
	// If currencies differ, we need an FX rate lookup. If currencies are
	// the same, this is a no-op (same-currency transfer).
	var (
		fxRate        *currency.FxRate
		fxRateLockAt  *time.Time
		toAmount      money.Money
		isCrossCur    bool
	)
	if srcBefore.Currency != dstBefore.Currency {
		if s.currencyLk == nil {
			return ledger.Transaction{}, fmt.Errorf(
				"%w: cross-currency transfer requires CurrencyLookup (multi-currency not enabled)",
				apperrors.ErrCurrencyMismatch,
			)
		}
		isCrossCur = true

		// If client pinned an FX rate, validate it; else lookup latest active.
		var rate currency.FxRate
		var err error
		if input.ExpectedFxRateID != nil {
			// Validate pinned rate matches the (from, to) pair.
			if !input.ExpectedRateLockAt.IsZero() && time.Since(*input.ExpectedRateLockAt).Abs() > 5*time.Minute {
				return ledger.Transaction{}, fmt.Errorf(
					"%w: expected_rate_lock_at outside tolerance window",
					currency.ErrInvalidWindow,
				)
			}
			// Pinned path: try GetLatestFxRate at the client-specified time;
			// if not available at that time, return not-found.
			at := time.Now().UTC()
			if input.ExpectedRateLockAt != nil {
				at = *input.ExpectedRateLockAt
			}
			rate, err = s.currencyLk.GetLatestFxRate(ctx, parseUUID(srcBefore.TenantID), srcBefore.Currency, dstBefore.Currency, at)
		} else {
			rate, err = s.currencyLk.GetLatestFxRate(ctx, parseUUID(srcBefore.TenantID), srcBefore.Currency, dstBefore.Currency, time.Now().UTC())
		}
		if err != nil {
			return ledger.Transaction{}, fmt.Errorf("fx rate lookup: %w", err)
		}

		// Convert amount from source currency to destination currency.
		fromCur, err := s.currencyLk.GetCurrency(ctx, srcBefore.Currency)
		if err != nil {
			return ledger.Transaction{}, fmt.Errorf("load source currency: %w", err)
		}
		toCur, err := s.currencyLk.GetCurrency(ctx, dstBefore.Currency)
		if err != nil {
			return ledger.Transaction{}, fmt.Errorf("load dest currency: %w", err)
		}
		converted, err := money.Convert(input.Amount, fromCur.DecimalPlaces, toCur.DecimalPlaces, rate.Rate)
		if err != nil {
			return ledger.Transaction{}, fmt.Errorf("convert amount: %w", err)
		}
		fxRate = &rate
		lockAt := time.Now().UTC()
		fxRateLockAt = &lockAt
		toAmount = converted
	} else {
		toAmount = input.Amount
	}

	// 3. Generate ID up front
	txID := uuid.NewString()
	now := time.Now().UTC()

	// 3a. Resolve period BEFORE opening the ledger tx.
	// Sprint 23.2: this replaces the old hardcoded seed UUID stub. The period
	// is read once here (outside any tx) and reused inside the tx. Race vs
	// concurrent period close is acceptable for MVP — migration 000008's
	// trigger blocks inserts pointing to a closed period.
	periodID, err := s.period.GetOrCreateOpenPeriod(ctx, srcBefore.TenantID, now)
	if err != nil {
		return ledger.Transaction{}, fmt.Errorf("resolve open period: %w", err)
	}

	var result ledger.Transaction

	err = s.db.ExecuteTx(ctx, func(tx ledger.Tx) error {
		// Sprint 15: RLS GUC vars are auto-bound by RunInTxDomain in tx_adapter.go
		// (sees *tenantctx.Info from context, sets Postgres session vars).
		// No per-closure binding needed here.

		// 4a. Lock both accounts (deterministic order to avoid deadlocks)
		firstID, secondID := input.FromAccountID, input.ToAccountID
		if firstID > secondID {
			firstID, secondID = secondID, firstID
		}

		first, err := s.accounts.LockForUpdate(ctx, tx, firstID)
		if err != nil {
			return fmt.Errorf("lock first account: %w", err)
		}
		second, err := s.accounts.LockForUpdate(ctx, tx, secondID)
		if err != nil {
			return fmt.Errorf("lock second account: %w", err)
		}

		var src, dst ledger.Account
		if first.ID == input.FromAccountID {
			src, dst = first, second
		} else {
			src, dst = second, first
		}

		// 4b. Re-validate under lock
		if src.Currency != dst.Currency && !isCrossCur {
			return apperrors.ErrCurrencyMismatch
		}
		if src.Status != ledger.AccountStatusActive {
			return apperrors.ErrAccountFrozen
		}
		if dst.Status != ledger.AccountStatusActive {
			return apperrors.ErrAccountFrozen
		}

		// 4c. Sufficient balance (in source currency)
		if src.CachedBalance < input.Amount {
			return fmt.Errorf(
				"insufficient balance on account %s: have %s, need %s: %w",
				src.ID, src.CachedBalance, input.Amount, apperrors.ErrInsufficientBalance,
			)
		}

		// 4d. Insert transaction header
		newBalance := src.CachedBalance.Sub(input.Amount)
		dstBalance := dst.CachedBalance.Add(toAmount)

		txn := ledger.Transaction{
			ID:             txID,
			IdempotencyKey: input.IdempotencyKey,
			Status:         ledger.TransactionStatusPending,
			Description:    input.Description,
			RefType:        input.RefType,
			RefID:          input.RefID,
			InitiatorID:    input.InitiatorID,
			TenantID:       src.TenantID,
			PeriodID:       periodID,
			Metadata:       input.Metadata,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		// Cross-currency snapshot fields (Sprint 12).
		if isCrossCur && fxRate != nil {
			txn.FxRateID = &fxRate.ID
			txn.FxRateLockedAt = fxRateLockAt
		}
		if err := s.transactions.Create(ctx, tx, txn); err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}

		// 4f. Insert 2 entries (double-entry, asymmetric if cross-currency).
		// For same-currency: debit = credit = input.Amount.
		// For cross-currency: debit = input.Amount (src.Currency),
		//                    credit = toAmount (dst.Currency).
		entries := []ledger.Entry{
			{
				ID:            uuid.NewString(),
				TransactionID: txID,
				AccountID:     src.ID,
				Amount:        input.Amount,
				Type:          ledger.EntryTypeDebit,
				RefType:       input.RefType,
				RefID:         input.RefID,
				PeriodID:      periodID,
				Description:   "Transfer out: " + input.Description,
				Currency:      src.Currency,
				Metadata:      input.Metadata,
				CreatedAt:     now,
			},
			{
				ID:            uuid.NewString(),
				TransactionID: txID,
				AccountID:     dst.ID,
				Amount:        toAmount,
				Type:          ledger.EntryTypeCredit,
				RefType:       input.RefType,
				RefID:         input.RefID,
				PeriodID:      periodID,
				Description:   "Transfer in: " + input.Description,
				Currency:      dst.Currency,
				Metadata:      input.Metadata,
				CreatedAt:     now,
			},
		}
		if err := s.entries.Insert(ctx, tx, entries); err != nil {
			return fmt.Errorf("insert entries: %w", err)
		}

		// 4g. Update cached balances (each in its own currency).
		if err := s.accounts.UpdateBalance(ctx, tx, src.ID, newBalance); err != nil {
			return fmt.Errorf("update source balance: %w", err)
		}
		if err := s.accounts.UpdateBalance(ctx, tx, dst.ID, dstBalance); err != nil {
			return fmt.Errorf("update dest balance: %w", err)
		}

		// 4h. Mark transaction as posted
		if err := s.transactions.MarkPosted(ctx, txID); err != nil {
			return fmt.Errorf("mark posted: %w", err)
		}

		// 4i. Outbox event (Sprint 24 / Fase 4A).
		// Atomic with the transfer write — if any of the above fails, this
		// never executes and no orphan event is published.
		event := s.buildTransferPostedEvent(txn, entries, src, dst, input.Amount, now)
		if err := s.outbox.AppendTransferPosted(ctx, tx, event); err != nil {
			return fmt.Errorf("append transfer.posted outbox event: %w", err)
		}

		postedAt := now
		result = txn
		result.Status = ledger.TransactionStatusPosted
		result.PostedAt = &postedAt
		result.FxRateID = txn.FxRateID
		result.FxRateLockedAt = txn.FxRateLockedAt
		result.Entries = entries

		s.log.Info("transfer completed",
			"transaction_id", txID,
			"from", src.ID,
			"to", dst.ID,
			"amount", input.Amount.String(),
		)
		return nil
	})

	if err != nil {
		s.log.Error("transfer failed",
			"error", err,
			"from", input.FromAccountID,
			"to", input.ToAccountID,
		)
		return ledger.Transaction{}, err
	}

	return result, nil
}

func (s *TransferService) validateInput(input ledger.TransferInput) error {
	if input.FromAccountID == "" {
		return fmt.Errorf("%w: FromAccountID required", apperrors.ErrInvalidInput)
	}
	if input.ToAccountID == "" {
		return fmt.Errorf("%w: ToAccountID required", apperrors.ErrInvalidInput)
	}
	if input.FromAccountID == input.ToAccountID {
		return fmt.Errorf("%w: cannot transfer to same account", apperrors.ErrInvalidInput)
	}
	if !input.Amount.IsPositive() {
		return fmt.Errorf("%w: amount must be positive, got %s", apperrors.ErrInvalidInput, input.Amount)
	}
	return nil
}

// parseUUID parses a string tenantID into uuid.UUID. Returns uuid.Nil on
// parse failure — caller is expected to use uuid.Nil as a "missing tenant"
// sentinel. Sprint 12 helper for cross-currency FX rate lookup.
func parseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// buildTransferPostedEvent constructs the outbox event for a posted transfer.
// Payload is intentionally flat + self-describing so subscribers can parse
// without needing the domain types. subject/event_type follow the standard
// format (see outbox.SubjectTransferPosted / outbox.EventTransferPosted).
func (s *TransferService) buildTransferPostedEvent(
	txn ledger.Transaction,
	entries []ledger.Entry,
	src, dst ledger.Account,
	amount money.Money,
	now time.Time,
) outbox.Event {
	tenantID, _ := uuid.Parse(src.TenantID)

	entryPayloads := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entryPayloads = append(entryPayloads, map[string]any{
			"entry_id":     e.ID,
			"account_id":   e.AccountID,
			"amount_minor": e.Amount.Minor(),
			"currency":     e.Currency,
			"type":         string(e.Type),
			"period_id":    e.PeriodID,
		})
	}

	payload, _ := json.Marshal(map[string]any{
		"transaction_id":  txn.ID,
		"tenant_id":       txn.TenantID,
		"period_id":       txn.PeriodID,
		"idempotency_key": txn.IdempotencyKey,
		"description":     txn.Description,
		"ref_type":        txn.RefType,
		"ref_id":          txn.RefID,
		"initiator_id":    txn.InitiatorID,
		"from_account_id": src.ID,
		"to_account_id":   dst.ID,
		"amount_minor":    amount.Minor(),
		"currency":        src.Currency,
		"is_cross_cur":    src.Currency != dst.Currency,
		"posted_at":       now.Format(time.RFC3339Nano),
		"entries":         entryPayloads,
	})

	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	return outbox.Event{
		ID:            uuid.New(),
		TenantID:      tenantID,
		AggregateType: outbox.AggregateTransfer,
		AggregateID:   uuid.MustParse(txn.ID),
		EventType:     outbox.EventTransferPosted,
		Subject:       outbox.SubjectTransferPosted,
		Payload:       payloadMap,
		CreatedAt:     now,
	}
}
