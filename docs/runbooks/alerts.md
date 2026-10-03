# Alerting Runbook (Sprint 59)

**Goal:** When an alert fires, ops/oncall can quickly diagnose + mitigate. Each alert in `deployments/prometheus/alerts.yml` has a corresponding entry here.

## Severity levels

| Severity | Response time | Channel (Sprint 59 default) | Production override |
|---|---|---|---|
| `critical` | Page within 5 min | `oncall` webhook | PagerDuty / OpsGenie |
| `warning` | Investigate within 1h | `ops` webhook | Slack `#ops-alerts` |
| `info` | Next business day | `info-log` webhook | Slack `#fmcg-info` |

All Sprint 59 receivers are stub webhooks (`http://127.0.0.1:5001/*`). For
production, mount `deployments/alertmanager/alertmanager.yml` with real
webhook URLs (Slack, PagerDuty, email) — see `alerting:` section below.

## How to test

```bash
# 1. Start the stack (Postgres + Prometheus + Alertmanager + Grafana)
make up

# 2. Wait for Prometheus to scrape once (~30s) and evaluate alerts
curl -s http://localhost:9090/api/v1/rules | jq '.data.groups[].rules[].name'
# → Lists all FMCG* alert names

# 3. View firing alerts in Alertmanager UI
open http://localhost:9093  # shows all firing + resolved alerts

# 4. Force a critical alert to test the pipeline
# (e.g., make API unreachable)
docker stop fmcg_api
# → FMCGApiDown alert fires within 2m
# → Alertmanager sends to 'oncall' receiver (webhook to 127.0.0.1:5001)

# 5. Restart API
docker start fmcg_api
# → alert resolves within 5m
```

---

## Alert reference

### `FMCGApiP99LatencyHigh` (warning) / `FMCGApiP99LatencyCritical` (critical)

**Severity:** warning / critical
**Metric:** `histogram_quantile(0.99, http_request_duration_seconds)`
**Threshold:** > 1s / > 2s for 5m

**What it means:** p99 request latency is too high. Users feel the API as laggy.

**First response:**
1. Check `System Health → Latency Percentiles` panel — is it a single endpoint or all?
2. Check `System Health → DB Connection Pool Usage` — pool exhaustion often causes this
3. Check `Tempo` traces for the slow endpoint — is it DB query, external call, or computation?
4. Check Loki logs filtered by `status=5xx OR duration>1s`

**Common causes:**
- DB connection pool exhausted (Sprint 26 alert handles this separately)
- Slow query (missing index, large table scan)
- External dependency (NATS, payment gateway) timing out
- Memory pressure / GC pause
- Recent deploy introduced regression

**Mitigation:**
- Scale DB connection pool (`DB_MAX_CONNS` env var)
- Roll back recent deploy if correlating with latency spike
- Add index if query plan shows seq scan

---

### `FMCGApiErrorRateElevated` (warning) / `FMCGApiErrorRateCritical` (critical)

**Severity:** warning / critical
**Metric:** `5xx rate / total rate * 100`
**Threshold:** > 1% / > 5% for 5m

**What it means:** More than 1% (or 5%) of requests are failing with 5xx.

**First response:**
1. Check `System Health → Error Rate by Status Code` panel — which 5xx is dominant?
2. Check Loki logs filtered by `status=5xx`
3. Is this correlated with deploy? (check timestamps vs latest release)

**Common causes:**
- Bad deploy (rollback)
- DB unavailable (check Postgres health)
- RLS policy rejects too many requests (auth code bug)
- Upstream API failure (NATS, payment gateway)

**Mitigation:**
- Roll back deploy if recent
- If DB unavailable, check Postgres container (`docker logs fmcg_postgres`)
- If RLS-related, check if user_credentials table locked (Sprint 03: revoked tokens)

---

### `FMCGApiDown` (critical)

**Severity:** critical
**Metric:** `up{job=~"fmcg-wallet-api.*"} == 0`
**Threshold:** for 2m

**What it means:** Prometheus can't scrape the API. Service is down or unreachable.

**First response:**
1. Check if API container is running: `docker ps | grep fmcg_api`
2. If running, check logs: `docker logs fmcg_api --tail 100`
3. If not running, check why it exited (OOM? panic?)

