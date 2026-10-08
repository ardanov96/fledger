// notification_handlers.go - HTTP endpoints for the notifications feed (Sprint 28).
//
// Routes (mounted under /v1):
//   GET    /notifications                  - list current user's notifications
//   GET    /notifications?unread=true      - filter to unread only
//   GET    /notifications/unread-count     - count of unread for badge
//   PATCH  /notifications/{id}/read        - mark one as read
//
// User + tenant are extracted from context (set by RequireAuth + TenantContext
// middlewares via httpx.GetUserID / httpx.GetTenantID).
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	
	"github.com/runut/fmcg-wallet/internal/domain/notification"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
)

// NotificationAPI is the interface handlers depend on (implemented by
// usecase.NotificationService).
type NotificationAPI interface {
	ListForUser(ctx context.Context, tenantID, userID uuid.UUID, limit int, unreadOnly bool) ([]notification.Notification, error)
	UnreadCount(ctx context.Context, tenantID, userID uuid.UUID) (int, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) error
}

// ListNotifications handles GET /v1/notifications.
func (h *Handlers) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	if h.Notifications == nil {
		httpx.JSON(w, http.StatusOK, []any{})
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	unreadOnly := r.URL.Query().Get("unread") == "true"

	list, err := h.Notifications.ListForUser(r.Context(), tenantID, userID, limit, unreadOnly)
	if err != nil {
			httpx.Error(w, r, fmt.Errorf("list: %w", err))
		return
	}
	if list == nil {
		list = []notification.Notification{}
	}
	httpx.JSON(w, http.StatusOK, list)
}

// UnreadCount handles GET /v1/notifications/unread-count.
func (h *Handlers) UnreadCount(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	tenantID, ok := h.requireTenant(w, r)
	if !ok {
		return
	}
	if h.Notifications == nil {
		httpx.JSON(w, http.StatusOK, map[string]int{"count": 0})
		return
	}
	count, err := h.Notifications.UnreadCount(r.Context(), tenantID, userID)
	if err != nil {
		httpx.Error(w, r, fmt.Errorf("count: %w", err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"count": count})
}

// MarkNotificationRead handles PATCH /v1/notifications/{id}/read.
func (h *Handlers) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUser(w, r)
	if !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid notification id")))
		return
	}
	if h.Notifications == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	if err := h.Notifications.MarkRead(r.Context(), id, userID); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			httpx.Error(w, r, apperrors.ErrNotFound)
			return
		}
		httpx.Error(w, r, fmt.Errorf("mark: %w", err))
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "read"})
}

// requireUser extracts user_id from request context. Sends 401 if absent.
func (h *Handlers) requireUser(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	idStr := httpx.GetUserID(r.Context())
	if idStr == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrUnauthorized, errors.New("no user_id in context")))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrUnauthorized, errors.New("invalid user_id")))
		return uuid.Nil, false
	}
	return id, true
}

// requireTenant extracts tenant_id from request context. Sends 400 if absent.
func (h *Handlers) requireTenant(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	idStr := httpx.GetTenantID(r.Context())
	if idStr == "" {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("no tenant_id in context")))
		return uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, errors.New("invalid tenant_id")))
		return uuid.Nil, false
	}
	return id, true
}
