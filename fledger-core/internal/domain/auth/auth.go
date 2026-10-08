// Package auth defines the auth domain â€” refresh tokens, user credentials,
// MFA challenges, login attempts. Sprint 13 / Fase 2E lanjutan.
//
// Domain is pure (no infra deps). Repository is an interface so use case
// can be unit-tested with in-memory fakes.
package auth

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Sentinel errors
// =============================================================================

var (
	// ErrInvalidCredentials â€” wrong username/password combo.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrAccountLocked â€” too many failed attempts, locked_until > now.
	ErrAccountLocked = errors.New("auth: account locked")

	// ErrMFARequired â€” user has MFA enabled, must complete challenge first.
	ErrMFARequired = errors.New("auth: mfa required")

	// ErrMFAFailed â€” TOTP code invalid.
	ErrMFAFailed = errors.New("auth: mfa failed")

	// ErrMFANotEnabled â€” trying to verify MFA on user without MFA.
	ErrMFANotEnabled = errors.New("auth: mfa not enabled")

	// ErrRefreshTokenInvalid â€” token not found / hash mismatch.
	ErrRefreshTokenInvalid = errors.New("auth: refresh token invalid")

	// ErrRefreshTokenExpired â€” expires_at <= now.
	ErrRefreshTokenExpired = errors.New("auth: refresh token expired")

	// ErrRefreshTokenRevoked â€” status='revoked' (logged out or admin).
	ErrRefreshTokenRevoked = errors.New("auth: refresh token revoked")

	// ErrRefreshTokenAlreadyRotated â€” status='rotated' â†’ REUSE DETECTED.
	ErrRefreshTokenReuse = errors.New("auth: refresh token reuse detected")

	// ErrInvalidInput â€” empty username/password/challenge token/etc.
	ErrInvalidInput = errors.New("auth: invalid input")

	// ErrUserNotFound â€” user lookup returned no rows (not always returned to client).
	ErrUserNotFound = errors.New("auth: user not found")

	// ErrMFAChallengeInvalid â€” challenge token wrong/expired/used.
	ErrMFAChallengeInvalid = errors.New("auth: mfa challenge invalid")

	// ErrMFAChallengeExpired â€” expires_at <= now.
	ErrMFAChallengeExpired = errors.New("auth: mfa challenge expired")

	// ErrMFAAttemptsExceeded â€” too many TOTP attempts on single challenge.
	ErrMFAAttemptsExceeded = errors.New("auth: mfa attempts exceeded")
)

// =============================================================================
// RefreshToken entity
// =============================================================================

// RefreshTokenStatus mirrors refresh_tokens.status CHECK constraint.
type RefreshTokenStatus string

const (
	RefreshTokenActive   RefreshTokenStatus = "active"
	RefreshTokenRotated  RefreshTokenStatus = "rotated"
	RefreshTokenRevoked  RefreshTokenStatus = "revoked"
)

// RefreshTokenRevokeReason â€” why a token was revoked.
type RefreshTokenRevokeReason string

const (
	RevokeReasonRotated       RefreshTokenRevokeReason = "rotated"
	RevokeReasonReuseDetected RefreshTokenRevokeReason = "reuse_detected"
	RevokeReasonUserLogout    RefreshTokenRevokeReason = "user_logout"
	RevokeReasonAdminRevoke   RefreshTokenRevokeReason = "admin_revoke"
)

// RefreshToken is the server-side record for a refresh token.
// The raw (opaque) token is NEVER stored â€” only its SHA-256 hash.
type RefreshToken struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	UserID        uuid.UUID
	FamilyID      uuid.UUID                  // rotation chain â€” reuse-detection boundary
	TokenHash     string                     // SHA-256 hex of raw token
	Status        RefreshTokenStatus
	RotatedTo     *uuid.UUID                 // next token in chain
	RevokedReason *RefreshTokenRevokeReason // why revoked
	IssuedAt      time.Time
	ExpiresAt     time.Time
	LastUsedAt    *time.Time
	UserAgent     string
	IPAddress     *netip.Addr
}

// IsUsable returns true if token is still active and not expired.
func (t RefreshToken) IsUsable(now time.Time) bool {
	return t.Status == RefreshTokenActive && now.Before(t.ExpiresAt)
}

// =============================================================================
// UserCredentials entity
// =============================================================================

// UserCredentials holds password + MFA per user. One row per user_id.
type UserCredentials struct {
	UserID             uuid.UUID
	TenantID           uuid.UUID
	PasswordHash       string     // bcrypt
	MFAEnabled         bool
	MFASecret          string     // base32-encoded TOTP secret
	MFARecoveryCodes   []string   // hashed one-time recovery codes
	FailedLoginCount   int
	LockedUntil        *time.Time // NULL = not locked
	LastLoginAt        *time.Time
	PasswordChangedAt  time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// IsLocked returns true if account is currently locked.
func (c UserCredentials) IsLocked(now time.Time) bool {
	return c.LockedUntil != nil && now.Before(*c.LockedUntil)
}

// =============================================================================
// MFAChallenge entity
// =============================================================================

// MFAChallenge is a short-lived TOTP challenge created during login.
type MFAChallenge struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	UserID         uuid.UUID
	ChallengeToken string     // opaque (NOT a JWT) â€” returned to client
	Verified       bool
	Attempts       int
	IssuedAt       time.Time
	ExpiresAt      time.Time
	VerifiedAt     *time.Time
}

