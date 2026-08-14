-- Migration 000012: Multi-currency (Fase 1D)
-- Sprint 12 — Multi-Currency support dengan rate snapshot per transaction
--
-- Adds:
--   1. Table `currencies` — ISO 4217 currency registry
--   2. Table `fx_rates` — FX rate history with TTL-based validity
--   3. Columns `fx_rate_id` + `fx_rate_locked_at` on `transactions`
--   4. Trigger enforcing rate snapshot for cross-currency transfers
--   5. Seed: IDR default + sample USD/IDR rate

BEGIN;

-- ============================================================================
-- Table: currencies
-- ============================================================================
CREATE TABLE IF NOT EXISTS currencies (
    code           TEXT PRIMARY KEY,
    decimal_places INT  NOT NULL CHECK (decimal_places BETWEEN 0 AND 6),
    name           TEXT NOT NULL,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default currency
INSERT INTO currencies (code, decimal_places, name, is_active)
VALUES ('IDR', 2, 'Indonesian Rupiah', TRUE)
ON CONFLICT (code) DO NOTHING;

-- ============================================================================
-- Table: fx_rates
-- ============================================================================
CREATE TABLE IF NOT EXISTS fx_rates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    from_currency   TEXT NOT NULL REFERENCES currencies(code),
    to_currency     TEXT NOT NULL REFERENCES currencies(code),
    rate            NUMERIC(20,10) NOT NULL CHECK (rate > 0),
    effective_at    TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    source          TEXT NOT NULL DEFAULT 'manual',
    created_by      UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fx_rates_different_currencies CHECK (from_currency <> to_currency),
    CONSTRAINT fx_rates_valid_window         CHECK (expires_at > effective_at),
    CONSTRAINT fx_rates_source_valid         CHECK (source IN ('manual', 'api', 'bank', 'seed'))
);

-- Lookup index: rate history for a currency pair, most recent first
CREATE INDEX IF NOT EXISTS fx_rates_lookup_idx
    ON fx_rates (from_currency, to_currency, effective_at DESC);

-- Tenant-wide recent rates
CREATE INDEX IF NOT EXISTS fx_rates_tenant_idx
    ON fx_rates (tenant_id, effective_at DESC);

-- Active rates (used by transfer-time lookup)
CREATE INDEX IF NOT EXISTS fx_rates_active_idx
    ON fx_rates (from_currency, to_currency, effective_at DESC, expires_at)
    WHERE expires_at > NOW();

-- ============================================================================
-- Columns on transactions: fx_rate snapshot for cross-currency transfers
-- ============================================================================
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS fx_rate_id         UUID REFERENCES fx_rates(id),
    ADD COLUMN IF NOT EXISTS fx_rate_locked_at  TIMESTAMPTZ;

-- Index for cross-currency transfer queries
CREATE INDEX IF NOT EXISTS transactions_fx_rate_idx
    ON transactions (fx_rate_id)
    WHERE fx_rate_id IS NOT NULL;

-- ============================================================================
-- Trigger: enforce rate snapshot for cross-currency transfers
-- ============================================================================
CREATE OR REPLACE FUNCTION enforce_fx_rate_snapshot()
RETURNS TRIGGER AS $$
DECLARE
    v_from_currency TEXT;
    v_to_currency   TEXT;
BEGIN
    -- If both currencies set on the transaction, ensure consistency
    IF NEW.from_currency IS NOT NULL AND NEW.to_currency IS NOT NULL THEN
        v_from_currency := NEW.from_currency;
        v_to_currency   := NEW.to_currency;

        -- Cross-currency: require fx_rate_id + fx_rate_locked_at
        IF v_from_currency <> v_to_currency THEN
            IF NEW.fx_rate_id IS NULL THEN
                RAISE EXCEPTION 'cross-currency transfer requires fx_rate_id (from=%, to=%)',
                    v_from_currency, v_to_currency
                    USING ERRCODE = '23514';
            END IF;
            IF NEW.fx_rate_locked_at IS NULL THEN
                RAISE EXCEPTION 'cross-currency transfer requires fx_rate_locked_at (from=%, to=%)',
                    v_from_currency, v_to_currency
                    USING ERRCODE = '23514';
            END IF;

            -- Validate fx_rate row matches the from/to currencies
            IF NOT EXISTS (
                SELECT 1 FROM fx_rates
                WHERE id = NEW.fx_rate_id
                  AND from_currency = v_from_currency
                  AND to_currency   = v_to_currency
            ) THEN
                RAISE EXCEPTION 'fx_rate % does not match currency pair (%, %)',
                    NEW.fx_rate_id, v_from_currency, v_to_currency
                    USING ERRCODE = '23514';
            END IF;
        ELSE
            -- Same currency: ensure no leftover rate snapshot
            IF NEW.fx_rate_id IS NOT NULL OR NEW.fx_rate_locked_at IS NOT NULL THEN
                RAISE EXCEPTION 'same-currency transfer must not have fx_rate_id or fx_rate_locked_at'
                    USING ERRCODE = '23514';
            END IF;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enforce_fx_rate_snapshot ON transactions;
CREATE TRIGGER trg_enforce_fx_rate_snapshot
    BEFORE INSERT OR UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION enforce_fx_rate_snapshot();

COMMIT;
