// tx_adapter_notification.go - wrapper to expose pgx.Tx as notification.Tx.
//
// Mirrors the pattern from tx_adapter_outbox.go. Provides
// UnwrapPgxTxFromNotification + WrapNotificationTx for cross-domain writes.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/notification"
	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
)

// notificationTxAdapter wraps pgx.Tx to satisfy notification.Tx (Exec only).
type notificationTxAdapter struct {
	pgxTx pgx.Tx
}

func wrapNotificationTx(tx pgx.Tx) notification.Tx {
	return &notificationTxAdapter{pgxTx: tx}
}

// WrapNotificationTx is the exported version for adapters in cmd/api that need
// to bridge between domain Tx types.
func WrapNotificationTx(tx pgx.Tx) notification.Tx {
	return wrapNotificationTx(tx)
}

// UnwrapPgxTxFromNotification extracts the underlying pgx.Tx from a notification.Tx.
func UnwrapPgxTxFromNotification(tx notification.Tx) (pgx.Tx, error) {
	a, ok := tx.(*notificationTxAdapter)
	if !ok {
		return nil, fmt.Errorf("postgres: notification.Tx is not a *notificationTxAdapter; got %T", tx)
	}
	return a.pgxTx, nil
}

func (a *notificationTxAdapter) Exec(ctx context.Context, sql string, args ...any) (notification.CommandTag, error) {
	tag, err := a.pgxTx.Exec(ctx, sql, args...)
	return notificationPgTag{tag}, err
}

type notificationPgTag struct{ tag interface{ RowsAffected() int64 } }

func (t notificationPgTag) RowsAffected() int64 { return t.tag.RowsAffected() }

// UnwrapPgxTxFromLedger is the inverse: given a ledger.Tx, return the pgx.Tx
// underneath. Used by cross-domain writes (e.g. NotificationWorker creating
// notifications inside a ledger tx).
func (db *DB) RunInTxNotificationDomain(ctx context.Context, fn func(notification.Tx) error) error {
	return db.runInTx(ctx, defaultTxOpts, func(pgxTx pgx.Tx) error {
		wrapped := wrapNotificationTx(pgxTx)
		if err := tenantctx.SetTenantContext(ctx, wrapped, tenantctx.InfoFromContext(ctx)); err != nil {
			return err
		}
		return fn(wrapped)
	})
}
