-- Migration 000013 (down): Refresh tokens + MFA + sessions — rollback
BEGIN;

DROP TABLE IF EXISTS login_attempts;
DROP TABLE IF EXISTS mfa_challenges;
DROP TABLE IF EXISTS user_credentials;
DROP TABLE IF EXISTS refresh_tokens;

COMMIT;
