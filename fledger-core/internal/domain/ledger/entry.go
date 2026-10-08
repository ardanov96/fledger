package ledger

import (
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// EntryType is the side of a double-entry posting.
type EntryType string

const (
	EntryTypeDebit  EntryType = "debit"
	EntryTypeCredit EntryType = "credit"
)

// Entry is an immutable ledger posting. Entries are NEVER updated or deleted
// after creation — corrections are done via reversal entries.
type Entry struct {
	ID            string
	TransactionID string
	AccountID     string
	Amount        money.Money
	Type          EntryType
	RefType       string
	RefID         string
	PeriodID      string
	Description   string
	Currency      string
	Metadata      map[string]any
	CreatedAt     time.Time

	// Hash chain (Fase 1C) — tamper detection.
	PrevHash  string
	EntryHash string
}

// SignedAmount returns the amount with sign applied (debit positive, credit negative).
func (e Entry) SignedAmount() money.Money {
	if e.Type == EntryTypeDebit {
		return e.Amount
	}
	return e.Amount.Neg()
}
