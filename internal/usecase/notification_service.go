// Package usecase - notification service (Sprint 28 / Fase 8).
//
// NotificationService subscribes to NATS outbox events (Sprint 24) and
// creates in-app notification rows. Sprint 24 publishes events with payload
// like {transaction_id, tenant_id, ...}; Sprint 28 transforms those into
// user-facing notifications keyed by recipient_id (often the destination
// account owner for transfers).
//
// Event types supported:
//   - transfer.posted      -> notify recipient + sender
//   - invoice.created      -> notify invoice customer
//   - invoice.overdue      -> notify customer + admin (future)
//   - period.closing       -> notify tenant admin (future)
//
// Production extension points:
//   - Email/SMS/push: add a dispatcher that reads from notifications table
//     on a poll interval. Same NotificationWorker writes the row; outbound
//     delivery is a separate worker.
//   - User preferences: filter by type before insert (skip "noisy" events).

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/runut/fmcg-wallet/internal/domain/notification"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
)

// EventSubscriber is the minimal interface NotificationService needs from
// a NATS client (or stub for tests). Matches the NATS client's signature
// exactly so no adapter is needed.
//
// Handler returns error for logging; the subscriber should NOT exit on
// handler errors (NATS redelivery semantics apply per subscription config).
type EventSubscriber interface {
	Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error)
}

// NotificationDeps bundles dependencies.
type NotificationDeps struct {
	Repo      notification.Repository
	Broker    EventSubscriber
	Subjects  []string // e.g. ["fmcg.>"] - which subjects to subscribe to
	Logger    *slog.Logger
	NowFunc   func() time.Time // optional; defaults to time.Now UTC
}

// NotificationService subscribes to NATS and creates notifications.
type NotificationService struct {
	repo     notification.Repository
	broker   EventSubscriber
	subjects []string
	log      *slog.Logger
	now      func() time.Time
}

// NewNotificationService constructs a NotificationService.
func NewNotificationService(deps NotificationDeps) *NotificationService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	now := deps.NowFunc
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &NotificationService{
		repo:     deps.Repo,
		broker:   deps.Broker,
		subjects: deps.Subjects,
		log:      log,
		now:      now,
	}
}

// Subscribe connects to NATS and starts receiving events. Returns immediately.
// Caller is responsible for keeping the process alive (or for canceling
// the underlying subscription via broker).
func (s *NotificationService) Subscribe(ctx context.Context) error {
	if len(s.subjects) == 0 {
		return fmt.Errorf("notification: no subjects configured")
	}
	for _, subj := range s.subjects {
		handler := func(msg *nats.Msg) {
			if err := s.handleEvent(ctx, msg.Subject, msg.Data); err != nil {
				s.log.Warn("notification: handler error",
					"subject", msg.Subject, "error", err)
			}
		}
		if _, err := s.broker.Subscribe(subj, handler); err != nil {
			return fmt.Errorf("notification: subscribe %s: %w", subj, err)
		}
		s.log.Info("notification: subscribed", "subject", subj)
	}
	return nil
}

