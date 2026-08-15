ADR-0007: `app_admin` Postgres Role for RLS Bypass

- **Status:** Accepted (2026-08-15)
- **Sprint:** Sprint 15 follow-up / Sprint 22A.4 documentation
- **Deciders:** Architecture team
- **Related:** [ADR-0006](0006-tenant-rls-strategy.md) — Tenant RLS strategy, [migrations/000015](../migrations/000015_app_admin_role.up.sql)

## Context and Problem Statement

Migration `000014` enables RLS on all 11 tenant-scoped tables, with policies
that constrain queries to `tenant_id = current_setting('app.current_tenant_id')::uuid`.
Postgres superusers bypass RLS by default — including the default `postgres`
role used by the migrator.

This creates an operational dilemma:
- Regular app runs as `fmcg` role — RLS-enforced (defense-in-depth, ✓).
- **But** legitimate cross-tenant admin operations (e.g. cross-tenant
  reporting, manual data fixes, debugging RLS policy bugs) currently
  require either:
  1. Disabling RLS temporarily (security risk: any code path can read all
     tenants during the window), OR
  2. Running as `postgres` superuser (audit-trail missing, credentials
     not Vault-managed, breaks the principle of least privilege), OR
  3. Re-implementing every admin query in SQL with explicit
     `WHERE tenant_id IN (...)` clauses (re-implements RLS by hand,
     error-prone).

We want a **dedicated, audit-friendly Postgres role** that bypasses RLS
*only inside an explicit transaction*, without weakening security for the
regular `fmcg` role.

## Decision Drivers

- **Least privilege:** admin role should not be the default role used by
  the app, migrator, or scheduled jobs.
- **Audit-friendly:** cross-tenant operations should be tagged in logs
  as `app_admin`, not blurred with `postgres` (where everything looks
  the same).
- **Reversible:** deployment should be incremental — if `app_admin`
  turns out unnecessary, drop the role and policies atomically.
- **No application-layer bypass:** RLS is the source of truth; no
  application code can disable it on a per-query basis (that would
  re-introduce single-point-of-failure).

## Considered Options

### Option A — Single superuser (rejected)
Keep using `postgres` superuser for admin operations.
- ✓ Trivial setup.
- ✗ No audit trail: a `postgres` login looks the same as a misconfiguration.
- ✗ Cannot be scoped: any process with `postgres` creds has full DB access
  including schema changes.
- ✗ Defeats defense-in-depth principle.

### Option B — Connection-per-tenant (rejected)
Each admin operation opens a dedicated connection and switches roles.
- ✓ Isolation at the connection layer.
- ✗ Connection-pool exhaustion under load (each cross-tenant report
  blocks 1 connection).
- ✗ Migrations require cross-tenant snapshots → don't fit the model.
- ✗ Doesn't compose with SET LOCAL ROLE pattern.

### Option C — `SET LOCAL ROLE app_admin` per transaction (✅ CHOSEN)
Create a `NOINHERIT` role `app_admin` that bypasses RLS for 11 tenant-scoped
tables. The `fmcg` role is granted membership and uses `SET LOCAL ROLE`
to elevate inside an explicit tx. After `COMMIT`/`ROLLBACK` the elevation
auto-reverts.
- ✓ Privileged elevation is **tx-scoped** — no leak across connections
  (verified by integration test: a session that elevates mid-tx cannot
  be observed from a sibling connection).
- ✓ Audit trail: log lines that include `SET LOCAL ROLE app_admin` are
  greppable in app log.
- ✓ Defense-in-depth still holds: regular `fmcg` queries without
  elevation remain RLS-scoped.
- ✓ Composable with existing pattern (`RunInTx*Domain` adapter
  closures).
- ✗ Requires explicit elevation code path (added only where needed;
  default is `fmcg`).
- ✗ Adds 1 Postgres role + 11 policies to manage.

### Option D — Schema-per-tenant (rejected for this scope)
Each tenant gets its own schema; admin tools span all schemas.
- ✓ Hard isolation at schema layer.
- ✗ Too disruptive for current scale (would require migrating all 11
  tables to per-tenant schemas + rewriting every query).
- ✗ Migrations and join-across-tenants operations become painful.
- ✗ Logically superset of option C; deferred for >10-tenant scale.

## Decision Outcome

**Chosen option: C — `app_admin` role via `SET LOCAL ROLE`.**

### Implementation

Migration `000015_app_admin_role.up.sql` (already merged):

