# Security Q&A — FMCG Wallet

5 anticipated pertanyaan interview tentang security dan jawaban berbasis codebase kami.

---

## Q1: JWT vs opaque refresh — why use both?

**Answer:**

Different security profiles for different token types — they serve different purposes:

### Access token: JWT (stateless)

**Properties:**
- Self-contained: includes `sub`, `tenant_id`, `role`, `exp`.
- Verifiable without DB lookup: just validate signature with `JWT_SECRET`.
- Fast: ~0.1ms validation.
- Stateless: no server-side state needed.

**Why JWT fits access token:**
- Sent on EVERY API request (potentially 100/sec/user).
- Must validate fast → DB lookup would add 1-5ms per request.
- Short TTL (15 min) limits blast radius if leaked.

### Refresh token: opaque + server-side hash

**Properties:**
- Opaque: random 32 bytes (256 bits entropy), base64-encoded.
- Server stores only **SHA-256 hash** of the token (not the raw token).
- Validation requires DB lookup (or cache).
- Stateful: server tracks family + rotation chain.

**Why opaque + hash fits refresh token:**
- Sent infrequently (every 15 min, on token expiry).
- DB lookup acceptable (~5ms one-time).
- Long TTL (168h = 7 days) means revocation MUST work — opaque + server-side enables instant revoke.
- Raw token never stored server-side → even DB breach can't replay tokens.

### Why both?

If both were JWT:
- ✅ Stateless, fast validation
- ❌ Can't revoke without changing JWT_SECRET (invalidates ALL tokens)
- ❌ Can't detect token theft (JWT can't tell if it's a replay)

If both were opaque:
- ✅ Revocable + theft-detectable
- ❌ DB lookup on EVERY API request (slow, expensive)

**Hybrid gives best of both:**
- Access (high frequency, short TTL): JWT — fast, no DB
- Refresh (low frequency, long TTL): opaque — revocable, theft-detectable

### Code reference

```go
// internal/usecase/auth_service.go
// Login: returns BOTH
result := authService.Login(ctx, in)
// result.AccessToken = signed JWT (15min TTL)
// result.RefreshToken = opaque random token (7-day TTL)

// Refresh: rotates refresh, validates raw → hash
newPair := authService.Refresh(ctx, RefreshInput{
    RefreshToken: presentedRawToken, // SHA-256 hashed before DB lookup
})
```

### Trade-off

- **More complex code path** (2 token types, 2 expiry checks).
- **Refresh token stored in DB** (vs stateless JWT). Trade storage for security.

### What's NOT protected

- Access token in `localStorage` is XSS-vulnerable (production fix: httpOnly cookies).
- Refresh token in DB needs protection (password hashing, restricted DB access). We rely on Postgres RLS + encrypted-at-rest (Fly.io default).

---

## Q2: MFA bypass scenarios?

**Answer:**

MFA is **defense-in-depth** — even if compromised, multiple layers prevent full breach.

### Layer 1: bcrypt password (Sprint 13)
- Cost 10 (demo) / 12 (prod). ~250ms per hash at cost 12.
- Salted hash. Even if DB breached, rainbow tables infeasible.
- 5 failed attempts → 15-minute lockout. Defeats brute force.

### Layer 2: TOTP MFA (RFC 6238)
- 6-digit code, 30s period, ±30s drift tolerance.
- 3 wrong attempts → 5-minute MFA lockout (DB enforced).
- One-time challenge token, 5-min TTL, can't be replayed.

### Layer 3: Reuse-detection (Sprint 13)
- If refresh token is rotated + old one is presented again → entire family revoked.
- Even if attacker has both access + refresh token, they can be cut off instantly.

### Layer 4: Audit trail (Sprint 5)
- Every login (success/failure) recorded with IP, user agent, timestamp.
- Brute-force pattern detectable from logs.

### Bypass scenarios analyzed

| Attack vector | Protected by | Residual risk |
|---|---|---|
| Password brute force | bcrypt + lockout (Sprint 13) | Lockout denial-of-service (5 attempts → 15min lockout per user) |
| Stolen password + stolen TOTP secret | Reuse-detection + anomaly detection | TOTP secret stored encrypted-at-rest (Fly.io default) |
| Session hijacking via XSS | (deferred) httpOnly cookies | Sprint 20+ to address |
| Phishing (real-time relay) | TOTP time-window (30s) limits relay window | Real-time phishing still works for 30s |
| SIM swap | Recovery codes (deferred) | Sprint 19+ |
| Insider attack (DB access) | DB RLS + audit + hash chain tamper detection | RLS limits blast radius |
| Brute-force MFA code | 3-attempt lockout + 5min cooldown | Low (3 attempts × 30s = 90s window max) |

