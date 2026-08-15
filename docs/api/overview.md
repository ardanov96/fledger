# API Overview

FMCG Wallet exposes a **RESTful HTTP/JSON API** at `/v1/*` for managing financial accounts, invoices, period closes, reconciliation, and field-force collections. All endpoints (except `/healthz`, `/readyz`, `/metrics`, and `/v1/auth/*`) require a valid JWT bearer token.

---

## Base URL

| Environment | URL |
|---|---|
| Production (Fly.io demo) | `https://fmcg-wallet-demo.fly.dev/v1` |
| Local development | `http://localhost:8080/v1` |

---

## Authentication

All authenticated endpoints require a JWT bearer token in the `Authorization` header:

```http
GET /v1/accounts HTTP/1.1
Host: fmcg-wallet-demo.fly.dev
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

**Obtain a token:** see [Authentication API](auth.md)

**Token lifecycle:**
- Access token TTL: **15 minutes** (configurable via `JWT_ACCESS_TTL`)
- Refresh token TTL: **168 hours** (7 days, configurable via `JWT_REFRESH_TTL`)
- Refresh rotation: every `/v1/auth/refresh` call mints a new pair and marks the old refresh as `rotated`
- Reuse detection: presenting a rotated refresh token revokes the entire token family

---

## Response envelope

All API responses use a consistent JSON envelope:

### Success
```json
{
  "data": { ... },
  "request_id": "01HXYZ...",
  "trace_id": "0af7651916cd43dd8448eb211c80319c"
}
```

### Error
```json
{
  "error": {
    "code": "INSUFFICIENT_BALANCE",
    "message": "Insufficient balance on account ...",
    "details": { "field": "amount_minor", "value": 1000 }
  },
  "request_id": "01HXYZ...",
  "trace_id": "0af7651916cd43dd8448eb211c80319c"
}
```

Every response includes:
- `request_id` — ULID for request correlation across logs
- `trace_id` — W3C `traceparent` 16-byte hex ID for distributed tracing

---

## HTTP status codes

| Code | Meaning | When |
|---|---|---|
| `200 OK` | Success (GET, PATCH) | Resource retrieved or updated |
| `201 Created` | Success (POST) | Resource created |
| `204 No Content` | Success (DELETE) | Resource deleted |
| `400 Bad Request` | Validation error | Invalid request body / query params |
| `401 Unauthorized` | Auth missing/invalid | Missing/expired/revoked token |
| `403 Forbidden` | RBAC denied | Authenticated but lacks permission |
| `404 Not Found` | Resource missing | ID does not exist |
| `409 Conflict` | State conflict | e.g., invoice already paid, period closed |
| `422 Unprocessable Entity` | Business rule violation | e.g., insufficient balance, credit limit exceeded |
| `429 Too Many Requests` | Rate limited | Token bucket exhausted (see `Retry-After`) |
| `500 Internal Server Error` | Unexpected | Should not happen; investigate via trace_id |

---

## Error code catalog

The `error.code` field uses a stable string identifier. See [`internal/platform/errors/errors.go`](https://github.com/runut/fmcg-wallet) for the full catalog. Common codes:

| Code | HTTP | Meaning |
|---|---|---|
| `INVALID_INPUT` | 400 | Generic input validation failure |
| `VALIDATION_FAILED` | 400 | Struct tag validation (e.g., missing required field) |
| `UNAUTHORIZED` | 401 | No token presented |
| `TOKEN_INVALID` | 401 | Token signature malformed |
| `TOKEN_EXPIRED` | 401 | Token past `exp` |
| `TOKEN_REUSE_DETECTED` | 401 | Refresh token replayed → family revoked |
| `FORBIDDEN` | 403 | RBAC denied |
| `NOT_FOUND` | 404 | Resource ID not found |
| `INVALID_STATE` | 409 | Lifecycle state violation |
| `IDEMPOTENCY_KEY_MISSING` | 400 | Required `Idempotency-Key` header not provided |
| `IDEMPOTENCY_CONFLICT` | 409 | Same key, different request body |
| `INSUFFICIENT_BALANCE` | 422 | Source account balance < transfer amount |
| `CREDIT_LIMIT_EXCEEDED` | 422 | Invoice creation exceeds customer credit cap |
| `PAYMENT_ALLOCATION_MISMATCH` | 422 | Manual allocation sum != payment amount |
| `CURRENCY_MISMATCH` | 422 | Transfer between incompatible accounts |
| `RATE_LIMITED` | 429 | Token bucket exhausted |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

---

## Rate limiting

| Scope | Limit | Configurable |
|---|---|---|
| Per-IP on `/v1/auth/login` | 5 burst, 0.5 rps sustained | `RATE_LIMIT_LOGIN_ENABLED=true` + burst/rps envs |
| Per-IP on `/v1/auth/refresh` | Inherits global limiter | (same) |
| Global (planned Sprint 14+) | Per-user + per-tenant token bucket | TBD |

When limited, response is HTTP 429 with `Retry-After` header (in seconds).

---

## Idempotency

`POST /v1/transfers` and `POST /v1/customers/{id}/payments` **require** an `Idempotency-Key` header:

```http
POST /v1/transfers HTTP/1.1
Idempotency-Key: my-unique-key-12345
Content-Type: application/json
```

Behavior:
- Same key + same body → return the original transaction (200 OK instead of 201)
- Same key + different body → 409 `IDEMPOTENCY_CONFLICT`
- Missing key → 400 `IDEMPOTENCY_KEY_MISSING`

Keys are scoped per (tenant, endpoint) and expire after 24 hours.

---

## Tenant context

Every authenticated request is scoped to a tenant via the `tenantctx` middleware (Sprint 15):

- `tenant_id` is extracted from the JWT `tenant_id` claim (NOT from a header — headers can be spoofed)
- Postgres RLS policies (migration `000014`) automatically filter `SELECT/INSERT/UPDATE/DELETE` to the requesting tenant
- `sales_rep` role additionally filters `collection_routes` to routes they own (`sales_rep_id = user_id`)

There is no need (and no way) to pass `tenant_id` in the request body — it's taken from the JWT.

---

## Versioning

The API uses **URL path versioning**: all endpoints are prefixed with `/v1`.

When breaking changes are required:
1. New endpoints go under `/v2/*`
2. `/v1/*` endpoints are maintained for at least 12 months after deprecation
3. Deprecation announced via `Deprecation` and `Sunset` response headers (RFC 8594)

Currently the project is at **v1** (single major version).

---

## Pagination

Pagination is **cursor-based** for list endpoints:

```http
GET /v1/invoices?customer_id=<uuid>&limit=50&cursor=<opaque-cursor>
```

Response includes:
```json
{
  "data": [...],
  "next_cursor": "<opaque-cursor>",  // null if no more pages
  "has_more": true
}
```

Default `limit=50`, max `limit=500`. Cursors are opaque — do not parse.

---

## Filtering & search

Most list endpoints support common filters:

| Endpoint | Filters |
|---|---|
| `GET /v1/accounts` | `type`, `status`, `limit`, `cursor` |
| `GET /v1/invoices` | `customer_id`, `status`, `aging_bucket`, `limit`, `cursor` |
| `GET /v1/customers/{id}/aging` | (none — derived from invoices) |
| `GET /v1/reconciler/runs` | `tenant_id`, `limit` |
| `GET /v1/routes` | `tenant_id`, `sales_rep_id`, `date` |

---

## Concurrency

For race-condition safety, the API implements:

| Mechanism | Where | Why |
|---|---|---|
| `Idempotency-Key` header | `POST /v1/transfers`, `POST /v1/customers/{id}/payments` | Safe client retries |
| `SELECT FOR UPDATE` (deterministic UUID-sorted order) | `TransferService` | Prevent deadlocks on same-pair transfers |
| Transaction-scoped GUC `app.current_tenant_id` | All `RunInTx*Domain` adapters | Postgres RLS filter |
| Hash chain (SHA-256) | `ledger_entries.prev_hash` | Tamper detection — modified row breaks chain |

See [ADR-0004: Locking strategy](../adr/0004-locking-strategy.md) for the full locking design rationale.

---

## Health & observability

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /healthz` | None | Liveness probe — returns `{"status":"alive"}` if process is running |
| `GET /readyz` | None | Readiness probe — checks Postgres connection |
| `GET /metrics` | None | Prometheus scrape target (counters, histograms) |
| `GET /v1/audit` | Bearer + RBAC `audit:read` | Tenant-scoped audit log |

---

## SDK / client libraries

**Official:** none yet (planned post-Sprint 20)

**Community:**
- OpenAPI spec auto-generated from handler signatures: `make openapi` (planned)
- Postman collection: `docs/postman/fmcg-wallet.json` (planned)

For now, use any HTTP client (curl, httpie, Postman, Insomnia, fetch, axios, etc).

---

## Related Documentation

| Document | Description |
|---|---|
| [Authentication API](auth.md) | Login, refresh, MFA flow |
| [Transfers API](transfers.md) | Double-entry ledger transfers |
| [Invoices API](invoices.md) | Customer receivables + FIFO payment |
| [Periods API](periods.md) | Two-step monthly close workflow |
| [Reconciler API](reconciler.md) | Trial balance + hash chain integration |
| [Collection API](collection.md) | Sales-rep field-force routes |
| [Currencies API](currencies.md) | ISO 4217 + FX rate snapshots |
| [Audit API](audit.md) | Tamper-evident audit log |
| [Architecture Overview](../architecture/overview.md) | System context |
| [C4 Diagrams](../architecture/c4-diagrams.md) | Container / Component views |
| [Sequence Flows](../architecture/sequences.md) | Critical user journeys |
| [ADRs](../adr/index.md) | Key architectural decisions |
