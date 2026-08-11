-- Migration 000003 rollback
BEGIN;
DROP TRIGGER IF EXISTS transactions_set_updated_at ON transactions;
DROP TABLE IF EXISTS transactions;
DROP TRIGGER IF EXISTS accounting_periods_set_updated_at ON accounting_periods;
DROP TABLE IF EXISTS accounting_periods;
COMMIT;
