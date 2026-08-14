package ledger

import (
	"time"

	"github.com/google/uuid"
)

// TransactionStatus is the lifecycle state of a multi-entry transaction.
type TransactionStatus string

const (
	// TransactionStatusPending — created but not yet committed.
	TransactionStatusPending TransactionStatus = "pending"
	// TransactionStatusPosted — successfully committed to ledger.
	TransactionStatusPosted TransactionStatus = "posted"
	// TransactionStatusFailed — errored, will not be committed.
	TransactionStatusFailed TransactionStatus = "failed"
	// TransactionStatusReversed — posted then fully reversed.
	TransactionStatusReversed TransactionStatus = "reversed"
)

// Transaction groups entries that belong to a single business event
// (e.g. one invoice payment, one collection, one transfer).
//
// A transaction must satisfy the double-entry invariant: SUM(debit) == SUM(credit).
type Transaction struct {
	ID            string
	IdempotencyKey string         // dedupe retries from client
	Status        TransactionStatus
	Description   string
	RefType       string         // e.g. "invoice_payment", "transfer", "collection"
	RefID         string         // FK to business entity
	InitiatorID   string         // user who initiated
	TenantID      string         // for multi-tenancy
	PeriodID      string         // accounting period
	Metadata      map[string]any // extensible
	PostedAt      *time.Time     // nil if not yet posted
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Cross-currency snapshot (Sprint 12 / Fase 1D). NULL when same-currency.
	// fx_rate_locked_at is the time at which the rate was frozen to the txn.
	FxRateID        *uuid.UUID
	FxRateLockedAt  *time.Time

	// Entries are populated when loading a transaction; not stored on the
	// transaction row itself (they live in the entries table).
	Entries []Entry `json:"entries,omitempty"`
}
