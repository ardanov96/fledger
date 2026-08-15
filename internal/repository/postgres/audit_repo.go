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

// Compile-time check: implements handler.AuditRepositoryExtension (Sprint 23/22B.5).
// Defined inline here so the repo file does not import the handler package
// (avoids import cycle: handler imports repo indirectly via cmd/api).
var _ AuditRepoExtension = (*AuditRepository)(nil)

// AuditRepoExtension is a duplicate of handler.AuditRepositoryExtension so
// we can satisfy it without an import cycle. Both interfaces have
// identical shape (duck-typed by compile-time check above).
type AuditRepoExtension interface {
	ListGUCBinds(ctx context.Context, tenantID, sinceRFC3339 string, limit int) ([]GUCBindRow, error)
}

// GUCBindRow mirrors a row of guc_bind_audit (migration 000017).
type GUCBindRow struct {
	ID         int64
	TenantID   string
	UserID     string
	IsSalesRep bool
	RequestID  string
	BoundAt    time.Time
}

// ListGUCBinds returns recent GUC bind events for a tenant (Sprint 23 / 22B.5).
//
// tenantID: required (caller should derive from JWT Principal — never from client).
// sinceRFC3339: optional RFC3339 timestamp; empty means "all history".
// limit: clamped to [1, 500].
//
// RLS note: the table has a tenant-scoped policy + app_admin bypass (migration 000017).
// If the calling role is `fmcg`, RLS automatically filters by tenant_id set via
// the GUC variables bound by tenantctx.SetTenantContext at the start of the tx.
func (r *AuditRepository) ListGUCBinds(ctx context.Context, tenantID, sinceRFC3339 string, limit int) ([]GUCBindRow, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("list guc binds: tenant_id required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	// The query uses tenant_id directly rather than the GUC vars because
	// tenantctx info isn't always set in this read path (audit endpoint runs
	// outside of any business tx). The authoriser (RBAC) has already gated
	// that the caller has audit_log read on this tenant, so we trust the
	// caller-supplied tenantID here.
	var (
		rows pgx.Rows
		err  error
	)
	if sinceRFC3339 != "" {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, tenant_id, user_id, is_sales_rep, COALESCE(request_id, ''), bound_at
			FROM guc_bind_audit
			WHERE tenant_id = $1 AND bound_at >= $2::timestamptz
			ORDER BY bound_at DESC
			LIMIT $3
		`, tenantID, sinceRFC3339, limit)
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, tenant_id, user_id, is_sales_rep, COALESCE(request_id, ''), bound_at
			FROM guc_bind_audit
			WHERE tenant_id = $1
			ORDER BY bound_at DESC
			LIMIT $2
		`, tenantID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list guc binds: %w", err)
	}
	defer rows.Close()

	out := make([]GUCBindRow, 0, limit)
	for rows.Next() {
		var row GUCBindRow
		if err := rows.Scan(&row.ID, &row.TenantID, &row.UserID, &row.IsSalesRep, &row.RequestID, &row.BoundAt); err != nil {
			return nil, fmt.Errorf("scan guc bind row: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

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
