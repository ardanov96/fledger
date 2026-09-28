// notification_adapter.go - bridges usecase.NotificationService to handler.NotificationAPI.
// Sprint 28 / Fase 8.

package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/notification"
	"github.com/runut/fmcg-wallet/internal/handler"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// notificationAPIAdapter satisfies handler.NotificationAPI by delegating
// to usecase.NotificationService. Kept thin so the handler doesn't depend
// directly on the postgres package.
type notificationAPIAdapter struct {
	svc *usecase.NotificationService
}

func newNotificationAPIAdapter(repo notification.Repository) *notificationAPIAdapter {
	// The worker process creates the real NotificationService with a NATS
	// broker. The API process creates a no-broker stub here - it only
	// exposes list/mark-read endpoints, doesn't subscribe.
	//
	// For the API, we still want to be able to ListForUser and MarkRead,
	// so we wrap the repo in a service with a nil broker. The Subscribe
	// call is never made from the API path (only the worker calls it).
	svc := usecase.NewNotificationService(usecase.NotificationDeps{
		Repo:     repo,
		Logger:   nil,
		Subjects: nil, // empty - API never subscribes
	})
	return &notificationAPIAdapter{svc: svc}
}

func (a *notificationAPIAdapter) ListForUser(ctx context.Context, tenantID, userID uuid.UUID, limit int, unreadOnly bool) ([]notification.Notification, error) {
	return a.svc.ListForUser(ctx, tenantID, userID, limit, unreadOnly)
}

func (a *notificationAPIAdapter) UnreadCount(ctx context.Context, tenantID, userID uuid.UUID) (int, error) {
	return a.svc.UnreadCount(ctx, tenantID, userID)
}

func (a *notificationAPIAdapter) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return a.svc.MarkRead(ctx, id, userID)
}

// Compile-time interface guard
var _ handler.NotificationAPI = (*notificationAPIAdapter)(nil)
