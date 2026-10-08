-- =============================================================================
-- Migration: 000028_fraud_flag_dedup
-- Description: UNIQUE constraint on fraud_flags (transfer_id, rule_name) (Sprint 42)
-- Author: FMCG Wallet
-- =============================================================================
-- Sprint 31 fraud scanner inserts a flag row per (transfer_id, rule_name).
-- When NATS redelivers a transfer.posted event (common in worker crashes),
-- the scanner fires the rules again and INSERTs duplicate rows.
--
-- Pre-Sprint 42: cascade. Existing rows have unique IDs so the INSERT
-- succeeds, creating noise in the fraud dashboard.
--
-- Sprint 42 fix: UNIQUE constraint on (transfer_id, rule_name) + ON CONFLICT
-- DO NOTHING in the repo. Same NATS event fires the same rule twice →
-- second insert is silently ignored, no duplicate flag.
--
-- Idempotent: safe to re-run (drops and re-adds the constraint).
--
-- Migration plan:
--   - Add UNIQUE constraint via ADD CONSTRAINT ... USING INDEX (Postgres 9.5+)
--   - Use a btree index on (transfer_id, rule_name)
-- =============================================================================

BEGIN;

-- Step 1: Create the unique index.
-- Using a separate CREATE UNIQUE INDEX then ADD CONSTRAINT so the
-- operation is online (doesn't block reads during creation).
CREATE UNIQUE INDEX IF NOT EXISTS fraud_flags_transfer_rule_unique_idx
    ON fraud_flags (transfer_id, rule_name);

-- Step 2: Add the constraint backed by the index.
-- (Alternatively could use ALTER TABLE ... ADD CONSTRAINT directly
-- with the inline unique; we use the index-backed form for online DDL.)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fraud_flags_transfer_rule_unique'
    ) THEN
        ALTER TABLE fraud_flags
            ADD CONSTRAINT fraud_flags_transfer_rule_unique
            UNIQUE USING INDEX fraud_flags_transfer_rule_unique_idx;
    END IF;
END $$;

COMMENT ON CONSTRAINT fraud_flags_transfer_rule_unique ON fraud_flags IS
    'Sprint 42: prevents duplicate flag rows from NATS redelivery. '
    'Combined with ON CONFLICT DO NOTHING in the repo, the same event firing '
    'the same rule twice produces exactly one flag row.';

COMMIT;

-- =============================================================================
-- DOWN (rollback)
-- =============================================================================
-- BEGIN;
-- ALTER TABLE fraud_flags DROP CONSTRAINT IF EXISTS fraud_flags_transfer_rule_unique;
-- DROP INDEX IF EXISTS fraud_flags_transfer_rule_unique_idx;
-- COMMIT;