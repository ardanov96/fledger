-- Migration: 000030_reconciler_account_results_rls.up.sql
-- Description: Add RLS policies on reconciler_account_results table

DROP POLICY IF EXISTS reconciler_account_results_select ON reconciler_account_results;
DROP POLICY IF EXISTS reconciler_account_results_modify ON reconciler_account_results;

CREATE POLICY reconciler_account_results_select ON reconciler_account_results
    FOR SELECT USING (
        current_setting('app.current_tenant_id', true) IS NULL OR
        current_setting('app.current_tenant_id', true) = '' OR
        EXISTS (
            SELECT 1 FROM reconciler_runs r
            WHERE r.id = reconciler_account_results.run_id
              AND r.tenant_id = current_setting('app.current_tenant_id', true)::uuid
        )
    );

CREATE POLICY reconciler_account_results_modify ON reconciler_account_results
    FOR ALL USING (
        current_setting('app.current_tenant_id', true) IS NULL OR
        current_setting('app.current_tenant_id', true) = '' OR
        EXISTS (
            SELECT 1 FROM reconciler_runs r
            WHERE r.id = reconciler_account_results.run_id
              AND r.tenant_id = current_setting('app.current_tenant_id', true)::uuid
        )
    )
    WITH CHECK (
        current_setting('app.current_tenant_id', true) IS NULL OR
        current_setting('app.current_tenant_id', true) = '' OR
        EXISTS (
            SELECT 1 FROM reconciler_runs r
            WHERE r.id = reconciler_account_results.run_id
              AND r.tenant_id = current_setting('app.current_tenant_id', true)::uuid
        )
    );
