ADR-0008: Sprint 22B Hardening Items — Design Decisions & Roadmap

- **Status:** Accepted (2026-08-15)
- **Sprint:** Sprint 22B (Tier-1 hardening items deferred from Sprint 13-15)
- **Deciders:** Architecture team
- **Related:** [ADR-0006](0006-tenant-rls-strategy.md) (GUC bind audit, follow-up #2),
  [ADR-0007](0007-app-admin-rls-bypass.md) (RLS bypass)

## Context and Problem Statement

After Sprint 15 close, audit gap analysis identified **5 Tier-1 hardening items**
that were deferred from earlier sprints due to effort budget. They are
collecting as technical debt:

| Item | Sprint | Originally planned | Status |
|---|---|---|---|
| 22B.1 Wire rate-limit metrics | 22B | "30 minutes" per Sprint 14 follow-up | ✅ DONE in `2b22e50` |
| 22B.1c main.go:376 wiring | 22B | 5-baris edit | ✅ DONE in `aaec962` |
| 22B.5 GUC bind audit trail | 22B | "2-3 hours" | ⏸ Deferred (this ADR) |
| 22B.2 MFA recovery codes | 13 | "1-2 days" | ⏸ Deferred (this ADR) |
| 22B.3 Session management list/revoke | 13 | "1 day" | ⏸ Deferred (this ADR) |
| 22B.4 Password policy validator | 13 | "3-4 hours" | ⏸ Deferred (this ADR) |

ADR-0006 explicitly listed `Audit trail untuk set_config GUC bind events` as
**ADR-0006 Open Follow-up #2**. This ADR upgrades that item into a concrete
design and groups the remaining 3 items into a coordinated roadmap so
they can be batched.

## Decision Drivers

- **Defense-in-depth observability** — every privileged operation should be
  auditable. Right now we have audit_logs (Sprint 5) for HTTP requests, but
  NOT for DB-level role elevations or RLS bypasses (Sprint 22D7). An operator
  investigating "who saw cross-tenant data at 14:32 UTC?" has no row to query.
- **MFA UX gap** — Sprint 13 delivered TOTP MFA + brute-force protection.
  Without recovery codes (one-time backup), a user who loses their phone
  is locked out permanently. This is operational defect waiting to happen.
- **Session visibility** — Refresh-token rotation delivers reuse-detection
  (Sprint 13). But users have no way to see "which devices am I logged in
  on" or terminate a specific session. Forced global logout only.
- **Password policy** — currently the only validation is the bcrypt
  accept-any-password. No length, complexity, or haveibeenpwned check.
  Brute-force protection (5 attempts / 15 min) mitigates attack but does
  not enforce hygiene.
- **Backwards compat** — none of these items break existing clients.
  All are additive on existing endpoints/tables.
- **Tests must catch regressions** — each item ships with 4-8 unit tests
  and at least 1 integration scenario.

## Considered Options

### Option A — Inline each item as a Sprint 22 sub-sprint (rejected)
Split into 4 mini-sprints (one per item), each shipped separately.
- ✓ Clear scoping, easy PR review.
- ✗ 4x overhead: migrations, ADRs, integration tests per item.
- ✗ High coordination cost (cross-PRs must all merge cleanly).

### Option B — Single mega-sprint 23 "MFA + Sessions + Audit + Policy" (✅ CHOSEN)
Bundle all 4 items into one Sprint 23 with one migration, one ADR (this
one), one integration scenario file.
- ✓ Single migration: `000018_sprint23_hardening.up.sql` lands 4 tables.
- ✓ Single integration test file: `internal/usecase/sprint23_test.go`.
- ✓ Single PR per area (auth, middleware, usecase, handler) — review-friendly.
- ✓ All other code change tightly localized.
- ✗ Larger PR per area (but area-bounded, not mega-monolithic).

### Option C — Defer to "someday" indefinitely (rejected)
Accept that these are operational gaps and move on.
- ✗ Backlog drift — already 3 sprints behind.
- ✗ Risk: if MFA UX lockout hits a real user, recovery needs to be
  manual (operator intervention) → expensive support burden.

## Decision Outcome

**Chosen option: B — single coordinated Sprint 23.**

### 22B.5 GUC bind audit trail

**Design:**

```sql
-- 000018_sprint23_hardening.up.sql

CREATE TABLE IF NOT EXISTS guc_bind_audit (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   UUID NOT NULL,
    user_id     UUID NOT NULL,
    is_sales_rep BOOLEAN NOT NULL,
    request_id  TEXT,        -- echo from HTTP request_id (W3C correlation)
    bound_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS guc_bind_audit_tenant_time_idx
    ON guc_bind_audit (tenant_id, bound_at DESC);
CREATE INDEX IF NOT EXISTS guc_bind_audit_user_time_idx
    ON guc_bind_audit (user_id, bound_at DESC);

-- RLS: app_admin can see all, regular fmcg blocked by tenant policy
ALTER TABLE guc_bind_audit ENABLE ROW LEVEL SECURITY;
ALTER TABLE guc_bind_audit FORCE ROW LEVEL SECURITY;
CREATE POLICY guc_bind_audit_admin_bypass ON guc_bind_audit TO app_admin USING (true) WITH CHECK (true);
CREATE POLICY guc_bind_audit_tenant_select ON guc_bind_audit
    FOR SELECT USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);
```

**Code change:**

```go
// tenantctx.SetTenantContext — inject INSERT after set_config
func SetTenantContext(ctx context.Context, tx any, info *Info) error {
    // ... existing 3 set_config calls ...
    _, err := exec.Exec(ctx,
        `INSERT INTO guc_bind_audit (tenant_id, user_id, is_sales_rep, request_id)
         VALUES ($1, $2, $3, $4)`,
        info.TenantID, info.UserID, info.IsSalesRep, getRequestID(ctx))
    return err
}
```

**Use case / handler:**

```go
// GET /v1/audit/guc-binds?tenant_id=<uuid>&since=<ts>&limit=100
// RBAC: hq_admin OR hq_finance (read-only audit access)
// Returns: paginated list of bind events for forensic investigation
```

**Effort estimate:** ~3 hours.

### 22B.2 MFA recovery codes

**Design:** One-time backup codes (10 codes, generated at MFA setup),
each ~10 hex chars (e.g. `9c4a-f7b2-83e1`). Stored hashed (SHA-256) same
as refresh tokens. `used_at` column for single-use enforcement.

```sql
CREATE TABLE mfa_recovery_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    code_hash TEXT NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX mfa_recovery_codes_hash_uidx ON mfa_recovery_codes (user_id, code_hash);
```

**Flow:**
1. `POST /v1/auth/mfa/setup` (existing, extended) — returns 10 plaintext codes
2. User saves them (printed/downloaded). Server stores only hashes.
3. `POST /v1/auth/mfa/recovery/verify { code }` — replaces TOTP for that
   session. Code marked `used_at = NOW()`.
4. After all 10 codes used → user must contact admin.

**Effort estimate:** ~1-2 days (migration + domain + use case + 2 handlers + 6 unit tests).

### 22B.3 Session management list/revoke

**Design:** Reuse existing `refresh_tokens` table as the source of truth.
Add 2 endpoints under `/v1/auth/sessions`:

```go
// List — find all active refresh tokens for current user
type SessionListItem struct {
    ID         uuid.UUID  // refresh_token.id (UUID)
    UserAgent  string     // parse from user-agent header on creation
    IP         netip.Addr // client IP
    CreatedAt  time.Time
    LastUsedAt time.Time  // nullable, updated on refresh
    Status     string     // 'active' / 'rotated' / 'revoked'
}

// Delete — revoke specific session
// Owner check: refresh_token.user_id == principal.user_id (else 403)
```

**Endpoint shape:**
- `GET /v1/auth/sessions` → `200 [{...}]` (active + recent rotated)
- `DELETE /v1/auth/sessions/{id}` → `204` (or `403` if not owner)

**Effort estimate:** ~1 day (no new table, just handlers + RBAC + tests).

### 22B.4 Password policy validator

**Design:** Handler-layer + service-layer dual validation.

```go
type PasswordPolicy struct {
    MinLength      int  // default 12
    RequireDigit   bool // default true
    RequireUpper   bool // default true
    RequireSymbol  bool // default true (or require_symbol OR require_special)
    MaxLength      int  // default 128 (bcrypt limit is 72 bytes)
}

// In auth_service.Login — validate BEFORE bcrypt verify
// If password fails policy → reject with 422 ErrPasswordPolicyFail
// On successful login, if password_hash exists BUT user never had policy
// enforced (legacy password), trigger SetPassword flow (force update).
```

**Backwards compat:** Existing users keep their old password. New signups
(login without MFA + password) require policy. Migration step adds a
`password_policy_enforced_at` column; if NULL, login still works but UI
prompts for update on next access.

**Effort estimate:** ~3-4 hours (config struct + validator func + handler messages + 4 tests).

## Combined Effort Estimate (all 4 items Sprint 23)

| Item | Effort | Files | LOC estimate |
|---|---|---|---|
| 22B.5 GUC bind audit | 3 hours | 3 (migration, tenantctx.go, audit handler) | +200 |
| 22B.2 MFA recovery codes | 1.5 days | 5 (migration, domain, use case, 2 handlers, tests) | +400 |
| 22B.3 Session list/revoke | 1 day | 3 (handlers in auth.go, repo query, RBAC) | +200 |
| 22B.4 Password policy | 4 hours | 4 (policy struct, validator, auth service, login handler) | +150 |
| **Total** | **4-5 days** | **15 new/modified files** | **~+950** |

## Consequences

### Positive

- All 4 operational gaps closed in **single coordinated sprint** with
  shared migration, shared integration test, shared PR review.
- Each item still ships in its own commit (sub-PRs per item) → atomic.
- ADR-0006 follow-up #2 closed (GUC bind audit).
- MFA recovery closes the "user locked out" UX risk before any real
  demo disaster.
- Session management gives operators debug visibility into auth state
  without relying on auth_logs alone.
- Password policy enforces baseline hygiene that bcrypt alone doesn't.

### Negative / Trade-offs

- Single mega-PR by feature area has higher review surface than
  minimal-scope PRs (mitigation: each area split into sub-commits).
- 22B.2 (MFA recovery) is the largest single item — testing 6 unique
  scenarios requires same density as the rest.
- Backwards-compat for legacy passwords requires careful rollout:
  - Day 0: migration adds column
  - Day 1-N: legacy passwords still login (grace period)
  - Day N+30: force `password_policy_enforced_at = NOW()` for all
    legacy users, require reset on next login

### Operational notes

- All 4 items are deployable independently. Recommend order:
  1. **22B.5** (3h) — smallest scope, lowest risk, ops visibility.
  2. **22B.4** (4h) — closes hygiene gap without breaking UX.
  3. **22B.3** (1d) — uses existing tables, additive.
  4. **22B.2** (1.5d) — biggest, can be rolled back by dropping
     `mfa_recovery_codes` table without data loss in other areas.
- Migration `000018_sprint23_hardening.up.sql` should be split per item
  to ease partial rollback (option: 4 migration files in same sprint).

## Migration Strategy

Two options:

**Option A (preferred):** one migration per item
- `000018_guc_bind_audit.up.sql`
- `000019_mfa_recovery.up.sql`
- `000020_session_list.up.sql` (no-op, reuses refresh_tokens)
- `000021_password_policy.up.sql`

**Option B (consolidated):** `000018_sprint23_hardening.up.sql` for all 4

Option A is more granular rollback but 4x migration noise. Choose based
on team preference — both acceptable.

## Open Follow-ups

1. **Audit table partitioning** — `guc_bind_audit` will grow unbounded.
   Plan for monthly partitions (similar to `login_attempts` if partitioned).
2. **Haveibeenpwned integration** — `22B.4` mentions but doesn't include.
   Sprint 24 candidate (network call in hot path = latency concern).
3. **Recovery code rotation** — should users be forced to regenerate
   codes every X months? Not in scope for `22B.2`.
4. **Session ID migration to opaque tokens** — current `id UUID` is
   enumerable. Could switch to base64 random for added security.
5. **Password strength meter** — UI feedback (zxcvbn library, no
   server-side state change). Frontend-only, out of scope.

## Decision Trace

- **ADR-0006** dated 2026-08-14: identified GUC bind audit as follow-up #2.
- **ADR-0007** dated 2026-08-15: closed migration 00015 app_admin docs.
- **This ADR (0008)** dated 2026-08-15: rolls remaining 4 deferred items
  into one Sprint 23, documents effort and design so the team can
  pick up the implementation later without re-doing the analysis.