### Trade-off

- **MFA recovery codes** deferred — without them, user who loses phone is locked out. Need secure backup flow.
- **Anomaly detection** (impossible travel, unusual hour) — not implemented. Should add for production.
- **Trusted device list** — not implemented. Production should allow 30-day skip on trusted devices.

---

## Q3: How do you prevent timing attacks on bcrypt?

**Answer:**

Bcrypt is **already timing-safe by design** at the algorithm level — but we add defense layers:

### Layer 1: bcrypt constant-time comparison
- `bcrypt.CompareHashAndPassword(hash, password)` uses constant-time comparison internally.
- Even if attacker measures response time, comparison doesn't leak info about which hash bytes match.

### Layer 2: Account lockout (Sprint 13)
- After 5 failed attempts → `failed_login_count` increments → 15-min lockout.
- This is **rate-based defense**, not timing-based, but effectively neutralizes timing attacks.
- After 5 fails, even a correct password returns "locked", not "wrong password" — attacker can't differentiate.

### Layer 3: Logging + monitoring
- Every login attempt (success/failure) logged with IP + timestamp.
- Anomalous pattern (many fails from 1 IP, many users) → alert.
- Rate-limited at network layer (Nginx, Cloudflare, or Fly.io firewall).

### Layer 4: Network-level defenses
- HTTPS only (Fly.io auto-redirect).
- TLS 1.2+ minimum.
- HSTS header to prevent downgrade attacks.
- (Production) WAF rules to block known attack patterns.

### What we're NOT protecting (yet)

- **Username enumeration via response timing:** `/v1/auth/login` returns "invalid credentials" for both wrong-user and wrong-password cases (single code path). But it might still leak via DB lookup time for user-existing case. Mitigated by `lockout` after 5 attempts (regardless of which failed).
- **Side-channel on bcrypt cost:** we use cost 10 (demo) or 12 (prod). Attacker could try to detect cost from timing, but bcrypt is well-tested for this.

### Production hardening (deferred)

- Use `crypto/subtle.ConstantTimeCompare` for hash comparison (Go stdlib already has this — bcrypt uses it internally).
- Add CAPTCHA after 3 failed attempts (prevents automated brute force).
- IP-based rate limit at edge (separate from app-level).

### Code reference

```go
// internal/platform/auth/hasher.go
func (h *BcryptPasswordHasher) Verify(hash, password string) (bool, error) {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    if err != nil {
        return false, nil // uniform error message
    }
    return true, nil
}
```

### Trade-off

- bcrypt cost 10 (~25ms) is fast for legitimate users, slow enough to deter brute force.
- For higher security, increase to 12 (~250ms) — but 10x latency hit.
- Demo uses cost 10 for UX; production uses cost 12.

---

## Q4: Rate limit false positives?

**Answer:**

Sprint 14 + Sprint 14 follow-up = 4 tiers of rate limit (per-IP for `/auth/login`, multi-tier IP+user+tenant for `/v1/*`). Risk of false positives analyzed:

### Sources of false positives

| Scenario | False positive? | Mitigation |
|---|---|---|
| Multiple users behind same NAT (office) | ❌ high | Use per-user tier (bypasses IP tier) |
| Mobile app backgrounded + reopens | ❌ medium | Refresh token tier limits requests, not refreshes |
| Webhook integrations (IP shared) | ❌ medium | Tier 3 (tenant) catches aggregate abuse |
| Burst legitimate load (payday) | ❌ low | Token bucket refills 50 rps sustained, 100 burst |
| CDN/proxy IP in front of many users | ❌ high | Use X-Forwarded-For + tier-key based on user, not IP |

### Defense against false positives

1. **Multi-tier chain (Sprint 14 follow-up):** if any tier allows, request proceeds. Per-IP can fail without blocking authenticated user.
2. **Generous defaults:** 100 burst, 50 rps sustained for global. Per-tenant 1000 burst, 500 rps.
3. **Per-user tier bypasses IP:** if user has valid JWT, per-user tier is checked, not just per-IP.
4. **Configurable:** all env vars overridable. Operations can relax if needed.

### Detection

