// tx_adapter_fraud.go - wrapper to expose pgx.Tx as fraud.Tx.
//
// Mirrors the pattern from tx_adapter_notification.go. Provides
// UnwrapPgxTxFromFraud + WrapFraudTx for cross-domain writes (e.g. fraud
// scanner wants to atomically persist a flag + a notification row inside
// the same tx in the future).
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/fraud"
	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
)

// fraudTxAdapter wraps pgx.Tx to satisfy fraud.Tx (Exec only).
type fraudTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapFraudTx(tx pgx.Tx) fraud.Tx {
	return &fraudTxAdapter{pgxTx: tx}
}

// WrapFraudTx is the exported version for adapters in cmd/worker that need
// to bridge between domain Tx types.
func WrapFraudTx(tx pgx.Tx) fraud.Tx {
	return wrapFraudTx(tx)
}

// UnwrapPgxTxFromFraud extracts the underlying pgx.Tx from a fraud.Tx.
func UnwrapPgxTxFromFraud(tx fraud.Tx) (pgx.Tx, error) {
	a, ok := tx.(*fraudTxAdapter)
	if !ok {
		return nil, fmt.Errorf("postgres: fraud.Tx is not a *fraudTxAdapter; got %T", tx)
	}
	return a.pgxTx, nil
}

func (a *fraudTxAdapter) Exec(ctx context.Context, sql string, args ...any) (fraud.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return fraudPgTag{tag}, err
}

type fraudPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t fraudPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

// RunInTxFraudDomain opens a tx, binds tenant GUC (if ctx has Info), and
// runs fn. Same pattern as RunInTxNotificationDomain (Sprint 28).
//
// Sprint 31: fraud flags inherit the originating transfer's tenant_id.
// The worker attaches `*tenantctx.Info{TenantID: ev.TenantID}` before
// calling PersistDirect (mirrors the notification pattern).
func (db *DB) RunInTxFraudDomain(ctx context.Context, fn func(fraud.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		wrapped := wrapFraudTx(pgxTx)
		if info := tenantctx.InfoFromContext(ctx); info != nil {
			if err := tenantctx.SetTenantContext(ctx, wrapped, info); err != nil {
				return err
			}
		}
		return fn(wrapped)
	})
}