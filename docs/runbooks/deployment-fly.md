# Deployment Runbook — Fly.io (Sprint 19)

**Goal:** Deploy FMCG Wallet ke Fly.io untuk demo interview, **strict $0/bulan** (free tier).
**Time estimate:** 30 menit untuk first-time setup, 5 menit untuk subsequent deploys.
**Audience:** Operator (you) yang baru pertama kali pakai Fly.io.

---

## 📋 Overview

```
GitHub Push
    ↓
GitHub Actions (fly-deploy.yml)
    ↓
Dockerfile.fly → Docker image
    ↓
Fly.io build + deploy
    ↓
┌─────────────────────────────────────────┐
│ Fly.io App: fmcg-wallet-demo            │
│ Region: Singapore (sin)                 │
│ Cost: $0/month ✅                        │
├─────────────────────────────────────────┤
│  supervisord (PID 1)                    │
│   ├── postgres (port 5432, internal)   │
│   ├── migrator (one-shot on boot)       │
│   └── api (port 8080, public HTTPS)     │
│                                         │
│  Volume: pg_data (1GB persistent)       │
│  URL: https://fmcg-wallet-demo.fly.dev  │
└─────────────────────────────────────────┘
```

**Single-machine architecture** untuk demo (production harus split API + DB ke 2 apps).

---

## 1. Account Setup (5 menit)

### 1.1 Buat Fly.io account (free)

1. Buka https://fly.io/app/sign-up
2. Sign up dengan email + GitHub
3. **Tidak perlu tambah payment method** untuk free tier

### 1.2 Install flyctl (CLI)

**macOS:**
```bash
brew install flyctl
```

**Linux:**
```bash
curl -L https://fly.io/install.sh | sh
export PATH="$HOME/.fly/bin:$PATH"
```

**Windows (Chocolatey):**
```powershell
choco install flyctl
```

**Verify:**
```bash
fly version
# Output: fly v0.x.x ...
```

### 1.3 Login

```bash
fly auth login
# Browser opens → authorize → kembali ke terminal
fly auth whoami
# Output: your-email@example.com
```

---

## 2. First-Time Deploy (15-20 menit)

### 2.1 Jalankan deploy script

```bash
cd /path/to/fmcg-wallet
./scripts/fly-deploy.sh
```

Script akan otomatis:
1. ✅ Verify `flyctl` installed + logged in
2. ✅ Create Fly app `fmcg-wallet-demo` di region Singapore
3. ✅ Create persistent volume `pg_data` (1GB)
4. ✅ Generate `JWT_SECRET` + set Fly secrets
5. ✅ Build image (`Dockerfile.fly`) — **5-10 menit**
6. ✅ Deploy + release — **2-3 menit**
7. ✅ Wait for health check
8. ✅ Print demo URL + credentials

### 2.2 Verifikasi deployment

```bash
# Health check
curl https://fmcg-wallet-demo.fly.dev/healthz
# Expected: {"status":"alive","uptime":"..."}

# Ready check (memvalidasi DB + Redis + dependencies)
curl https://fmcg-wallet-demo.fly.dev/readyz
# Expected: {"status":"ready",...}

# Metrics (Prometheus format)
curl https://fmcg-wallet-demo.fly.dev/metrics | head -20
```

### 2.3 Seed demo data

```bash
./scripts/seed-demo-data.sh
```

**Important:** Bcrypt hash untuk `demo123` di script sudah verified. Jika Anda ingin ganti password, generate hash baru dengan:
```bash
docker run --rm golang:1.23-alpine sh -c '
  go install golang.org/x/crypto/bcrypt@latest
  echo -n "your-new-password" | bcrypt-cli
'
```

---

## 3. Demo Credentials

Setelah seed, login dengan:

| Username | Password | Role | Access |
|---|---|---|---|
| `admin@demo.fmcg-wallet` | `demo123` | `hq_admin` | Both tenants (Acme + Beta) |
| `sales@demo.fmcg-wallet` | `demo123` | `sales_rep` | Tenant 1 (Acme) only |

### Test login via curl:

```bash
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin@demo.fmcg-wallet","password":"demo123"}'
```

Response:
```json
{
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "abc123...",
    "expires_in": 900
  }
}
```

### Test authenticated endpoint:

