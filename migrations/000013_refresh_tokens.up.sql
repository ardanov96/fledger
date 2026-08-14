-- Migration 000013: Refresh tokens + MFA + session management (Fase 2E lanjutan)
-- Sprint 13
--
-- Adds:
--   1. Table `refresh_tokens` — opaque token with reuse-detection (rotation pattern)
--   2. Table `user_credentials` — password hash + MFA secret per user
--   3. Table `mfa_challenges` — short-lived TOTP challenge during login
--   4. Table `login_attempts` — brute force protection (account lockout after N attempts)
--   5. Indexes untuk hot-path queries
--   6. DB-level constraints untuk ensure token hygiene

BEGIN;

-- ============================================================================
-- Table: refresh_tokens
-- ============================================================================
-- Sprint 13 — refresh token rotation pattern with reuse detection.
-- A refresh token is opaque (NOT a JWT); it has a hash stored server-side.
-- On rotation:
--   1. Client sends refresh_token + access_token (for context)
--   2. Server hash-matches; if found + not revoked + not expired → mint NEW pair
--   3. Mark old token as 'rotated' (NOT deleted) for audit trail
--   4. If a 'rotated' token is presented again → REUSE DETECTED → revoke ENTIRE
--      family (all tokens with same family_id) — this is the token-theft defense
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    user_id         UUID NOT NULL,
    family_id       UUID NOT NULL,                  -- rotation chain; reused-detection boundary
    token_hash      TEXT NOT NULL,                  -- SHA-256 of opaque token; raw token never stored
    status          TEXT NOT NULL DEFAULT 'active', -- 'active' | 'rotated' | 'revoked'
    rotated_to      UUID REFERENCES refresh_tokens(id), -- next token in chain
    revoked_reason  TEXT,                          -- 'rotated' | 'reuse_detected' | 'user_logout' | 'admin_revoke'
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,           -- typically 7-30 days
    last_used_at    TIMESTAMPTZ,
    user_agent      TEXT,
    ip_address      INET,
    CONSTRAINT refresh_tokens_status_valid CHECK (status IN ('active', 'rotated', 'revoked')),
    CONSTRAINT refresh_tokens_expires_after_issued CHECK (expires_at > issued_at),
    CONSTRAINT refresh_tokens_token_hash_unique UNIQUE (token_hash)
);

-- Hot-path: lookup by token_hash (auth check on every refresh request)
-- Already unique, but explicit index helps planner.
CREATE INDEX IF NOT EXISTS refresh_tokens_token_hash_idx
    ON refresh_tokens (token_hash);

-- Per-user session list (UI: "active sessions" page)
CREATE INDEX IF NOT EXISTS refresh_tokens_user_active_idx
    ON refresh_tokens (user_id, issued_at DESC)
    WHERE status = 'active';

-- Reuse-detection scan: find all tokens in a family
CREATE INDEX IF NOT EXISTS refresh_tokens_family_idx
    ON refresh_tokens (family_id);

-- TTL cleanup job (run nightly): DELETE WHERE expires_at < NOW() AND status IN ('revoked', 'rotated')
CREATE INDEX IF NOT EXISTS refresh_tokens_ttl_idx
    ON refresh_tokens (expires_at)
    WHERE status IN ('rotated', 'revoked');

-- ============================================================================
-- Table: user_credentials
-- ============================================================================
-- One row per user_id. Holds bcrypt password hash + MFA secret + recovery codes.
-- Separated from any "users" table (Fase 5+ will introduce a proper users table).
CREATE TABLE IF NOT EXISTS user_credentials (
    user_id           UUID PRIMARY KEY,            -- 1:1 with user_id (from JWT claims)
    tenant_id         UUID NOT NULL,
    password_hash     TEXT NOT NULL,                 -- bcrypt
    mfa_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_secret        TEXT,                         -- base32-encoded TOTP secret (encrypted at rest in production)
    mfa_recovery_codes TEXT[],                       -- hashed one-time recovery codes
    failed_login_count INT  NOT NULL DEFAULT 0,
    locked_until      TIMESTAMPTZ,                  -- NULL = not locked; non-NULL = locked until this time
    last_login_at     TIMESTAMPTZ,
    password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS user_credentials_tenant_idx
    ON user_credentials (tenant_id);

-- ============================================================================
-- Table: mfa_challenges
-- ============================================================================
-- Short-lived (5 min) TOTP challenge created during login flow when MFA is enabled.
-- Once verified, the issued refresh/access tokens are bound to this challenge_id.
CREATE TABLE IF NOT EXISTS mfa_challenges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    user_id         UUID NOT NULL,
    challenge_token TEXT NOT NULL UNIQUE,           -- opaque token returned to client (NOT a JWT)
    verified        BOOLEAN NOT NULL DEFAULT FALSE,
    attempts        INT NOT NULL DEFAULT 0,         -- TOTP attempts; max 3
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,           -- typically 5 minutes
    verified_at     TIMESTAMPTZ,
    CONSTRAINT mfa_challenges_expires_after_issued CHECK (expires_at > issued_at),
    CONSTRAINT mfa_challenges_attempts_nonneg CHECK (attempts >= 0)
);

CREATE INDEX IF NOT EXISTS mfa_challenges_token_idx
    ON mfa_challenges (challenge_token);

-- TTL cleanup
CREATE INDEX IF NOT EXISTS mfa_challenges_ttl_idx
    ON mfa_challenges (expires_at);

-- ============================================================================
-- Table: login_attempts (audit trail)
-- ============================================================================
-- Append-only history of all login attempts (success + failure) for
-- forensics + rate-limiting / lockout heuristics.
CREATE TABLE IF NOT EXISTS login_attempts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    user_id      UUID,                              -- NULL if username not found (still record for brute-force pattern detection)
    username     TEXT NOT NULL,                     -- raw input (even if user not found) for forensics
    succeeded    BOOLEAN NOT NULL,
    failure_reason TEXT,                            -- 'invalid_credentials' | 'mfa_failed' | 'account_locked' | 'rate_limited' | NULL on success
    ip_address   INET,
    user_agent   TEXT,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS login_attempts_user_time_idx
    ON login_attempts (user_id, attempted_at DESC);

CREATE INDEX IF NOT EXISTS login_attempts_ip_time_idx
    ON login_attempts (ip_address, attempted_at DESC);

CREATE INDEX IF NOT EXISTS login_attempts_username_time_idx
    ON login_attempts (username, attempted_at DESC);

COMMIT;
