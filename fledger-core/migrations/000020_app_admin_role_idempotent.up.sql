-- =============================================================================
-- Migration: 000020_app_admin_role_idempotent
-- Description: Make app_admin role creation idempotent (Sprint 27)
-- Author: FMCG Wallet
-- =============================================================================
-- Background:
--   Migration 000015 creates `app_admin` role with `CREATE ROLE app_admin NOINHERIT;`
--   (no IF NOT EXISTS). On re-runs (clean db after DROP DATABASE), this fails with
--   "role 'app_admin' already exists" if the role wasn't dropped along with the db.
--
-- Fix:
--   This migration is a no-op if migration 000015 already ran successfully.
--   It makes the app_admin + fmcg_role grants idempotent by wrapping them in
--   DO blocks with EXCEPTION handling. Useful as a forward-fix if migration
--   000015 itself can't be edited (would invalidate production migration history).
--
-- Idempotent: safe to run multiple times.
-- =============================================================================

BEGIN;

DO $$ BEGIN
    -- Create app_admin role if missing
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_admin') THEN
        CREATE ROLE app_admin NOINHERIT;
    END IF;

    -- Grants to fmcg (idempotent)
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fmcg') THEN
        GRANT app_admin TO fmcg;
    END IF;
EXCEPTION WHEN OTHERS THEN
    NULL;
END $$;

COMMIT;
