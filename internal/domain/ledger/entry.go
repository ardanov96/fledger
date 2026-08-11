package ledger

import (
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// EntryType is the side of a double-entry posting.
type EntryType string

const (
	// EntryTypeDebit increases asset/expense accounts, decreases liability/equity/revenue.
	EntryTypeDebit EntryType = "debit"
	// EntryTypeCredit decreases asset/expense accounts, increases liability/equity/revenue.
	EntryTypeCredit EntryType = "credit"
)

// Entry is an immutable ledger posting. Entries are NEVER updated or deleted
// after creation — corrections are done via reversal entries.
type Entry struct {
	ID            string
	TransactionID string         // groups entries that belong to the same business event
	AccountID     string
	Amount        money.Money    // always positive; type conveys direction
	Type          EntryType
	RefType       string         // e.g. "invoice", "payment", "collection", "write_off"
	RefID         string         // foreign key to the business entity
	PeriodID      string         // accounting period this entry belongs to
	Description   string
	Currency      string
	Metadata      map[string]any // extensible, e.g. {"route_id":"...", "outlet_id":"..."}
	CreatedAt     time.Time
}

// SignedAmount returns the amount with sign applied (debit positive, credit negative).
// Useful for sum calculations: total balance = sum(signed_amounts).
func (e Entry) SignedAmount() money.Money {
	if e.Type == EntryTypeDebit {
		return e.Amount
	}
	return e.Amount.Neg()
}
