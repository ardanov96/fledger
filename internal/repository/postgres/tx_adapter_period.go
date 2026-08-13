// tx_adapter_period.go — wrapper to expose pgx.Tx as period.Tx.
//
// Each domain (ledger, invoice, period) defines its own Tx interface with
// its own CommandTag type — this is a small adapter to bridge pgx.Tx into
// the period-domain Tx shape.
//
// RunInTxPeriodDomain exposes a pgx.Tx under period.Tx.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/period"
)

// periodTxAdapter wraps pgx.Tx to satisfy period.Tx.
type periodTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapPeriodTx(tx pgx.Tx) period.Tx {
	return &periodTxAdapter{pgxTx: tx}
}

// RunInTxPeriodDomain exposes a pgx.Tx under period.Tx.
func (db *DB) RunInTxPeriodDomain(ctx context.Context, fn func(period.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		return fn(wrapPeriodTx(pgxTx))
	})
}

func (a *periodTxAdapter) Exec(ctx context.Context, sql string, args ...any) (period.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return periodPgTag{tag}, err
}

func (a *periodTxAdapter) Query(ctx context.Context, sql string, args ...any) (period.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &periodRowsAdapter{rows: rows}, nil
}

func (a *periodTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) period.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type periodPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t periodPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

type periodRowsAdapter struct {
	rows pgx.Rows
}

func (r *periodRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *periodRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *periodRowsAdapter) Err() error             { return r.rows.Err() }
func (r *periodRowsAdapter) Close()                 { r.rows.Close() }
