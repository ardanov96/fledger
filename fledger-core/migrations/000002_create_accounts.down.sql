-- Migration 000002 rollback
BEGIN;
DROP TRIGGER IF EXISTS accounts_set_updated_at ON accounts;
DROP TABLE IF EXISTS accounts;
-- Keep trigger function (used by other tables); drop manually if not needed
COMMIT;
