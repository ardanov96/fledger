package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
)

// =============================================================================
// txAdapter — wraps pgx.Tx to satisfy the domain's ledger.Tx interface.
// =============================================================================

type txAdapter struct {
	pgxTx pgx.Tx
}

func wrapTx(tx pgx.Tx) ledger.Tx {
	return &txAdapter{pgxTx: tx}
}

func pgxTxOrPanic(tx ledger.Tx) pgx.Tx {
	a, ok := tx.(*txAdapter)
	if !ok {
		panic("postgres: ledger.Tx is not a *txAdapter; this is a bug")
	}
	return a.pgxTx
}

func (a *txAdapter) Exec(ctx context.Context, sql string, args ...any) (ledger.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return pgxCommandTag{tag}, err
}

func (a *txAdapter) Query(ctx context.Context, sql string, args ...any) (ledger.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	return rows, err
}

func (a *txAdapter) QueryRow(ctx context.Context, sql string, args ...any) ledger.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type pgxCommandTag struct {
	tag interface{ RowsAffected() int64 }
}

func (c pgxCommandTag) RowsAffected() int64 { return c.tag.RowsAffected() }

// RunInTxDomain runs fn inside a transaction, exposing a ledger.Tx.
func (db *DB) RunInTxDomain(ctx context.Context, fn func(ledger.Tx) error) error {
	return db.RunInTx(ctx, func(pgxTx pgx.Tx) error {
		return fn(wrapTx(pgxTx))
	})
}
