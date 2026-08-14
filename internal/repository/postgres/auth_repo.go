// auth_repo.go — Postgres-backed implementation of auth.Repository.
//
// Sprint 13 / Fase 2E lanjutan. All writes use auth.Tx (wrapping pgx.Tx)
// so use cases control the transaction boundary.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/runut/fmcg-wallet/internal/domain/auth"
)

// =============================================================================
// Repo
// =============================================================================

// AuthRepository implements auth.Repository against Postgres.
type AuthRepository struct {
	db *DB
}

// NewAuthRepository constructs an AuthRepository.
func NewAuthRepository(db *DB) *AuthRepository {
	return &AuthRepository{db: db}
}

// =============================================================================
// DTOs
// =============================================================================

type refreshTokenDTO struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	UserID        uuid.UUID
	FamilyID      uuid.UUID
	TokenHash     string
	Status        string
	RotatedTo     *uuid.UUID
	RevokedReason *string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	LastUsedAt    *time.Time
	UserAgent     string
	IPAddress     *netip.Addr
}

type userCredentialsDTO struct {
	UserID            uuid.UUID
	TenantID          uuid.UUID
	PasswordHash      string
	MFAEnabled        bool
	MFASecret         *string
	MFARecoveryCodes  []string
	FailedLoginCount  int
	LockedUntil       *time.Time
	LastLoginAt       *time.Time
	PasswordChangedAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type mfaChallengeDTO struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	UserID         uuid.UUID
	ChallengeToken string
	Verified       bool
	Attempts       int
	IssuedAt       time.Time
	ExpiresAt      time.Time
	VerifiedAt     *time.Time
}

// =============================================================================
// Refresh tokens
// =============================================================================

// CreateRefreshToken inserts a new refresh token. Tx-bound.
func (r *AuthRepository) CreateRefreshToken(ctx context.Context, tx auth.Tx, rt auth.RefreshToken) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`INSERT INTO refresh_tokens
            (id, tenant_id, user_id, family_id, token_hash, status, expires_at, user_agent, ip_address)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		rt.ID, rt.TenantID, rt.UserID, rt.FamilyID, rt.TokenHash, string(rt.Status),
		rt.ExpiresAt, nullString(rt.UserAgent), addrArg(rt.IPAddress),
	)
	if err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("create refresh token: expected 1 row, got %d", tag.RowsAffected())
	}
	return nil
}

// GetRefreshTokenByHash reads a refresh token by its hash.
func (r *AuthRepository) GetRefreshTokenByHash(ctx context.Context, hash string) (auth.RefreshToken, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, family_id, token_hash, status, rotated_to,
                revoked_reason, issued_at, expires_at, last_used_at, user_agent, ip_address
         FROM refresh_tokens
         WHERE token_hash = $1`,
		hash,
	)
	d, err := scanRefreshToken(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.RefreshToken{}, auth.ErrRefreshTokenInvalid
		}
		return auth.RefreshToken{}, fmt.Errorf("get refresh token: %w", err)
	}
	return dtoToRefreshToken(d), nil
}

// MarkRefreshTokenRotated flips status to 'rotated' and sets rotated_to pointer.
func (r *AuthRepository) MarkRefreshTokenRotated(ctx context.Context, tx auth.Tx, id, rotatedToID uuid.UUID) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`UPDATE refresh_tokens
         SET status = 'rotated', rotated_to = $2, revoked_reason = 'rotated'
         WHERE id = $1 AND status = 'active'`,
		id, rotatedToID,
	)
	if err != nil {
		return fmt.Errorf("mark refresh token rotated: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrRefreshTokenInvalid
	}
	return nil
}

// RevokeRefreshTokenFamily revokes ALL tokens in a family (reuse-detection response).
func (r *AuthRepository) RevokeRefreshTokenFamily(ctx context.Context, tx auth.Tx, familyID uuid.UUID, reason auth.RefreshTokenRevokeReason) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	_, err = pgxTx.Exec(ctx,
		`UPDATE refresh_tokens
         SET status = 'revoked', revoked_reason = $2
         WHERE family_id = $1 AND status = 'active'`,
		familyID, string(reason),
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token family: %w", err)
	}
	return nil
}

