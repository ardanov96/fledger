-- Migration: 000030_reconciler_account_results_rls.down.sql
-- Description: Drop RLS policies on reconciler_account_results table

DROP POLICY IF EXISTS reconciler_account_results_select ON reconciler_account_results;
DROP POLICY IF EXISTS reconciler_account_results_modify ON reconciler_account_results;
