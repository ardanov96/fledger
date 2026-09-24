// outbox_adapter.go — Sprint 24 / Fase 4A.
//
// Bridges usecase.OutboxWriter to postgres.OutboxRepository. Extracts the
// underlying pgx.Tx from the ledger.Tx (passed into the business tx closure)
// and wraps it as outbox.Tx so the outbox insert runs in the same tx as
// the ledger writes. This is the cross-domain write pattern.
package main

import (
	"context"
	"fmt"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/domain/outbox"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// outboxWriterAdapter implements usecase.OutboxWriter over the postgres
// OutboxRepository.
type outboxWriterAdapter struct {
	repo *postgres.OutboxRepository
}

func newOutboxWriterAdapter(repo *postgres.OutboxRepository) *outboxWriterAdapter {
	return &outboxWriterAdapter{repo: repo}
}

// AppendTransferPosted extracts the pgx.Tx from the ledger.Tx (passed into
// the TransferService tx closure) and writes the event in the same tx.
func (a *outboxWriterAdapter) AppendTransferPosted(ctx context.Context, tx ledger.Tx, e outbox.Event) error {
	pgxTx, err := postgres.UnwrapPgxTxFromLedger(tx)
	if err != nil {
		return fmt.Errorf("outbox adapter: %w", err)
	}
	outboxTx := postgres.WrapOutboxTx(pgxTx)
	return a.repo.Insert(ctx, outboxTx, e)
}

// Compile-time guard: ensure outboxWriterAdapter satisfies usecase.OutboxWriter.
var _ usecase.OutboxWriter = (*outboxWriterAdapter)(nil)
