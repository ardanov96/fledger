// tx_adapter_auth.go — bridge pgx.Tx ↔ auth.Tx.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/auth"
)

// ErrNotAuthTx is returned when an auth.Tx is not actually an authTxAdapter.
var ErrNotAuthTx = errors.New("postgres: tx is not *authTxAdapter")

// authTxAdapter wraps pgx.Tx to satisfy auth.Tx.
type authTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapAuthTx(tx pgx.Tx) auth.Tx {
	return &authTxAdapter{pgxTx: tx}
}

// UnwrapPgxTxFromAuth extracts the underlying pgx.Tx from an auth.Tx.
func UnwrapPgxTxFromAuth(tx auth.Tx) (pgx.Tx, error) {
	a, ok := tx.(*authTxAdapter)
	if !ok {
		return nil, ErrNotAuthTx
	}
	return a.pgxTx, nil
}

// RunInTxAuthDomain runs fn inside an auth-flavored transaction.
func (db *DB) RunInTxAuthDomain(ctx context.Context, fn func(auth.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		return fn(wrapAuthTx(pgxTx))
	})
}

// Exec implements auth.Tx.
func (a *authTxAdapter) Exec(ctx context.Context, sql string, args ...any) (auth.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return authPgTag{tag}, err
}

// Query implements auth.Tx.
func (a *authTxAdapter) Query(ctx context.Context, sql string, args ...any) (auth.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &authRowsAdapter{rows: rows}, nil
}

// QueryRow implements auth.Tx.
func (a *authTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) auth.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type authPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t authPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

type authRowsAdapter struct{ rows pgx.Rows }

func (r *authRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *authRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *authRowsAdapter) Err() error             { return r.rows.Err() }
func (r *authRowsAdapter) Close()                 { r.rows.Close() }
