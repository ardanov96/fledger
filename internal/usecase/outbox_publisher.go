// Package usecase — outbox_publisher implements the transactional outbox
// pattern's "publish then mark" worker logic.
//
// Algorithm (single batch per RunOnce call):
//  1. Fetch up to `batchSize` oldest unpublished events from outbox_events
//  2. For each event:
//     a. Serialize payload to JSON
//     b. Publish to NATS via broker.Publish(subject, payload)
//     c. On success → add to "published" batch
//     d. On error   → call repo.IncrementAttempts(id, errString) and log
//  3. After the batch, call repo.MarkPublished(publishedIDs) — single UPDATE
//     for the whole batch (more efficient than per-event).
//
// Concurrency: a single instance is safe because outbox rows are stateful
// (published_at IS NULL). Multiple instances would need FOR UPDATE SKIP LOCKED
// in FetchUnpublished — deferred to a later sprint.
//
// Failure modes:
//   - Broker unreachable → each event's attempts incremented, no rows marked
//     published. Next tick retries. No events lost.
//   - MarkPublished fails (DB down) → events re-published on next tick. NATS
//     consumers must dedupe by event.ID.
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/outbox"
)

// EventBroker is the minimal interface the publisher needs from a NATS client.
// Defined here so the use case can be unit-tested with a fake broker.
type EventBroker interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

// OutboxPublisher polls the outbox and publishes events to the broker.
type OutboxPublisher struct {
	repo      outbox.Repository
	broker    EventBroker
	log       *slog.Logger
	batchSize int
}

// OutboxPublisherDeps bundles dependencies.
type OutboxPublisherDeps struct {
	Repo      outbox.Repository
	Broker    EventBroker
	Logger    *slog.Logger
	BatchSize int // optional; default 50
}

// NewOutboxPublisher constructs an OutboxPublisher.
func NewOutboxPublisher(deps OutboxPublisherDeps) *OutboxPublisher {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	batch := deps.BatchSize
	if batch <= 0 {
		batch = 50
	}
	return &OutboxPublisher{
		repo:      deps.Repo,
		broker:    deps.Broker,
		log:       log,
		batchSize: batch,
	}
}

// RunOnce executes one poll-publish-mark cycle. Returns the number of
// events successfully published in this cycle. Errors are returned only
// for infrastructure problems (DB query, etc.); per-event publish failures
// are recorded via IncrementAttempts and do NOT cause RunOnce to return
// an error.
func (p *OutboxPublisher) RunOnce(ctx context.Context) (published int, err error) {
	events, err := p.repo.FetchUnpublished(ctx, p.batchSize)
	if err != nil {
		return 0, fmt.Errorf("fetch unpublished: %w", err)
	}
	if len(events) == 0 {
		return 0, nil
	}

	p.log.Debug("outbox publisher: fetched batch", "count", len(events))

	publishedIDs := make([]uuid.UUID, 0, len(events))
	failedIDs := make([]uuid.UUID, 0)

	for _, e := range events {
		payload, perr := json.Marshal(e.Payload)
		if perr != nil {
			p.log.Error("outbox publisher: marshal payload failed",
				"event_id", e.ID,
				"subject", e.Subject,
				"error", perr,
			)
			_ = p.repo.IncrementAttempts(ctx, e.ID, "marshal: "+perr.Error())
			failedIDs = append(failedIDs, e.ID)
			continue
		}

		if perr := p.broker.Publish(ctx, e.Subject, payload); perr != nil {
			p.log.Warn("outbox publisher: broker publish failed",
				"event_id", e.ID,
				"subject", e.Subject,
				"attempts", e.Attempts+1,
				"error", perr,
			)
			_ = p.repo.IncrementAttempts(ctx, e.ID, perr.Error())
			failedIDs = append(failedIDs, e.ID)
			continue
		}

		publishedIDs = append(publishedIDs, e.ID)
	}

	if len(publishedIDs) > 0 {
		if merr := p.repo.MarkPublished(ctx, publishedIDs); merr != nil {
			// Already published to broker but DB mark failed. Next tick
			// will re-publish. Consumers MUST dedupe by event ID.
			p.log.Error("outbox publisher: mark-published failed",
				"count", len(publishedIDs),
				"error", merr,
			)
			return 0, fmt.Errorf("mark published: %w", merr)
		}
		p.log.Info("outbox publisher: published batch",
			"count", len(publishedIDs),
			"failed", len(failedIDs),
		)
	}

	return len(publishedIDs), nil
}
