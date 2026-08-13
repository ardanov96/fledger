// tx_adapter_collection.go — bridge pgx.Tx ↔ collection.Tx.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/collection"
)

// ErrNotCollectionTx is returned when a collection.Tx is not actually a
// collectionTxAdapter.
var ErrNotCollectionTx = errors.New("postgres: tx is not *collectionTxAdapter")

// collectionTxAdapter wraps pgx.Tx to satisfy collection.Tx.
type collectionTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapCollectionTx(tx pgx.Tx) collection.Tx {
	return &collectionTxAdapter{pgxTx: tx}
}

// UnwrapPgxTxFromCollection extracts the underlying pgx.Tx from a collection.Tx.
func UnwrapPgxTxFromCollection(tx collection.Tx) (pgx.Tx, error) {
	a, ok := tx.(*collectionTxAdapter)
	if !ok {
		return nil, ErrNotCollectionTx
	}
	return a.pgxTx, nil
}

// RunInTxCollectionDomain runs fn inside a collection-flavored transaction.
func (db *DB) RunInTxCollectionDomain(ctx context.Context, fn func(collection.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		return fn(wrapCollectionTx(pgxTx))
	})
}

// Exec implements collection.Tx.
func (a *collectionTxAdapter) Exec(ctx context.Context, sql string, args ...any) (collection.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return collectionPgTag{tag}, err
}

// Query implements collection.Tx.
func (a *collectionTxAdapter) Query(ctx context.Context, sql string, args ...any) (collection.Rows, error) {
	rows, err := a.pgxTx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &collectionRowsAdapter{rows: rows}, nil
}

// QueryRow implements collection.Tx.
func (a *collectionTxAdapter) QueryRow(ctx context.Context, sql string, args ...any) collection.Row {
	return a.pgxTx.QueryRow(ctx, sql, args...)
}

type collectionPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t collectionPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

type collectionRowsAdapter struct{ rows pgx.Rows }

func (r *collectionRowsAdapter) Next() bool             { return r.rows.Next() }
func (r *collectionRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *collectionRowsAdapter) Err() error             { return r.rows.Err() }
func (r *collectionRowsAdapter) Close()                 { r.rows.Close() }
