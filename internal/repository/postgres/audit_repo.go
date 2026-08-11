package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/runut/fmcg-wallet/internal/domain/audit"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
)

// AuditRepository implements audit.Repository against Postgres.
type AuditRepository struct {
	db *DB
}

// NewAuditRepository constructs an AuditRepository backed by the given DB pool.
func NewAuditRepository(db *DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Compile-time interface check.
var _ audit.Repository = (*AuditRepository)(nil)

// auditEntryRow mirrors the audit_logs table row.
type auditEntryRow struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ActorID      *uuid.UUID
	ActorType    string
	Action       string
	ResourceType *string
	ResourceID   *string
	Outcome      string
	RequestID    *string
	IPAddress    *string
	UserAgent    *string
	Metadata     []byte
	OccurredAt   time.Time
}

func (r auditEntryRow) toDomain() (audit.Entry, error) {
	entry := audit.Entry{
		ID:         r.ID.String(),
		TenantID:   r.TenantID.String(),
		Action:     audit.Action(r.Action),
		Outcome:    audit.Outcome(r.Outcome),
		OccurredAt: r.OccurredAt,
		Metadata:   parseMetadata(r.Metadata),
	}
	if r.ActorType != "" {
		entry.ActorType = audit.ActorType(r.ActorType)
	}
	if r.ActorID != nil {
		entry.ActorID = r.ActorID.String()
	}
	if r.ResourceType != nil {
		entry.ResourceType = *r.ResourceType
	}
	if r.ResourceID != nil {
		entry.ResourceID = *r.ResourceID
	}
	if r.RequestID != nil {
		entry.RequestID = *r.RequestID
	}
	if r.IPAddress != nil {
		entry.IPAddress = *r.IPAddress
	}
	if r.UserAgent != nil {
		entry.UserAgent = *r.UserAgent
	}
	return entry, nil
}

// Record appends a new audit entry.
func (r *AuditRepository) Record(ctx context.Context, e audit.Entry) error {
	const q = `
INSERT INTO audit_logs (
    tenant_id, actor_id, actor_type, action,
    resource_type, resource_id, outcome,
    request_id, ip_address, user_agent, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`
	var tenantID, actorID *uuid.UUID
	if e.TenantID != "" {
		id, err := uuid.Parse(e.TenantID)
		if err != nil {
			return fmt.Errorf("parse tenant id: %w", err)
		}
		tenantID = &id
	}
	if e.ActorID != "" {
		id, err := uuid.Parse(e.ActorID)
		if err != nil {
			return fmt.Errorf("parse actor id: %w", err)
		}
		actorID = &id
	}

	_, err := r.db.Pool.Exec(ctx, q,
		tenantID,
		actorID,
		string(e.ActorType),
		string(e.Action),
		nullStr(e.ResourceType),
		nullStr(e.ResourceID),
		string(e.Outcome),
		nullStr(e.RequestID),
		nullStr(e.IPAddress),
		nullStr(e.UserAgent),
		jsonRaw(e.Metadata),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return apperrors.ErrAlreadyExists
		}
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// List returns recent entries for a tenant, newest first. limit clamped to 500.
func (r *AuditRepository) List(ctx context.Context, tenantID string, limit int) ([]audit.Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("parse tenant id: %w", err)
	}

	const q = `
SELECT id, tenant_id, actor_id, actor_type, action,
       resource_type, resource_id, outcome,
       request_id, ip_address, user_agent, metadata, occurred_at
FROM audit_logs
WHERE tenant_id = $1
ORDER BY occurred_at DESC
LIMIT $2
`
	rows, err := r.db.Pool.Query(ctx, q, tid, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()

	out := make([]audit.Entry, 0, limit)
	for rows.Next() {
		var row auditEntryRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.ActorID, &row.ActorType, &row.Action,
			&row.ResourceType, &row.ResourceID, &row.Outcome,
			&row.RequestID, &row.IPAddress, &row.UserAgent, &row.Metadata, &row.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		entry, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// GetByID fetches a single entry.
func (r *AuditRepository) GetByID(ctx context.Context, id string) (audit.Entry, error) {
	idUUID, err := uuid.Parse(id)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("parse id: %w", err)
	}

	const q = `
SELECT id, tenant_id, actor_id, actor_type, action,
       resource_type, resource_id, outcome,
       request_id, ip_address, user_agent, metadata, occurred_at
FROM audit_logs
WHERE id = $1
`
	var row auditEntryRow
	err = r.db.Pool.QueryRow(ctx, q, idUUID).Scan(
		&row.ID, &row.TenantID, &row.ActorID, &row.ActorType, &row.Action,
		&row.ResourceType, &row.ResourceID, &row.Outcome,
		&row.RequestID, &row.IPAddress, &row.UserAgent, &row.Metadata, &row.OccurredAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return audit.Entry{}, apperrors.ErrNotFound
		}
		return audit.Entry{}, fmt.Errorf("get audit: %w", err)
	}
	return row.toDomain()
}

// ListByActor returns recent entries for (tenant, actor), newest first.
func (r *AuditRepository) ListByActor(ctx context.Context, tenantID, actorID string, limit int) ([]audit.Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("parse tenant id: %w", err)
	}
	aid, err := uuid.Parse(actorID)
	if err != nil {
		return nil, fmt.Errorf("parse actor id: %w", err)
	}

	const q = `
SELECT id, tenant_id, actor_id, actor_type, action,
       resource_type, resource_id, outcome,
       request_id, ip_address, user_agent, metadata, occurred_at
FROM audit_logs
WHERE tenant_id = $1 AND actor_id = $2
ORDER BY occurred_at DESC
LIMIT $3
`
	rows, err := r.db.Pool.Query(ctx, q, tid, aid, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit by actor: %w", err)
	}
	defer rows.Close()

	out := make([]audit.Entry, 0, limit)
	for rows.Next() {
		var row auditEntryRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.ActorID, &row.ActorType, &row.Action,
			&row.ResourceType, &row.ResourceID, &row.Outcome,
			&row.RequestID, &row.IPAddress, &row.UserAgent, &row.Metadata, &row.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		entry, err := row.toDomain()
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}
