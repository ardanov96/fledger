# Secret Rotation Runbook

**Goal:** Periodic rotation of secrets (JWT signing key, database password, RBAC policies) without service disruption.

---

## Secrets inventory

| Secret | Where stored | Rotation cadence | Impact of leak |
|---|---|---|---|
| **JWT_SECRET** | Fly.io `fly secrets set` | Every 90 days | Attacker can mint valid JWTs for any user/tenant |
| **DB password** | Postgres role `fmcg` | Every 180 days | DB read/write access |
| **RBAC policy CSV** | `internal/auth/rbac/policies/rbac_policy.csv` | On-demand | Privilege escalation |
| **JWT_ACCESS_TTL** | 15 min (env) | Static | N/A (no secret) |
| **JWT_REFRESH_TTL** | 168 h (env) | Static | N/A (no secret) |

---

## 1. JWT_SECRET rotation (zero-downtime)

**Critical:** changing JWT_SECRET invalidates all existing tokens. Strategy: support **2 active keys** during rotation window.

### Step 1: Generate new key

```bash
NEW_JWT_SECRET=$(openssl rand -hex 32)
echo "New secret (store securely): $NEW_JWT_SECRET"
```

### Step 2: Set multi-key signing (planned Sprint 14+)

For zero-downtime rotation, the JWT signer must accept multiple keys. Current code only supports 1 key, so rotation requires brief downtime (~30s).

#### Zero-downtime procedure (production)

If `jwt.Signer` supports multi-key (`StaticSecret` slice):

```bash
# Set new secret as additional (not yet replacing primary)
fly secrets set JWT_SECRET_PRIMARY="$CURRENT_SECRET" JWT_SECRET_SECONDARY="$NEW_JWT_SECRET"

# Deploy new image (accepts either key)
fly deploy

# After 24h (all old tokens expired), promote new as primary
fly secrets set JWT_SECRET_PRIMARY="$NEW_JWT_SECRET" JWT_SECRET_SECONDARY=""
fly deploy

# After another 24h (all "old" tokens expired), remove secondary
fly secrets unset JWT_SECRET_SECONDARY
fly deploy
```

#### Brief-downtime procedure (current demo)

```bash
# 1. Generate new secret
NEW_JWT_SECRET=$(openssl rand -hex 32)

# 2. Set new secret (this invalidates ALL existing tokens)
fly secrets set JWT_SECRET="$NEW_JWT_SECRET" --app fmcg-wallet-demo

# 3. Restart app to pick up new secret
fly apps restart fmcg-wallet-demo

# Downtime: ~30s (Fly.io restart)
# All users must re-login
```

### Step 3: Verify rotation

```bash
# Try old token (should fail with 401 TOKEN_INVALID)
curl https://fmcg-wallet-demo.fly.dev/v1/accounts \
  -H "Authorization: Bearer $OLD_TOKEN"
# Expected: 401 TOKEN_INVALID

# Login with new credentials (should succeed)
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

Store new `JWT_SECRET` in:
- 1Password / Vault / equivalent (team-wide)
- CI/CD secrets (GitHub Actions)
- Backup runbook (printed paper in safe)

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
- [Sprint 13: Refresh Token Rotation + MFA](../../SPRINTS.md#sprint-13--refresh-token-rotation--mfa-fase-2e-lanjutan--2026-08-14)
- [Audit API](../api/audit.md) — post-rotation forensic queries
