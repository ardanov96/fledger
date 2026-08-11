package ledger

import (
	"context"

	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// AccountRepository defines persistence operations for accounts.
type AccountRepository interface {
	Create(ctx context.Context, account Account) error
	GetByID(ctx context.Context, id string) (Account, error)
	GetByCode(ctx context.Context, code string) (Account, error)
	List(ctx context.Context, filter AccountFilter) ([]Account, error)
	Update(ctx context.Context, account Account) error
	LockForUpdate(ctx context.Context, tx Tx, id string) (Account, error)
	UpdateBalance(ctx context.Context, tx Tx, id string, balance money.Money) error
}

// EntryRepository defines persistence operations for ledger entries.
type EntryRepository interface {
	Insert(ctx context.Context, tx Tx, entries []Entry) error
	ListByTransaction(ctx context.Context, transactionID string) ([]Entry, error)
	ListByAccount(ctx context.Context, accountID string, filter EntryFilter) ([]Entry, error)
	SumForAccount(ctx context.Context, accountID string) (money.Money, error)
}

// TransactionRepository defines persistence operations for transactions.
type TransactionRepository interface {
	Create(ctx context.Context, tx Tx, transaction Transaction) error
	GetByID(ctx context.Context, id string, withEntries bool) (Transaction, error)
	GetByIdempotencyKey(ctx context.Context, key string) (Transaction, error)
	MarkPosted(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string) error
	MarkReversed(ctx context.Context, id string) error
}

// AccountFilter holds filter criteria for List operations.
type AccountFilter struct {
	Type     AccountType
	Status   AccountStatus
	TenantID string
	Limit    int
	Cursor   string
}

// EntryFilter holds filter criteria for entry listing.
type EntryFilter struct {
	RefType string
	RefID   string
	From    int64
	To      int64
	Limit   int
	Cursor  string
}

// =============================================================================
// Transaction abstraction
// =============================================================================
//
// Tx is intentionally MINIMAL — only the methods our repos actually need
// for the non-batch use case. For batch operations, repos access the
// concrete pgx.Tx via type assertion (in the postgres package).
//
// This keeps the domain layer free of pgx dependencies.

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// CommandTag is the result of Exec.
type CommandTag interface {
	RowsAffected() int64
}

// Row is a single row from QueryRow.
type Row interface {
	Scan(dest ...any) error
}

// Rows is an iterator of rows.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}