// RevokeRefreshToken revokes a single token.
func (r *AuthRepository) RevokeRefreshToken(ctx context.Context, tx auth.Tx, id uuid.UUID, reason auth.RefreshTokenRevokeReason) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`UPDATE refresh_tokens
         SET status = 'revoked', revoked_reason = $2
         WHERE id = $1 AND status = 'active'`,
		id, string(reason),
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrRefreshTokenInvalid
	}
	return nil
}

// ListActiveRefreshTokensByUser returns active sessions for a user.
func (r *AuthRepository) ListActiveRefreshTokensByUser(ctx context.Context, userID uuid.UUID) ([]auth.RefreshToken, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, tenant_id, user_id, family_id, token_hash, status, rotated_to,
                revoked_reason, issued_at, expires_at, last_used_at, user_agent, ip_address
         FROM refresh_tokens
         WHERE user_id = $1 AND status = 'active'
         ORDER BY issued_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list active refresh tokens: %w", err)
	}
	defer rows.Close()

	var out []auth.RefreshToken
	for rows.Next() {
		d, err := scanRefreshToken(rows)
		if err != nil {
			return nil, fmt.Errorf("scan refresh token: %w", err)
		}
		out = append(out, dtoToRefreshToken(d))
	}
	return out, rows.Err()
}

// =============================================================================
// User credentials
// =============================================================================

// CreateUserCredentials inserts a new user_credentials row. Tx-bound.
func (r *AuthRepository) CreateUserCredentials(ctx context.Context, tx auth.Tx, c auth.UserCredentials) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`INSERT INTO user_credentials
            (user_id, tenant_id, password_hash, mfa_enabled, mfa_secret, mfa_recovery_codes)
         VALUES ($1, $2, $3, $4, $5, $6)`,
		c.UserID, c.TenantID, c.PasswordHash, c.MFAEnabled,
		nullStringPtr(c.MFASecret), c.MFARecoveryCodes,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("user_credentials already exists: %w", err)
		}
		return fmt.Errorf("create user credentials: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("create user credentials: expected 1 row, got %d", tag.RowsAffected())
	}
	return nil
}

// GetUserCredentialsByUsername looks up credentials by (tenant, username).
//
// NOTE: Sprint 13 ships a minimal lookup. A proper "users" table (Fase 5+)
// will replace this with a JOIN. For now, username = user_id (string UUID).
func (r *AuthRepository) GetUserCredentialsByUsername(ctx context.Context, tenantID uuid.UUID, username string) (auth.UserCredentials, error) {
	// username = user_id (UUID string) in this minimal schema.
	uid, err := uuid.Parse(username)
	if err != nil {
		return auth.UserCredentials{}, auth.ErrUserNotFound
	}
	return r.GetUserCredentialsByID(ctx, uid)
}

// GetUserCredentialsByID reads by primary key.
func (r *AuthRepository) GetUserCredentialsByID(ctx context.Context, userID uuid.UUID) (auth.UserCredentials, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT user_id, tenant_id, password_hash, mfa_enabled, mfa_secret,
                mfa_recovery_codes, failed_login_count, locked_until, last_login_at,
                password_changed_at, created_at, updated_at
         FROM user_credentials
         WHERE user_id = $1`,
		userID,
	)
	d, err := scanUserCredentials(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.UserCredentials{}, auth.ErrUserNotFound
		}
		return auth.UserCredentials{}, fmt.Errorf("get user credentials: %w", err)
	}
	return dtoToUserCredentials(d), nil
}

// UpdateUserCredentials updates mutable fields. Tx-bound.
func (r *AuthRepository) UpdateUserCredentials(ctx context.Context, tx auth.Tx, c auth.UserCredentials) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`UPDATE user_credentials
         SET password_hash = $2, mfa_enabled = $3, mfa_secret = $4,
             mfa_recovery_codes = $5, failed_login_count = $6,
             locked_until = $7, last_login_at = $8, updated_at = NOW()
         WHERE user_id = $1`,
		c.UserID, c.PasswordHash, c.MFAEnabled, nullStringPtr(c.MFASecret),
		c.MFARecoveryCodes, c.FailedLoginCount, c.LockedUntil, c.LastLoginAt,
	)
	if err != nil {
		return fmt.Errorf("update user credentials: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrUserNotFound
	}
	return nil
}

// IncrementFailedLogin bumps failed_login_count by 1.
func (r *AuthRepository) IncrementFailedLogin(ctx context.Context, tx auth.Tx, userID uuid.UUID) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	_, err = pgxTx.Exec(ctx,
		`UPDATE user_credentials SET failed_login_count = failed_login_count + 1 WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("increment failed login: %w", err)
	}
	return nil
}

