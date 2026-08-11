-- =============================================================================
-- Migration: 000001_init_extensions (ROLLBACK)
-- WARNING: Dropping extensions in production can break other apps
--          that depend on them. Only run this in dev.
-- =============================================================================

BEGIN;

DROP EXTENSION IF EXISTS "btree_gist";
DROP EXTENSION IF EXISTS "citext";
-- Keep pg_stat_statements; it's a monitoring extension, harmless to leave
-- DROP EXTENSION IF EXISTS "pg_stat_statements";
-- Keep pgcrypto; it provides gen_random_uuid() used by many tables
-- DROP EXTENSION IF EXISTS "pgcrypto";

COMMIT;
