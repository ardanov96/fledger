// Package usecase — AuthService (Sprint 13 / Fase 2E lanjutan).
//
// Use cases for hardened authentication:
//   - Login: bcrypt verify password → if MFA enabled, issue challenge token.
//     If MFA not enabled, mint access (JWT) + refresh (opaque) tokens.
//   - VerifyMFA: client submits TOTP code → verify → mint token pair.
//   - Refresh: rotate refresh token (reuse detection = revoke family).
//   - Logout: revoke the refresh token (status='revoked', reason='user_logout').
//   - SetupMFA: enable MFA for a user (returns secret + otpauth:// URL).
//
// All write methods go through RunInTxAuthDomain so multi-step ops (rotate
// + mint new pair + record attempt) are atomic.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	"github.com/google/uuid"
	jwtv5 "github.com/golang-jwt/jwt/v5"

	"github.com/runut/fmcg-wallet/internal/auth/jwt"
	"github.com/runut/fmcg-wallet/internal/domain/auth"
	platformauth "github.com/runut/fmcg-wallet/internal/platform/auth"
)

// =============================================================================
// Errors (mapped to HTTP in handler)
// =============================================================================

var (
	ErrAuthInvalidInput = errors.New("auth: invalid input")
)

// =============================================================================
// Dependency interfaces (narrow, for testability)
// =============================================================================

// PasswordHasherDep is the narrow hash interface (satisfied by platform/auth.BcryptPasswordHasher).
type PasswordHasherDep interface {
	Hash(plaintext string) (string, error)
	Verify(plaintext, hash string) bool
}

// TokenGeneratorDep produces opaque tokens + their hashes.
type TokenGeneratorDep interface {
	Generate() (raw string, hash string, err error)
}

// TOTPGeneratorDep verifies TOTP codes.
type TOTPGeneratorDep interface {
	GenerateSecret() (string, error)
	Verify(secret, code string) bool
	OTPURL(secret, label, issuer string) string
}

// AuthTxRunner runs a function inside an auth-domain transaction.
type AuthTxRunner interface {
	RunInTxAuthDomain(ctx context.Context, fn func(auth.Tx) error) error
}

// JWTSignerDep signs access-token JWTs.
type JWTSignerDep interface {
	Sign(c jwt.Claims, ttl time.Duration) (string, error)
}

// AuthConfig bundles timeouts/policies.
type AuthConfig struct {
	AccessTokenTTL    time.Duration // default 15 min
	RefreshTokenTTL   time.Duration // default 7 days
	MFASessionTTL     time.Duration // default 5 min (challenge token lifetime)
	LockoutThreshold  int           // default 5 failed attempts
	LockoutDuration   time.Duration // default 15 min
	// PasswordPolicy is enforced on Login (Sprint 23 / 22B.4). The empty
	// value falls back to platformauth.DefaultPasswordPolicy() — supply one
	// to override (e.g. legacy demo data with weak passwords bypass via
	// production: skip policy by setting MinLength: 1).
	PasswordPolicy platformauth.PasswordPolicy
}

// DefaultAuthConfig returns sensible defaults.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		AccessTokenTTL:   15 * time.Minute,
		RefreshTokenTTL:  7 * 24 * time.Hour,
		MFASessionTTL:    5 * time.Minute,
		LockoutThreshold: 5,
		LockoutDuration:  15 * time.Minute,
		PasswordPolicy:   platformauth.DefaultPasswordPolicy(),
	}
}

// =============================================================================
// Input / Output
// =============================================================================

// LoginInput — first step of authentication.
type LoginInput struct {
	TenantID  uuid.UUID
	Username  string // in Sprint 13 = user_id UUID string
	Password  string
	UserAgent string
	IPAddress *netip.Addr
}

// LoginResult — response after username/password validated.
type LoginResult struct {
	// When MFA required:
	MFAChallengeToken string // opaque; client passes to VerifyMFA
	MFAGenerated      bool

	// When MFA not required (or after VerifyMFA): returned here for callers
	// that wrap the flow.
	UserID        uuid.UUID
	TenantID      uuid.UUID
	Role          string
	Scopes        []string

	AccessToken  string
	RefreshToken string
}

// VerifyMFAInput — client submits the TOTP code.
type VerifyMFAInput struct {
	ChallengeToken string
	Code           string
	UserAgent      string
	IPAddress      *netip.Addr
}

// RefreshInput — client rotates a refresh token.
type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IPAddress    *netip.Addr
}

