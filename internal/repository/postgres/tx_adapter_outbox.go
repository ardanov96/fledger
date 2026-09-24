// tx_adapter_outbox.go — wrapper to expose pgx.Tx as outbox.Tx.
//
// Mirrors the pattern from tx_adapter_period.go and tx_adapter_invoice.go.
// Provides UnwrapPgxTxFromOutbox for tests/adaptors that need the raw pgx.Tx.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/outbox"
)

// outboxTxAdapter wraps pgx.Tx to satisfy outbox.Tx (Exec only — the outbox
// domain only writes inside a business tx; reads use the pool).
type outboxTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapOutboxTx(tx pgx.Tx) outbox.Tx {
	return &outboxTxAdapter{pgxTx: tx}
}

// WrapOutboxTx is the exported version of wrapOutboxTx for use by adapters
// in cmd/api that need to bridge between domain Tx types.
func WrapOutboxTx(tx pgx.Tx) outbox.Tx {
	return wrapOutboxTx(tx)
}

// UnwrapPgxTxFromOutbox extracts the underlying pgx.Tx from an outbox.Tx.
// Useful when a use case needs to run an outbox op alongside ledger ops in
// the same pgx.Tx (e.g. cross-domain writes inside a single transaction).
func UnwrapPgxTxFromOutbox(tx outbox.Tx) (pgx.Tx, error) {
	a, ok := tx.(*outboxTxAdapter)
	if !ok {
		return nil, fmt.Errorf("postgres: outbox.Tx is not a *outboxTxAdapter; got %T", tx)
	}
	return a.pgxTx, nil
}

func (a *outboxTxAdapter) Exec(ctx context.Context, sql string, args ...any) (outbox.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return outboxPgTag{tag}, err
}

type outboxPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t outboxPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

// UnwrapPgxTxFromLedger is the inverse: given a ledger.Tx, return the pgx.Tx
// underneath. Used by cross-domain writes inside the ledger tx (e.g. the
// TransferService hook in Sprint 24 writes an outbox event in the same tx
// as the ledger writes — it extracts the pgx.Tx here and wraps it as
// outbox.Tx via wrapOutboxTx).
func UnwrapPgxTxFromLedger(tx any) (pgx.Tx, error) {
	a, ok := tx.(*txAdapter)
	if !ok {
		return nil, fmt.Errorf("postgres: ledger.Tx is not a *txAdapter; got %T", tx)
	}
	return a.pgxTx, nil
}
