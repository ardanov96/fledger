-- Migration 000028 (down): reverse fraud_flag dedup constraint.
BEGIN;

ALTER TABLE fraud_flags DROP CONSTRAINT IF EXISTS fraud_flags_transfer_rule_unique;
DROP INDEX IF EXISTS fraud_flags_transfer_rule_unique_idx;

COMMIT;