```bash
TOKEN="<paste-access-token-here>"

# List accounts
curl https://fmcg-wallet-demo.fly.dev/v1/accounts \
  -H "Authorization: Bearer $TOKEN"

# List tenants (admin only)
curl https://fmcg-wallet-demo.fly.dev/v1/tenants \
  -H "Authorization: Bearer $TOKEN"

# Get exchange rate
curl https://fmcg-wallet-demo.fly.dev/v1/fx-rates/latest \
  -H "Authorization: Bearer $TOKEN"
```

---

## 4. Subsequent Deploys (5 menit)

### Option A: Manual

```bash
./scripts/fly-deploy.sh --quick
```

### Option B: Auto-deploy via CI (recommended)

**Setup (one-time):**
1. Get API token:
   ```bash
   fly auth token
   # Output: FlyV1 <long-token>
   ```
2. Add to GitHub: Settings → Secrets and variables → Actions → New repository secret
   - Name: `FLY_API_TOKEN`
   - Value: `<paste-token>`
3. Push to main → auto-deploy triggers

**Verify CI:**
- GitHub → Actions tab → "Deploy to Fly.io" workflow
- Green check = deployed successfully

---

## 5. Monitoring

### 5.1 Fly.io Dashboard

https://fly.io/apps/fmcg-wallet-demo

Menampilkan:
- CPU/memory usage (should stay <80% untuk free tier)
- Network egress (cap 100GB/month)
- Request count + latency

### 5.2 Logs

```bash
# Tail logs (last 100 lines)
fly logs --app fmcg-wallet-demo

# Filter by process
fly logs --app fmcg-wallet-demo | grep "api\|postgres\|migrate"

# Save to file
fly logs --app fmcg-wallet-demo > fly-logs-$(date +%Y%m%d).log
```

### 5.3 SSH ke Machine (untuk debug)

```bash
fly ssh console --app fmcg-wallet-demo

# Inside container:
supervisorctl status
supervisorctl restart api
ps aux | grep -E "postgres|api"
tail -f /var/log/supervisor/api.err.log
```

### 5.4 Metrics

```bash
# Prometheus metrics
curl https://fmcg-wallet-demo.fly.dev/metrics
```

---

## 6. Troubleshooting

### ❌ Health check failed

```bash
# Check logs
fly logs --app fmcg-wallet-demo

# Common causes:
# 1. Migration timeout (cold start) — wait 30s, retry
# 2. Volume not mounted — fly volumes list
# 3. Secret not set — fly secrets list
```

### ❌ "Postgres not ready"

```bash
fly ssh console --app fmcg-wallet-demo
su - postgres -c "pg_isready -h 127.0.0.1"
# If false: check /var/log/supervisor/postgres.err.log
```

### ❌ "Migrations failed"

```bash
fly ssh console --app fmcg-wallet-demo
cd /app
DATABASE_URL="postgres://fmcg:fmcg_demo_password@127.0.0.1:5432/fmcg_wallet?sslmode=disable" ./migrator up
# See actual error message
```

### ❌ Demo data not visible

```bash
# Re-seed (idempotent)
./scripts/seed-demo-data.sh

# Or check if data exists
fly ssh console --app fmcg-wallet-demo
su - postgres -c "psql -d fmcg_wallet -c 'SELECT COUNT(*) FROM tenants;'"
```

### ❌ Cold start latency (>5s first request)

Normal untuk free tier setelah inactivity. Untuk keep-warm:
- Set up cron-job.org (free) untuk ping `https://fmcg-wallet-demo.fly.dev/healthz` setiap 14 menit
- Atau upgrade ke paid tier (~$5/month) — TIDAK DIREKOMENDASIKAN untuk demo

---

## 7. Cost Monitoring (Penting!)

**Constraint: $0/month strict.** Monitor regularly:

```bash
# Show resource usage
fly status --app fmcg-wallet-demo

# Estimated monthly cost (should show $0.00)
fly platform costs --app fmcg-wallet-demo
```

**Free tier limits (DO NOT exceed):**
- 3 shared-cpu-1x VMs (we use 1)
- 3GB volume storage (we use 1GB)
- 100GB egress/month

**Danger signs (auto-stop demo jika terlihat):**
- ⚠️ Egress >80GB/month → reduce load test frequency
- ⚠️ VM memory >90% → restart, or reduce BCRYPT_COST in fly.toml
- ⚠️ Multiple machines running → check `fly scale show`

---

## 8. Backup & Restore

### 8.1 Create snapshot (free)

```bash
fly volumes snapshots create pg_data --app fmcg-wallet-demo
```

Snapshots disimpan 7 hari (free tier limit).

