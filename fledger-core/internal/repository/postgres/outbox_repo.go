package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/outbox"
)

// =============================================================================
// DTO
// =============================================================================

type outboxEventDTO struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	Subject       string
	Payload       []byte
	CreatedAt     time.Time
	PublishedAt   *time.Time
	Attempts      int
	LastError     string
	Metadata      []byte
}

// =============================================================================
// Repository
// =============================================================================

type OutboxRepository struct {
	db *DB
}

func NewOutboxRepository(db *DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

var _ outbox.Repository = (*OutboxRepository)(nil)

// ----- Insert -----

func (r *OutboxRepository) Insert(ctx context.Context, tx outbox.Tx, e outbox.Event) error {
	a, ok := tx.(*outboxTxAdapter)
	if !ok {
		return fmt.Errorf("postgres: expected *outboxTxAdapter, got %T", tx)
	}

	const q = `
INSERT INTO outbox_events (
    id, tenant_id, aggregate_type, aggregate_id, event_type,
    subject, payload, metadata
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8
)
`
	_, err := a.pgxTx.Exec(ctx, q,
		e.ID, e.TenantID, e.AggregateType, e.AggregateID, e.EventType,
		e.Subject, jsonRaw(e.Payload), jsonRaw(e.Metadata),
	)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

// ----- FetchUnpublished -----

// FetchUnpublished polls outbox_events across ALL tenants for unpublished
// events to feed the outbox publisher.
//
// Sprint 30: switched from bare Pool.Query to RunInAdminTx. The publisher
// worker has no tenant context (events span tenants), so RLS USING
// tenant_id = current_setting(...) would evaluate to NULL and return
// zero rows. SET LOCAL ROLE app_admin bypasses RLS for this scan.
//
// Sprint 43: uses FOR UPDATE SKIP LOCKED for multi-publisher safety.
//   - When two publisher instances run simultaneously, each gets a
//     disjoint set of events (Postgres row-locks them)
//   - Without SKIP LOCKED, both would block waiting for each other's
//     tx to commit, then both would publish the same event
//   - Lock is released at tx end (COMMIT/ROLLBACK), so events stay
//     locked only for the duration of FetchUnpublished
//
// Important: caller is responsible for the tx — they should:
//   - Publish to NATS (or fail)
//   - MarkPublished (or IncrementAttempts)
//   - Commit the tx
//
// The lock is released on COMMIT. Long-running txs hold locks longer;
// prefer small limits (50) for low latency.
func (r *OutboxRepository) FetchUnpublished(ctx context.Context, limit int) ([]outbox.Event, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `
SELECT id, tenant_id, aggregate_type, aggregate_id, event_type,
       subject, payload, created_at, published_at, attempts,
       COALESCE(last_error, ''), metadata
FROM outbox_events
WHERE published_at IS NULL
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
`
	var out []outbox.Event
	err := r.db.RunInAdminTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, q, limit)
		if err != nil {
			return fmt.Errorf("fetch unpublished outbox: %w", err)
		}
		defer rows.Close()

		out = make([]outbox.Event, 0, limit)
		for rows.Next() {
			var dto outboxEventDTO
			if err := rows.Scan(
				&dto.ID, &dto.TenantID, &dto.AggregateType, &dto.AggregateID, &dto.EventType,
				&dto.Subject, &dto.Payload, &dto.CreatedAt, &dto.PublishedAt, &dto.Attempts,
				&dto.LastError, &dto.Metadata,
			); err != nil {
				return fmt.Errorf("scan outbox row: %w", err)
			}
			out = append(out, dtoToEvent(dto))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// FetchUnpublishedLocked fetches events INSIDE a tx that the caller controls.
// Returns the events AND the pgx.Tx handle. The caller MUST commit or
// rollback the tx once done. This is the multi-publisher-safe variant
// used when multiple worker instances may run simultaneously.
//
// Sprint 43 pattern:
//   1. BEGIN tx
//   2. SELECT ... FOR UPDATE SKIP LOCKED → rows are locked to this tx
//   3. Publish to NATS for each event
//   4. UPDATE SET published_at = now() WHERE id IN (...)
//   5. COMMIT → releases locks; events are marked done
//
// If anything in steps 3-4 fails, ROLLBACK → events stay unpublished
// and are picked up by the next cycle (or another publisher instance).
//
// Other publisher instances calling the same method concurrently will
// SKIP the locked rows and get the next batch (Postgres semantics).
//
// The returned events are scanned in the tx; the caller MUST commit or
// rollback before issuing further queries on the SAME events.
//
// To get just the events without holding a tx, use FetchUnpublished
// (the simpler method). It does NOT use SKIP LOCKED, so concurrent
// instances may double-publish. Use FetchUnpublishedLocked for multi-
// instance deployments.
func (r *OutboxRepository) FetchUnpublishedLocked(ctx context.Context, limit int) (pgx.Tx, []outbox.Event, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `
SELECT id, tenant_id, aggregate_type, aggregate_id, event_type,
       subject, payload, created_at, published_at, attempts,
       COALESCE(last_error, ''), metadata
FROM outbox_events
WHERE published_at IS NULL
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
`
	tx, err := r.db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}

	var out []outbox.Event
	err = func() error {
		rows, err := tx.Query(ctx, q, limit)
		if err != nil {
			return fmt.Errorf("fetch unpublished outbox (locked): %w", err)
		}
		defer rows.Close()

		out = make([]outbox.Event, 0, limit)
		for rows.Next() {
			var dto outboxEventDTO
			if err := rows.Scan(
				&dto.ID, &dto.TenantID, &dto.AggregateType, &dto.AggregateID, &dto.EventType,
				&dto.Subject, &dto.Payload, &dto.CreatedAt, &dto.PublishedAt, &dto.Attempts,
				&dto.LastError, &dto.Metadata,
			); err != nil {
				return fmt.Errorf("scan outbox row: %w", err)
			}
			out = append(out, dtoToEvent(dto))
		}
		return rows.Err()
	}()
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, nil, err
	}
	return tx, out, nil
}

// ----- MarkPublished -----

// MarkPublished marks the given outbox events as published (sets
// published_at = now() WHERE published_at IS NULL).
//
// Sprint 30: switched to RunInAdminTx (cross-tenant update from worker).
func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `
UPDATE outbox_events
SET published_at = now()
WHERE id = ANY($1) AND published_at IS NULL
`
	return r.db.RunInAdminTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, q, ids); err != nil {
			return fmt.Errorf("mark outbox published: %w", err)
		}
		return nil
	})
}

