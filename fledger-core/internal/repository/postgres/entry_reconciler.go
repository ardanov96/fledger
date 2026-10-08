package postgres

import (
	"context"
	"fmt"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
)

// ListByPeriod returns all entries for a period, sorted for hash chain
// verification (account_id, created_at, id).
//
// Used by ReconcilerService for hash-chain audit (Fase 1B).
func (r *EntryRepository) ListByPeriod(ctx context.Context, periodID string) ([]ledger.Entry, error) {
	const q = `
SELECT id, transaction_id, account_id, amount, type, ref_type, ref_id,
       period_id, description, currency, metadata, created_at,
       prev_hash, entry_hash
FROM ledger_entries
WHERE period_id = $1
ORDER BY account_id ASC, created_at ASC, id ASC
`
	rows, err := r.db.Pool.Query(ctx, q, periodID)
	if err != nil {
		return nil, fmt.Errorf("list entries by period: %w", err)
	}
	defer rows.Close()

	entries := make([]ledger.Entry, 0, 32)
	for rows.Next() {
		var dto EntryDTO
		if err := rows.Scan(
			&dto.ID, &dto.TransactionID, &dto.AccountID, &dto.Amount, &dto.Type,
			&dto.RefType, &dto.RefID, &dto.PeriodID, &dto.Description,
			&dto.Currency, &dto.Metadata, &dto.CreatedAt,
			&dto.PrevHash, &dto.EntryHash,
		); err != nil {
			return nil, fmt.Errorf("scan entry (period): %w", err)
		}
		entries = append(entries, dtoToEntry(dto))
	}
	return entries, rows.Err()
}
