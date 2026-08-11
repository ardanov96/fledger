//go:build !windows
// +build !windows

package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// buildChain constructs N entries with correct hashes. Returns them sorted
// by (account_id, created_at, id) — required by the verifier.
func buildChain(t *testing.T, accountID string, n int) []ledger.Entry {
	t.Helper()
	entries := make([]ledger.Entry, 0, n)
	var prev string = ledger.ZeroHash

	for i := 0; i < n; i++ {
		now := time.Date(2026, 8, 11, 12, 0, i, 0, time.UTC)
		e := ledger.Entry{
			ID:            accountID + "-e-" + string(rune('0'+i)),
			AccountID:     accountID,
			TransactionID: "txn-1",
			PeriodID:      "00000000-0000-0000-0000-000000000001",
			Type:          ledger.EntryTypeDebit,
			Amount:        money.NewFromMinor(int64(10000 * (i + 1))),
			Currency:      "IDR",
			RefType:       "transfer",
			RefID:         "txn-1",
			Description:  "transfer " + string(rune('A'+i)),
			CreatedAt:    now,
			PrevHash:      prev,
		}
		h, err := ledger.ComputeHash(ledger.HashInput{
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
		require.NoError(t, err)
		e.EntryHash = h
		prev = h
		entries = append(entries, e)
	}
	return entries
}

func TestVerifier_CleanChain_OK(t *testing.T) {
	t.Parallel()
	v := NewVerifier(nil)
	chain := buildChain(t, "acct-1", 5)
	errs := v.Verify(context.Background(), chain)
	assert.Empty(t, errs, "clean chain should produce no errors")
}

func TestVerifier_MultipleAccounts_OK(t *testing.T) {
	t.Parallel()
	v := NewVerifier(nil)
	chain := append(buildChain(t, "acct-1", 3), buildChain(t, "acct-2", 4)...)
	errs := v.Verify(context.Background(), chain)
	assert.Empty(t, errs)
}

func TestVerifier_TamperAmount_DetectedAtEntry(t *testing.T) {
	t.Parallel()
	v := NewVerifier(nil)
	chain := buildChain(t, "acct-1", 5)

	// Attacker modifies amount of entry 2 (index 1).
	chain[1].Amount = money.NewFromMinor(99999999)

	errs := v.Verify(context.Background(), chain)
	require.NotEmpty(t, errs, "tamper must be detected")

	// Find the mismatch for entry 2.
	var foundEntryHashMismatch *VerifyError
	var foundTamperCascade bool
	for _, e := range errs {
		if ve, ok := e.(*VerifyError); ok {
			if ve.EntryID == chain[1].ID {
				if ve.Field == "entry_hash" {
					foundEntryHashMismatch = ve
				}
			}
		}
	}
	require.NotNil(t, foundEntryHashMismatch, "must detect entry_hash mismatch on tampered entry")
	_ = foundTamperCascade
}

func TestVerifier_TamperDescription_Detected(t *testing.T) {
	t.Parallel()
	v := NewVerifier(nil)
	chain := buildChain(t, "acct-1", 3)

	// Modify description of last entry.
	last := &chain[len(chain)-1]
	last.Description = "ATTACKER INJECTED MESSAGE"
	// entry_hash on the row is still the original (so a naive DB check
	// might miss this); but the verifier recomputes from fields.

	errs := v.Verify(context.Background(), chain)
	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "entry_hash")
}

func TestVerifier_BrokenPrevHash_Detected(t *testing.T) {
	t.Parallel()
	v := NewVerifier(nil)
	chain := buildChain(t, "acct-1", 4)

	// Break prev_hash on entry 3 (index 2): it claims to chain to the wrong
	// previous hash. The verifier must catch this.
	chain[2].PrevHash = "deadbeef" + chain[1].EntryHash[8:] // invalid

	errs := v.Verify(context.Background(), chain)
	require.NotEmpty(t, errs)

	var foundPrevMismatch bool
	for _, e := range errs {
		if ve, ok := e.(*VerifyError); ok {
			if ve.EntryID == chain[2].ID && ve.Field == "prev_hash" {
				foundPrevMismatch = true
			}
		}
	}
	assert.True(t, foundPrevMismatch, "must detect prev_hash mismatch")
}

func TestVerifier_EmptyChain_OK(t *testing.T) {
	t.Parallel()
	v := NewVerifier(nil)
	errs := v.Verify(context.Background(), nil)
	assert.Empty(t, errs)
}

func TestVerifierError_Message_ContainsDetails(t *testing.T) {
	t.Parallel()
	e := &VerifyError{
		EntryID: "e1", Field: "entry_hash",
		Expected: "aaaa", Actual: "bbbb",
	}
	msg := e.Error()
	assert.Contains(t, msg, "e1")
	assert.Contains(t, msg, "entry_hash")
	assert.Contains(t, msg, "aaaa")
	assert.Contains(t, msg, "bbbb")
}
