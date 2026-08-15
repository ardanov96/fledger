# Load Testing Runbook (Sprint 18)

**Goal:** Validate API throughput, latency, and error rate under realistic load.
**Tools:** k6 (Grafana k6) — light-weight, scriptable, CI-friendly.
**Scope:** Sprint 18 ships load test for `/v1/transfers` (the hot path).
Subsequent sprints extend to reconciler, period close, and collection routes.

---

## 1. Quick Start (Local)

### Option A — k6 binary (recommended for dev)

```bash
# Install (one-time)
# macOS:  brew install k6
# Linux:  sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
#         echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
#         sudo apt-get update && sudo apt-get install k6
# Windows: choco install k6

# Run smoke test (10s, 20 VUs, low rate)
k6 run --duration 30s --vus 20 load-test/transfer.js
```

### Option B — Docker (no install)

```bash
docker run --rm -i grafana/k6 run - <load-test/transfer.js
```

---

## 2. Configuration

Edit environment variables before running:

| Var | Default | Description |
|---|---|---|
| `BASE_URL` | `http://localhost:8080` | API base URL |
| `TOKEN` | `test-jwt-token` | Bearer token (need real JWT from `/v1/auth/login`) |
| `FROM_ACCT` | UUID placeholder | Source account ID (must have balance) |
| `TO_ACCT` | UUID placeholder | Destination account ID |
| `AMOUNT_MINOR` | `100` | Transfer amount in minor units (IDR 1.00) |

Example:
```bash
k6 run \
  --duration 60s \
  --vus 50 \
  -e BASE_URL=https://api.staging.example.com \
  -e TOKEN=$STAGING_JWT \
  -e FROM_ACCT=11111111-1111-1111-1111-111111111111 \
  -e TO_ACCT=22222222-2222-2222-2222-222222222222 \
  load-test/transfer.js
```

---

## 3. Load Profile (Default)

The default script (`load-test/transfer.js`) runs a 4-stage ramp:

```
Stage 1: Ramp up    0→20 VUs   over 10s
Stage 2: Steady     50 VUs     for 20s   ← baseline
Stage 3: Spike      50→100 VUs over 10s   ← stress test
Stage 4: Ramp down  100→0 VUs  over 10s
```

Total runtime: **50 seconds**.

---

## 4. SLO Thresholds (k6 fails the run if violated)

Configured in `options.thresholds`:

| Metric | Threshold | Why |
|---|---|---|
| `http_req_duration` p95 | < 200ms | 95% of requests must complete under 200ms |
| `http_req_duration` p99 | < 500ms | Tail latency budget |
| `http_req_failed` rate   | < 1% | Error budget (1% of requests may fail) |

**Override at runtime:**
```bash
k6 run --threshold 'http_req_duration=p(95)<300' load-test/transfer.js
```

---

## 5. Expected Behavior on a Healthy Setup

For a single API instance (1 vCPU, 2GB RAM) against local Postgres:

- **p50:** ~50-80ms
- **p95:** ~150-250ms
- **p99:** ~300-600ms (warm cache) or ~2-5s (cold cache + DB lock contention)
- **Throughput:** 200-500 req/s sustained
- **Error rate:** < 0.5% (excludes rate-limit 429s if `RATE_LIMIT_LOGIN_ENABLED`)

If p99 > 500ms with cold cache, investigate:
1. DB connection pool saturation (`max_conns` in `cmd/api/main.go`)
2. Lock contention on hot accounts (run `pg_locks` query)
3. Reconciler worker tick (every 1h — `RECONCILER_HASH_CHECK=true` adds overhead)

---

## 6. Integration with CI

The Sprint 18 CI step runs integration tests (`-tags=integration`) on every PR.
Load tests are **not** part of CI by default — too slow + flaky on shared runners.

To run load tests in CI manually:
```yaml
- name: Load test (manual trigger only)
  if: github.event_name == 'workflow_dispatch'
  run: |
    docker run --rm -i grafana/k6 run - <load-test/transfer.js
```

Wire it to scheduled cron for nightly benchmark:
```yaml
on:
  schedule:
    - cron: '0 2 * * *'  # 02:00 UTC daily
```

---

## 7. Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| `401 Unauthorized` | JWT expired/invalid | Re-login via `/v1/auth/login` |
| `429 Too Many Requests` | Rate limit hit | Disable `RATE_LIMIT_LOGIN_ENABLED` or lower VUs |
| `500 Internal Server Error` | DB issue | Check `journalctl` for Postgres errors |
| p99 spikes | DB lock contention | Reduce VUs or add DB read replica |
| Connection refused | API not running | Start API: `go run ./cmd/api` |

---

## 8. Next Steps (Sprint 18+) — NOT YET DONE

- [ ] Reconciler load test (`/v1/reconciler/run` with `run_hash_check=true`)
- [ ] Period close stress test (concurrent close requests → expect 1 wins, rest 409)
- [ ] Collection route load test (`/v1/routes` + `/v1/routes/{id}/settle`)
- [ ] Cross-currency transfer load test (FX rate lookup under contention)
- [ ] Grafana dashboard for k6 results (TIG stack already deployed)

See `docs/runbooks/integration-tests.md` for the unit/integration test contract.
