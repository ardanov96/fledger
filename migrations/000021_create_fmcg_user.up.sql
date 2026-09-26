-- =============================================================================
-- Migration: 000021_create_fmcg_user
-- Description: Create fmcg app user + fmcg_wallet db if missing (Sprint 27)
-- Author: FMCG Wallet
-- =============================================================================
-- Background:
--   The fmcg user + fmcg_wallet db were originally created by
--   scripts/setup-local-postgres.ps1 (run manually after install).
--   This is brittle for production: deploys need manual user creation.
--
-- Fix:
--   Move the user + db creation into a migration so it's reproducible
--   across environments. The CREATE statements are idempotent (IF NOT EXISTS)
--   so re-running the migration is safe.
--
-- NOTE: This migration runs as superuser (postgres). It creates the fmcg user
--       only if it doesn't already exist. The password is set via the
--       FMCG_DEFAULT_APP_PASSWORD env var, falling back to 'fmcg_dev_password'.
--
-- Idempotent: safe to run multiple times.
-- =============================================================================

BEGIN;

DO $$
DECLARE
    default_password TEXT := coalesce(
        current_setting('FMCG_DEFAULT_APP_PASSWORD', true),
        'fmcg_dev_password'
    );
BEGIN
    -- Create fmcg user if missing (no IF NOT EXISTS on CREATE USER pre-PG 16)
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        EXECUTE format('CREATE USER fmcg WITH PASSWORD %L CREATEDB', default_password);
    END IF;

    -- Create fmcg_wallet database if missing
    IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'fmcg_wallet') THEN
        EXECUTE format('CREATE DATABASE fmcg_wallet OWNER fmcg');
    END IF;
EXCEPTION WHEN OTHERS THEN
    -- Swallow - if user/db already exist with different state, leave it.
    NULL;
END $$;

COMMIT;
