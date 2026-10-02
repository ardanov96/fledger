// tx_extractor.go - generic pgx.Tx extractor for Sprint 45 outbox adapter.
//
// Different domain Tx types (ledger.Tx, invoice.Tx, etc.) wrap the same
// underlying pgx.Tx. The outbox adapter in cmd/api receives tx as `any`
// and needs to extract the pgx.Tx regardless of which domain wrapper
// holds it. This file provides UnwrapPgxTx that type-switches on the
// known adapter struct types.
//
// If the adapter struct changes (new tx wrapper added), extend the
// type switch below — the build will fail at compile time on the
// switch's exhaustive coverage check (Go does not check this for any,
// so we rely on tests).
package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UnwrapPgxTx extracts the underlying pgx.Tx from any of the known
// domain Tx wrappers (or a raw pgx.Tx).
//
// Used by the outbox adapter in cmd/api since Sprint 45 — the same
// OutboxWriter interface handles transfer/invoice/payment events,
// each with a different domain Tx type.
//
// To add a new domain Tx wrapper:
//   1. Create a tx_adapter_<domain>.go file with the wrapper struct
//   2. Add the struct type to the switch below
//   3. Done — outbox adapter now supports the new domain automatically
func UnwrapPgxTx(tx any) (pgx.Tx, error) {
	switch t := tx.(type) {
	case pgx.Tx:
		// Raw pgx.Tx (used by tests, direct callers)
		return t, nil
	case *txAdapter:
		return t.pgxTx, nil
	case *fraudTxAdapter:
		return t.pgxTx, nil
	case *notificationTxAdapter:
		return t.pgxTx, nil
	case *invoiceTxAdapter:
		return t.pgxTx, nil
	}
	return nil, fmt.Errorf("UnwrapPgxTx: unsupported tx type %T", tx)
}