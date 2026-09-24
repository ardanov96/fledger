// Package outbox defines the transactional outbox pattern domain.
//
// Design notes:
//
//   - OutboxEvent is the storage-level representation. Domain has zero infra
//     deps (no pgx, no NATS) so it can be unit-tested without a broker.
//
//   - AggregateType is the bounded context that produced the event
//     (transfer, invoice, period, etc.). Subject is the NATS routing key
//     (e.g. "fmcg.transfer.posted"). EventType is a logical name
//     ("transfer.posted") that subscribers dispatch on.
//
//   - Tx interface mirrors period/ledger Tx shape (Exec only — outbox only
//     inserts in the business tx; reads use the pool). This keeps the domain
//     dependency-free.
package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Event
// =============================================================================

// Event is one row in the outbox_events table.
//
// Lifecycle:
//   1. Business code calls Repository.Insert(ctx, tx, e) in the same DB tx as
//      the business write. Insert is atomic with the business operation.
//   2. Publisher worker periodically calls Repository.FetchUnpublished and
//      publishes to the broker. On success, Repository.MarkPublished is called.
//   3. Subscribers receive the event and handle it idempotently. NATS redelivers
//      on consumer crash; the dedup key is `event.ID` so duplicates are detectable.
type Event struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Subject       string
	Payload       map[string]any
	CreatedAt     time.Time
	PublishedAt   *time.Time
	Attempts      int
	LastError     string
	Metadata      map[string]any
}

// AggregateType enum mirrors the CHECK constraint on outbox_events.aggregate_type.
const (
	AggregateTransfer   = "transfer"
	AggregateInvoice    = "invoice"
	AggregatePeriod     = "period"
	AggregatePayment    = "payment"
	AggregateReconciler = "reconciler"
	AggregateAuth       = "auth"
)

// Standard event types for Sprint 24 (Sprint 24 ships only transfer.posted;
// additional events added per-domain in later sprints).
const (
	EventTransferPosted = "transfer.posted"
)

// Standard subject prefixes. Matches NATS_STREAM_SUBJECTS default "fmcg.>".
const (
	SubjectPrefix = "fmcg."

	SubjectTransferPosted = "fmcg.transfer.posted"
)

// =============================================================================
// Tx (minimal — only Exec needed for Insert)
// =============================================================================

// Tx is the minimal interface outbox.Repository.Insert needs from a DB tx.
// Callers pass a value that satisfies this (e.g. *outbox.txAdapter wrapping
// pgx.Tx) — see postgres/tx_adapter_outbox.go.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
}

// CommandTag is the result of Exec.
type CommandTag interface {
	RowsAffected() int64
}

// =============================================================================
// Repository
// =============================================================================

// Repository defines persistence operations for the outbox.
//
// Insert runs in caller's tx (so the outbox write is atomic with the business
// write). FetchUnpublished / MarkPublished / IncrementAttempts run OUTSIDE
// any tx — the publisher worker calls them on its own connection.
type Repository interface {
	// Insert writes one outbox row. Callers pass the same `tx` they used for
	// the business write so the outbox insert is atomic with that write.
	Insert(ctx context.Context, tx Tx, e Event) error

	// FetchUnpublished returns up to `limit` oldest unpublished events.
	// Caller should use SELECT ... FOR UPDATE SKIP LOCKED in production to
	// allow multiple publisher instances (Sprint 24 ships single-instance).
	FetchUnpublished(ctx context.Context, limit int) ([]Event, error)

	// MarkPublished sets published_at = now() for the given event IDs.
	// Called by the publisher after a successful broker ack.
	MarkPublished(ctx context.Context, ids []uuid.UUID) error

	// IncrementAttempts bumps attempts and records last_error for an event.
	// Called by the publisher on transient broker failure.
	IncrementAttempts(ctx context.Context, id uuid.UUID, lastErr string) error
}
