# Secret Rotation Runbook

**Goal:** Periodic rotation of secrets (JWT signing key, database password, RBAC policies) without service disruption.

**Sprint 33 update:** JWT rotation now supports **zero-downtime** via the
new `JWT_SECRET_PRIMARY` + `JWT_SECRET_SECONDARY` env vars. The Signer
uses Primary; the Verifier accepts either. Procedure updated below.

---

## Secrets inventory

| Secret | Where stored | Rotation cadence | Impact of leak |
|---|---|---|---|
| **JWT_SECRET_PRIMARY** (signs new) | Fly.io `fly secrets set` | Every 90 days | Attacker can mint valid JWTs |
| **JWT_SECRET_SECONDARY** (verifies old during rotation) | Fly.io | Only during rotation windows | N/A in steady state |
| **JWT_SECRET** (legacy single-key) | Fly.io | Every 90 days | (Deprecated — use PRIMARY/SECONDARY) |
| **DB password** | Postgres role `fmcg` | Every 180 days | DB read/write access |
| **RBAC policy CSV** | `internal/auth/rbac/policies/rbac_policy.csv` | On-demand | Privilege escalation |
| **JWT_ACCESS_TTL** | 15 min (env) | Static | N/A (no secret) |
| **JWT_REFRESH_TTL** | 168 h (env) | Static | N/A (no secret) |

---

## 1. JWT_SECRET rotation (zero-downtime — Sprint 33)

**Critical:** changing the primary signing key would invalidate all tokens.
Strategy: support **2 active keys** during rotation window. The Signer
always uses PRIMARY; the Verifier accepts PRIMARY OR SECONDARY.

### Step 1: Generate new key

```bash
NEW_JWT_SECRET=$(openssl rand -hex 32)
echo "New secret (store securely): $NEW_JWT_SECRET"
```

### Step 2: Zero-downtime rotation (production procedure)

**Phase A — Add new key as SECONDARY (signing continues with PRIMARY):**

```bash
# CURRENT primary stays the same; new key becomes secondary for verification.
fly secrets set JWT_SECRET_PRIMARY="$CURRENT_SECRET" JWT_SECRET_SECONDARY="$NEW_JWT_SECRET"
fly deploy
# After deploy: Signer still uses OLD (PRIMARY). Verifier accepts BOTH OLD + NEW.
# This is the rotation window (default 24h).
```

**Phase B — Promote new key to PRIMARY (after access tokens have expired):**

Wait at least `JWT_ACCESS_TTL` (15 min) for all access tokens to expire.
Wait additional `JWT_REFRESH_TTL` grace period if you want refresh tokens
also rotated (recommended: 24h to be safe).

```bash
fly secrets set JWT_SECRET_PRIMARY="$NEW_JWT_SECRET" JWT_SECRET_SECONDARY=""
fly deploy
# After deploy: Signer uses NEW. Verifier only accepts NEW. All old tokens now rejected.
```

**Phase C — Cleanup:**

```bash
# Remove SECRET_KEY env from any old config files / CI secrets.
# Done — rotation complete.
```

### Step 3: Verify rotation (after Phase B deploy)

```bash
# Try old token (should fail with 401 TOKEN_INVALID)
curl https://fmcg-wallet-demo.fly.dev/v1/accounts \
  -H "Authorization: Bearer $OLD_TOKEN"
# Expected: 401 TOKEN_INVALID

# Login with credentials (should succeed)
NEW_TOKEN=$(curl -sX POST https://fmcg-wallet-demo.fly.dev/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@demo.fmcg-wallet","password":"demo123"}' \
  | jq -r '.data.access_token')

# Use new token (should succeed)
curl https://fmcg-wallet-demo.fly.dev/v1/accounts \
  -H "Authorization: Bearer $NEW_TOKEN"
# Expected: 200 OK with accounts
```

### Step 4: Update password manager

Store new `JWT_SECRET_PRIMARY` in:
- 1Password / Vault / equivalent (team-wide)
- CI/CD secrets (GitHub Actions)
- Backup runbook (printed paper in safe)

### Legacy mode (single JWT_SECRET)

If you haven't migrated to PRIMARY/SECONDARY yet, the legacy single-key
mode still works. Rotation requires a brief downtime (~30s for app restart):

```bash
NEW_JWT_SECRET=$(openssl rand -hex 32)
fly secrets set JWT_SECRET="$NEW_JWT_SECRET" --app fmcg-wallet-demo
fly apps restart fmcg-wallet-demo
# Downtime: ~30s. All users must re-login.
```

**Recommendation:** migrate to PRIMARY/SECONDARY by next rotation.

---

## 2. Database password rotation

**Lower urgency** (Postgres only accessible via private network in Fly.io).