**Common causes:**
- API process panic (check for stack traces in logs)
- OOM kill (check `docker inspect fmcg_api | grep OOM`)
- Network issue (Prometheus can't reach host.docker.internal:8080)

**Mitigation:**
- Restart container: `docker restart fmcg_api`
- If OOM, increase memory limit or find leak
- If panic, roll back + fix + redeploy

---

### `FMCDbPoolSaturated` (warning) / `FMCDbPoolExhausted` (critical)

**Severity:** warning / critical
**Metric:** `pgxpool_acquired_conns / pgxpool_total_conns`
**Threshold:** > 80% / > 95% for 5m

**What it means:** Most/all DB connections are checked out. New requests will block or time out.

**First response:**
1. Check `System Health → DB Connection Pool Usage` panel
2. Check `Loki: SBC2 with conn pool acquired` for queries holding connections
3. Is there a long-running query? `SELECT * FROM pg_stat_activity WHERE state='active';`
4. Is there a deadlock? `SELECT * FROM pg_locks WHERE NOT granted;`

**Common causes:**
- Connection leak (forgot to `defer tx.Rollback()`)
- Long-running query blocking the pool
- Deadlock causing pile-up of pending queries
- DB pool size too small for current load

**Mitigation:**
- Kill the blocking query: `SELECT pg_cancel_backend(pid);`
- Increase pool size: `DB_MAX_CONNS=50` (was 20)
- Find the leak: Sprint 41 chaos tests should catch this in CI

---

### `FMCOutboxPublisherLag` (warning) / `FMCOutboxPublisherCritical` (critical)

**Severity:** warning / critical
**Metric:** `SELECT COUNT(*) FROM outbox_events WHERE published_at IS NULL`
**Threshold:** > 1000 / > 10000 for 5m

**What it means:** The outbox publisher is falling behind. Events accumulate unpublished. Consumers (notification dispatcher, fraud scanner) won't see new events.

**First response:**
1. Check worker container logs: `docker logs fmcg_worker --tail 100`
2. Check NATS connection: `docker logs fmcg_nats --tail 50`
3. Check if publisher process is running: `docker ps | grep fmcg_worker`

**Common causes:**
- NATS broker unreachable
- Worker process crashed (OOM, panic)
- Database write storm slowing the publisher's SELECT
- Sprint 41 chaos test scenario (verify recovery works)

**Mitigation:**
- Restart worker: `docker restart fmcg_worker`
- If NATS down, check `docker logs fmcg_nats`
- If DB write storm, check Postgres slow log

---

### `FMCAuthFailureSpike` (warning)

**Severity:** warning
**Metric:** `sum(rate(http_requests_total{status=~"40[13]"}[5m]))`
**Threshold:** > 50/s for 5m

**What it means:** Spike in 401/403 errors. Could be attack, expired tokens, or misconfigured client.

**First response:**
1. Check `Loki: 401/403 with top IPs`
2. Is this a single IP? (attack) or many? (clock skew causing expired tokens)
3. Correlate with login activity (high login rate?)

**Common causes:**
- Credential stuffing attack (single IP, many usernames)
- Bot scanning for valid tokens
- Clock skew on client side (tokens expired)
- Mass logout after Sprint 22 password policy enforcement

**Mitigation:**
- If single IP, block via WAF/CDN
- If clock skew, document NTP requirement for clients
- If mass logout, expected — no action

---

### `FMCLoginRateLimitHits` (info)

**Severity:** info
**Metric:** `sum(rate(http_requests_total{path="/v1/auth/login", status="429"}[5m]))`
**Threshold:** > 10/s for 5m

**What it means:** The rate limiter (Sprint 14/22 follow-up) is hitting. This is expected behavior under attack but a useful signal.

**First response:** Confirm with FMCAuthFailureSpike. If both elevated, real attack. If only this, likely legitimate retry loop.

---

### `FMCNatsDisconnected` (warning)

**Severity:** warning
**Metric:** `nats_reconnects_total`
**Threshold:** > 5 reconnects in 5m

**What it means:** The worker is reconnecting to NATS frequently. Broker may be unstable.

**First response:**
1. Check `docker logs fmcg_nats`
2. Check network connectivity between worker and NATS
3. Check NATS cluster health (single-node in dev, multi-node in prod)

**Common causes:**
- NATS container restarted (OOM, crash)
- Network blip
- Misconfigured NATS cluster (Sprint 46: enable JetStream for durability)

---

## Production override

For production, override `deployments/alertmanager/alertmanager.yml` with real receivers:

```yaml
receivers:
  - name: 'ops'
    slack_configs:
      - api_url: https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
        channel: '#fmcg-ops-alerts'
        send_resolved: true
    email_configs:
      - to: 'oncall@fmcg-wallet.example.com'
        send_resolved: true

  - name: 'oncall'
    pagerduty_configs:
      - service_key: YOUR_INTEGRATION_KEY
        send_resolved: true
```

Then mount via docker-compose volume override or configmap in production.

---

## Sprint 59 follow-ups

- Sprint 59.1: add real Slack/PagerDuty receivers
- Sprint 59.2: per-tenant rate limits (some customers want different SLAs)
- Sprint 59.3: rate-limiter-specific alerts (current MultiTierLimiter doesn't expose metrics)
- Sprint 59.4: fraud flag spike alerts (Sprint 31 follow-up)

---

## References

- Sprint 59 commit: `8d2c4d1` (alert rules + Alertmanager + runbook)
- `deployments/prometheus/alerts.yml`: all alert definitions
- `deployments/alertmanager/alertmanager.yml`: routing + receivers
- `deployments/grafana/dashboards/system-health.json`: dashboard with alert overlays
- Prometheus docs: https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
- Alertmanager docs: https://prometheus.io/docs/alerting/latest/alertmanager/