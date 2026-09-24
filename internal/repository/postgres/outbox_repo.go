package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

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
`
	rows, err := r.db.Pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unpublished outbox: %w", err)
	}
	defer rows.Close()

	out := make([]outbox.Event, 0, limit)
	for rows.Next() {
		var dto outboxEventDTO
		if err := rows.Scan(
			&dto.ID, &dto.TenantID, &dto.AggregateType, &dto.AggregateID, &dto.EventType,
			&dto.Subject, &dto.Payload, &dto.CreatedAt, &dto.PublishedAt, &dto.Attempts,
			&dto.LastError, &dto.Metadata,
		); err != nil {
			return nil, fmt.Errorf("scan outbox row: %w", err)
		}
		out = append(out, dtoToEvent(dto))
	}
	return out, rows.Err()
}

// ----- MarkPublished -----

func (r *OutboxRepository) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `
UPDATE outbox_events
SET published_at = now()
WHERE id = ANY($1) AND published_at IS NULL
`
	_, err := r.db.Pool.Exec(ctx, q, ids)
	if err != nil {
		return fmt.Errorf("mark outbox published: %w", err)
	}
	return nil
}

// ----- IncrementAttempts -----

func (r *OutboxRepository) IncrementAttempts(ctx context.Context, id uuid.UUID, lastErr string) error {
	const q = `
UPDATE outbox_events
SET attempts   = attempts + 1,
    last_error = $2
WHERE id = $1 AND published_at IS NULL
`
	_, err := r.db.Pool.Exec(ctx, q, id, lastErr)
	if err != nil {
		return fmt.Errorf("increment outbox attempts: %w", err)
	}
	return nil
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

