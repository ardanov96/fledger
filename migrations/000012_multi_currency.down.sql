-- Migration 000012 (down): Multi-currency — rollback
BEGIN;

-- Drop trigger first
DROP TRIGGER IF EXISTS trg_enforce_fx_rate_snapshot ON transactions;
DROP FUNCTION IF EXISTS enforce_fx_rate_snapshot();

-- Drop indexes
DROP INDEX IF EXISTS transactions_fx_rate_idx;
DROP INDEX IF EXISTS fx_rates_active_idx;
DROP INDEX IF EXISTS fx_rates_tenant_idx;
DROP INDEX IF EXISTS fx_rates_lookup_idx;

-- Drop columns on transactions
ALTER TABLE transactions
    DROP COLUMN IF EXISTS fx_rate_locked_at,
    DROP COLUMN IF EXISTS fx_rate_id;

-- Drop tables
DROP TABLE IF EXISTS fx_rates;
DROP TABLE IF EXISTS currencies;

COMMIT;
