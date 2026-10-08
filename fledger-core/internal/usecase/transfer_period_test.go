// transfer_period_test.go — Sprint 23.2 property tests for period resolution.
//
// Verifies that the TransferService no longer uses the hardcoded seed UUID
// (`00000000-0000-0000-0000-000000000001`) as the period_id. The fix
// replaces ensureOpenPeriod with a PeriodResolver dependency that resolves
// the open period for the tenant.
//
// What we verify:
//  1. Default stub resolver returns a real UUID (not the seed)
//  2. Custom resolver's return value is honored
//  3. Resolver is called once per Transfer (not on error paths)
//  4. Resolver error propagates to caller
//  5. PeriodID on resulting transaction matches what resolver returned

package usecase

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// seedPeriodUUID is the value the old hardcoded stub used to return.
// We explicitly assert this is NEVER the value of any Transaction.PeriodID
// in the fixed code path.
const seedPeriodUUID = "00000000-0000-0000-0000-000000000001"

// recordingPeriodResolver captures calls so tests can assert invocation count
// and returns whatever PeriodID the test wants.
type recordingPeriodResolver struct {
	periodID    string
	err         error
	calls       atomic.Int32
	lastTenant  atomic.Value // string
	lastTimeSet atomic.Bool
}

func (r *recordingPeriodResolver) GetOrCreateOpenPeriod(_ context.Context, tenantID string, _ time.Time) (string, error) {
	r.calls.Add(1)
	r.lastTenant.Store(tenantID)
	r.lastTimeSet.Store(true)
	if r.err != nil {
		return "", r.err
	}
	return r.periodID, nil
}

func TestTransfer_DefaultStubResolverDoesNotReturnSeedUUID(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 100_000, 0)

	res, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(10_000),
		IdempotencyKey: "stub-period-" + uuid.NewString(),
	})
	require.NoError(t, err)

	assert.NotEqual(t, seedPeriodUUID, res.PeriodID,
		"Sprint 23.2: TransferService must NOT use the hardcoded seed UUID as PeriodID")
	_, parseErr := uuid.Parse(res.PeriodID)
	assert.NoError(t, parseErr, "PeriodID must be a valid UUID, got %q", res.PeriodID)
}

func TestTransfer_CustomResolverPeriodIDHonored(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 100_000, 0)

	want := uuid.NewString()
	resolver := &recordingPeriodResolver{periodID: want}
	svc.period = resolver

	res, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(10_000),
		IdempotencyKey: "resolver-period-" + uuid.NewString(),
	})
	require.NoError(t, err)
	assert.Equal(t, want, res.PeriodID, "Transaction.PeriodID must match the resolver's return value")
	assert.Equal(t, int32(1), resolver.calls.Load(), "resolver should be called exactly once per Transfer")
}

func TestTransfer_ResolverErrorPropagates(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 100_000, 0)

	wantErr := errors.New("simulated period store failure")
	resolver := &recordingPeriodResolver{err: wantErr}
	svc.period = resolver

	_, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(10_000),
		IdempotencyKey: "resolver-err-" + uuid.NewString(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr, "resolver error must bubble up unchanged")
}

func TestTransfer_ResolverNotCalledOnIdempotentReplay(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 100_000, 0)

	resolver := &recordingPeriodResolver{periodID: uuid.NewString()}
	svc.period = resolver

	idemKey := "replay-" + uuid.NewString()
	in := ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(10_000),
		IdempotencyKey: idemKey,
	}
	_, err := svc.Transfer(context.Background(), in)
	require.NoError(t, err)
	firstCount := resolver.calls.Load()

	_, err = svc.Transfer(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, firstCount, resolver.calls.Load(),
		"resolver must NOT be called on idempotent replay (no work done)")
}

func TestTransfer_ResolverNotCalledOnValidationError(t *testing.T) {
	t.Parallel()
	svc, _, srcID, _ := setupTransferTest(t, 100_000, 0)

	resolver := &recordingPeriodResolver{periodID: uuid.NewString()}
	svc.period = resolver

	_, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    srcID,
		Amount:         money.NewFromMinor(10_000),
		IdempotencyKey: "validation-err-" + uuid.NewString(),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrInvalidInput))
	assert.Equal(t, int32(0), resolver.calls.Load(),
		"resolver must NOT be called when input validation fails")
}

func TestTransfer_ResolverReceivesSourceTenantID(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 100_000, 0)

	resolver := &recordingPeriodResolver{periodID: uuid.NewString()}
	svc.period = resolver

	_, err := svc.Transfer(context.Background(), ledger.TransferInput{
		FromAccountID:  srcID,
		ToAccountID:    dstID,
		Amount:         money.NewFromMinor(10_000),
		IdempotencyKey: "tenant-id-" + uuid.NewString(),
	})
	require.NoError(t, err)

	gotTenant, _ := resolver.lastTenant.Load().(string)
	srcAcc, _ := svc.accounts.GetByID(context.Background(), srcID)
	assert.Equal(t, srcAcc.TenantID, gotTenant,
		"resolver must be called with the source account's tenant_id")
}

func TestTransfer_PropertyTest_NoSeedUUIDAcrossManyTransfers(t *testing.T) {
	t.Parallel()
	svc, _, srcID, dstID := setupTransferTest(t, 1_000_000, 0)

	for i := 0; i < 50; i++ {
		res, err := svc.Transfer(context.Background(), ledger.TransferInput{
			FromAccountID:  srcID,
			ToAccountID:    dstID,
			Amount:         money.NewFromMinor(100),
			IdempotencyKey: "prop-" + uuid.NewString(),
		})
		require.NoError(t, err)
		assert.NotEqual(t, seedPeriodUUID, res.PeriodID,
			"iteration %d: seed UUID must never appear", i)
	}
}
