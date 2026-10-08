-- =============================================================================
-- Migration: 000025_login_attempts_partitioning
-- Description: Partition login_attempts by month (Sprint 32)
-- Author: FMCG Wallet
-- =============================================================================
-- Sprint 32 follow-up to migration 000013. The login_attempts table grows
-- unbounded (audit trail of every login + brute-force protection). For a
-- production wallet with thousands of tenants, this can grow to millions
-- of rows in months. The indexes (user_id, ip_address, username) become
-- expensive to maintain, and old data has no operational value beyond a
-- 6-12 month retention window for security forensics.
--
-- Strategy (Postgres 12+ declarative partitioning):
--   1. Rename existing table to login_attempts_old (preserves data)
--   2. Create new login_attempts table PARTITION BY RANGE (attempted_at)
--   3. Pre-create monthly partitions for [last 3 months ... current +
--   next 12 months] (total 16 partitions)
--   4. Copy data from old table to new partitioned table (Postgres
--      routes inserts to the right partition based on attempted_at)
--   5. Drop old table
--   6. Add helper function `fmcg_create_login_attempts_partition(year, month)`
--      for ops to create future partitions beyond the pre-created window
--   7. Idempotent: safe to re-run (drops + recreates old table)
--
-- Operational notes:
--   - To create a new partition: SELECT fmcg_create_login_attempts_partition(2027, 6);
--   - To drop old partition for retention: DROP TABLE login_attempts_2025_07;
--   - Retention policy is operator's choice (default 12 months)
--   - Runbook reference: docs/runbooks/login-attempts-retention.md
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Rename existing table (preserve data)
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'login_attempts')
        AND NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'login_attempts_old') THEN
        ALTER TABLE login_attempts RENAME TO login_attempts_old;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 2. Create new partitioned table (same schema as migration 000013)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS login_attempts (
    id            UUID         NOT NULL DEFAULT gen_random_uuid(),
    tenant_id     UUID         NOT NULL,
    user_id       UUID,                                       -- NULL if username not found
    username      TEXT         NOT NULL,
    succeeded     BOOLEAN      NOT NULL,
    failure_reason TEXT,                                       -- see auth.LoginAttemptFailureReason
    ip_address    INET,
    user_agent    TEXT,
    attempted_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, attempted_at)                            -- partition key must be in PK
) PARTITION BY RANGE (attempted_at);

-- ---------------------------------------------------------------------------
-- 3. Pre-create monthly partitions for [last 3 ... next 12 months]
-- ---------------------------------------------------------------------------
-- Helper: generates partition DDL for a given (year, month). Uses immutable
-- date arithmetic so partition names are deterministic.
DO $$
DECLARE
    start_date DATE := date_trunc('month', NOW())::DATE - INTERVAL '3 months';
    end_date   DATE := date_trunc('month', NOW())::DATE + INTERVAL '12 months';
    cur        DATE;
    partition_name TEXT;
    next_month DATE;
BEGIN
    cur := start_date;
    WHILE cur < end_date LOOP
        next_month := cur + INTERVAL '1 month';
        partition_name := format('login_attempts_%s', to_char(cur, 'YYYY_MM'));
        EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF login_attempts FOR VALUES FROM (%L) TO (%L)',
                partition_name, cur, next_month
            );
        cur := next_month;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- 4. Per-partition indexes (auto-attached to parent via PARTITION OF)
-- ---------------------------------------------------------------------------
-- We can't create the indexes directly on the partitioned table; they get
-- auto-created on each partition via the partition DDL. But since we want
-- identical indexes to the pre-partitioning version, we re-create them on
-- the partitioned table and Postgres propagates to all partitions.
DO $$
DECLARE
    t TEXT;
BEGIN
    FOR t IN
        SELECT inhrelid::regclass::text
        FROM pg_inherits
        WHERE inhparent = 'login_attempts'::regclass
    LOOP
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %s (user_id, attempted_at DESC)', t || '_user_time_idx', t);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %s (ip_address, attempted_at DESC)', t || '_ip_time_idx', t);
        EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %s (username, attempted_at DESC)', t || '_username_time_idx', t);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- 5. Copy data from old table (Postgres routes inserts to right partition)
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'login_attempts_old') THEN
        INSERT INTO login_attempts
            (id, tenant_id, user_id, username, succeeded, failure_reason, ip_address, user_agent, attempted_at)
        SELECT id, tenant_id, user_id, username, succeeded, failure_reason, ip_address, user_agent, attempted_at
        FROM login_attempts_old
        ON CONFLICT DO NOTHING;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 6. Drop old table
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS login_attempts_old;

-- ---------------------------------------------------------------------------
-- 7. Helper function for ops to create future partitions
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION fmcg_create_login_attempts_partition(target_year INT, target_month INT)
RETURNS TEXT AS $$
DECLARE
    start_date DATE := make_date(target_year, target_month, 1);
    end_date   DATE := start_date + INTERVAL '1 month';
    partition_name TEXT := format('login_attempts_%s', to_char(start_date, 'YYYY_MM'));
BEGIN
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF login_attempts FOR VALUES FROM (%L) TO (%L)',
        partition_name, start_date, end_date
    );
    RETURN partition_name;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION fmcg_create_login_attempts_partition IS
    'Sprint 32: create a monthly partition for login_attempts. '
    'Example: SELECT fmcg_create_login_attempts_partition(2027, 6); '
    'For retention, simply DROP TABLE login_attempts_YYYY_MM;';

COMMENT ON TABLE login_attempts IS
    'Sprint 32: append-only audit trail of login attempts, partitioned by month. '
    'Pre-created partitions cover [now-3 months ... now+12 months]. '
    'Use fmcg_create_login_attempts_partition(year, month) to create future partitions. '
    'Use DROP TABLE login_attempts_YYYY_MM to enforce retention.';

COMMIT;

-- =============================================================================
-- DOWN (rollback: drop function, but keep the now-partitioned table intact
-- since reversing the data migration would lose post-mitigation rows)
-- =============================================================================
-- BEGIN;
-- DROP FUNCTION IF EXISTS fmcg_create_login_attempts_partition(INT, INT);
-- -- To fully reverse: drop all partitions + recreate non-partitioned table.
-- -- This is destructive; not provided here. Run a forward-fix migration instead.
-- COMMIT;