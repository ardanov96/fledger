// tx_adapter_reconciler.go — bridge pgx.Tx ↔ reconciler.Tx.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/reconciler"
)

// ErrNotReconcilerTx is returned when a reconciler.Tx is not actually a
// reconcilerTxAdapter (e.g. when callers accidentally pass a different
// domain's Tx such as invoice.Tx or period.Tx).
var ErrNotReconcilerTx = errors.New("postgres: tx is not *reconcilerTxAdapter")

// reconcilerTxAdapter wraps pgx.Tx to satisfy reconciler.Tx.
type reconcilerTxAdapter struct {
	pgxTx pgx.Tx
}

// wrapReconcilerTx wraps pgx.Tx into a reconciler.Tx.
func wrapReconcilerTx(tx pgx.Tx) reconciler.Tx {
	return &reconcilerTxAdapter{pgxTx: tx}
}

// UnwrapPgxTxFromReconciler extracts the underlying pgx.Tx from a reconciler.Tx.
// Returns ErrNotReconcilerTx if the input is not a reconcilerTxAdapter.
//
// Use this when you need to run SQL directly with pgx while staying inside the
// reconciler-flavored transaction (e.g. for trial-balance aggregate queries).
func UnwrapPgxTxFromReconciler(tx reconciler.Tx) (pgx.Tx, error) {
	a, ok := tx.(*reconcilerTxAdapter)
	if !ok {
		return nil, ErrNotReconcilerTx
	}
	return a.pgxTx, nil
}

// RunInTxReconcilerDomain runs fn inside a reconciler-flavored transaction.
func (db *DB) RunInTxReconcilerDomain(ctx context.Context, fn func(reconciler.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		return fn(wrapReconcilerTx(pgxTx))
	})
}

// Exec implements reconciler.Tx.
func (a *reconcilerTxAdapter) Exec(ctx context.Context, sql string, args ...any) (reconciler.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return reconcilerPgTag{tag}, err
}

// Query implements reconciler.Tx.
func (a *reconcilerTxAdapter) Query(ctx context.Context, sql string, args ...any) (reconciler.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &reconcilerRowsAdapter{rows: rows}, nil
}

// QueryRow implements reconciler.Tx.
func (a *reconcilerTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) reconciler.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type reconcilerPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t reconcilerPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

type reconcilerRowsAdapter struct {
	rows pgx.Rows
}

func (r *reconcilerRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *reconcilerRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *reconcilerRowsAdapter) Err() error             { return r.rows.Err() }
func (r *reconcilerRowsAdapter) Close()                 { r.rows.Close() }
