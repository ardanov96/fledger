// notification_repo.go - Postgres impl of notification.Repository (Sprint 28).
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/domain/notification"
)

// NotificationRepository implements notification.Repository against Postgres.
type NotificationRepository struct {
	db *DB
}

// NewNotificationRepository constructs a NotificationRepository.
func NewNotificationRepository(db *DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

var _ notification.Repository = (*NotificationRepository)(nil)

// =============================================================================
// Create
// =============================================================================

func (r *NotificationRepository) Create(ctx context.Context, tx notification.Tx, n notification.Notification) error {
	a, ok := tx.(*notificationTxAdapter)
	if !ok {
		return fmt.Errorf("postgres: expected *notificationTxAdapter, got %T", tx)
	}

	const q = `
INSERT INTO notifications (
    id, tenant_id, user_id, type, title, body, severity, status, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
`
	bodyJSON, err := json.Marshal(n.Body)
	if err != nil {
		return fmt.Errorf("marshal notification body: %w", err)
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}

	_, err = a.pgxTx.Exec(ctx, q,
		n.ID, n.TenantID, n.UserID, n.Type, n.Title,
		string(bodyJSON), string(n.Severity), string(n.Status), n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// =============================================================================
// Reads (use pool; rely on RLS to scope by current_setting('app.current_tenant_id'))
// =============================================================================

func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (notification.Notification, error) {
	const q = `
SELECT id, tenant_id, user_id, type, title, body, severity, status, created_at, read_at
FROM notifications
WHERE id = $1
`
	var n notification.Notification
	var bodyRaw []byte
	var severity, status string
	err := r.db.Pool.QueryRow(ctx, q, id).Scan(
		&n.ID, &n.TenantID, &n.UserID, &n.Type, &n.Title,
		&bodyRaw, &severity, &status, &n.CreatedAt, &n.ReadAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return notification.Notification{}, apperrors.ErrNotFound
		}
		return notification.Notification{}, fmt.Errorf("get notification: %w", err)
	}
	n.Severity = notification.Severity(severity)
	n.Status = notification.Status(status)
	if len(bodyRaw) > 0 {
		_ = json.Unmarshal(bodyRaw, &n.Body)
	}
	return n, nil
}

// List returns notifications matching filter, newest first.
// Filters explicitly by tenant_id + user_id in the WHERE clause so we don't
// need to wrap in a tenant-bound tx. (The pool connection used here has no
// GUC context; relying on RLS would require tx-bound execution.)
func (r *NotificationRepository) List(ctx context.Context, f notification.ListFilter) ([]notification.Notification, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Build query dynamically. tenant_id is part of the WHERE so we don't
	// need tenant GUC binding (unlike RLS-only path).
	q := `
SELECT id, tenant_id, user_id, type, title, body, severity, status, created_at, read_at
FROM notifications
WHERE tenant_id = $1 AND user_id = $2
`
	args := []any{f.TenantID, f.UserID}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		q += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if !f.OnlyAfter.IsZero() {
		args = append(args, f.OnlyAfter)
		q += fmt.Sprintf(" AND created_at > $%d", len(args))
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	out := make([]notification.Notification, 0, limit)
	for rows.Next() {
		var n notification.Notification
		var bodyRaw []byte
		var severity, status string
		if err := rows.Scan(
			&n.ID, &n.TenantID, &n.UserID, &n.Type, &n.Title,
			&bodyRaw, &severity, &status, &n.CreatedAt, &n.ReadAt,
		); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		n.Severity = notification.Severity(severity)
		n.Status = notification.Status(status)
		if len(bodyRaw) > 0 {
			_ = json.Unmarshal(bodyRaw, &n.Body)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepository) CountUnread(ctx context.Context, tenantID, userID uuid.UUID) (int, error) {
	const q = `
SELECT COUNT(*)
FROM notifications
WHERE tenant_id = $1 AND user_id = $2 AND status = 'unread'
`
	var n int
	if err := r.db.Pool.QueryRow(ctx, q, tenantID, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	return n, nil
}

func (r *NotificationRepository) MarkRead(ctx context.Context, id, userID uuid.UUID) (int64, error) {
	const q = `
UPDATE notifications
SET status = 'read', read_at = COALESCE(read_at, now())
WHERE id = $1 AND user_id = $2 AND status = 'unread'
`
	tag, err := r.db.Pool.Exec(ctx, q, id, userID)
	if err != nil {
		return 0, fmt.Errorf("mark read: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PersistDirect is a helper used by the NotificationWorker to insert a
// notification row using the pool directly (no business tx wrapper).
// Bypasses the domain Tx interface requirement for fire-and-forget writes.
func PersistDirect(ctx context.Context, r *NotificationRepository, n notification.Notification) error {
	bodyJSON, err := json.Marshal(n.Body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	const q = `
INSERT INTO notifications (
    id, tenant_id, user_id, type, title, body, severity, status, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
`
	_, err = r.db.Pool.Exec(ctx, q,
		n.ID, n.TenantID, n.UserID, n.Type, n.Title,
		string(bodyJSON), string(n.Severity), string(n.Status), n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// PersistNotificationDirect is the exported wrapper so the usecase package
// can call it without importing postgres directly. Type-asserts the repo
// to *NotificationRepository (the only implementation in this codebase).
func PersistNotificationDirect(ctx context.Context, r notification.Repository, n notification.Notification) error {
	if concrete, ok := r.(*NotificationRepository); ok {
		return PersistDirect(ctx, concrete, n)
	}
	return fmt.Errorf("notification: repo is not a *postgres.NotificationRepository (got %T)", r)
}
