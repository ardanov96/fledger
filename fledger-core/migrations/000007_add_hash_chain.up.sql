-- =============================================================================
-- Migration: 000007_add_hash_chain
-- Description: Add prev_hash + entry_hash columns to ledger_entries for
--              tamper detection (Fase 1C — Hash Chain).
-- =============================================================================

BEGIN;

ALTER TABLE ledger_entries
    ADD COLUMN IF NOT EXISTS prev_hash  TEXT NOT NULL DEFAULT '0000000000000000000000000000000000000000000000000000000000000000',
    ADD COLUMN IF NOT EXISTS entry_hash TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS ledger_entries_account_created_idx
    ON ledger_entries(account_id, created_at DESC, id);

COMMIT;