// ResetFailedLogin resets failed_login_count to 0.
func (r *AuthRepository) ResetFailedLogin(ctx context.Context, tx auth.Tx, userID uuid.UUID) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	_, err = pgxTx.Exec(ctx,
		`UPDATE user_credentials SET failed_login_count = 0, locked_until = NULL WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("reset failed login: %w", err)
	}
	return nil
}

// =============================================================================
// MFA challenges
// =============================================================================

// CreateMFAChallenge inserts a new challenge. Tx-bound.
func (r *AuthRepository) CreateMFAChallenge(ctx context.Context, tx auth.Tx, c auth.MFAChallenge) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`INSERT INTO mfa_challenges
            (id, tenant_id, user_id, challenge_token, expires_at)
         VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.TenantID, c.UserID, c.ChallengeToken, c.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create mfa challenge: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("create mfa challenge: expected 1 row, got %d", tag.RowsAffected())
	}
	return nil
}

// GetMFAChallengeByToken looks up a challenge by its opaque token.
func (r *AuthRepository) GetMFAChallengeByToken(ctx context.Context, token string) (auth.MFAChallenge, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, challenge_token, verified, attempts,
                issued_at, expires_at, verified_at
         FROM mfa_challenges
         WHERE challenge_token = $1`,
		token,
	)
	d, err := scanMFAChallenge(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.MFAChallenge{}, auth.ErrMFAChallengeInvalid
		}
		return auth.MFAChallenge{}, fmt.Errorf("get mfa challenge: %w", err)
	}
	return dtoToMFAChallenge(d), nil
}

// MarkMFAChallengeVerified flips verified=true and sets verified_at.
func (r *AuthRepository) MarkMFAChallengeVerified(ctx context.Context, tx auth.Tx, id uuid.UUID) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	tag, err := pgxTx.Exec(ctx,
		`UPDATE mfa_challenges SET verified = TRUE, verified_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("mark mfa challenge verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrMFAChallengeInvalid
	}
	return nil
}

// IncrementMFAChallengeAttempts bumps attempts by 1.
func (r *AuthRepository) IncrementMFAChallengeAttempts(ctx context.Context, tx auth.Tx, id uuid.UUID) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	_, err = pgxTx.Exec(ctx,
		`UPDATE mfa_challenges SET attempts = attempts + 1 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("increment mfa challenge attempts: %w", err)
	}
	return nil
}

// =============================================================================
// Login attempts
// =============================================================================

// RecordLoginAttempt appends to login_attempts. Tx-bound.
func (r *AuthRepository) RecordLoginAttempt(ctx context.Context, tx auth.Tx, a auth.LoginAttempt) error {
	pgxTx, err := UnwrapPgxTxFromAuth(tx)
	if err != nil {
		return err
	}
	_, err = pgxTx.Exec(ctx,
		`INSERT INTO login_attempts
            (id, tenant_id, user_id, username, succeeded, failure_reason, ip_address, user_agent)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.TenantID, a.UserID, a.Username, a.Succeeded,
		nullFailureReason(a.FailureReason), addrArg(a.IPAddress), nullString(a.UserAgent),
	)
	if err != nil {
		return fmt.Errorf("record login attempt: %w", err)
	}
	return nil
}

