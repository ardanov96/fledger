# Observability Runbook

**Goal:** Understand what each observability signal means, how to query it, and how to debug common issues.

---

## Signals overview

| Signal | Endpoint | Format | Retention |
|---|---|---|---|
| **Metrics** | `GET /metrics` | Prometheus exposition | 30 days (in demo) |
| **Logs** | stdout (structured JSON) | JSON | 7 days (Loki demo) |
| **Traces** | W3C `traceparent` header | hex | 48 hours (Tempo) |
| **Health** | `GET /healthz` + `/readyz` | JSON | N/A |

---

## 1. `/healthz` — Liveness probe

Always returns 200 if process is alive. Used by Fly.io / Kubernetes to know when to restart container.

```bash
curl https://fmcg-wallet-demo.fly.dev/healthz
```

**Response:**
```json
{"status":"alive","uptime":"5h23m"}
```

**When it fails:** container is hung or crashed. Fly.io will auto-restart.

---

## 2. `/readyz` — Readiness probe

Returns 200 only if app is ready to serve traffic (DB connection OK, dependencies OK).

```bash
curl https://fmcg-wallet-demo.fly.dev/readyz
```

**Response (ready):**
```json
{
  "status": "ready",
  "checks": {"postgres": "up"}
}
```

**Response (not ready):**
```json
{
  "status": "not_ready",
  "checks": {"postgres": "DOWN: connection refused"}
}
```

Returns HTTP 503 when not ready. Load balancer should stop sending traffic until ready.

---

## 3. `/metrics` — Prometheus metrics

```bash
curl https://fmcg-wallet-demo.fly.dev/metrics
```

Returns Prometheus exposition format. Key metrics:

### HTTP request metrics

```
# Total HTTP requests by status code
http_requests_total{method="POST",path="/v1/transfers",status="201"} 1234
http_requests_total{method="POST",path="/v1/transfers",status="422"} 5

# Request duration histogram (seconds)
http_request_duration_seconds_bucket{path="/v1/transfers",le="0.1"} 800
http_request_duration_seconds_bucket{path="/v1/transfers",le="0.5"} 1200
http_request_duration_seconds_bucket{path="/v1/transfers",le="1.0"} 1230

# In-flight requests (gauge)
http_requests_in_flight 12
```

### Business metrics

```
# Total transfers
transfers_total{tenant_id="..."} 1500

# Total reconciler runs (status)
reconciler_runs_total{status="balanced"} 100
reconciler_runs_total{status="imbalanced"} 2
reconciler_runs_total{status="tampered"} 0

# Total invoices by status
invoices_total{status="open"} 50
invoices_total{status="paid"} 200
invoices_total{status="overdue"} 5

# Total collection routes
routes_total{status="completed"} 30
```

### Go runtime metrics

```
# Goroutines, GC pause, memory
go_goroutines 25
go_gc_duration_seconds_sum 0.05
go_memstats_alloc_bytes 52428800
```

### Postgres metrics (planned — Sprint 18+)

```
pg_stat_activity_count 8
pg_locks_count 0
pg_stat_database_tup_fetched_total 123456
```

---

## 4. Structured JSON logs

All logs are emitted as JSON to stdout. Promtail (in observability stack) ships them to Loki.

### Log structure

```json
{
  "time": "2026-08-15T10:00:00.123456789Z",
  "level": "INFO",
  "msg": "http request",
  "method": "POST",
  "path": "/v1/transfers",
  "status": 201,
  "bytes": 234,
  "duration_ms": 23,
  "request_id": "01HXYZ...",
  "trace_id": "0af7651916cd43dd8448eb211c80319c",
  "remote_addr": "203.0.113.42"
}
```

### Key fields for correlation

| Field | Purpose | Set by |
|---|---|---|
| `request_id` | ULID, unique per HTTP request | `httpx.RequestIDMiddleware` |
| `trace_id` | W3C `traceparent` 16-byte hex, propagates to downstream calls | `middleware.TraceMiddleware` |
| `user_id` | From JWT `sub` claim (when authenticated) | (planned) |
| `tenant_id` | From JWT `tenant_id` claim (when authenticated) | (planned) |

