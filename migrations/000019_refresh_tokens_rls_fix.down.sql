BEGIN;

DROP POLICY IF EXISTS refresh_tokens_select_auth ON refresh_tokens;
DROP POLICY IF EXISTS refresh_tokens_modify_auth ON refresh_tokens;

CREATE POLICY tenant_isolation_select ON refresh_tokens
    FOR SELECT USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

CREATE POLICY tenant_isolation_modify ON refresh_tokens
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant_id', true)::uuid);

COMMIT;
