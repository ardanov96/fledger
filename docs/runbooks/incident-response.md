# Incident Response Runbook

**Goal:** Standardized procedures for responding to production incidents — severity classification, escalation, common playbooks, postmortem.

---

## Severity classification

| Sev | Impact | Examples | Response time |
|---|---|---|---|
| **Sev 1** | Service down OR data integrity compromised OR security breach | - /readyz failing >5 min<br>- Reconciler detects `tampered` status<br>- Token reuse attack in progress<br>- DB credentials leaked | Page immediately, all-hands response, war room |
| **Sev 2** | Major functionality degraded | - Transfer endpoint failing >10% of requests<br>- Auth broken for one tenant<br>- /metrics endpoint down<br>- Reconciler finding consistent `imbalanced` | Page on-call, fix within 4 hours |
| **Sev 3** | Minor degradation | - Single endpoint slow (p99 >2s)<br>- One tenant's data not loading<br>- Background reconciler worker stuck | Slack alert, fix within 24 hours |
| **Sev 4** | Cosmetic / informational | - Wrong dashboard color<br>- Typo in error message<br>- Non-critical log line noisy | Slack, fix in next sprint |

---

## On-call roles

| Role | Contact | Responsibility |
|---|---|---|
| **Primary on-call** | (rotation) | First responder, runs Sev 1/2 playbooks |
| **Secondary on-call** | (rotation) | Backup if primary unavailable >15 min |
| **Security team** | security@ | Token leak, RBAC violation, breach response |
| **Database admin** | dba@ | DB failures, RLS issues, migrations |
| **Infrastructure** | infra@ | Fly.io / VPS / cloud issues |

---

## Common Sev 1 playbooks

### Playbook 1: Reconciler detects `tampered` status

**Symptom:** `reconciler_runs.status = 'tampered'` OR `hash_chain_errors > 0`

**Root cause candidates:**
- Direct DB tampering (compromised credentials)
- Bug in hash chain computation
- Bug in entry creation (missing prev_hash update)

**Response:**

```bash
# 1. STOP the API to prevent further damage (optional, depends on cause)
fly scale count 0 --app fmcg-wallet-demo

# 2. Identify which period + accounts affected
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT run_id, period_id, hash_chain_errors FROM reconciler_runs WHERE status='tampered' ORDER BY started_at DESC LIMIT 5;\\\"\""

# 3. Identify tampered entries
# Compare ledger_entries.entry_hash vs recomputed hash via HashChainVerifier locally
go run ./cmd/api --verify-hashchain --period=<id>

# 4. If cause is malicious tampering:
#    a. Force-rotate JWT_SECRET (see secret-rotation.md)
#    b. Force-rotate DB password
#    c. Restore from last known-good backup (see backup-restore.md)
#    d. Enable MFA for all users (see incident-response.md#force-mfa)
#    e. File incident report
#    f. Page security team

# 5. If cause is bug (no malicious tampering):
#    a. Identify the buggy code (recent commit?)
#    b. Roll back to last good release
#    c. Write fix + test
#    d. Re-deploy
```

**Postmortem required:** root cause analysis, blast radius (which tenants affected, how many transactions), mitigation effectiveness.

---

### Playbook 2: Token reuse attack spike

**Symptom:** `auth_token_reuse_detected_total` rate >0 OR audit_logs shows many `auth.token_reuse_detected` events from same IP range.

**Response:**

```bash
# 1. Identify the affected user(s) and IP range
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT actor_id, ip_address, occurred_at FROM audit_logs WHERE action='auth.token_reuse_detected' AND occurred_at > NOW() - INTERVAL '1 hour';\\\"\""

# 2. Check if legitimate user (their refresh token was stolen) OR credential stuffing (random usernames)
# Look for: multiple actor_ids vs single actor_id, multiple IPs vs single IP

# 3a. If legitimate user (single actor_id, single IP):
#     - Confirm with user via out-of-band channel (phone call)
#     - Reset all their refresh tokens (force re-login)
#     - Recommend MFA setup

# 3b. If credential stuffing (multiple actor_ids OR many IPs):
#     - Block source IP range at Fly.io firewall
fly ips release  # (manual: add to fly.toml firewall section)
#     - Tighten /auth/login rate limit (lower burst, shorter recovery)
#     - Enable CAPTCHA (planned)
#     - Notify affected users via email

# 4. After containment:
#    - Verify normal login activity resumes
#    - File incident report
```

---

### Playbook 3: Database connection pool exhaustion

**Symptom:** `/readyz` returns 503, logs show `pgx: connection pool exhausted`

**Root cause candidates:**
- Long-running query (missing timeout)
- Connection leak (missing `defer Close()`)
- Sudden traffic spike (load test gone wrong)

**Response:**

```bash
# 1. Check current connections
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT count(*), state FROM pg_stat_activity GROUP BY state;\\\"\""

# 2. Identify long-running queries
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT pid, now() - query_start AS duration, state, query FROM pg_stat_activity WHERE state='active' AND query NOT LIKE '%pg_stat_activity%' ORDER BY duration DESC LIMIT 10;\\\"\""

# 3. If specific query is hung:
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT pg_terminate_backend(<pid>);\\\"\""

# 4. If many slow queries (CPU-bound):
#    - Check if missing index (EXPLAIN ANALYZE)
#    - Increase DB connection pool size (in fly.toml env DB_MAX_CONNS)
#    - Enable statement timeout (already set to 10s)

# 5. If connection leak (connections never released):
#    - Recent code change? Review defer Close() patterns
#    - Roll back if needed

# 6. Add capacity if sustained traffic spike:
#    - Vertical: upgrade vm.size (⚠️ costs money)
#    - Horizontal: add machine (⚠️ out of free tier)
```

