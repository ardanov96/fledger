# ADR-0006: Tenant RLS + Field-Level Authz Strategy

- **Status:** Accepted (2026-08-14)
- **Sprint:** Sprint 15 (Fase 2B + 5A)
- **Deciders:** Architecture team
- **Supersedes:** —

## Context

Sistem multi-tenant untuk distributor FMCG Indonesia. Setiap tenant punya data terpisah (accounts, invoices, transfers, dll). Sebelum Sprint 15, isolasi data hanya dijaga di application layer (handler + use case manually filter by `tenant_id`). Risiko: bug di handler/use case = data leak antar-tenant. Plus, untuk `sales_rep` field-level authorization (sales_rep hanya boleh lihat route milik sendiri), tidak ada enforcement mekanis di DB.

Migration `000014_tenant_rls.up.sql` sudah mendefinisikan Postgres Row-Level Security policies — tapi belum pernah di-bind dari app layer (helper `tenantctx.SetTenantContext` ada tapi tidak dipanggil).

## Decision

**Aktifkan defense-in-depth dengan Postgres RLS + bind GUC variables di awal setiap transaction.**

### Strategy

1. **HTTP layer**: Extract JWT claims via existing `RequireAuth` middleware → push `Principal` ke request context.

2. **Tenant binding middleware**: New `TenantContextMiddleware` derives `*tenantctx.Info` (TenantID, UserID, IsSalesRep) dari Principal, simpan di context via SHARED key di package `internal/platform/tenantctx` (sehingga middleware HTTP dan usecase layer reference same key tanpa import cycle).

3. **Tx-layer injection**: Set Postgres session-level GUC variables di awal **setiap transaction** via 7 `RunInTx*Domain` adapter functions:

   ```sql
   SELECT set_config('app.current_tenant_id', $1, true);
   SELECT set_config('app.current_user_id', $1, true);
   SELECT set_config('app.is_sales_rep', $1, true);
   ```

   `is_local=true` flag → settings auto-revert pada COMMIT/ROLLBACK (no leakage antar tx).

4. **RLS policies** (sudah ada di migration 000014) baca GUC variables dan enforce row-level filtering:

   ```sql
   CREATE POLICY tenant_isolation_select ON accounts
       FOR SELECT USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
   ```

5. **Field-level authz untuk sales_rep** via additional policy on `collection_routes`:

   ```sql
   CREATE POLICY sales_rep_scope ON collection_routes
       FOR SELECT USING (
           current_setting('app.is_sales_rep', true) IS DISTINCT FROM 'true'
           OR sales_rep_id = current_setting('app.current_user_id')::uuid
       );
   ```

## Consequences

### Positif

- **Defense-in-depth**: Even kalau handler bug atau query ditulis langsung di DB tool, RLS tetap enforce tenant isolation.
- **Single source of truth**: Migration 000014 mendokumentasikan security model; code hanya consume.
- **Backward compatible**: Tests tanpa JWT tetap jalan — `tenantctx.SetTenantContext` no-op kalau `*Info` nil (graceful degradation, RLS akan reject di DB kalau middleware lupa set → fail loud).
- **Generic signature**: `SetTenantContext(ctx, tx, *Info)` menerima `tx any` (structurally compatible dengan semua 7 domain.Tx) — 1 helper, 7 tx_adapter, 19 closure sites tanpa duplikasi.
- **Centralized di tx_adapter**: Perubahan ke binding logic cukup edit 7 file tx_adapter, bukan 19 closure sites + 7 service methods.

### Negatif / Trade-offs

- **Extra round-trip per tx**: 3 `SELECT set_config(...)` calls per transaction. ~negligible latency (~1ms each on local Postgres).
- **Migration 000014 already FORCE RLS**: kalau lupa set GUC variable, query akan return error "invalid input syntax for type uuid" (NULL cast fail) — diagnostics harus recognize error pattern.
- **Public endpoints** (`/auth/*`, `/healthz`, `/readyz`) HARUS tidak enable RLS / skip GUC binding — auth schema tables tidak enabled RLS, jadi aman.
- **`user_credentials` table**: sengaja TIDAK enabled RLS (admin tools perlu akses bypass); Sprint follow-up bisa add dedicated `app_admin` role.

## Alternatives Considered

### A. Application-layer tenant filtering saja (current pre-Sprint-15)

