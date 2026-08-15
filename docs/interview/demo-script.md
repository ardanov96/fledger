# Demo Script — FMCG Wallet (Sprint 21)

**Purpose:** 5-7 minute structured demo untuk interview. Format: `masalah → solusi → live demo → Q&A handoff`.

**Audience:** Technical interviewer (engineering manager / senior engineer / hiring committee).

**Total length:** 7-10 minutes including Q&A teaser. Adjust to fit interview time budget.

---

## 🎬 Pre-Demo Checklist (5 menit before interview)

- [ ] Demo URL live: https://fmcg-wallet-demo.fly.dev/healthz returns 200
- [ ] Run `./scripts/seed-demo-data.sh` if seed hasn't been applied
- [ ] Web dashboard running: `node web/server.js` at http://localhost:3000
- [ ] Terminal scratchpad open with cURL commands (below)
- [ ] Browser tabs pre-loaded:
  - [ ] https://fmcg-wallet-demo.fly.dev/healthz
  - [ ] https://fmcg-wallet-demo.fly.dev/metrics
  - [ ] https://fmcg-wallet-demo.fly.dev/v1/accounts (will 401 — shows auth works)
  - [ ] http://localhost:3000 (web dashboard login page)
  - [ ] https://runut.github.io/fmcg-wallet/ (documentation site)

---

## 🎬 Act 1 — Context & Problem (60 detik)

**Talking points:**

> "Indonesia punya 270,000+ distributor FMCG. Mereka butuh catat transfer uang antar distributor harian — Rp 5-50 juta per transaksi. Masalahnya: tidak ada sistem yang double-entry + immutable + auditable + multi-tenant-friendly.
>
> Saya bangun **FMCG Wallet** — production-grade hybrid wallet backend dengan Go + PostgreSQL, fokus pada 3 hal: **defense-in-depth**, **observability**, dan **production-ready patterns**. Demo saya tunjukkan bagaimana setiap requirement non-functional (audit, RLS, idempotency) jadi code, bukan docs."

