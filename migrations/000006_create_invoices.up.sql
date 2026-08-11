-- =============================================================================
-- Migration: 000006_create_invoices
-- Description: Create invoices, invoice_payments, credit_limits tables + aging view
-- Author: FMCG Wallet
-- =============================================================================
-- Implements Sprint 2 — Receivables & Credit Layer (MVP).
--
-- Tables:
--   * invoices         — outstanding receivables per customer
--   * invoice_payments — payment allocations (1 payment → many invoices)
--   * credit_limits    — per-customer credit cap
--
-- Views:
--   * v_invoice_aging  — aging buckets (current/7/30/60/90+ hari) untuk query aging report
--
-- Design notes:
--   * `paid_amount` maintained atomically dengan `invoice_payments` insert (1 trigger)
--   * `credit_limits.used_amount` maintained oleh application layer (CreateInvoice tx)
--   * Aging pakai SQL view, bukan materialized (Fase 4D nanti); cukup untuk MVP
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- invoices
-- ---------------------------------------------------------------------------
CREATE TABLE invoices (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    customer_id     UUID            NOT NULL REFERENCES accounts(id),
    code            TEXT            NOT NULL,            -- INV-2026-0001 format
    amount          BIGINT          NOT NULL,            -- minor units (sen)
    paid_amount     BIGINT          NOT NULL DEFAULT 0,
    due_date        DATE            NOT NULL,
    status          TEXT            NOT NULL DEFAULT 'open',
    issued_at       TIMESTAMPTZ     NOT NULL DEFAULT now(),
    period_id       UUID            NOT NULL REFERENCES accounting_periods(id),
    description     TEXT,
    metadata        JSONB           NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    CONSTRAINT invoices_status_check CHECK (
        status IN ('open', 'partial', 'paid', 'overdue', 'written_off')
    ),
    CONSTRAINT invoices_amount_positive CHECK (amount > 0),
    CONSTRAINT invoices_paid_nonneg CHECK (paid_amount >= 0),
    CONSTRAINT invoices_paid_lte_amount CHECK (paid_amount <= amount)
);

CREATE UNIQUE INDEX invoices_tenant_code_unique
    ON invoices(tenant_id, code);

-- Common query patterns
CREATE INDEX invoices_tenant_customer_due_idx
    ON invoices(tenant_id, customer_id, due_date);

CREATE INDEX invoices_tenant_status_idx
    ON invoices(tenant_id, status);

-- Partial index for outstanding invoices (aging query hot path)
CREATE INDEX invoices_open_due_idx
    ON invoices(tenant_id, due_date)
    WHERE status IN ('open', 'partial', 'overdue');

CREATE TRIGGER invoices_set_updated_at
    BEFORE UPDATE ON invoices
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();

-- ---------------------------------------------------------------------------
-- invoice_payments — many-to-many: 1 payment can cover N invoices
-- ---------------------------------------------------------------------------
CREATE TABLE invoice_payments (
    id              UUID            PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID            NOT NULL,
    payment_id      UUID            NOT NULL,            -- groups allocations from same payment event
    invoice_id      UUID            NOT NULL REFERENCES invoices(id),
    customer_id     UUID            NOT NULL REFERENCES accounts(id),
    amount          BIGINT          NOT NULL,            -- portion of payment allocated to this invoice
    method          TEXT            NOT NULL DEFAULT 'cash',  -- cash|transfer|qris|manual_adjust
    allocated_at    TIMESTAMPTZ     NOT NULL DEFAULT now(),
    metadata        JSONB           NOT NULL DEFAULT '{}',

    CONSTRAINT invoice_payments_amount_positive CHECK (amount > 0),
    CONSTRAINT invoice_payments_method_check CHECK (
        method IN ('cash', 'transfer', 'qris', 'manual_adjust')
    )
);

CREATE INDEX invoice_payments_payment_idx
    ON invoice_payments(payment_id);