// IsUsable returns true if challenge is valid (not expired, not used, attempts < 3).
func (c MFAChallenge) IsUsable(now time.Time) bool {
	return !c.Verified && now.Before(c.ExpiresAt) && c.Attempts < 3
}

// =============================================================================
// LoginAttempt entity
// =============================================================================

// LoginAttemptFailureReason â€” why a login failed.
type LoginAttemptFailureReason string

// Constants are declared as vars (not consts) so callers can take their address
// when passing them to functions expecting *LoginAttemptFailureReason.
var (
	LoginFailureInvalidCredentials = LoginAttemptFailureReason("invalid_credentials")
	LoginFailureMFAFailed          = LoginAttemptFailureReason("mfa_failed")
	LoginFailureAccountLocked      = LoginAttemptFailureReason("account_locked")
	LoginFailureRateLimited        = LoginAttemptFailureReason("rate_limited")
	LoginFailurePolicyViolation    = LoginAttemptFailureReason("password_policy_violation") // Sprint 23 / 22B.4
)

// LoginAttempt records one login attempt (success or failure).
type LoginAttempt struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	UserID        *uuid.UUID // NULL if username not found
	Username      string     // raw input (even on user-not-found, for forensics)
	Succeeded     bool
	FailureReason *LoginAttemptFailureReason
	IPAddress     *netip.Addr
	UserAgent     string
	AttemptedAt   time.Time
}

// =============================================================================
// Repository interface
// =============================================================================

// Repository is the persistence boundary for auth entities.
// All write methods take a Tx so use cases control transaction boundary.
type Repository interface {
	// ---- Refresh tokens ----
	CreateRefreshToken(ctx context.Context, tx Tx, rt RefreshToken) error
	GetRefreshTokenByHash(ctx context.Context, hash string) (RefreshToken, error)
	MarkRefreshTokenRotated(ctx context.Context, tx Tx, id, rotatedToID uuid.UUID) error
	RevokeRefreshTokenFamily(ctx context.Context, tx Tx, familyID uuid.UUID, reason RefreshTokenRevokeReason) error
	RevokeRefreshToken(ctx context.Context, tx Tx, id uuid.UUID, reason RefreshTokenRevokeReason) error
	ListActiveRefreshTokensByUser(ctx context.Context, userID uuid.UUID) ([]RefreshToken, error)

	// ---- User credentials ----
	CreateUserCredentials(ctx context.Context, tx Tx, c UserCredentials) error
	GetUserCredentialsByUsername(ctx context.Context, tenantID uuid.UUID, username string) (UserCredentials, error)
	GetUserCredentialsByID(ctx context.Context, userID uuid.UUID) (UserCredentials, error)
	UpdateUserCredentials(ctx context.Context, tx Tx, c UserCredentials) error
	IncrementFailedLogin(ctx context.Context, tx Tx, userID uuid.UUID) error
	ResetFailedLogin(ctx context.Context, tx Tx, userID uuid.UUID) error

	// ---- MFA challenges ----
	CreateMFAChallenge(ctx context.Context, tx Tx, c MFAChallenge) error
	GetMFAChallengeByToken(ctx context.Context, token string) (MFAChallenge, error)
	MarkMFAChallengeVerified(ctx context.Context, tx Tx, id uuid.UUID) error
	IncrementMFAChallengeAttempts(ctx context.Context, tx Tx, id uuid.UUID) error

	// ---- Login attempts ----
	RecordLoginAttempt(ctx context.Context, tx Tx, a LoginAttempt) error
	CountRecentFailedLogins(ctx context.Context, userID uuid.UUID, since time.Time) (int, error)
}

// =============================================================================
// Tx abstraction (mirror pattern from period/reconciler/collection/currency)
// =============================================================================

// Tx is the transaction abstraction used by the auth module.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// CommandTag is the result of an Exec.
type CommandTag interface {
	RowsAffected() int64
}

// Row is a single-row query result.
type Row interface {
	Scan(dest ...any) error
}

// Rows is a multi-row query result.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// ErrSessionNotOwner — Sprint 23 / 22B.3: caller tried to act on a session
// that belongs to a different user. Handler maps this to 403 (not 404)
// deliberately to refuse enumeration: the SAME response (404) is returned
// whether the session exists for another user or doesn't exist at all.
var ErrSessionNotOwner = errors.New("auth: session does not belong to caller")
