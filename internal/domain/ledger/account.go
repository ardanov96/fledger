// Package ledger defines the core ledger domain — accounts, entries, and
// transactions for the FMCG Wallet.
//
// This package is interface-only; the Postgres implementation lives in
// internal/repository/postgres. Domain has zero dependencies on infrastructure
// (no SQL, no HTTP) so it can be unit-tested without spinning up services.
package ledger

import (
	"time"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// AccountType classifies an account in the chart of accounts.
type AccountType string

const (
	// AccountTypeHQ represents HQ/distributor accounts (central treasury).
	AccountTypeHQ AccountType = "hq"
	// AccountTypeOutlet represents individual outlet/retailer accounts.
	AccountTypeOutlet AccountType = "outlet"
	// AccountTypeSalesRep represents sales rep accounts (collection float).
	AccountTypeSalesRep AccountType = "sales_rep"
	// AccountTypeCustomer represents end-customer accounts (B2C).
	AccountTypeCustomer AccountType = "customer"
	// AccountTypeRevenue holds accumulated revenue (credit-normal).
	AccountTypeRevenue AccountType = "revenue"
	// AccountTypeReceivable holds outstanding invoices (debit-normal).
	AccountTypeReceivable AccountType = "receivable"
	// AccountTypePayable holds outstanding bills to suppliers (credit-normal).
	AccountTypePayable AccountType = "payable"
	// AccountTypeCash holds physical/bank cash balances.
	AccountTypeCash AccountType = "cash"
	// AccountTypeSuspense holds temporary accounts for unclear transactions.
	AccountTypeSuspense AccountType = "suspense"
)

// AccountStatus is the lifecycle state of an account.
type AccountStatus string

const (
	AccountStatusActive AccountStatus = "active"
	AccountStatusFrozen AccountStatus = "frozen"
	AccountStatusClosed AccountStatus = "closed"
)

// Account represents a ledger account in the chart of accounts.
//
// CachedBalance is a denormalized value maintained for performance; the
// authoritative balance is always recomputable as sum(entries).
type Account struct {
	ID            string
	Code          string          // human-friendly, e.g. "HQ-001", "OUTLET-JKT-12"
	Name          string
	Type          AccountType
	Status        AccountStatus
	Currency      string          // "IDR" for MVP
	CachedBalance money.Money     // maintained in same unit as entries
	OwnerID       string          // polymorphic ref to user/customer/outlet
	TenantID      string          // for multi-tenancy (Fase 5)
	Metadata      map[string]any  // extensible attributes
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// IsActive returns true if the account can post entries.
func (a Account) IsActive() bool {
	return a.Status == AccountStatusActive
}
