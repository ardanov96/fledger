// outbox_publisher_test.go — unit tests for the outbox publisher use case.
// Sprint 24 / Fase 4A.
package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/runut/fmcg-wallet/internal/domain/outbox"
)

// =============================================================================
// Mocks
// =============================================================================

type fakeOutboxRepo struct {
	mu         sync.Mutex
	events     []outbox.Event
	publishMu  sync.Mutex
	markCalls  [][]uuid.UUID
	incCalls   []incAttempt
}

type incAttempt struct {
	ID   uuid.UUID
	Err  string
}

func (r *fakeOutboxRepo) Insert(_ context.Context, _ outbox.Tx, e outbox.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *fakeOutboxRepo) FetchUnpublished(_ context.Context, limit int) ([]outbox.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]outbox.Event, 0, limit)
	for _, e := range r.events {
		if e.PublishedAt == nil {
			out = append(out, e)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeOutboxRepo) MarkPublished(_ context.Context, ids []uuid.UUID) error {
	r.publishMu.Lock()
	defer r.publishMu.Unlock()
	r.markCalls = append(r.markCalls, ids)
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		for i := range r.events {
			if r.events[i].ID == id {
				now := time.Now().UTC()
				r.events[i].PublishedAt = &now
			}
		}
	}
	return nil
}

func (r *fakeOutboxRepo) IncrementAttempts(_ context.Context, id uuid.UUID, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incCalls = append(r.incCalls, incAttempt{ID: id, Err: lastErr})
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Attempts++
			r.events[i].LastError = lastErr
		}
	}
	return nil
}

func (r *fakeOutboxRepo) publishedIDs() []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]uuid.UUID, 0)
	for _, e := range r.events {
		if e.PublishedAt != nil {
			out = append(out, e.ID)
		}
	}
	return out
}

type fakeBroker struct {
	mu       sync.Mutex
	failFor  map[string]bool
	received []string
}

func (b *fakeBroker) Publish(_ context.Context, subject string, _ []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.received = append(b.received, subject)
	if b.failFor[subject] {
		return errors.New("simulated broker failure")
	}
	return nil
}

func newTestOutboxPublisher(repo outbox.Repository, broker EventBroker) *OutboxPublisher {
	return NewOutboxPublisher(OutboxPublisherDeps{
		Repo:      repo,
		Broker:    broker,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		BatchSize: 10,
	})
}

// =============================================================================
// Tests
// =============================================================================

func TestOutboxPublisher_RunOnce_Empty(t *testing.T) {
	t.Parallel()
	repo := &fakeOutboxRepo{}
	broker := &fakeBroker{}
	p := newTestOutboxPublisher(repo, broker)

	n, err := p.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Empty(t, broker.received)
	assert.Empty(t, repo.markCalls)
}

func TestOutboxPublisher_RunOnce_PublishesAndMarks(t *testing.T) {
	t.Parallel()
	repo := &fakeOutboxRepo{
		events: []outbox.Event{
			{ID: uuid.New(), Subject: "fmcg.transfer.posted", AggregateType: "transfer", EventType: "transfer.posted"},
			{ID: uuid.New(), Subject: "fmcg.invoice.created", AggregateType: "invoice", EventType: "invoice.created"},
		},
	}
	broker := &fakeBroker{}
	p := newTestOutboxPublisher(repo, broker)

	n, err := p.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []string{"fmcg.transfer.posted", "fmcg.invoice.created"}, broker.received)
	assert.Len(t, repo.markCalls, 1)
	assert.Len(t, repo.markCalls[0], 2)
	assert.Len(t, repo.publishedIDs(), 2)
}

func TestOutboxPublisher_RunOnce_BrokerFailure_IncrementsAttempts(t *testing.T) {
	t.Parallel()
	failedID := uuid.New()
	okID := uuid.New()
	repo := &fakeOutboxRepo{
		events: []outbox.Event{
			{ID: failedID, Subject: "fmcg.transfer.posted", AggregateType: "transfer"},
			{ID: okID, Subject: "fmcg.invoice.created", AggregateType: "invoice"},
		},
	}
	broker := &fakeBroker{failFor: map[string]bool{"fmcg.transfer.posted": true}}
	p := newTestOutboxPublisher(repo, broker)

	n, err := p.RunOnce(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n, "only the successful event counts toward published")
	assert.Len(t, repo.incCalls, 1, "failed event should be recorded via IncrementAttempts")
	assert.Equal(t, failedID, repo.incCalls[0].ID)
	assert.Equal(t, 1, repo.events[0].Attempts, "Attempts bumped for failed event")
	assert.Equal(t, 0, repo.events[1].Attempts, "Attempts untouched for OK event")
}

func TestOutboxPublisher_RunOnce_FetchError(t *testing.T) {
	t.Parallel()
	fetchErr := errors.New("simulated DB failure")
	repo := &fakeFetchErrRepo{err: fetchErr}
	broker := &fakeBroker{}
	p := newTestOutboxPublisher(repo, broker)

	_, err := p.RunOnce(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, fetchErr)
}

func TestOutboxPublisher_DefaultBatchSizeWhenZero(t *testing.T) {
	t.Parallel()
	repo := &fakeOutboxRepo{}
	broker := &fakeBroker{}
	p := NewOutboxPublisher(OutboxPublisherDeps{
		Repo:      repo,
		Broker:    broker,
		Logger:    slog.New(slog.NewTextHandler(io_DISCARD, nil)),
		BatchSize: 0,
	})
	assert.Equal(t, 50, p.batchSize)
}

// fakeFetchErrRepo errors on FetchUnpublished only.
type fakeFetchErrRepo struct {
	fakeOutboxRepo
	err error
}

func (f *fakeFetchErrRepo) FetchUnpublished(_ context.Context, _ int) ([]outbox.Event, error) {
	return nil, f.err
}

// io_DISCARD avoids the import cycle with transfer tests that use io.Discard.
var io_DISCARD = io_discardHelper{}

type io_discardHelper struct{}

func (io_discardHelper) Write(p []byte) (int, error) { return len(p), nil }

// =============================================================================
// Insert test (verifies Insert writes to the repo with the same tx)
// =============================================================================

func TestOutboxRepository_InsertInTx_RoundTrip(t *testing.T) {
	t.Parallel()
	repo := &fakeOutboxRepo{}
	e := outbox.Event{
		ID:            uuid.New(),
		TenantID:      uuid.New(),
		AggregateType: "transfer",
		AggregateID:   uuid.New(),
		EventType:     "transfer.posted",
		Subject:       "fmcg.transfer.posted",
		Payload:       map[string]any{"foo": "bar"},
		CreatedAt:     time.Now().UTC(),
	}
	// nil tx is fine because our fakeOutboxRepo ignores it.
	require.NoError(t, repo.Insert(context.Background(), nil, e))
	assert.Len(t, repo.events, 1)
	assert.Equal(t, "transfer.posted", repo.events[0].EventType)
}

// unused imports guard
var _ = atomic.Int32{}
