//go:build !windows
// +build !windows

package ledger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeHash_Deterministic(t *testing.T) {
	t.Parallel()
	in := HashInput{
		PrevHash:      ZeroHash,
		AccountID:     "acct-1",
		TransactionID: "txn-1",
		PeriodID:      "00000000-0000-0000-0000-000000000001",
		Type:          EntryTypeDebit,
		AmountMinor:   10000,
		Currency:      "IDR",
		RefType:       "transfer",
		RefID:         "txn-1",
		Description:  "test entry",
		CreatedAt:    time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC),
	}
	h1, err := ComputeHash(in)
	require.NoError(t, err)
	assert.Len(t, h1, 64)
	h2, err := ComputeHash(in)
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
}

func TestComputeHash_DifferentAmountDifferentHash(t *testing.T) {
	t.Parallel()
	base := HashInput{PrevHash: ZeroHash, AccountID: "a", AmountMinor: 10000,
		CreatedAt: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)}
	h1, _ := ComputeHash(base)
	base.AmountMinor = 10001
	h2, _ := ComputeHash(base)
	assert.NotEqual(t, h1, h2)
}

func TestComputeHash_DifferentDescriptionDifferentHash(t *testing.T) {
	t.Parallel()
	base := HashInput{PrevHash: ZeroHash, AccountID: "a", AmountMinor: 10000,
		CreatedAt: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)}
	h1, _ := ComputeHash(base)
	base.Description = "tampered"
	h2, _ := ComputeHash(base)
	assert.NotEqual(t, h1, h2)
}

func TestComputeHash_DifferentCreatedAtDifferentHash(t *testing.T) {
	t.Parallel()
	base := HashInput{PrevHash: ZeroHash, AccountID: "a", AmountMinor: 10000,
		CreatedAt: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)}
	h1, _ := ComputeHash(base)
	base.CreatedAt = base.CreatedAt.Add(1)
	h2, _ := ComputeHash(base)
	assert.NotEqual(t, h1, h2)
}

func TestComputeHash_RequiresPrevHash(t *testing.T) {
	t.Parallel()
	_, err := ComputeHash(HashInput{AccountID: "a"})
	assert.Error(t, err)
}

func TestZeroHash_Format(t *testing.T) {
	t.Parallel()
	assert.Len(t, ZeroHash, 64)
}
