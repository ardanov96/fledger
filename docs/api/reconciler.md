# Reconciler API

Trial balance verification + hash chain tamper detection. Runs as background job (every 1h) or on-demand via API.

---

## Endpoints

| Method | Path | RBAC Action |
|---|---|---|
| `POST` | `/v1/reconciler/run` | `reconciler:run` |
| `GET` | `/v1/reconciler/runs` | `reconciler:read` |
| `GET` | `/v1/reconciler/runs/{id}` | `reconciler:read` |
| `GET` | `/v1/reconciler/runs/{id}/accounts` | `reconciler:read` |

---

## Status precedence

| Condition | Status |
|---|---|
| Hash chain fails | `tampered` (highest) |
| SUM(debit) ≠ SUM(credit) | `imbalanced` |
| All checks pass | `balanced` |
| Unexpected error | `error` |

`tampered > imbalanced > balanced` — hash chain check failure overrides trial balance.

---

## `POST /v1/reconciler/run`

Manually trigger a reconciliation run. Returns immediately with the run ID; check status via GET.

### Request body
```json
{
  "tenant_id": "11111111-1111-1111-1111-111111111111",
  "period_id": "period_xyz",
  "run_hash_check": true
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `tenant_id` | UUID | ✅ | Tenant scope |
| `period_id` | UUID | ✅ | Period to reconcile |
| `run_hash_check` | bool | ❌ default false | If true, walks hash chain (slower but catches tampering) |

### Response 202 Accepted
```json
{
  "data": {
    "run_id": "run_abc123",
    "tenant_id": "...",
    "period_id": "period_xyz",
    "status": "running",
    "started_at": "2026-08-15T10:00:00Z",
    "triggered_by": "manual"
  }
}
```

### cURL example
```bash
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/reconciler/run \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "period_id": "period_xyz",
    "run_hash_check": true
  }'
```

---

## `GET /v1/reconciler/runs?tenant_id=...&limit=50`

List runs (most recent first).

### Response 200 OK
```json
{
  "data": [
    {
      "id": "run_abc123",
      "tenant_id": "...",
      "period_id": "period_xyz",
      "started_at": "2026-08-15T10:00:00Z",
      "finished_at": "2026-08-15T10:00:03Z",
      "status": "balanced",
      "total_debit_minor": 5000000000,
      "total_credit_minor": 5000000000,
      "imbalance_minor": 0,
      "hash_chain_ok": true,
      "hash_chain_errors": 0,
      "triggered_by": "manual"
    }
  ]
}
```

---

## `GET /v1/reconciler/runs/{id}/accounts`

Per-account breakdown for a specific run.

### Response 200 OK
```json
{
  "data": [
    {
      "account_id": "11111111-...",
      "debit_minor": 3000000000,
      "credit_minor": 1500000000,
      "signed_balance_minor": 1500000000,
      "entry_count": 8,
      "currency": "IDR"
    }
  ]
}
```

---

## Background worker

A ticker-based worker runs every **1 hour** by default (`reconcilerWorker.Interval = 1h`). It iterates all `open` and `closing` periods across all tenants.

**Hash chain check** is gated by `RECONCILER_HASH_CHECK=true` env var (default: false). Manual API runs always respect the per-request `run_hash_check` flag.

---

## Implementation

- `internal/usecase/reconciler_service.go` — `ReconcilerService`
- `internal/repository/postgres/reconciler_repo.go` — runs + account results persistence
- `internal/worker/reconciler_worker.go` — background ticker
- `internal/usecase/hashchain_verifier.go` — `Verifier.Verify(entries)` for tamper detection
- `migrations/000010_reconciler.up.sql` — schema
- [Sprint 10: Reconciler & Trial Balance](../SPRINTS.md#sprint-10--reconciler--trial-balance-fase-1b--2026-08-13)