### Step 1: Generate new password

```bash
NEW_DB_PASS=$(openssl rand -base64 24)
```

### Step 2: Update Postgres role password

```bash
fly ssh console --app fmcg-wallet-demo
su - postgres -c "psql -c \"ALTER USER fmcg PASSWORD '$NEW_DB_PASS';\""
```

### Step 3: Update Fly.io secrets

```bash
fly secrets set DB_PASSWORD="$NEW_DB_PASS" --app fmcg-wallet-demo

# Restart to pick up new env
fly apps restart fmcg-wallet-demo
```

### Step 4: Verify

```bash
# Check /readyz returns 200 (DB connection works with new password)
curl https://fmcg-wallet-demo.fly.dev/readyz
# Expected: {"status":"ready",...}

# Check authenticated endpoint still works
curl https://fmcg-wallet-demo.fly.dev/v1/accounts \
  -H "Authorization: Bearer $NEW_TOKEN"
# Expected: 200 OK
```

---

## 3. RBAC policy rotation (on-demand)

Use case: privilege escalation attempt detected → need to revoke permissions for a role.

### Step 1: Identify change

Edit `internal/auth/rbac/policies/rbac_policy.csv`. Example:

```csv
# Before
p, sales_rep, collection_route, read, allow
# After (revoke)
p, sales_rep, collection_route, read, deny
```

### Step 2: Deploy

```bash
git add internal/auth/rbac/policies/rbac_policy.csv
git commit -m "rbac: revoke sales_rep read on collection_routes (incident #1234)"
git push origin main
```

If using Fly.io auto-deploy (`.github/workflows/fly-deploy.yml`), the change goes live in ~3 minutes.

### Step 3: Verify

```bash
# As sales_rep, try to list collection routes
curl https://fmcg-wallet-demo.fly.dev/v1/routes \
  -H "Authorization: Bearer $SALES_REP_TOKEN"
# Expected: 403 FORBIDDEN (was 200 before rotation)
```

### Step 4: Audit log review

Check `audit_logs` for which `sales_rep` users accessed `collection_routes` in the past 90 days (incident investigation).

---

## 4. API key rotation (planned — Sprint 14+)

Currently no third-party API keys used. When added (e.g., payment gateway, FX provider), follow pattern:

```bash
# 1. Generate new key from provider dashboard
# 2. Update Fly secret (no app restart needed — process reads on startup)
fly secrets set PAYMENT_GATEWAY_API_KEY="$NEW_KEY"

# 3. Verify old key still works for in-flight requests (grace period)
# 4. Revoke old key from provider dashboard
```

---

## Rotation schedule (recommended)

| Secret | Frequency | Owner | Documented in |
|---|---|---|---|
| JWT_SECRET | Every 90 days | Security team | This file |
| DB password | Every 180 days | DBA | This file |
| RBAC policy | On-demand | Security team | Git history (commits) |
| API keys | Every 90 days | Per-team | Per-secret |
| TLS certs (Fly.io auto) | Every 90 days | Fly.io (auto) | (managed) |

---

## Emergency rotation (compromise suspected)

⚠️ **All steps in parallel, no grace period.**

```bash
# 1. Rotate JWT_SECRET immediately
NEW_JWT=$(openssl rand -hex 32)
fly secrets set JWT_SECRET="$NEW_JWT" --app fmcg-wallet-demo
fly apps restart fmcg-wallet-demo

# 2. Rotate DB password
NEW_DB=$(openssl rand -base64 24)
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -c \\\"ALTER USER fmcg PASSWORD '$NEW_DB';\\\"\""
fly secrets set DB_PASSWORD="$NEW_DB" --app fmcg-wallet-demo
fly apps restart fmcg-wallet-demo

# 3. Force logout all users (revoke all refresh tokens)
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"UPDATE refresh_tokens SET status='revoked', revoked_reason='emergency_rotation' WHERE status='active';\\\"\""

# 4. Review audit logs
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT actor_id, action, occurred_at FROM audit_logs WHERE occurred_at > NOW() - INTERVAL '24 hours' ORDER BY occurred_at DESC LIMIT 100;\\\"\""

# 5. File incident report
# (include: timeline, affected secrets, mitigation steps, residual risk)
```

---

## Related Documentation

- [Deployment (Fly.io)](deployment-fly.md) — `fly secrets set` usage
- [Architecture: Auth (JWT + RBAC)](../architecture/sequences.md#1-login-no-mfa)
- [Sprint 13: Refresh Token Rotation + MFA](../SPRINTS.md#sprint-13-refresh-token-rotation-mfa-fase-2e-lanjutan-2026-08-14)
- [Audit API](../api/audit.md) — post-rotation forensic queries
