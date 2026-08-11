-- =============================================================================
-- Migration: 000004_create_entries
-- Description: Create ledger_entries table (immutable)
-- Author: FMCG Wallet
-- =============================================================================
-- The heart of the ledger. Each entry belongs to a transaction (group) and
-- an account. Entries are IMMUTABLE — DB triggers prevent UPDATE/DELETE.
-- Corrections must be done by inserting reversal entries.
-- =============================================================================

BEGIN;

CREATE TABLE ledger_entries (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID            NOT NULL REFERENCES transactions(id),
    account_id      UUID            NOT NULL REFERENCES accounts(id),
    amount          BIGINT          NOT NULL,                   -- always positive (minor units)
    type            TEXT            NOT NULL,                   -- 'debit' | 'credit'
    ref_type        TEXT,                                       -- e.g. 'invoice', 'payment'
    ref_id          UUID,
    period_id       UUID            NOT NULL REFERENCES accounting_periods(id),
    description     TEXT,
    currency        TEXT            NOT NULL DEFAULT 'IDR',
    metadata        JSONB           NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT ledger_entries_amount_positive CHECK (amount > 0),
    CONSTRAINT ledger_entries_type_check CHECK (type IN ('debit', 'credit')),
    CONSTRAINT ledger_entries_currency_check CHECK (length(currency) = 3)
);

-- ---------------------------------------------------------------------------
-- Immutability: entries cannot be UPDATEd or DELETEd
-- Reversals are done by inserting NEW entries (referencing the original via metadata)
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION prevent_entry_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries are immutable (id=%, txn=%, account=%)',
        OLD.id, OLD.transaction_id, OLD.account_id
        USING ERRCODE = 'check_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER no_entry_update
    BEFORE UPDATE ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_entry_modification();

CREATE TRIGGER no_entry_delete
    BEFORE DELETE ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION prevent_entry_modification();

-- ---------------------------------------------------------------------------
-- Indexes — optimized for common access patterns
-- ---------------------------------------------------------------------------
-- Account history (statement view)
CREATE INDEX ledger_entries_account_created_idx
    ON ledger_entries(account_id, created_at DESC);

-- Transaction drill-down
CREATE INDEX ledger_entries_transaction_idx
    ON ledger_entries(transaction_id);

-- Reference lookup (e.g. "find all entries for invoice X")
CREATE INDEX ledger_entries_ref_idx
    ON ledger_entries(ref_type, ref_id)
    WHERE ref_id IS NOT NULL;

-- Period-based queries (e.g. trial balance per period)
CREATE INDEX ledger_entries_period_idx
    ON ledger_entries(period_id);

-- Tenant scoping for multi-tenancy
CREATE INDEX ledger_entries_tenant_idx
    ON ledger_entries(period_id) INCLUDE (account_id, type, amount);

-- ---------------------------------------------------------------------------
-- Constraint: every account posting must be in the same currency
-- Enforced via trigger because it requires a JOIN
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION check_entry_currency_matches_account()
RETURNS TRIGGER AS $$
DECLARE
    account_currency TEXT;
BEGIN
    SELECT currency INTO account_currency
    FROM accounts
    WHERE id = NEW.account_id;

    IF account_currency IS NULL THEN
        RAISE EXCEPTION 'account % not found', NEW.account_id;
    END IF;

    IF account_currency != NEW.currency THEN
        RAISE EXCEPTION 'entry currency % does not match account currency %',
            NEW.currency, account_currency
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_entries_currency_check
    BEFORE INSERT ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION check_entry_currency_matches_account();

COMMIT;