---

### Playbook 4: Region-wide outage (Fly.io)

**Symptom:** All Fly.io apps in region are down, OR Fly.io status page shows incident

**Response:**

```bash
# 1. Check Fly.io status
open https://status.fly.io/

# 2. If temporary, wait for Fly.io to recover (no action)

# 3. If prolonged (>30 min), consider failover:
#    a. Deploy to alternate region (sin → syd)
#    b. Update DNS records
#    c. Restore data from S3 backup to new region

# 4. Communicate with users via status page / Twitter / email
```

---

## Common Sev 2 playbooks

### Playbook 5: One tenant's data not loading

**Symptom:** User from Tenant A reports "I can't see my accounts" while other tenants work fine.

**Response:**

```bash
# 1. Check RLS policy for tenant
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT * FROM pg_policies WHERE schemaname='public' ORDER BY tablename, policyname;\\\"\""

# 2. Verify tenant_id is in JWT
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SELECT id, tenant_id, role FROM user_credentials WHERE username='<username>';\\\"\""

# 3. Test query as that tenant
fly ssh console --app fmcg-wallet-demo -C "su - postgres -c \"psql -d fmcg_wallet -c \\\"SET app.current_tenant_id='<tenant_uuid>'; SELECT count(*) FROM accounts;\\\"\""

# 4. If empty: data wasn't created (check audit logs)
#    If non-empty but API returns empty: RLS bug (revert recent changes)
```

---

### Playbook 6: Transfer endpoint failing intermittently

**Symptom:** `POST /v1/transfers` returns 500 for ~5% of requests

**Response:**

```bash
# 1. Check recent errors in logs
# Query Loki: {app="fmcg-wallet-api"} | json | path="/v1/transfers" | status="500"

# 2. Identify common error message
# Group by msg field

# 3. Common causes:
#    - Lock contention (high concurrent traffic)
#    - DB pool exhaustion
#    - Specific account invalid (e.g., frozen after fraud detection)
#    - FX rate missing (after rate expired)

# 4. Mitigation depends on cause:
#    - Lock: increase DB_MAX_CONNS, add retry logic
#    - Pool: see Playbook 3
#    - Frozen account: confirm with user, unfreeze if legitimate
#    - FX rate: add fallback rate (manual entry from operations)
```

---

## Postmortem template

After every Sev 1 / Sev 2, file a postmortem within 5 business days.

```markdown
# Incident Postmortem: [TITLE]

**Date:** YYYY-MM-DD
**Duration:** HH:MM to HH:MM (total: X minutes)
**Severity:** Sev 1 / Sev 2 / Sev 3
**Detected by:** [user report / alert / monitoring]
**Resolved by:** [engineer name]

## Impact

- Users affected: N
- Transactions affected: N (or $X)
- Data loss: yes/no (if yes, what)
- SLA breach: yes/no

## Timeline

- HH:MM — symptom first appeared
- HH:MM — detected by [source]
- HH:MM — on-call paged
- HH:MM — root cause identified
- HH:MM — fix deployed
- HH:MM — service fully restored

## Root cause

[Detailed explanation. Include relevant code, config, or external factors.]

## Resolution

[What was done to fix. Include code/config changes, runbook updates.]

## Why didn't we catch it earlier?

[List gaps: missing test, missing alert, missing runbook, missing dashboard, etc.]

## Action items

- [ ] [Preventive measure 1 — owner — due date]
- [ ] [Preventive measure 2 — owner — due date]
- [ ] [Detection improvement — owner — due date]
- [ ] [Documentation update — owner — due date]

## Lessons learned

[What went well, what could improve]
```

---

## Communication

| Severity | Channel | Audience |
|---|---|---|
| Sev 1 | Phone call to on-call + Slack #incidents + email status page | Engineering + Customer success + Management |
| Sev 2 | Slack #incidents | Engineering |
| Sev 3 | Slack #alerts | Engineering |
| Sev 4 | GitHub issue | Engineering |

**Status page:** maintain public-facing status (https://status.example.com or statuspage.io). Update for Sev 1 + Sev 2 only.

---

## Force MFA for all users (incident response)

If credentials were leaked, force all users to enable MFA on next login:

```sql
-- Update user_credentials.mfa_required = true
UPDATE user_credentials
SET mfa_required = true
WHERE mfa_enabled = false;

-- Existing sessions invalidated (force re-login)
UPDATE refresh_tokens
SET status = 'revoked', revoked_reason = 'security_incident'
WHERE status = 'active';
```

Next time a user logs in, the system requires MFA setup before granting access.

---

## Related Documentation

- [Observability Runbook](observability.md) — how to detect incidents
- [Backup & Restore Runbook](backup-restore.md) — disaster recovery
- [Secret Rotation Runbook](secret-rotation.md) — emergency rotation procedures
- [Architecture Overview](../architecture/overview.md) — system context
- [ADR-0003: Double-entry ledger](../adr/0003-double-entry-ledger.md) — immutability principles