**Show:** [Architecture Overview page](https://runut.github.io/fmcg-wallet/architecture/overview/)

---

## 🎬 Act 2 — Live API Demo (3-4 menit)

### Step 2.1 — Login + Token Lifecycle (30 detik)

**Talking point:** "Auth pakai refresh token rotation dengan reuse-detection. Kalau attacker steal refresh token setelah legitimate rotation, entire family revoked."

```bash
# Login → get token pair
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@demo.fmcg-wallet","password":"demo123"}'
```

**Highlight in response:**
- `access_token` (15 min TTL, JWT)
- `refresh_token` (168h TTL, opaque + SHA-256 hash only)
- `expires_in: 900`

```bash
# Save token for next steps
TOKEN=$(curl -sX POST https://fmcg-wallet-demo.fly.dev/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@demo.fmcg-wallet","password":"demo123"}' \
  | jq -r '.data.access_token')
echo "Got token: ${TOKEN:0:30}..."
```

### Step 2.2 — Double-Entry Transfer (60 detik)

**Talking point:** "POST /v1/transfers dengan Idempotency-Key. Concurrent transfers same source dijamin serial via UUID-sorted LockPairForUpdate — zero deadlock."

```bash
# Get two accounts (read-only demo data)
curl -sH "Authorization: Bearer $TOKEN" \
  https://fmcg-wallet-demo.fly.dev/v1/accounts | jq '.data[] | {id, code, name, cached_balance_minor, currency}'
```

```bash
# Create transfer (idempotent: safe to retry without double-charge)
IDEM=$(uuidgen)
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/transfers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $IDEM" \
  -H "Content-Type: application/json" \
  -d '{
    "from_account_id": "<cash-id>",
    "to_account_id": "<bca-id>",
    "amount_minor": 50000000,
    "currency": "IDR",
    "description": "Demo transfer — interview proof"
  }'
```

**Highlight in response:**
- `status: "posted"`
- 2 entries created (debit + credit)
- Both accounts balance updated atomically

### Step 2.3 — Cross-Currency FX (45 detik)

**Talking point:** "USD ke IDR dengan rate snapshot per transaction. Historis bisa replay rate exact, bukan rate hari ini."

```bash
# Preview conversion (no side effect)
curl -sX POST https://fmcg-wallet-demo.fly.dev/v1/currencies/convert \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "from_currency": "USD",
    "to_currency": "IDR",
    "amount_minor": 10000
  }' | jq

# Expected: USD 100 → IDR 1,575,000 (rate=15750, rate_id=r_xyz)
```

### Step 2.4 — Tamper Detection (60 detik)

**Talking point:** "Hash chain: SHA-256 prev_hash per entry. Modify 1 byte → entire chain breaks. Reconciler catches it within 1 hour (background) atau instant (manual API)."

```bash
# Trigger reconciler for current period
PERIOD_ID="77777777-7777-7777-7777-777777777777"
TENANT_ID="11111111-1111-1111-1111-111111111111"

curl -X POST https://fmcg-wallet-demo.fly.dev/v1/reconciler/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"tenant_id\":\"$TENANT_ID\",\"period_id\":\"$PERIOD_ID\",\"run_hash_check\":true}"
```

```bash
# Then in another tab, tamper a ledger entry directly:
psql $DATABASE_URL -c "UPDATE ledger_entries SET amount = 999999 WHERE id = '<entry-id>';"

# Re-run reconciler — status will be 'tampered':
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/reconciler/run ... (same as above)

# Get latest run
curl -sH "Authorization: Bearer $TOKEN" \
  "https://fmcg-wallet-demo.fly.dev/v1/reconciler/runs?tenant_id=$TENANT_ID&limit=1" \
  | jq '.data[0] | {status, hash_chain_ok, hash_chain_errors}'
```

**Expected:** `status: "tampered", hash_chain_ok: false, hash_chain_errors: 3+`

---

## 🎬 Act 3 — Web Dashboard (60 detik)

**Show:** [http://localhost:3000](http://localhost:3000)

**Walk-through:**
1. **Login** with demo credentials (auto-prefilled)
2. **Dashboard** → see stats cards (2 accounts, 1 invoice, etc.)
3. **Accounts** → click through, see balance + status
4. **Transfers** → submit form → see green success "✅ Transfer tx_xxx created"
5. **Currencies** → FX converter widget — type USD → IDR → see live result

**Highlight:**
- "Zero npm dependencies — vanilla JS + Node stdlib only. Portfolio demo, not production SPA."
- "9 views consuming ~25 endpoints. Hash-based routing (#dashboard, #accounts)."

---

## 🎬 Act 4 — Production-Grade Evidence (60 detik)

### Show `mkdocs.yml` rendered docs site

**Highlight 4 runbooks:**
- `docs/runbooks/backup-restore.md` — disaster recovery (Fly volumes snapshots + PITR for production)
- `docs/runbooks/secret-rotation.md` — JWT_SECRET zero-downtime rotation procedure
- `docs/runbooks/observability.md` — /healthz, /readyz, /metrics, structured JSON logs
- `docs/runbooks/incident-response.md` — Sev 1/2/3/4 classification + 6 playbooks

**Show metrics endpoint:**
```bash
curl https://fmcg-wallet-demo.fly.dev/metrics | grep -E "^(http_requests_total|reconciler_runs_total|transfers_total)" | head -20
```

### Show test coverage

```bash
# Sprint counts (visible from CI):
go test -count=1 -tags=integration ./internal/usecase/...
# 15 property tests (Fase 7A) + 100+ unit tests + 5 E2E integration tests
```

---

## 🎬 Act 5 — Architecture Quick-Dive (90 detik)

**Show:** [C4 Diagrams page](https://runut.github.io/fmcg-wallet/architecture/c4-diagrams.md)

**Highlight:**
- **C4 Level 1:** 3 actor types (HQ admin, field rep, auditor)
- **C4 Level 2:** Single Fly.io app, multi-process via supervisord (postgres + migrator + api)
- **C4 Level 3:** 4 layers (handler → service → domain → repository), clean-lite architecture
- **Multi-tenant isolation:** Postgres RLS + tx-scoped GUC variable (defense-in-depth)
- **Audit:** Tamper-evident (immutable ledger_entries + audit_logs table)

**Key differentiators (say this out loud):**
> "Yang paling saya banggakan: **Tier 1 defense-in-depth**. Setiap requirement non-functional punya multiple layers:
> - Audit: middleware logs + DB trigger + immutable table (no UPDATE/DELETE)
> - RLS: middleware sets GUC + DB policy enforces + 7 tx_adapters inject automatically
> - Idempotency: client header + DB unique key + service-level check + error code
> - Rate limiting: 3-tier chain (IP + user + tenant) — fail fast on first tier reject"

---

## 🎬 Act 6 — Q&A Handoff (30 detik)

> "Itu tadi demo singkat. Saya bisa jawab lebih dalam di Q&A — misalnya:
> - Double-entry accounting vs single-entry tradeoff
> - Hash chain trade-off vs simpler hash
> - Multi-tenant strategy: app-layer filter vs DB RLS
> - Why Go for fintech backend
>
> Atau kita bisa dive ke salah satu code path — kasih tahu area mana yang paling menarik buat Anda."

---

## 📋 cURL Commands Cheat Sheet

Copy-paste ke terminal scratchpad:

```bash
BASE="https://fmcg-wallet-demo.fly.dev"
TOKEN=$(curl -sX POST $BASE/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@demo.fmcg-wallet","password":"demo123"}' \
  | jq -r '.data.access_token')

# 1. Login (already done above)

# 2. List accounts
curl -sH "Authorization: Bearer $TOKEN" $BASE/v1/accounts | jq '.data[]'

# 3. Get FX rate USD→IDR
curl -sH "Authorization: Bearer $TOKEN" \
  "$BASE/v1/fx-rates/latest?tenant_id=11111111-1111-1111-1111-111111111111&from=USD&to=IDR" | jq

# 4. FX converter (no side effect)
curl -sX POST $BASE/v1/currencies/convert \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"11111111-1111-1111-1111-111111111111","from_currency":"USD","to_currency":"IDR","amount_minor":10000}' | jq

# 5. Trigger reconciler
curl -sX POST $BASE/v1/reconciler/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"11111111-1111-1111-1111-111111111111","period_id":"77777777-7777-7777-7777-777777777777","run_hash_check":true}'

# 6. Latest reconciler run
curl -sH "Authorization: Bearer $TOKEN" \
  "$BASE/v1/reconciler/runs?tenant_id=11111111-1111-1111-1111-111111111111&limit=1" | jq '.data[0]'

# 7. Audit log
curl -sH "Authorization: Bearer $TOKEN" "$BASE/v1/audit?limit=20" | jq '.data[] | {action, occurred_at}'

# 8. Metrics
curl -s $BASE/metrics | grep -E "^(http_requests_total|reconciler_runs_total)"
```

---

## ❓ Anticipated Q&A (use these to prep)

| Category | Question | Reference |
|---|---|---|
| Fintech | "Why double-entry vs single-entry?" | `q-fintech.md` |
| Fintech | "How does hash chain prevent tampering?" | `q-fintech.md` |
| Fintech | "How do you handle FX rate volatility?" | `q-fintech.md` |
| Fintech | "What about reconciliation — manual vs automated?" | `q-fintech.md` |
| Fintech | "Idempotency-Key TTL — how long is safe?" | `q-fintech.md` |
| Distributed | "Why UUID-sorted lock ordering?" | `q-distributed-systems.md` |
| Distributed | "RLS vs application-layer tenant filter — why both?" | `q-distributed-systems.md` |
| Distributed | "How do you handle clock skew for distributed systems?" | `q-distributed-systems.md` |
| Distributed | "What's your consistency model — strong or eventual?" | `q-distributed-systems.md` |
| Distributed | "How would you scale to 1M req/s?" | `q-distributed-systems.md` |
| Security | "JWT vs opaque refresh — why?" | `q-security.md` |
| Security | "MFA bypass scenarios?" | `q-security.md` |
| Security | "How do you prevent timing attacks on bcrypt?" | `q-security.md` |
| Security | "Rate limit false positives?" | `q-security.md` |
| Security | "RLS bypass scenarios (e.g., super_admin)?" | `q-security.md` |

---

## 🎯 Closing Lines

**Strong close (pick one):**

> "Saya sengaja pilih single Fly.io app multi-process instead of 2-app split — free tier constraint + demo simplicity. Untuk production saya akan split jadi 2 apps + managed Postgres + Redis-backed rate limit."

> "Yang saya paling banggakan bukan tech stack — tapi Tier 1 defense-in-depth. Setiap concern (audit, RLS, idempotency) punya minimal 2 layers. Single point of failure hampir tidak ada."

> "Total ada 19 sprint selesai, 9 docs API ref, 4 runbooks, C4 + sequence diagrams. Portfolio ini bukan cuma 'running code' — tapi 'production-disciplined code'."

---

## 🎥 Video Production Notes (for future self-recording)

When actually recording the video:

- **Tool:** OBS Studio + screen recorder + decent microphone
- **Length:** Target 5-7 minutes. Rehearse 2-3 times for flow.
- **Pacing:** Slow on Act 2 (API demo) — that's the core proof. Fast on Act 1 + Act 4.
- **Cursor visibility:** Use large font (16-18pt) so interviewer can read code on screen.
- **B-roll:** Cut to architecture diagrams between live API demos.
- **Captions:** Auto-generate, then edit for technical terms (Idempotency-Key, RLS, etc).
- **Export:** 1080p MP4, H.264, <50MB for sharing on portfolio site.

**Optional follow-up:**
- Host video on YouTube (unlisted) + embed in portfolio site
- Create 30-second teaser for social media
- Create 2-minute technical deep-dive for advanced audiences

---

**End of demo script.** Good luck dengan interview! 🎯
