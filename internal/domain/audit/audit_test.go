//go:build !windows
// +build !windows

package audit_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/audit"
)

func TestMemoryRepository_RecordAndList(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	ctx := context.Background()

	now := time.Now()
	entries := []audit.Entry{
		{ID: "1", Action: audit.ActionAccountCreate, Outcome: audit.OutcomeSuccess, OccurredAt: now.Add(-3 * time.Hour)},
		{ID: "2", Action: audit.ActionTransferCreate, Outcome: audit.OutcomeSuccess, OccurredAt: now.Add(-2 * time.Hour)},
		{ID: "3", Action: audit.ActionAccountFreeze, Outcome: audit.OutcomeFailure, OccurredAt: now.Add(-1 * time.Hour)},
	}
	for _, e := range entries {
		require.NoError(t, repo.Record(ctx, e))
	}

	got, err := repo.List(ctx, "any-tenant", 10)
	require.NoError(t, err)
	assert.Len(t, got, 3)
	// Most recent first
	assert.Equal(t, "3", got[0].ID)
	assert.Equal(t, "2", got[1].ID)
	assert.Equal(t, "1", got[2].ID)
}

func TestMemoryRepository_ListLimit(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = repo.Record(ctx, audit.Entry{
			ID:         string(rune('a' + i)),
			OccurredAt: time.Now(),
		})
	}

	got, err := repo.List(ctx, "any", 2)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestMemoryRepository_EmptyList(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	got, err := repo.List(context.Background(), "any", 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestMemoryRepository_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	repo := audit.NewMemoryRepository()
	ctx := context.Background()

	const n = 100
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(i int) {
			_ = repo.Record(ctx, audit.Entry{
				ID:         string(rune('a' + (i % 26))),
				OccurredAt: time.Now(),
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}
	got, _ := repo.List(ctx, "any", 0)
	assert.LessOrEqual(t, len(got), n)
}

func TestNopRepository_NoOp(t *testing.T) {
	t.Parallel()
	repo := audit.NopRepository{}
	require.NoError(t, repo.Record(context.Background(), audit.Entry{ID: "x"}))
	got, err := repo.List(context.Background(), "any", 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}