// CountRecentFailedLogins counts failures within the window.
func (r *AuthRepository) CountRecentFailedLogins(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	row := r.db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM login_attempts
         WHERE user_id = $1 AND succeeded = FALSE AND attempted_at >= $2`,
		userID, since,
	)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("count recent failed logins: %w", err)
	}
	return n, nil
}

// =============================================================================
// Helpers
// =============================================================================

func scanRefreshToken(row pgx.Row) (refreshTokenDTO, error) {
	var d refreshTokenDTO
	err := row.Scan(
		&d.ID, &d.TenantID, &d.UserID, &d.FamilyID, &d.TokenHash, &d.Status,
		&d.RotatedTo, &d.RevokedReason, &d.IssuedAt, &d.ExpiresAt,
		&d.LastUsedAt, &d.UserAgent, &d.IPAddress,
	)
	if err != nil {
		return refreshTokenDTO{}, err
	}
	return d, nil
}

func dtoToRefreshToken(d refreshTokenDTO) auth.RefreshToken {
	out := auth.RefreshToken{
		ID:            d.ID,
		TenantID:      d.TenantID,
		UserID:        d.UserID,
		FamilyID:      d.FamilyID,
		TokenHash:     d.TokenHash,
		Status:        auth.RefreshTokenStatus(d.Status),
		RotatedTo:     d.RotatedTo,
		IssuedAt:      d.IssuedAt,
		ExpiresAt:     d.ExpiresAt,
		LastUsedAt:    d.LastUsedAt,
		UserAgent:     d.UserAgent,
		IPAddress:     d.IPAddress,
	}
	if d.RevokedReason != nil {
		r := auth.RefreshTokenRevokeReason(*d.RevokedReason)
		out.RevokedReason = &r
	}
	return out
}

func scanUserCredentials(row pgx.Row) (userCredentialsDTO, error) {
	var d userCredentialsDTO
	err := row.Scan(
		&d.UserID, &d.TenantID, &d.PasswordHash, &d.MFAEnabled, &d.MFASecret,
		&d.MFARecoveryCodes, &d.FailedLoginCount, &d.LockedUntil, &d.LastLoginAt,
		&d.PasswordChangedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return userCredentialsDTO{}, err
	}
	return d, nil
}

func dtoToUserCredentials(d userCredentialsDTO) auth.UserCredentials {
	out := auth.UserCredentials{
		UserID:            d.UserID,
		TenantID:          d.TenantID,
		PasswordHash:      d.PasswordHash,
		MFAEnabled:        d.MFAEnabled,
		FailedLoginCount:  d.FailedLoginCount,
		LockedUntil:       d.LockedUntil,
		LastLoginAt:       d.LastLoginAt,
		PasswordChangedAt: d.PasswordChangedAt,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
		MFARecoveryCodes:  d.MFARecoveryCodes,
	}
	if d.MFASecret != nil {
		out.MFASecret = *d.MFASecret
	}
	return out
}

func scanMFAChallenge(row pgx.Row) (mfaChallengeDTO, error) {
	var d mfaChallengeDTO
	err := row.Scan(
		&d.ID, &d.TenantID, &d.UserID, &d.ChallengeToken, &d.Verified,
		&d.Attempts, &d.IssuedAt, &d.ExpiresAt, &d.VerifiedAt,
	)
	if err != nil {
		return mfaChallengeDTO{}, err
	}
	return d, nil
}

func dtoToMFAChallenge(d mfaChallengeDTO) auth.MFAChallenge {
	return auth.MFAChallenge{
		ID:             d.ID,
		TenantID:       d.TenantID,
		UserID:         d.UserID,
		ChallengeToken: d.ChallengeToken,
		Verified:       d.Verified,
		Attempts:       d.Attempts,
		IssuedAt:       d.IssuedAt,
		ExpiresAt:      d.ExpiresAt,
		VerifiedAt:     d.VerifiedAt,
	}
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullStringPtr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func addrArg(a *netip.Addr) any {
	if a == nil {
		return nil
	}
	return *a
}

func nullFailureReason(r *auth.LoginAttemptFailureReason) any {
	if r == nil {
		return nil
	}
	return string(*r)
}

// Ensure pgconn import isn't dropped by linter (we use isUniqueViolation).
var _ = pgconn.PgError{}
