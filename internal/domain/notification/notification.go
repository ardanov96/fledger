// Package notification defines the in-app notification feed domain (Sprint 28 / Fase 8).
//
// Notifications are created by the NotificationWorker when it consumes
// outbox events from NATS (Sprint 24). End users see them via
// GET /v1/notifications and can mark them read via PATCH.
//
// Design notes:
//   - Pure domain (zero infra deps). Repository is an interface so the use
//     case can be unit-tested with an in-memory fake.
//   - Tx is minimal (Exec only). Used by Create; reads use the pool directly
//     (no RLS context during API GET / no tenant middleware scope).
//   - Status enum: 'unread' -> 'read' -> 'archived'. Archived is for future
//     bulk-cleanup; not exposed in API yet.
//   - Severity enum: 'info' | 'warn' | 'critical'. UI may style differently.
package notification

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Entities
// =============================================================================

// Severity of a notification. Mirrors notifications_severity_valid CHECK.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityCritical Severity = "critical"
)

// Status of a notification (read/unread). Mirrors notifications_status_valid CHECK.
type Status string

const (
	StatusUnread   Status = "unread"
	StatusRead     Status = "read"
	StatusArchived Status = "archived"
)

// Notification is one row in the notifications table.
type Notification struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Body      map[string]any
	Severity  Severity
	Status    Status
	CreatedAt time.Time
	ReadAt    *time.Time
}

// ListFilter scopes a List query.
type ListFilter struct {
	TenantID  uuid.UUID // REQUIRED for tenant isolation
	UserID    uuid.UUID
	Status    *Status   // nil = all statuses
	Limit     int       // default 50, capped at 200
	OnlyAfter time.Time // nil = no cursor (newest first)
}

// =============================================================================
// Tx abstraction (minimal — only Create needs tx; reads use pool)
// =============================================================================

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
}

type CommandTag interface {
	RowsAffected() int64
}

// =============================================================================
// Repository interface
// =============================================================================

// Repository persists notifications. Create runs inside caller's tx; reads
// use the pool directly (caller responsible for tenant GUC binding via
// tenantctx.SetTenantContext).
type Repository interface {
	// Create inserts a new notification row. Must run inside caller's tx
	// so outbox-event + notification are atomic.
	Create(ctx context.Context, tx Tx, n Notification) error

	// GetByID returns one notification by ID. Returns ErrNotFound if missing.
	GetByID(ctx context.Context, id uuid.UUID) (Notification, error)

	// List returns notifications matching filter, newest first.
	List(ctx context.Context, f ListFilter) ([]Notification, error)

	// MarkRead sets status='read' and read_at=now() if currently 'unread'.
	// No-op if already 'read' or 'archived'. Returns rows affected.
	MarkRead(ctx context.Context, id uuid.UUID, userID uuid.UUID) (int64, error)

	// CountUnread returns count of 'unread' notifications for user (for badge).
	CountUnread(ctx context.Context, tenantID, userID uuid.UUID) (int, error)
}