// ----- IncrementAttempts -----

// IncrementAttempts records a publish failure for one outbox event.
//
// Sprint 30: switched to RunInAdminTx (cross-tenant update from worker).
func (r *OutboxRepository) IncrementAttempts(ctx context.Context, id uuid.UUID, lastErr string) error {
	const q = `
UPDATE outbox_events
SET attempts   = attempts + 1,
    last_error = $2
WHERE id = $1 AND published_at IS NULL
`
	return r.db.RunInAdminTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, q, id, lastErr); err != nil {
			return fmt.Errorf("increment outbox attempts: %w", err)
		}
		return nil
	})
}

// =============================================================================
// DTO helpers
// =============================================================================

func dtoToEvent(dto outboxEventDTO) outbox.Event {
	return outbox.Event{
		ID:            dto.ID,
		TenantID:      dto.TenantID,
		AggregateType: dto.AggregateType,
		AggregateID:   dto.AggregateID,
		EventType:     dto.EventType,
		Subject:       dto.Subject,
		Payload:       parseMetadata(dto.Payload),
		CreatedAt:     dto.CreatedAt,
		PublishedAt:   dto.PublishedAt,
		Attempts:      dto.Attempts,
		LastError:     dto.LastError,
		Metadata:      parseMetadata(dto.Metadata),
	}
}