```sql
-- 1. Create role
CREATE ROLE app_admin NOINHERIT;

-- 2. Grant fmcg the ability to switch
GRANT app_admin TO fmcg;

-- 3. Per-table admin bypass policy
CREATE POLICY <table>_admin_bypass ON <table>
    TO app_admin
    USING (true) WITH CHECK (true);
```

Application usage (admin tooling that needs cross-tenant visibility):

```go
err := db.RunInTxDomain(ctx, func(tx pgx.Tx) error {
    // elevate to app_admin inside this tx only
    if _, err := tx.Exec(ctx, "SET LOCAL ROLE app_admin"); err != nil {
        return err
    }
    // cross-tenant query — policies with `TO app_admin USING (true)`
    // permit seeing every row
    rows := tx.Query(ctx, "SELECT tenant_id, count(*) FROM accounts GROUP BY 1")
    ...
    // COMMIT/ROLLBACK auto-reverts role
})
```

### Operational characteristics

- `app_admin` is `NOINHERIT` — a fresh connection does NOT inherit it,
  so misconfigured deployment cannot accidentally elevate.
- `SET LOCAL ROLE` is **tx-scoped** — role elevation is auto-reverted
  on `COMMIT`/`ROLLBACK`. No risk of cross-tx leak.
- Default app code path uses `fmcg` role only — zero change to existing
  query execution.
- Audit: every elevated query is identifiable in `pg_stat_activity` /
  `pgaudit` logs as the `app_admin` role.

### Verification (runbook excerpt)

```sql
-- After migration apply, verify:
SELECT rolname FROM pg_roles WHERE rolname = 'app_admin';
-- Expected: 1 row

-- Verify fmcg can elevate:
SET ROLE app_admin;
SELECT current_user;   -- Expected: app_admin
RESET ROLE;

-- Verify admin sees all tenants:
SET ROLE app_admin;
SELECT tenant_id, COUNT(*) FROM accounts GROUP BY tenant_id;
-- Expected: counts across all tenants (RLS bypassed)

-- And back to fmcg (RLS enforced again):
RESET ROLE;
SET app.current_tenant_id = '<tenant-a>';
SELECT * FROM accounts;
-- Expected: only tenant A's accounts
```

## Consequences

### Positive

- Admin operations gain a **principled, audited** privilege model — no
  more "borrow the postgres password" anti-pattern.
- RLS continues to be the source of truth for regular app traffic —
  defense-in-depth unaffected.
- Role elevation is tx-scoped; cannot leak across connections or after
  COMMIT.
- Per-table policies (`<table>_admin_bypass`) are additive — they
  re-state their `USING (true)` for each RLS-enabled table, ensuring
  the bypass is explicit (an accidental DROP of one policy doesn't
  accidentally grant admin to other tables).

### Negative / Trade-offs

- 11 new policies (1 per RLS-enabled table) to maintain alongside
  their `tenant_isolation_*` counterparts. Drift over time is possible
  — mitigated by `RunInTx*Domain` integration test (`TestIntegration_RLSIsolation`)
  which asserts both behaviors on every CI run.
- Deployment order: migration 000015 must apply AFTER migration 000014
  (so RLS is already enabled and policies exist before admin bypass
  policies can target `TO app_admin`).
- `app_admin` has no per-admin audit table (who switched role at what
  time) — open follow-up, see below.

### Operational notes

- Run `migrations/000015_app_admin_role.up.sql` during the next deploy
  window — it's idempotent (uses `DROP POLICY IF EXISTS` and
  conditional grants).
- If an admin tool needs elevation, **always** wrap it in a tx so the
  elevation is reversible. Pattern: see implementation above.

## Open Follow-ups

1. **Audit table for role switches** — record every `SET LOCAL ROLE
   app_admin` invocation (who/when/why) in `admin_role_switch_audit`
   for forensics. Listed as Sprint 22C follow-up.
2. **Per-admin user mapping** — currently `app_admin` is a single
   role. If admin staff grows beyond 2-3 persons, consider
   `app_admin_<user>` roles per admin so `pg_stat_activity` can
   attribute activity to individuals.
3. **Vault-managed credentials** — `app_admin` does not have a
   password (uses `SET ROLE` elevation); if password is added later,
   store in Vault per the secret-rotation runbook.
4. **Drift test in CI** — already covered by
   `TestIntegration_RLSIsolation` but a dedicated
   `TestAdminRoleBypassEnforced` would assert that without explicit
   `SET ROLE`, even same-tenant queries remain RLS-scoped (guard against
   accidental GRANTs that make `app_admin` the default).
