-- Migration 000004 rollback
BEGIN;
DROP TRIGGER IF EXISTS ledger_entries_currency_check ON ledger_entries;
DROP TRIGGER IF EXISTS no_entry_delete ON ledger_entries;
DROP TRIGGER IF EXISTS no_entry_update ON ledger_entries;
DROP TABLE IF EXISTS ledger_entries;
-- Functions (prevent_entry_modification, check_entry_currency_matches_account) intentionally
-- left in place; they will be re-used if the table is recreated in the future.
COMMIT;