// RefreshResult — new token pair + the (now-rotated) old refresh token info.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	UserID       uuid.UUID
	TenantID     uuid.UUID
}

// LogoutInput — revoke a refresh token (status='revoked', reason='user_logout').
type LogoutInput struct {
	RefreshToken string
}

// SetupMFAInput — enable MFA for a user (admin or self).
type SetupMFAInput struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
	Label    string // human-readable label for QR code, e.g. "user@tenant"
	Issuer   string // org name, e.g. "fmcg-wallet"
}

// SetupMFAResult — secret + otpauth:// URL for QR code provisioning.
type SetupMFAResult struct {
	Secret string
	OTPURL string
}

// =============================================================================
// Service
// =============================================================================

// AuthService orchestrates login, refresh, logout, MFA setup/verify.
type AuthService struct {
	repo          auth.Repository
	tx            AuthTxRunner
	hasher        PasswordHasherDep
	tokens        TokenGeneratorDep
	totp          TOTPGeneratorDep
	signer        JWTSignerDep
	cfg           AuthConfig
	log           *slog.Logger
	now           func() time.Time
}

// NewAuthService constructs the service.
func NewAuthService(
	repo auth.Repository,
	tx AuthTxRunner,
	hasher PasswordHasherDep,
	tokens TokenGeneratorDep,
	totp TOTPGeneratorDep,
	signer JWTSignerDep,
	cfg AuthConfig,
	log *slog.Logger,
) *AuthService {
	if log == nil {
		log = slog.Default()
	}
	if cfg.AccessTokenTTL == 0 {
		cfg = DefaultAuthConfig()
	}
	return &AuthService{
		repo:   repo,
		tx:     tx,
		hasher: hasher,
		tokens: tokens,
		totp:   totp,
		signer: signer,
		cfg:    cfg,
		log:    log,
		now:    time.Now,
	}
}

// =============================================================================
// Login
// =============================================================================

// Login validates username + password. If MFA is enabled, returns
// a challenge token (caller must then call VerifyMFA). Otherwise returns
// full token pair.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if in.Username == "" || in.Password == "" || in.TenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing required field", ErrAuthInvalidInput)
	}

	// Look up credentials.
	creds, err := s.repo.GetUserCredentialsByUsername(ctx, in.TenantID, in.Username)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			// Record a failed attempt (no user_id, but record username for forensics).
			_ = s.recordAttempt(ctx, in.TenantID, nil, in.Username, false, &auth.LoginFailureInvalidCredentials, in.IPAddress, in.UserAgent)
			return nil, auth.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("login: lookup credentials: %w", err)
	}

	// Sprint 23 (22B.4): enforce password policy BEFORE bcrypt to prevent
	// an attacker from probing weak passwords against the bcrypt timing
	// oracle (success/failure latency differs by ~250ms). The check is
	// only meaningful if the password satisfies the policy.
	//
	// Trade-off: this exposes the policy as a separate error path before
	// the user is identified. We treat it as "weak password supplied"
	// rather than leaking policy details — see Validate() for the
	// deliberate "missing required character class" generic message.
	if err := s.cfg.PasswordPolicy.Validate(in.Password); err != nil {
		_ = s.recordAttempt(ctx, in.TenantID, &creds.UserID, in.Username, false, &auth.LoginFailurePolicyViolation, in.IPAddress, in.UserAgent)
		return nil, err
	}

	// Lockout check.
	now := s.now()
	if creds.IsLocked(now) {
		_ = s.recordAttempt(ctx, in.TenantID, &creds.UserID, in.Username, false, &auth.LoginFailureAccountLocked, in.IPAddress, in.UserAgent)
		return nil, auth.ErrAccountLocked
	}

	// Password verify.
	if !s.hasher.Verify(in.Password, creds.PasswordHash) {
		// Increment failed counter; lock account if threshold reached.
		_ = s.recordFailedAttempt(ctx, &creds, in.IPAddress, in.UserAgent)
		return nil, auth.ErrInvalidCredentials
	}

	// Credentials OK. If MFA enabled, issue a challenge and return early.
	if creds.MFAEnabled {
		challengeRaw, _, err := s.tokens.Generate()
		if err != nil {
			return nil, fmt.Errorf("login: mfa challenge token: %w", err)
		}
		challengeID := uuid.New()
		expires := s.now().Add(s.cfg.MFASessionTTL)
		err = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
			return s.repo.CreateMFAChallenge(ctx, tx, auth.MFAChallenge{
				ID:             challengeID,
				TenantID:       in.TenantID,
				UserID:         creds.UserID,
				ChallengeToken: challengeRaw,
				ExpiresAt:      expires,
			})
		})
		if err != nil {
			return nil, fmt.Errorf("login: persist mfa challenge: %w", err)
		}
		_ = s.recordAttempt(ctx, in.TenantID, &creds.UserID, in.Username, true, nil, in.IPAddress, in.UserAgent)
		return &LoginResult{
			MFAChallengeToken: challengeRaw,
			MFAGenerated:      true,
			UserID:            creds.UserID,
			TenantID:          in.TenantID,
		}, nil
	}

	// No MFA — mint full token pair.
	pair, err := s.mintPair(ctx, creds, in.TenantID, in.UserAgent, in.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("login: mint token pair: %w", err)
	}
	_ = s.recordAttempt(ctx, in.TenantID, &creds.UserID, in.Username, true, nil, in.IPAddress, in.UserAgent)

	return &LoginResult{
		UserID:       creds.UserID,
		TenantID:     in.TenantID,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}

