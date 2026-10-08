-- Migration 000025 (down): reverse login_attempts partitioning.
--
-- WARNING: destructive. The original migration renames login_attempts →
-- login_attempts_old, copies data, and drops the old table. A full rollback
-- would need to:
--   1. Recreate login_attempts_old (no data; lost on forward migration)
--   2. Drop the partitioned login_attempts
--   3. Rename / recreate non-partitioned table
-- For production safety, run a forward-fix migration instead. This file only
-- drops the helper function so future migrations can redefine it.

BEGIN;
DROP FUNCTION IF EXISTS fmcg_create_login_attempts_partition(INT, INT);
COMMIT;