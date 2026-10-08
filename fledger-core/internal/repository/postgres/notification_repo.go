// notification_repo.go - Postgres impl of notification.Repository (Sprint 28/29).
//
// Sprint 29: ALL read & write paths now run inside a tx that binds tenant
// GUC variables via `r.db.RunInReadTx` / `r.db.RunInTx`. This fixes the
// Sprint 28 bug where bare-Pool queries against the RLS-enabled
// `notifications` table returned zero rows (RLS USING evaluated against
// NULL when `app.current_tenant_id` was unset on the pool connection).
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
	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
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
// Reads & writes (Sprint 29 fix: every path runs inside a tx that binds
// tenant GUC via tenantctx.SetTenantContext). This makes RLS evaluate
// against the correct tenant_id instead of NULL.
// =============================================================================

func (r *NotificationRepository) GetByID(ctx context.Context, id uuid.UUID) (notification.Notification, error) {
	const q = `
SELECT id, tenant_id, user_id, type, title, body, severity, status, created_at, read_at
FROM notifications
WHERE id = $1
`
	var (
		n         notification.Notification
		bodyRaw   []byte
		severity  string
		statusStr string
	)
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, id).Scan(
			&n.ID, &n.TenantID, &n.UserID, &n.Type, &n.Title,
			&bodyRaw, &severity, &statusStr, &n.CreatedAt, &n.ReadAt,
		)
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return notification.Notification{}, apperrors.ErrNotFound
		}
		return notification.Notification{}, fmt.Errorf("get notification: %w", err)
	}
	n.Severity = notification.Severity(severity)
	n.Status = notification.Status(statusStr)
	if len(bodyRaw) > 0 {
		_ = json.Unmarshal(bodyRaw, &n.Body)
	}
	return n, nil
}

// List returns notifications matching filter, newest first.
//
// Sprint 29: runs inside a read tx with tenant GUC bound from the context
// (set by TenantContextMiddleware for /v1/* routes). RLS now scopes rows
// correctly and the app-layer WHERE provides defense-in-depth filtering.
func (r *NotificationRepository) List(ctx context.Context, f notification.ListFilter) ([]notification.Notification, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

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

	var out []notification.Notification
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("list notifications: %w", err)
		}
		defer rows.Close()

		out = make([]notification.Notification, 0, limit)
		for rows.Next() {
			var (
				n         notification.Notification
				bodyRaw   []byte
				severity  string
				statusStr string
			)
			if err := rows.Scan(
				&n.ID, &n.TenantID, &n.UserID, &n.Type, &n.Title,
				&bodyRaw, &severity, &statusStr, &n.CreatedAt, &n.ReadAt,
			); err != nil {
				return fmt.Errorf("scan notification: %w", err)
			}
			n.Severity = notification.Severity(severity)
			n.Status = notification.Status(statusStr)
			if len(bodyRaw) > 0 {
				_ = json.Unmarshal(bodyRaw, &n.Body)
			}
			out = append(out, n)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, tenantID, userID uuid.UUID) (int, error) {
	const q = `
SELECT COUNT(*)
FROM notifications
WHERE tenant_id = $1 AND user_id = $2 AND status = 'unread'
`
	var n int
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, tenantID, userID).Scan(&n)
	})
	if err != nil {
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
	var affected int64
	err := r.db.RunInReadTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, q, id, userID)
		if err != nil {
			return fmt.Errorf("mark read: %w", err)
		}
		affected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// PersistDirect is a helper used by the NotificationWorker to insert a
// notification row. Sprint 29: now runs inside a write tx with GUC bound
// from ctx — the worker is expected to attach a *tenantctx.Info to ctx
// before calling (derived from the originating outbox event's tenant_id).
//
// RLS WITH CHECK on the table is `tenant_id = current_setting(...).uuid OR
// current_user='app_admin'`. Without GUC, fmcg-role inserts would be
// rejected; the tx + SetTenantContext flow makes them accepted.
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
	return r.db.RunInTx(ctx, func(tx pgx.Tx) error {
		// Ensure GUC is bound even if caller forgot to attach Info.
		// The worker SHOULD attach *tenantctx.Info{TenantID: n.TenantID}
		// so this is a fallback path that derives tenant from the row.
		if tenantctx.InfoFromContext(ctx) == nil {
			info := &tenantctx.Info{TenantID: n.TenantID, UserID: n.UserID}
			if err := tenantctx.SetTenantContext(ctx, tx, info); err != nil {
				return fmt.Errorf("persist direct: bind tenant: %w", err)
			}
		} else if err := tenantctx.SetTenantContext(ctx, tx, tenantctx.InfoFromContext(ctx)); err != nil {
			return fmt.Errorf("persist direct: bind tenant: %w", err)
		}
		_, err := tx.Exec(ctx, q,
			n.ID, n.TenantID, n.UserID, n.Type, n.Title,
			string(bodyJSON), string(n.Severity), string(n.Status), n.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert notification: %w", err)
		}
		return nil
	})
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