CREATE INDEX invoice_payments_invoice_idx
    ON invoice_payments(invoice_id);

CREATE INDEX invoice_payments_tenant_customer_idx
    ON invoice_payments(tenant_id, customer_id, allocated_at DESC);

-- ---------------------------------------------------------------------------
-- Maintain invoices.paid_amount atomically with allocations
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION invoices_apply_allocation()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE invoices
    SET paid_amount = paid_amount + NEW.amount,
        status = CASE
            WHEN paid_amount + NEW.amount >= amount THEN 'paid'
            WHEN paid_amount + NEW.amount > 0       THEN 'partial'
            ELSE status
        END,
        updated_at = now()
    WHERE id = NEW.invoice_id;

    -- Reject over-payment via constraint check (defense-in-depth)
    IF (SELECT paid_amount FROM invoices WHERE id = NEW.invoice_id) >
       (SELECT amount FROM invoices WHERE id = NEW.invoice_id) THEN
        RAISE EXCEPTION 'invoice % overpaid (would exceed amount)', NEW.invoice_id
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER invoice_payments_apply_allocation
    AFTER INSERT ON invoice_payments
    FOR EACH ROW
    EXECUTE FUNCTION invoices_apply_allocation();

-- ---------------------------------------------------------------------------
-- credit_limits
-- ---------------------------------------------------------------------------
CREATE TABLE credit_limits (
    tenant_id       UUID            NOT NULL,
    customer_id     UUID            NOT NULL REFERENCES accounts(id),
    limit_amount    BIGINT          NOT NULL,            -- minor units
    used_amount     BIGINT          NOT NULL DEFAULT 0,
    effective_from  DATE            NOT NULL DEFAULT CURRENT_DATE,
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, customer_id),
    CONSTRAINT credit_limit_amount_nonneg CHECK (limit_amount >= 0),
    CONSTRAINT credit_used_nonneg CHECK (used_amount >= 0),
    CONSTRAINT credit_used_lte_limit CHECK (used_amount <= limit_amount)
);

CREATE INDEX credit_limits_customer_idx
    ON credit_limits(tenant_id, customer_id);

-- ---------------------------------------------------------------------------
-- Aging view — classify outstanding invoices into buckets
-- ---------------------------------------------------------------------------
CREATE OR REPLACE VIEW v_invoice_aging AS
SELECT
    tenant_id,
    customer_id,
    CASE
        WHEN due_date >= CURRENT_DATE                       THEN 'current'
        WHEN CURRENT_DATE - due_date BETWEEN 1 AND 7       THEN 'd_1_7'
        WHEN CURRENT_DATE - due_date BETWEEN 8 AND 30      THEN 'd_8_30'
        WHEN CURRENT_DATE - due_date BETWEEN 31 AND 60     THEN 'd_31_60'
        WHEN CURRENT_DATE - due_date BETWEEN 61 AND 90     THEN 'd_61_90'
        ELSE 'd_90_plus'
    END AS bucket,
    COUNT(*)                              AS invoice_count,
    (amount - paid_amount)                AS outstanding_minor
FROM invoices
WHERE status IN ('open', 'partial', 'overdue')
GROUP BY tenant_id, customer_id, bucket;

COMMENT ON VIEW v_invoice_aging IS
    'Aging buckets per customer. outstanding_minor = amount - paid_amount (sen).';

COMMIT;

-- =============================================================================
-- DOWN
-- =============================================================================
-- DROP VIEW IF EXISTS v_invoice_aging;
-- DROP TABLE IF EXISTS credit_limits;
-- DROP TRIGGER IF EXISTS invoice_payments_apply_allocation ON invoice_payments;
-- DROP FUNCTION IF EXISTS invoices_apply_allocation();
-- DROP TABLE IF EXISTS invoice_payments;
-- DROP TRIGGER IF EXISTS invoices_set_updated_at ON invoices;
-- DROP TABLE IF EXISTS invoices;
