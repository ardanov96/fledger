-- Migration 000014 (down): Tenant RLS — disable policies and row-level security.
BEGIN;

DO $$
DECLARE t TEXT;
BEGIN
  FOR t IN SELECT unnest(ARRAY[
    'accounts','transactions','ledger_entries','invoices','invoice_payments',
    'credit_limits','accounting_periods','period_close_requests','period_snapshots',
    'reconciler_runs','reconciler_account_results','collection_routes','refresh_tokens'
  ])
  LOOP
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_select ON %I', t);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation_modify ON %I', t);
    EXECUTE format('DROP POLICY IF EXISTS sales_rep_scope ON %I', t);
    EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', t);
  END LOOP;
END $$;

DROP INDEX IF EXISTS accounts_tenant_idx;
DROP INDEX IF EXISTS invoices_tenant_idx;
DROP INDEX IF EXISTS collection_routes_tenant_idx;
DROP INDEX IF EXISTS refresh_tokens_tenant_idx;
DROP INDEX IF EXISTS transactions_tenant_idx;

COMMIT;