// =============================================================================
// VerifyMFA
// =============================================================================

// VerifyMFA verifies the TOTP code from a challenge and returns token pair.
func (s *AuthService) VerifyMFA(ctx context.Context, in VerifyMFAInput) (*RefreshResult, error) {
	if in.ChallengeToken == "" || in.Code == "" {
		return nil, fmt.Errorf("%w: missing challenge or code", ErrAuthInvalidInput)
	}

	challenge, err := s.repo.GetMFAChallengeByToken(ctx, in.ChallengeToken)
	if err != nil {
		if errors.Is(err, auth.ErrMFAChallengeInvalid) {
			return nil, auth.ErrMFAChallengeInvalid
		}
		return nil, fmt.Errorf("verify mfa: lookup challenge: %w", err)
	}

	// Validate challenge state.
	now := s.now()
	if challenge.Verified {
		return nil, auth.ErrMFAChallengeInvalid // already used
	}
	if !now.Before(challenge.ExpiresAt) {
		return nil, auth.ErrMFAChallengeExpired
	}
	if challenge.Attempts >= 3 {
		return nil, auth.ErrMFAAttemptsExceeded
	}

	// Look up credentials for MFA secret.
	creds, err := s.repo.GetUserCredentialsByID(ctx, challenge.UserID)
	if err != nil {
		return nil, fmt.Errorf("verify mfa: lookup credentials: %w", err)
	}
	if !creds.MFAEnabled || creds.MFASecret == "" {
		return nil, auth.ErrMFANotEnabled
	}

	// Verify TOTP code.
	if !s.totp.Verify(creds.MFASecret, in.Code) {
		// Increment attempts (best-effort).
		_ = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
			return s.repo.IncrementMFAChallengeAttempts(ctx, tx, challenge.ID)
		})
		_ = s.recordAttempt(ctx, creds.TenantID, &creds.UserID, "", false, &auth.LoginFailureMFAFailed, in.IPAddress, in.UserAgent)
		return nil, auth.ErrMFAFailed
	}

	// TOTP OK — mark challenge verified + mint token pair.
	err = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		if err := s.repo.MarkMFAChallengeVerified(ctx, tx, challenge.ID); err != nil {
			return err
		}
		return s.repo.ResetFailedLogin(ctx, tx, creds.UserID)
	})
	if err != nil {
		return nil, fmt.Errorf("verify mfa: persist: %w", err)
	}

	pair, err := s.mintPair(ctx, creds, creds.TenantID, in.UserAgent, in.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("verify mfa: mint pair: %w", err)
	}
	return &RefreshResult{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		UserID:       creds.UserID,
		TenantID:     creds.TenantID,
	}, nil
}

// =============================================================================
// Refresh — rotate refresh token with reuse-detection
// =============================================================================

