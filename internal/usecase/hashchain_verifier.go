// Hash chain verifier (Fase 1C).
//
// The verifier walks all entries for a given tenant, ordered by account_id
// + created_at + id, recomputes the expected hash for each, and reports any
// mismatch. Pure-Go (no DB) — caller provides the entries.
package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
)

// Verifier walks the entries and reports any mismatch.
//
// entries is expected to be sorted by (account_id, created_at, id).
type Verifier struct {
	log *slog.Logger
}

// NewVerifier constructs a Verifier.
func NewVerifier(log *slog.Logger) *Verifier {
	if log == nil {
		log = slog.Default()
	}
	return &Verifier{log: log}
}

// VerifyError describes a mismatch found by the verifier.
type VerifyError struct {
	EntryID  string
	Expected string
	Actual   string
	Field    string // "prev_hash", "entry_hash", or "field"
}

func (e *VerifyError) Error() string {
	return fmt.Sprintf("hash chain mismatch at entry %s: field=%s expected=%s actual=%s",
		e.EntryID, e.Field, e.Expected, e.Actual)
}

// Verify walks the entries (sorted by account_id, created_at, id) and checks
// each entry's prev_hash matches the previous entry's entry_hash, plus that
// each entry_hash recomputes correctly from its fields.
func (v *Verifier) Verify(_ context.Context, entries []ledger.Entry) []error {
	var errs []error
	prevByAccount := map[string]string{}

	for _, e := range entries {
		prevHash, ok := prevByAccount[e.AccountID]
		if !ok {
			prevHash = ledger.ZeroHash
		}

		// 1) prev_hash on this entry must equal previous entry_hash (or zero).
		if e.PrevHash != prevHash {
			errs = append(errs, &VerifyError{
				EntryID:  e.ID,
				Expected: prevHash,
				Actual:   e.PrevHash,
				Field:    "prev_hash",
			})
		}

		// 2) entry_hash must recompute correctly.
		expected, err := ledger.ComputeHash(ledger.HashInput{
			PrevHash:      e.PrevHash,
			AccountID:     e.AccountID,
			TransactionID: e.TransactionID,
			PeriodID:      e.PeriodID,
			Type:          e.Type,
			AmountMinor:   e.Amount.Minor(),
			Currency:      e.Currency,
			RefType:       e.RefType,
			RefID:         e.RefID,
			Description:  e.Description,
			CreatedAt:    e.CreatedAt,
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("entry %s: compute hash: %w", e.ID, err))
			continue
		}
		if e.EntryHash != expected {
			errs = append(errs, &VerifyError{
				EntryID:  e.ID,
				Expected: expected,
				Actual:   e.EntryHash,
				Field:    "entry_hash",
			})
		}

		prevByAccount[e.AccountID] = e.EntryHash
	}

	if len(errs) > 0 {
		v.log.Warn("hash chain verification found mismatches", "count", len(errs))
	}
	return errs
}