- **MultiTierLimiterMetrics:** per-tier counters (allowed/rejected). Visible via Prometheus (planned Sprint 18+).
- **Audit log:** 429 responses include `X-RateLimit-Rejected-By: <tier>` header.
- Operator can investigate false positives: `grep 'RATE_LIMITED' /var/log/api.log`.

### Future work

- **Adaptive limiting:** increase limit if user has high historical success rate.
- **Whitelist:** known good IPs (office, CDN) bypass IP tier.
- **Alert:** notify user via email if their account is rate-limited (avoid confusion).

### Trade-offs

- **More tiers = more compute per request.** Acceptable (~0.1ms per tier).
- **False positives in shared-IP scenarios** are inherent to IP-based limiting. Mitigated by per-user tier.
- **Strict limits hurt legitimate users in low-bandwidth regions.** Configurable.

---

## Q5: RLS bypass scenarios (e.g., super_admin)?

**Answer:**

RLS is designed to enforce tenant isolation for **normal operations**. Super admin operations need a different path.

### Current state

- `fmcg` Postgres role = RLS-enforced (regular app user).
- **No `app_admin` role yet** — Sprint 18+ follow-up to create.
- Migration scripts run as Postgres superuser (which bypasses RLS) — but they're not accessible from the app at runtime.

### Bypass scenarios currently

1. **Migration scripts** (`migrator` binary) — runs as DB superuser. Bypasses RLS by design (for schema migrations). Run via CI/CD or operator, not from API.
2. **Direct psql access** — if operator has DB credentials, can query any tenant. But this is by design (DBA needs it).
3. **`fmcg_demo` role** (only on demo seed) — bypasses RLS for seeding convenience. NOT used in production.

### Planned `app_admin` role (Sprint 18+)

```sql
-- migration: add app_admin role
CREATE ROLE app_admin NOINHERIT;
GRANT app_admin TO fmcg; -- fmcg can SET ROLE app_admin

-- RLS policies: bypass for app_admin
ALTER TABLE accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_admin_bypass ON accounts
  TO app_admin USING (true);  -- admin sees all tenants
```

```go
// In tx_adapter: optional SET LOCAL ROLE app_admin for admin operations
if isAdminRequest(ctx) {
    _, err := tx.Exec(ctx, "SET LOCAL ROLE app_admin")
}
```

### What protects us if admin role leaks

- **Audit log** records WHO accessed WHAT. Admin reading tenant A's data is visible in audit.
- **Break-glass procedure** — admin actions require out-of-band approval (e.g., PR + 2 reviewers).
- **Rate limit** still applies (even admins can't bulk-export without alerting).
- **Reconciliation** — admin edits would create imbalance, reconciler flags it.

### What's NOT protected

- **Malicious insider with DB superuser access:** can disable RLS entirely (`ALTER TABLE ... DISABLE ROW LEVEL SECURITY`). No defense at app level. Mitigated by:
  - Postgres audit log (`pgaudit` extension)
  - DB activity monitoring
  - Principle of least privilege (DB access only for specific operators)
- **Privilege escalation in app code:** if app code has bug that grants admin role accidentally, no detection. Mitigated by:
  - Code review
  - Integration tests covering role assignment
  - Periodic security audit

### Trade-offs

- **`app_admin` role adds complexity** — must audit admin operations carefully.
- **No automatic break-glass detection** — relies on audit log review (manual).
- **Production needs separate audit pipeline** (e.g., SIEM integration) — deferred.

---

## Reference

- [Auth API](../api/auth.md) — Login, refresh, MFA flow
- [Architecture: Sequences — Login with MFA](../architecture/sequences.md#2-login-with-mfa)
- [Architecture: Sequences — Refresh Token Rotation](../architecture/sequences.md#3-refresh-token-rotation-with-reuse-detection)
- [ADR-0006: Tenant RLS + Field-Level Authz Strategy](../adr/0006-tenant-rls-strategy.md) — RLS design
- [Sprint 13: Refresh Token Rotation + MFA](../SPRINTS.md#sprint-13--refresh-token-rotation--mfa-fase-2e-lanjutan--2026-08-14)
- [Sprint 14 follow-up: Multi-Tier Rate Limiting](../SPRINTS.md#sprint-14-followup--multi-tier-rate-limiting-fase-2d-lanjutan--2026-08-15)
- [Secret Rotation Runbook](../runbooks/secret-rotation.md)
- [Demo Script](demo-script.md)
- [Fintech Q&A](q-fintech.md)
- [Distributed Systems Q&A](q-distributed-systems.md)