### 8.2 List snapshots

```bash
fly volumes snapshots list pg_data --app fmcg-wallet-demo
```

### 8.3 Restore from snapshot

⚠️ **Destructive**: akan replace volume content.

```bash
# 1. List snapshot IDs
fly volumes snapshots list pg_data --app fmcg-wallet-demo

# 2. Stop app (to release volume)
fly scale count 0 --app fmcg-wallet-demo

# 3. Restore
fly volumes snapshots restore <snapshot-id> --app fmcg-wallet-demo

# 4. Restart
fly scale count 1 --app fmcg-wallet-demo
```

### 8.4 Scheduled backup (cron via Fly Machines API)

⚠️ Fly tidak punya built-in cron. Untuk production:
- Setup GitHub Action cron workflow (daily)
- Atau upgrade ke paid tier + use Fly Machines scheduled restart

---

## 9. Demo Script untuk Interview

### Persiapan sebelum interview (10 menit):

1. **Verify demo live:**
   ```bash
   curl https://fmcg-wallet-demo.fly.dev/healthz
   curl https://fmcg-wallet-demo.fly.dev/v1/auth/login -X POST \
     -H "Content-Type: application/json" \
     -d '{"username":"admin@demo.fmcg-wallet","password":"demo123"}'
   ```

2. **Open browser tabs:**
   - https://fmcg-wallet-demo.fly.dev/healthz
   - https://fmcg-wallet-demo.fly.dev/metrics
   - https://runut.github.io/fmcg-wallet/ (docs site)

3. **Have curl commands ready** (in terminal scratchpad):
   - Login → get token
   - List accounts
   - Create transfer
   - Cross-currency transfer
   - Reconciler run

### During interview (15-20 menit):

1. **Architecture overview** (3 min): Show repo structure, ADRs, SPRINTS.md
2. **Live API demo** (5 min): curl commands showing double-entry transfer
3. **Code deep-dive** (5 min): Open `transfer_service.go`, explain locking + idempotency
4. **Observability** (2 min): Show /metrics, Grafana dashboards
5. **Defense-in-depth** (3 min): Explain RLS, hash chain, rate limiting
6. **Q&A** (remaining time)

### Post-interview cleanup:

Demo data persists. Tidak perlu cleanup unless interviewer membuat test data.

---

## 10. Common Operations

| Task | Command |
|---|---|
| View logs | `fly logs --app fmcg-wallet-demo` |
| Check status | `fly status --app fmcg-wallet-demo` |
| SSH ke machine | `fly ssh console --app fmcg-wallet-demo` |
| Restart app | `fly apps restart fmcg-wallet-demo` |
| Update secret | `./scripts/fly-deploy.sh --only-secrets` |
| Re-deploy | `./scripts/fly-deploy.sh --quick` |
| Re-seed data | `./scripts/seed-demo-data.sh` |
| Backup volume | `fly volumes snapshots create pg_data --app fmcg-wallet-demo` |
| Destroy app (cleanup) | `fly apps destroy fmcg-wallet-demo` ⚠️ permanent |

---

## 11. Next Steps (Setelah Beli VPS / Production)

Single-Fly-app architecture ini **cocok untuk demo, BUKAN production**. Untuk production:

1. **Split ke 2 apps:**
   - `fmcg-wallet-api` (Dockerfile biasa tanpa Postgres)
   - `fmcg-wallet-db` (Postgres-only Fly Machine dengan daily backup)
2. **Atau pakai managed Postgres:** Fly Postgres, Supabase, Neon
3. **Atau deploy ke VPS** (Hetzner/DO) pakai `docker-compose.prod.yml` (Sprint 19 backend)

Tapi untuk demo interview, single-app Fly.io sudah cukup impressive — interviewer akan terkesan dengan:
- Live URL yang bisa langsung dicoba
- Production-grade patterns (Dockerfile, supervisord, migrations, health checks)
- Cost awareness ($0/bulan)

---

## Related Documentation

- [Fly.io Documentation](https://fly.io/docs/)
- [Sprint 19 — Fly.io Deployment (SPRINTS.md)](../../SPRINTS.md#sprint-19--deployment-flyio-fase-3b--2026-08-15)
- [ADR-0006: Tenant RLS + Field-Level Authz Strategy](../adr/0006-tenant-rls-strategy.md)
- [Integration Tests Runbook](integration-tests.md)
- [Load Testing Runbook](load-test.md)