// handleEvent is the NATS message handler. Decodes the payload, derives
// notification recipients, and inserts rows.
func (s *NotificationService) handleEvent(ctx context.Context, subject string, payload []byte) error {
	// Parse the outbox event payload
	var ev struct {
		ID         uuid.UUID       `json:"id"`
		TenantID   uuid.UUID       `json:"tenant_id"`
		EventType  string          `json:"event_type"`
		Subject    string          `json:"subject"`
		Payload    json.RawMessage `json:"payload"`
		OccurredAt time.Time       `json:"created_at"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		s.log.Warn("notification: malformed event payload", "subject", subject, "error", err)
		return nil // swallow - don't crash the consumer on bad message
	}
	if ev.TenantID == uuid.Nil || ev.EventType == "" {
		s.log.Warn("notification: missing tenant_id/type", "event_id", ev.ID)
		return nil
	}

	// Derive notification(s) from event type
	recipients := s.recipientsFor(ev.EventType)
	if len(recipients) == 0 {
		s.log.Debug("notification: no recipients", "type", ev.EventType, "event_id", ev.ID)
		return nil
	}

	title := s.titleFor(ev.EventType)
	bodyMap := map[string]any{
		"event_id":    ev.ID.String(),
		"occurred_at": ev.OccurredAt.Format(time.RFC3339Nano),
	}
	if len(ev.Payload) > 0 {
		_ = json.Unmarshal(ev.Payload, &bodyMap)
	}
	severity := s.severityFor(ev.EventType)

	for _, recipientID := range recipients {
		if recipientID == uuid.Nil {
			continue
		}
		n := notification.Notification{
			ID:        uuid.New(),
			TenantID:  ev.TenantID,
			UserID:    recipientID,
			Type:      ev.EventType,
			Title:     title,
			Body:      bodyMap,
			Severity:  severity,
			Status:    notification.StatusUnread,
			CreatedAt: s.now(),
		}
		if err := s.persistNotification(ctx, n); err != nil {
			s.log.Warn("notification: create failed",
				"type", ev.EventType, "recipient", recipientID, "error", err)
		}
	}
	return nil
}

// persistNotification writes a notification row using the repo's Create with
// a tiny inline tx. Since Repo.Create needs a domain Tx, we use the
// postgres.RunInTxNotificationDomain wrapper via the postgres package.
// For now, we delegate to the repo via a direct method that takes a pool.
func (s *NotificationService) persistNotification(ctx context.Context, n notification.Notification) error {
	// Use a tiny inline tx via a separate package-level helper. This works
	// because Postgres.NotificationRepo wraps the same DB pool.
		return postgres.PersistNotificationDirect(ctx, s.repo, n)
}

// recipientsFor returns user IDs to notify for the given event type.
// MVP: hardcoded mapping. Production: lookup recipients from DB
// (e.g. account owner for transfers, customer for invoices).
func (s *NotificationService) recipientsFor(eventType string) []uuid.UUID {
	switch eventType {
	case "transfer.posted":
		// Notify admin user (the seeded hq_admin). In production this would
		// be the destination account owner + maybe an approval-required party.
		return []uuid.UUID{
			uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		}
	case "invoice.created":
		// Notify admin (in production: customer + admin).
		return []uuid.UUID{
			uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		}
	default:
		return nil
	}
}

// titleFor generates a short human-readable title for the event.
func (s *NotificationService) titleFor(eventType string) string {
	switch eventType {
	case "transfer.posted":
		return "Transfer posted"
	case "invoice.created":
		return "Invoice created"
	case "invoice.overdue":
		return "Invoice overdue"
	case "period.closing":
		return "Period closing"
	default:
		return eventType
	}
}

// severityFor returns the notification severity for the event type.
func (s *NotificationService) severityFor(eventType string) notification.Severity {
	switch eventType {
	case "invoice.overdue":
		return notification.SeverityCritical
	case "period.closing":
		return notification.SeverityWarn
	default:
		return notification.SeverityInfo
	}
}

// =============================================================================
// User-facing API (also used by HTTP handler)
// =============================================================================

// ListForUser returns the user's notifications, newest first.
func (s *NotificationService) ListForUser(ctx context.Context, tenantID, userID uuid.UUID, limit int, unreadOnly bool) ([]notification.Notification, error) {
	f := notification.ListFilter{
		TenantID: tenantID,
		UserID:   userID,
		Limit:    limit,
	}
	if unreadOnly {
		st := notification.StatusUnread
		f.Status = &st
	}
	// tenant_id not in filter (repo uses RLS via GUC) - but we filter
	// via user_id which is unique per tenant. For multi-tenant safety,
	// pass tenant_id through and add to query.
	list, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	// Defensive: filter to tenant in case RLS was bypassed
	out := make([]notification.Notification, 0, len(list))
	for _, n := range list {
		if n.TenantID == tenantID {
			out = append(out, n)
		}
	}
	return out, nil
}

// MarkRead marks one notification as read.
func (s *NotificationService) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	_, err := s.repo.MarkRead(ctx, id, userID)
	if err != nil {
		if err == apperrors.ErrNotFound {
			return apperrors.ErrNotFound
		}
		return fmt.Errorf("mark read: %w", err)
	}
	return nil
}

// UnreadCount returns the unread count for the user (used for badge).
func (s *NotificationService) UnreadCount(ctx context.Context, tenantID, userID uuid.UUID) (int, error) {
	return s.repo.CountUnread(ctx, tenantID, userID)
}

// Compile-time interface guard for pgx compatibility (worker may inject a
// pool-backed EventBroker at runtime).
// (removed: pgx.Tx guard - unused after refactor)