// Refresh rotates a refresh token. Old token becomes 'rotated'; new
// token is 'active'. If a 'rotated' token is presented, ALL tokens in
// the family are revoked (theft defense).
func (s *AuthService) Refresh(ctx context.Context, in RefreshInput) (*RefreshResult, error) {
	if in.RefreshToken == "" {
		return nil, fmt.Errorf("%w: empty refresh token", ErrAuthInvalidInput)
	}

	rawToken := in.RefreshToken
	hash := sha256Hex(rawToken)

	existing, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, auth.ErrRefreshTokenInvalid) {
			return nil, auth.ErrRefreshTokenInvalid
		}
		return nil, fmt.Errorf("refresh: lookup: %w", err)
	}

	now := s.now()
	if existing.Status == auth.RefreshTokenRevoked {
		return nil, auth.ErrRefreshTokenRevoked
	}
	if !now.Before(existing.ExpiresAt) {
		return nil, auth.ErrRefreshTokenExpired
	}

	// Reuse detection: if token already rotated, treat as compromise.
	if existing.Status == auth.RefreshTokenRotated {
		// Revoke entire family.
		_ = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
			return s.repo.RevokeRefreshTokenFamily(ctx, tx, existing.FamilyID, auth.RevokeReasonReuseDetected)
		})
		s.log.Warn("refresh token reuse detected — family revoked",
			"family_id", existing.FamilyID,
			"user_id", existing.UserID,
		)
		return nil, auth.ErrRefreshTokenReuse
	}

	// Happy path: status='active'. Mark old as rotated, mint new pair.
	creds, err := s.repo.GetUserCredentialsByID(ctx, existing.UserID)
	if err != nil {
		return nil, fmt.Errorf("refresh: lookup user: %w", err)
	}

	newRaw, newHash, err := s.tokens.Generate()
	if err != nil {
		return nil, fmt.Errorf("refresh: gen new token: %w", err)
	}
	newID := uuid.New()

	err = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		// 1. Mark old as rotated.
		if err := s.repo.MarkRefreshTokenRotated(ctx, tx, existing.ID, newID); err != nil {
			return err
		}
		// 2. Create new.
		return s.repo.CreateRefreshToken(ctx, tx, auth.RefreshToken{
			ID:         newID,
			TenantID:   existing.TenantID,
			UserID:     existing.UserID,
			FamilyID:   existing.FamilyID,
			TokenHash:  newHash,
			Status:     auth.RefreshTokenActive,
			ExpiresAt:  now.Add(s.cfg.RefreshTokenTTL),
			UserAgent:  in.UserAgent,
			IPAddress:  in.IPAddress,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("refresh: persist new pair: %w", err)
	}

	// Mint new access token.
	accessToken, err := s.signer.Sign(jwt.Claims{
		UserID: creds.UserID.String(),
		Tenant: creds.TenantID.String(),
		Role:   "", // populated from claims lookup (out of Sprint 13 scope)
	}, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("refresh: sign access token: %w", err)
	}

	return &RefreshResult{
		AccessToken:  accessToken,
		RefreshToken: newRaw,
		UserID:       creds.UserID,
		TenantID:     creds.TenantID,
	}, nil
}

// =============================================================================
// Logout
// =============================================================================

// Logout revokes a refresh token.
func (s *AuthService) Logout(ctx context.Context, in LogoutInput) error {
	if in.RefreshToken == "" {
		return fmt.Errorf("%w: empty refresh token", ErrAuthInvalidInput)
	}
	hash := sha256Hex(in.RefreshToken)
	existing, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, auth.ErrRefreshTokenInvalid) {
			return auth.ErrRefreshTokenInvalid
		}
		return fmt.Errorf("logout: lookup: %w", err)
	}
	if existing.Status == auth.RefreshTokenRevoked {
		return auth.ErrRefreshTokenRevoked
	}
	return s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		return s.repo.RevokeRefreshToken(ctx, tx, existing.ID, auth.RevokeReasonUserLogout)
	})
}

// =============================================================================
// SetupMFA
// =============================================================================

// SetupMFA generates a new TOTP secret for a user. Does NOT enable MFA yet —
// user must complete a verification step (out of scope for Sprint 13; UI
// flow: scan QR → enter first TOTP → call enable endpoint).
func (s *AuthService) SetupMFA(ctx context.Context, in SetupMFAInput) (*SetupMFAResult, error) {
	if in.UserID == uuid.Nil || in.TenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: missing user/tenant", ErrAuthInvalidInput)
	}
	secret, err := s.totp.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("setup mfa: generate secret: %w", err)
	}
	// Persist secret (mfa_enabled still false until verification).
	err = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		c, err := s.repo.GetUserCredentialsByID(ctx, in.UserID)
		if err != nil {
			return err
		}
		c.MFASecret = secret
		return s.repo.UpdateUserCredentials(ctx, tx, c)
	})
	if err != nil {
		return nil, fmt.Errorf("setup mfa: persist: %w", err)
	}
	url := s.totp.OTPURL(secret, in.Label, in.Issuer)
	return &SetupMFAResult{Secret: secret, OTPURL: url}, nil
}

