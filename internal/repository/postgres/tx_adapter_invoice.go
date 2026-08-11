// Wrapper to expose pgx.Tx as invoice.Tx (different return-type wrappers).
//
// ledger.Tx uses ledger.CommandTag, invoice.Tx uses invoice.CommandTag — both
// wrap pgconn.CommandTag but Go treats them as distinct types, so we need a
// tiny adapter per domain to keep the two Tx interfaces structurally compatible.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
)

// invoiceTxAdapter wraps pgx.Tx to satisfy invoice.Tx.
//
// The underlying pgx.Tx is the same one produced by db.runInTx; only the
// interface return types differ (invoice.CommandTag vs ledger.CommandTag).
type invoiceTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapInvoiceTx(tx pgx.Tx) invoice.Tx {
	return &invoiceTxAdapter{pgxTx: tx}
}

// RunInTxInvoiceDomain exposes a pgx.Tx under invoice.Tx.
func (db *DB) RunInTxInvoiceDomain(ctx context.Context, fn func(invoice.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		return fn(wrapInvoiceTx(pgxTx))
	})
}

func (a *invoiceTxAdapter) Exec(ctx context.Context, sql string, args ...any) (invoice.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return invoicePgTag{tag}, err
}

func (a *invoiceTxAdapter) Query(ctx context.Context, sql string, args ...any) (invoice.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &invoiceRowsAdapter{rows: rows}, nil
}

func (a *invoiceTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) invoice.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type invoicePgTag struct{ tag interface{ RowsAffected() int64 } }

func (t invoicePgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

type invoiceRowsAdapter struct {
	rows pgx.Rows
}

func (r *invoiceRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *invoiceRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *invoiceRowsAdapter) Err() error             { return r.rows.Err() }
func (r *invoiceRowsAdapter) Close()                 { r.rows.Close() }
