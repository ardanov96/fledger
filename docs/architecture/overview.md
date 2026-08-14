# Architecture Overview

High-level view of the FMCG Wallet system architecture, designed per the [C4 model](https://c4model.com/).

For more detail:
- [C4 Diagrams](c4-diagrams.md) — Context / Container / Component views
- [Sequence Flows](sequences.md) — critical user journeys
- [Domain Glossary](../domain/glossary.md) — domain terms
- [ADRs](../adr/index.md) — key architectural decisions

---

## Why this architecture?

FMCG Wallet serves Indonesian FMCG/F&B distributors that need:

| Need | Solution |
|---|---|
| Audit-ready financial records | Double-entry ledger with hash chain tamper detection |
| Real-time visibility | REST API + Postgres views for HQ finance dashboard |
| Field-force collections | Sales-rep workflow (PlanRoute → Visit → Settle) with audit trail |
| Multi-tenancy | Per-tenant data isolation via Postgres RLS |
| Compliance-friendly | Immutable entries, MFA, brute-force protection, WORM-style audit log |

---

## Layered architecture

The codebase follows a **clean-lite** architecture with 4 layers:

```mermaid
flowchart TB
    Client["HTTP Client<br/>(curl / browser / mobile)"]
    subgraph Edge["Edge / Transport"]
        Handler["Handler Layer<br/>(chi routes + DTOs)"]
        Middleware["Middleware<br/>(Auth + RBAC + Trace + RateLimit)"]
    end
    subgraph App["Application / Use Case"]
        Service["Service Layer<br/>(TransferService, InvoiceService,<br/>PeriodService, ReconcilerService,<br/>CollectionService, CurrencyService,<br/>AuthService)"]
    end
    subgraph Domain["Domain Layer"]
        Domain["Pure entities + interfaces<br/>(Ledger, Invoice, Period,<br/>Reconciler, Collection, Currency, Auth)"]
    end
    subgraph Infra["Infrastructure Layer"]
        Repo["Postgres Repositories<br/>(pgx + sqlc)"]
        Crypto["Crypto helpers<br/>(bcrypt + TOTP + SHA-256)"]
        Token["Token helpers<br/>(JWT Signer + Refresh tokens)"]
    end
    DB[("PostgreSQL 16<br/>+ RLS policies")]

    Client -->|HTTPS + Bearer JWT| Middleware
    Middleware --> Handler
    Handler -->|thin mapping| Service
    Service -->|orchestrates| Domain
    Service --> Repo
    Service --> Crypto
    Service --> Token
    Repo --> DB
```

**Key principle:** domain layer has zero infrastructure imports. Use cases orchestrate domain logic via interfaces implemented by infrastructure adapters.

---

## Module boundaries

The system has 9 cohesive domain modules, each owning its own:

| Module | Owns | Sprint |
|---|---|---|
| **Ledger** | Accounts, transactions, entries (double-entry invariant) | 2-4 |
| **Invoice + Credit** | Customer invoices, payments, FIFO allocation, credit limits | 8 |
| **Period Close** | Two-step approval workflow + period snapshots | 9 |
| **Reconciler** | Background trial-balance check + hash chain integration | 10 |
| **Collection Routes** | Sales-rep field workflow (plan → visit → settle) | 11 |
| **Currency / FX** | ISO 4217 registry + rate snapshot per transaction | 12 |
| **Auth** | Refresh-token rotation + TOTP MFA + brute-force lockout | 13 |
| **Rate Limit** | Token-bucket per-IP middleware | 14 |
| **Tenant RLS** | Postgres row-level security + sales-rep scope | 15 |

All modules share:

- `internal/platform/money` — int64 minor-units arithmetic (decimal precision)
- `internal/platform/httpx` — response envelope + request_id middleware
- `internal/platform/errors` — typed `AppError` catalog (stable HTTP codes)
- `internal/middleware` — auth, RBAC, trace, rate-limit
- `internal/repository/postgres` — pgx adapters for each domain

---

## Concurrency model

| Mechanism | Where | Purpose |
|---|---|---|
| `SELECT FOR UPDATE` | `LockPairForUpdate` (Sprint 7) | Deterministic lock ordering for transfer accounts (prevents deadlocks) |
| Transaction-scoped GUC | `tenantctx.SetTenantContext` (Sprint 15) | RLS policy sees per-request tenant |
| Opaque refresh tokens | `auth.RefreshToken` (Sprint 13) | Server can revoke; reuse-detection on rotate |
| Idempotency-Key | `transfer` + `payment` (Sprint 2, 8) | Safe client retries |
| Hash chain (SHA-256) | `ledger_entries.prev_hash` (Sprint 8) | Tamper detection — modified row breaks chain |

---

## Observability

| Signal | Tool | Where |
|---|---|---|
| Metrics (Prometheus) | `/metrics` endpoint | Counters: requests_total, transfers_total, reconcile_failures_total |
| Logs (structured JSON) | log/slog → Loki via Promtail | All requests with request_id + trace_id |
| Traces (W3C traceparent) | TraceMiddleware → Tempo | HTTP → service → repo call chain |
| Health | `/healthz` + `/readyz` | k8s liveness/readiness probes |

---

## Failure modes & responses

| Failure | Response |
|---|---|
| Postgres down | `/readyz` returns 503; API returns `INTERNAL_ERROR` |
| Redis down | Future asynq workers fail; current setup doesn't depend on Redis at runtime |
| Bad input | HTTP 400 `VALIDATION_FAILED` with field details |
| Insufficient balance | HTTP 422 `INSUFFICIENT_BALANCE` |
| Account locked | HTTP 403 `ACCOUNT_LOCKED` |
| Invalid token | HTTP 401 `TOKEN_INVALID` / `TOKEN_EXPIRED` |
| Reuse detected | HTTP 401 `REFRESH_TOKEN_REUSE` (whole family revoked) |
| Rate limited | HTTP 429 `RATE_LIMITED` with `Retry-After: 1` |

See [Runbook: Incident Response](runbooks/incident-response.md) for the full incident flow.