// =============================================================================
// Internal helpers
// =============================================================================

// mintedPair is the result of mintPair (internal).
type mintedPair struct {
	AccessToken  string
	RefreshToken string
}

// mintPair creates an access+refresh token pair with a fresh family_id.
func (s *AuthService) mintPair(ctx context.Context, creds auth.UserCredentials, tenantID uuid.UUID, userAgent string, ip *netip.Addr) (*mintedPair, error) {
	now := s.now()

	// 1. Sign access JWT.
	accessToken, err := s.signer.Sign(jwt.Claims{
		UserID: creds.UserID.String(),
		Tenant: tenantID.String(),
	}, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// 2. Generate + persist refresh token.
	raw, hash, err := s.tokens.Generate()
	if err != nil {
		return nil, fmt.Errorf("gen refresh token: %w", err)
	}
	rtID := uuid.New()
	familyID := uuid.New()

	err = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		return s.repo.CreateRefreshToken(ctx, tx, auth.RefreshToken{
			ID:         rtID,
			TenantID:   tenantID,
			UserID:     creds.UserID,
			FamilyID:   familyID,
			TokenHash:  hash,
			Status:     auth.RefreshTokenActive,
			ExpiresAt:  now.Add(s.cfg.RefreshTokenTTL),
			UserAgent:  userAgent,
			IPAddress:  ip,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("persist refresh token: %w", err)
	}

	// 3. Update last_login_at.
	_ = s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		c, err := s.repo.GetUserCredentialsByID(ctx, creds.UserID)
		if err != nil {
			return err
		}
		c.LastLoginAt = &now
		c.FailedLoginCount = 0
		c.LockedUntil = nil
		return s.repo.UpdateUserCredentials(ctx, tx, c)
	})

	return &mintedPair{AccessToken: accessToken, RefreshToken: raw}, nil
}

// recordFailedAttempt increments failed_login_count + applies lockout.
func (s *AuthService) recordFailedAttempt(ctx context.Context, creds *auth.UserCredentials, ip *netip.Addr, ua string) error {
	return s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		if err := s.repo.IncrementFailedLogin(ctx, tx, creds.UserID); err != nil {
			return err
		}
		// If threshold reached, set locked_until.
		updated, err := s.repo.GetUserCredentialsByID(ctx, creds.UserID)
		if err != nil {
			return err
		}
		if updated.FailedLoginCount >= s.cfg.LockoutThreshold {
			lockUntil := s.now().Add(s.cfg.LockoutDuration)
			updated.LockedUntil = &lockUntil
			if err := s.repo.UpdateUserCredentials(ctx, tx, updated); err != nil {
				return err
			}
		}
		return s.recordAttemptTx(ctx, tx, creds.TenantID, &creds.UserID, "", false, &auth.LoginFailureInvalidCredentials, ip, ua)
	})
}

// recordAttempt writes a login_attempts row outside any business tx.
func (s *AuthService) recordAttempt(ctx context.Context, tenantID uuid.UUID, userID *uuid.UUID, username string, ok bool, reason *auth.LoginAttemptFailureReason, ip *netip.Addr, ua string) error {
	return s.tx.RunInTxAuthDomain(ctx, func(tx auth.Tx) error {
		return s.recordAttemptTx(ctx, tx, tenantID, userID, username, ok, reason, ip, ua)
	})
}

func (s *AuthService) recordAttemptTx(ctx context.Context, tx auth.Tx, tenantID uuid.UUID, userID *uuid.UUID, username string, ok bool, reason *auth.LoginAttemptFailureReason, ip *netip.Addr, ua string) error {
	a := auth.LoginAttempt{
		ID:            uuid.New(),
		TenantID:      tenantID,
		UserID:        userID,
		Username:      username,
		Succeeded:     ok,
		FailureReason: reason,
		IPAddress:     ip,
		UserAgent:     ua,
		AttemptedAt:   s.now(),
	}
	return s.repo.RecordLoginAttempt(ctx, tx, a)
}

// sha256Hex returns the hex SHA-256 of input. Local helper to avoid importing
// crypto/sha256 into the usecase layer (the platform/auth package does it).
func sha256Hex(s string) string {
	return sha256HexBytes([]byte(s))
}

func sha256HexBytes(b []byte) string {
	h := sha256.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// _ = jwtv5.NewNumericDate silences "imported but not used" if the import
// is otherwise only referenced via the embedded jwt.Claims type.
var _ = jwtv5.NewNumericDate
