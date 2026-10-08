-- =============================================================================
-- Migration: 000001_init_extensions
-- Description: Enable required PostgreSQL extensions
-- Author: FMCG Wallet
-- =============================================================================
-- This migration sets up extensions that all other migrations depend on.
-- Run FIRST. It is intentionally minimal — no business tables yet.

BEGIN;

-- pgcrypto: for gen_random_uuid(), digest(), encryption helpers
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- pg_stat_statements: query performance monitoring (used by observability)
-- Requires shared_preload_libraries = 'pg_stat_statements' in postgresql.conf
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- btree_gist: needed for GiST indexes on btree types (used by exclusion
-- constraints, e.g. prevent overlapping date ranges in accounting periods)
CREATE EXTENSION IF NOT EXISTS "btree_gist";

-- citext: case-insensitive text (useful for emails, codes, etc.)
CREATE EXTENSION IF NOT EXISTS "citext";

-- uuid-ossp: alternative UUID generator (we use gen_random_uuid from pgcrypto,
-- but having both available gives flexibility)
-- Note: Skip if pgcrypto already provides what we need
-- CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

COMMIT;

-- =============================================================================
-- DOWN
-- =============================================================================
-- Rollback (in development only; do NOT drop extensions in production):
-- DROP EXTENSION IF EXISTS "btree_gist";
-- DROP EXTENSION IF EXISTS "citext";
-- DROP EXTENSION IF EXISTS "pg_stat_statements";
-- DROP EXTENSION IF EXISTS "pgcrypto";
