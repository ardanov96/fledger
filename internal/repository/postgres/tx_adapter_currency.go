// tx_adapter_currency.go — bridge pgx.Tx � currency.Tx.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/currency"
	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
)

// ErrNotCurrencyTx is returned when a currency.Tx is not actually a
// currencyTxAdapter.
var ErrNotCurrencyTx = errors.New("postgres: tx is not *currencyTxAdapter")

// currencyTxAdapter wraps pgx.Tx to satisfy currency.Tx.
type currencyTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapCurrencyTx(tx pgx.Tx) currency.Tx {
	return &currencyTxAdapter{pgxTx: tx}
}

// UnwrapPgxTxFromCurrency extracts the underlying pgx.Tx from a currency.Tx.
func UnwrapPgxTxFromCurrency(tx currency.Tx) (pgx.Tx, error) {
	a, ok := tx.(*currencyTxAdapter)
	if !ok {
		return nil, ErrNotCurrencyTx
	}
	return a.pgxTx, nil
}

// RunInTxCurrencyDomain runs fn inside a currency-flavored transaction.
// Sprint 15: bind RLS GUC vars at tx start (currency is shared lookup
// data so this is mostly for future-proofing; tx binds tenant id anyway).
func (db *DB) RunInTxCurrencyDomain(ctx context.Context, fn func(currency.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		wrapped := wrapCurrencyTx(pgxTx)
		if err := tenantctx.SetTenantContext(ctx, wrapped, tenantctx.InfoFromContext(ctx)); err != nil {
			return err
		}
		return fn(wrapped)
	})
}

// Exec implements currency.Tx.
func (a *currencyTxAdapter) Exec(ctx context.Context, sql string, args ...any) (currency.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return currencyPgTag{tag}, err
}

// Query implements currency.Tx.
func (a *currencyTxAdapter) Query(ctx context.Context, sql string, args ...any) (currency.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &currencyRowsAdapter{rows: rows}, nil
}

// QueryRow implements currency.Tx.
func (a *currencyTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) currency.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type currencyPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t currencyPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

type currencyRowsAdapter struct{ rows pgx.Rows }

func (r *currencyRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *currencyRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *currencyRowsAdapter) Err() error             { return r.rows.Err() }
func (r *currencyRowsAdapter) Close()                 { r.rows.Close() }