- **Pro:** No migration needed; tests trivial.
- **Con:** Single point of failure — bug di satu use case = data leak. Tidak bisa audit dari DB mana query berasal.
- **Rejected:** Tidak cukup defensible untuk production-grade.

### B. Connection-per-tenant + manual GUC set per connection

- **Pro:** GUC lifetime = connection lifetime (bukan tx lifetime), fewer round-trips.
- **Con:** Connection pool management complicated; tidak multi-tenant di 1 connection; cold start cost.
- **Rejected:** Connection multiplexing jauh lebih penting untuk utilization di pool kecil.

### C. App-layer middleware check + DB trigger

- **Pro:** Defense at trigger level (existing pattern dari Sprint 8 trigger untuk period close).
- **Con:** Trigger-level filtering di `BEFORE SELECT` susah ekspres (cuma bisa RAISE EXCEPTION, bukan filter rows). Plus overhead per-row.
- **Rejected:** RLS native untuk row-level filtering, tidak reinvent.

### D. Schema-per-tenant

- **Pro:** Strongest isolation (different schema = different permissions grant).
- **Con:** Connection routing must route query to right schema; migrasi harus N schema; reporting cross-tenant must UNION ALL over schemas.
- **Rejected:** Operational complexity terlalu tinggi untuk use case yang ada.

## Implementation Reference

```
middleware (auth.go)
  └── RequireAuth(verifier)              -- extracts Principal from JWT
middleware (tenant_context.go)
  └── TenantContextMiddleware()          -- derives *tenantctx.Info
middleware (req response chain)
  └── r.Use(middleware.TenantContextMiddleware())

postgres adapter layer
  └── RunInTxDomain(ctx, fn)             -- injects GUC at tx start
  └── RunInTxInvoiceDomain(ctx, fn)      -- ditto
  └── RunInTxPeriodDomain(ctx, fn)       -- ditto
  └── RunInTxReconcilerDomain(ctx, fn)   -- ditto
  └── RunInTxCollectionDomain(ctx, fn)   -- ditto
  └── RunInTxCurrencyDomain(ctx, fn)     -- ditto
  └── RunInTxAuthDomain(ctx, fn)         -- ditto (auth: nil info ok, no-op)

migrations/000014_tenant_rls
  ├── ENABLE ROW LEVEL SECURITY on 12 tables
  ├── CREATE POLICY tenant_isolation_select (all tenant tables)
  ├── CREATE POLICY tenant_isolation_modify (all tenant tables)
  └── CREATE POLICY sales_rep_scope (collection_routes only)
```

## Operational Notes

### Adding a new tenant-scoped table

1. Add `tenant_id UUID` column in new migration.
2. Add table to `migration/000014` RLS section (or new migration if Sprint 15 already applied).
3. Done — use case tx automatically binds GUC via tx_adapter.

### Adding a new sales_rep-scoped table

1. Add `sales_rep_id UUID` column (nullable; non-rep rows have NULL).
2. Add RLS policy like `collection_routes` has — allow admins (`app.is_sales_rep != 'true'`) OR filter by `sales_rep_id = current_user_id`.
3. Done.

### Production deployment

1. Apply migration 000014 FIRST (RLS active).
2. THEN deploy app with sprint 15 changes (binds GUC).
3. **Do NOT deploy app without 000014 applied** — would result in missing GUC and `NULL::uuid` errors for all queries.
4. **Reverse order also invalid**: if migration applies first, all queries fail until app ships (CI/CD must coordinate).

## Verification

- `go build ./...` — zero compile errors
- Unit tests: `tenant_context_test.go` covers middleware, `infoFromPrincipal` (8 cases)
- Integration test (Sprint 17 follow-up): 2 tenants, verify tenant A query returns 0 rows from tenant B tables
- Manual smoke: apply 000014 + run app + insert test rows for 2 tenants + verify isolation

## Related

- ADR-0001 (Go as backend) — Go context package enables our middleware chain
- ADR-0004 (Locking) — defends against race conditions; RLS defends against data leaks
- Migration 000014_tenant_rls — RLS policies
- Sprint 8 pattern — DB triggers for period close enforce (defense-in-depth precedent)

## Open Follow-ups

1. **`app_admin` role + login flow** — bypass RLS via dedicated Postgres role for admin tooling.
2. **Defense-in-depth audit trail** — log every tenant-context bind event ke audit_logs table untuk forensics.
3. **Integration test** (Sprint 17) — actual E2E with 2 tenants dan tenant-isolation assertions.