### Querying logs (Loki LogQL)

```logql
# All logs for a specific request_id
{app="fmcg-wallet-api"} |= "01HXYZ..."

# All errors in last 1h
{app="fmcg-wallet-api"} | json | level="ERROR" | line_format "{{.msg}}"

# All requests taking >1s
{app="fmcg-wallet-api"} | json | duration_ms > 1000

# All failed login attempts
{app="fmcg-wallet-api"} | json | msg="login failed" | line_format "{{.remote_addr}}"
```

---

## 5. W3C Distributed tracing

Each HTTP request generates a `trace_id` (32 hex chars). For inter-service tracing (planned Sprint 18+), spans are linked via `traceparent` header:

```
traceparent: 00-<trace_id>-<span_id>-<flags>
```

Example: `00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01`

### Propagation

Currently propagated to **logs only** (via `trace_id` field). Full OTel SDK + Tempo exporter is deferred to future sprint.

When OTel is integrated, traces will be exportable in Tempo (UI: `http://localhost:3200` in local dev stack).

---

## 6. Common debugging queries

### Find a specific request by ID

```bash
# Get request_id from client (returned in response header X-Request-ID)
# Then query:
curl -s "http://loki:3100/loki/api/v1/query?query={app=\"fmcg-wallet-api\"}%20%7C%3D%20%2201HXYZ%22" \
  -H "X-Scope-OrgID: fmcg" | jq
```

### Identify slow endpoints

```logql
# Top 10 slowest endpoints (p99) in last 1h
{app="fmcg-wallet-api"} | json | __error__="" | line_format "{{.path}}: {{.duration_ms}}ms"
```

### Find tenant with high error rate

```logql
{app="fmcg-wallet-api"} | json | status >= 500
```

---

## 7. Alerting (planned)

Production would have alerts on:

| Condition | Severity | Action |
|---|---|---|
| `/readyz` returns 503 for >1 min | Sev 1 | Page on-call |
| Error rate >5% over 5 min | Sev 2 | Slack alert |
| p99 latency >2s over 10 min | Sev 3 | Slack warning |
| Disk usage >80% | Sev 3 | Slack warning |
| Reconciler finds `tampered` status | Sev 1 | Page on-call (potential DB compromise) |
| Token reuse detected spike | Sev 2 | Slack (potential credential stuffing) |

---

## 8. Tools integration

### Grafana dashboards

Pre-built dashboard: `deployments/grafana/dashboards/system-health.json`

Includes:
- HTTP request rate by endpoint (Prometheus)
- p50/p95/p99 latency by endpoint
- Error rate (4xx + 5xx)
- Reconciler status distribution
- Database connection pool usage
- Go runtime (goroutines, memory, GC pause)

### Prometheus queries

```promql
# Request rate per second by endpoint
sum by (path) (rate(http_requests_total[5m]))

# 95th percentile latency
histogram_quantile(0.95, sum by (path, le) (rate(http_request_duration_seconds_bucket[5m])))

# Error rate %
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
```

---

## 9. Local development setup

```bash
# Start full observability stack (TIG)
docker compose up prometheus loki promtail tempo grafana

# Query logs at http://localhost:3100
# Query metrics at http://localhost:9090
# Visualize at http://localhost:3000 (admin/admin)
```

---

## Related Documentation

- [Architecture Overview](../architecture/overview.md) — observability stack in diagram
- [Architecture: Sequences](../architecture/sequences.md) — request lifecycle
- [Sprint 18: Observability finishing touches + Load test](../../SPRINTS.md#sprint-18--observability-finishing-touches--load-test-foundation-fase-3b--2026-08-15)
- [Load Testing Runbook](load-test.md) — capacity planning
- [Incident Response Runbook](incident-response.md) — what to do when alerts fire
