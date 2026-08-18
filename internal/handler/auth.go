// Package handler — auth.go (Sprint 13 + Sprint 23 / 22B.3).
//
// REST endpoints for authentication:
//   POST   /v1/auth/login            — username + password (returns MFA challenge OR token pair)
//   POST   /v1/auth/mfa/verify       — TOTP code + challenge token (returns token pair)
//   POST   /v1/auth/refresh          — refresh token rotation (returns new token pair)
//   POST   /v1/auth/logout           — revoke refresh token
//   POST   /v1/auth/mfa/setup        — generate TOTP secret + otpauth URL for QR provisioning
//   GET    /v1/auth/sessions         — list active sessions for the authenticated user
//   DELETE /v1/auth/sessions/{id}    — revoke a specific session (owner check)
//
// Most auth routes are PUBLIC except sessions endpoints (RequireAuth) and
// /v1/auth/logout (also accepts refresh token only).
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	platformauth "github.com/runut/fmcg-wallet/internal/platform/auth"
	"github.com/runut/fmcg-wallet/internal/platform/httpx"
	"github.com/runut/fmcg-wallet/internal/domain/auth"
	"github.com/runut/fmcg-wallet/internal/middleware"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// AuthAPI — interface implemented by adapter
// =============================================================================

// AuthAPI — interface implemented by adapter (cmd/api/auth_adapters.go).
//
// Sprint 23 / 22B.3 session-management methods (ListSessions, RevokeSession)
// are NOT wired here yet because the corresponding usecase methods
// (usecase.ListSessionsInput etc.) have not been implemented. Once the
// use-case surface lands, restore them and re-register the routes below.
type AuthAPI interface {
	Login(ctx context.Context, in usecase.LoginInput) (*usecase.LoginResult, error)
	VerifyMFA(ctx context.Context, in usecase.VerifyMFAInput) (*usecase.RefreshResult, error)
	Refresh(ctx context.Context, in usecase.RefreshInput) (*usecase.RefreshResult, error)
	Logout(ctx context.Context, in usecase.LogoutInput) error
	SetupMFA(ctx context.Context, in usecase.SetupMFAInput) (*usecase.SetupMFAResult, error)
	// Sprint 23 / 22B.3 — session management (list + revoke).
	ListSessions(ctx context.Context, in usecase.ListSessionsInput) ([]usecase.SessionInfo, error)
	RevokeSession(ctx context.Context, in usecase.RevokeSessionInput) error
}

// =============================================================================
// DTOs
// =============================================================================

// LoginRequest — body of POST /v1/auth/login.
type LoginRequest struct {
	TenantID string `json:"tenant_id" validate:"required,uuid"`
	Username string `json:"username"  validate:"required,min=1,max=200"`
	Password string `json:"password"  validate:"required,min=1,max=200"`
}

// LoginResponseMFA — when MFA required.
type LoginResponseMFA struct {
	MFAChallengeToken string    `json:"mfa_challenge_token"`
	ExpiresAt         time.Time `json:"mfa_expires_at"`
}

// LoginResponseTokens — when no MFA OR after MFA verified.
type LoginResponseTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       uuid.UUID `json:"user_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
}

// VerifyMFARequest — body of POST /v1/auth/mfa/verify.
type VerifyMFARequest struct {
	ChallengeToken string `json:"challenge_token" validate:"required,min=1"`
	Code           string `json:"code"            validate:"required,len=6"`
}

// RefreshRequest — body of POST /v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,min=1"`
}

// LogoutRequest — body of POST /v1/auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,min=1"`
}

// SetupMFARequest — body of POST /v1/auth/mfa/setup.
type SetupMFARequest struct {
	UserID   string `json:"user_id"   validate:"required,uuid"`
	TenantID string `json:"tenant_id" validate:"required,uuid"`
	Label    string `json:"label"     validate:"required,min=1,max=100"`
	Issuer   string `json:"issuer"    validate:"required,min=1,max=100"`
}

// SetupMFAResponse — body of POST /v1/auth/mfa/setup.
type SetupMFAResponse struct {
	Secret string `json:"secret"`
	OTPURL string `json:"otpauth_url"`
}

