-- Rollback for migration 000006 — invoices + payments + credit_limits
BEGIN;

DROP VIEW IF EXISTS v_invoice_aging;
DROP TABLE IF EXISTS credit_limits;
DROP TRIGGER IF EXISTS invoice_payments_apply_allocation ON invoice_payments;
DROP FUNCTION IF EXISTS invoices_apply_allocation();
DROP TABLE IF EXISTS invoice_payments;
DROP TRIGGER IF EXISTS invoices_set_updated_at ON invoices;
DROP TABLE IF EXISTS invoices;

COMMIT;