// SessionResponseItem — one row of GET /v1/auth/sessions.
type SessionResponseItem struct {
	ID         uuid.UUID  `json:"id"`
	UserAgent  string     `json:"user_agent"`
	IPAddress  string     `json:"ip_address,omitempty"`
	IssuedAt   time.Time  `json:"issued_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	Status     string     `json:"status"`
}

// =============================================================================
// Handlers
// =============================================================================

// Login handles POST /v1/auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}

	tenantID, _ := uuid.Parse(req.TenantID)
	res, err := h.Auth.Login(r.Context(), usecase.LoginInput{
		TenantID:  tenantID,
		Username:  req.Username,
		Password:  req.Password,
		UserAgent: r.UserAgent(),
		IPAddress: clientIP(r),
	})
	if err != nil {
		mapAuthErr(w, r, err)
		return
	}

	if res.MFAGenerated {
		httpx.JSON(w, http.StatusOK, LoginResponseMFA{
			MFAChallengeToken: res.MFAChallengeToken,
			ExpiresAt:         time.Now().Add(5 * time.Minute), // matches default MFASessionTTL
		})
		return
	}
	httpx.JSON(w, http.StatusOK, LoginResponseTokens{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresAt:    time.Now().Add(15 * time.Minute), // matches default AccessTokenTTL
		UserID:       res.UserID,
		TenantID:     res.TenantID,
	})
}

// VerifyMFA handles POST /v1/auth/mfa/verify.
func (h *Handlers) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req VerifyMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}

	res, err := h.Auth.VerifyMFA(r.Context(), usecase.VerifyMFAInput{
		ChallengeToken: req.ChallengeToken,
		Code:           req.Code,
		UserAgent:      r.UserAgent(),
		IPAddress:      clientIP(r),
	})
	if err != nil {
		mapAuthErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, LoginResponseTokens{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresAt:    time.Now().Add(15 * time.Minute),
		UserID:       res.UserID,
		TenantID:     res.TenantID,
	})
}

// Refresh handles POST /v1/auth/refresh.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}

	res, err := h.Auth.Refresh(r.Context(), usecase.RefreshInput{
		RefreshToken: req.RefreshToken,
		UserAgent:    r.UserAgent(),
		IPAddress:    clientIP(r),
	})
	if err != nil {
		mapAuthErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, LoginResponseTokens{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresAt:    time.Now().Add(15 * time.Minute),
		UserID:       res.UserID,
		TenantID:     res.TenantID,
	})
}

// Logout handles POST /v1/auth/logout.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}

	if err := h.Auth.Logout(r.Context(), usecase.LogoutInput{
		RefreshToken: req.RefreshToken,
	}); err != nil {
		// Logout is idempotent — even if token is already revoked, return 200.
		if errors.Is(err, auth.ErrRefreshTokenRevoked) || errors.Is(err, auth.ErrRefreshTokenInvalid) {
			httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		mapAuthErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// SetupMFA handles POST /v1/auth/mfa/setup.
func (h *Handlers) SetupMFA(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	var req SetupMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}
	if err := h.Validator.Struct(req); err != nil {
		httpx.ErrorWithDetails(w, r, apperrors.ErrValidationFailed, map[string]any{"validation": err.Error()})
		return
	}

	userID, _ := uuid.Parse(req.UserID)
	tenantID, _ := uuid.Parse(req.TenantID)
	res, err := h.Auth.SetupMFA(r.Context(), usecase.SetupMFAInput{
		UserID:   userID,
		TenantID: tenantID,
		Label:    req.Label,
		Issuer:   req.Issuer,
	})
	if err != nil {
		mapAuthErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, SetupMFAResponse{
		Secret: res.Secret,
		OTPURL: res.OTPURL,
	})
}

// =============================================================================
// Sprint 23 / 22B.3 — Sessions list + revoke
// =============================================================================

// ListSessions handles GET /v1/auth/sessions.
//
// Owner check is implicit because we filter by Principal.user_id (from JWT).
// Users can ONLY ever see their own active refresh-token sessions.
func (h *Handlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	uid, ok := principalUserID(w, r)
	if !ok {
		return
	}
	tenant, ok := principalTenantID(w, r)
	if !ok {
		return
	}

	sessions, err := h.Auth.ListSessions(r.Context(), usecase.ListSessionsInput{
		UserID:   uid,
		TenantID: tenant,
	})
	if err != nil {
		mapAuthErr(w, r, err)
		return
	}

	resp := make([]SessionResponseItem, 0, len(sessions))
	for _, s := range sessions {
		item := SessionResponseItem{
			ID:        s.ID,
			UserAgent: s.UserAgent,
			IssuedAt:  s.IssuedAt,
			ExpiresAt: s.ExpiresAt,
			Status:    string(s.Status),
		}
		if s.IPAddress != nil {
			item.IPAddress = s.IPAddress.String()
		}
		if s.LastUsedAt != nil {
			t := *s.LastUsedAt
			item.LastUsedAt = &t
		}
		resp = append(resp, item)
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RevokeSession handles DELETE /v1/auth/sessions/{id}.
//
// Owner check: the use case layer enforces session.user_id == Principal.user_id
// and returns ErrSessionNotOwner for cross-user attempts. We map that to 403
// deliberately to refuse enumeration (vs 404 which would leak existence).
func (h *Handlers) RevokeSession(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil {
		httpx.Error(w, r, apperrors.ErrNotFound)
		return
	}
	uid, ok := principalUserID(w, r)
	if !ok {
		return
	}
	tenant, ok := principalTenantID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Error(w, r, errors.Join(apperrors.ErrInvalidInput, err))
		return
	}

	err = h.Auth.RevokeSession(r.Context(), usecase.RevokeSessionInput{
		SessionID: sessionID,
		UserID:    uid,
		TenantID:  tenant,
	})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrSessionNotOwner):
			httpx.Error(w, r, apperrors.New("SESSION_FORBIDDEN", "session does not belong to caller", http.StatusForbidden))
		case errors.Is(err, auth.ErrRefreshTokenInvalid):
			httpx.Error(w, r, apperrors.New("SESSION_NOT_FOUND", "session not found", http.StatusNotFound))
		default:
			mapAuthErr(w, r, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// principalUserID pulls user_id from the JWT Principal injected by RequireAuth.
func principalUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil || p.UserID == "" {
		httpx.Error(w, r, apperrors.New("UNAUTHENTICATED", "missing principal", http.StatusUnauthorized))
		return uuid.Nil, false
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		httpx.Error(w, r, apperrors.New("INVALID_PRINCIPAL", "principal user_id is not a uuid", http.StatusUnauthorized))
		return uuid.Nil, false
	}
	return uid, true
}

func principalTenantID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil || p.TenantID == "" {
		httpx.Error(w, r, apperrors.New("UNAUTHENTICATED", "missing principal tenant", http.StatusUnauthorized))
		return uuid.Nil, false
	}
	tid, err := uuid.Parse(p.TenantID)
	if err != nil {
		httpx.Error(w, r, apperrors.New("INVALID_PRINCIPAL", "principal tenant_id is not a uuid", http.StatusUnauthorized))
		return uuid.Nil, false
	}
	return tid, true
}

// =============================================================================
// Error mapping
// =============================================================================

// mapAuthErr converts auth-domain sentinels to HTTP errors.
func mapAuthErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.Error(w, r, apperrors.New("INVALID_CREDENTIALS", "invalid username or password", http.StatusUnauthorized))
	case errors.Is(err, auth.ErrAccountLocked):
		httpx.Error(w, r, apperrors.New("ACCOUNT_LOCKED", "account locked due to too many failed attempts", http.StatusForbidden))
	case errors.Is(err, auth.ErrMFARequired):
		httpx.Error(w, r, apperrors.New("MFA_REQUIRED", "MFA verification required", http.StatusUnauthorized))
	case errors.Is(err, auth.ErrMFAFailed):
		httpx.Error(w, r, apperrors.New("MFA_FAILED", "invalid TOTP code", http.StatusUnauthorized))
	case errors.Is(err, auth.ErrMFAAttemptsExceeded):
		httpx.Error(w, r, apperrors.New("MFA_ATTEMPTS_EXCEEDED", "too many MFA attempts", http.StatusTooManyRequests))
	case errors.Is(err, auth.ErrMFAChallengeExpired):
		httpx.Error(w, r, apperrors.New("MFA_CHALLENGE_EXPIRED", "MFA challenge expired", http.StatusGone))
	case errors.Is(err, auth.ErrRefreshTokenInvalid):
		httpx.Error(w, r, apperrors.New("REFRESH_TOKEN_INVALID", "refresh token invalid", http.StatusUnauthorized))
	case errors.Is(err, auth.ErrRefreshTokenExpired):
		httpx.Error(w, r, apperrors.New("REFRESH_TOKEN_EXPIRED", "refresh token expired", http.StatusUnauthorized))
	case errors.Is(err, auth.ErrRefreshTokenRevoked):
		httpx.Error(w, r, apperrors.New("REFRESH_TOKEN_REVOKED", "refresh token revoked", http.StatusUnauthorized))
	case errors.Is(err, auth.ErrRefreshTokenReuse):
		httpx.Error(w, r, apperrors.New("REFRESH_TOKEN_REUSE", "refresh token reuse detected — all sessions revoked", http.StatusUnauthorized))
	case errors.Is(err, platformauth.ErrPasswordPolicyFail):
		// Sprint 23 / 22B.4: weak password rejected before bcrypt verify.
		httpx.Error(w, r, apperrors.New("PASSWORD_POLICY", "password does not meet security requirements", http.StatusUnprocessableEntity))
	default:
		httpx.Error(w, r, err)
	}
}

// clientIP extracts client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) *netip.Addr {
	addr := r.Header.Get("X-Forwarded-For")
	if addr == "" {
		addr = r.RemoteAddr
	}
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		if a, err := netip.ParseAddrPort(addr); err == nil {
			ip := a.Addr()
			return &ip
		}
	}
	if a, err := netip.ParseAddr(strings.Split(addr, ":")[0]); err == nil {
		return &a
	}
	return nil
}

var _ = chi.NewRouter
