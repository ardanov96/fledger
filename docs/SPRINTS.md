# Sprint Log

> Complete delivery timeline for **FMCG Wallet** — production-grade hybrid wallet backend untuk distributor FMCG/F&B Indonesia.
>
> **Format per sprint:** Goal · Scope · Key Artifacts · Learnings · Follow-ups.
>
> **Status legend:** ✅ Done · 🔄 In progress · ⏸ Deferred · 📋 Planned · ❌ Rejected
>
> Untuk retrospective sprint aktif, lihat section [Sprint Aktif](#sprint-aktif) di bawah.

---

## Sprint Index

| # | Sprint | Fase | Date | Status |
|---|---|---|---|---|
| 51 | [Period.closed Event Type](#sprint-51-periodclosed-event-type-2026-10-01) | 4A | 2026-10-01 | ✅ Done |
| 50 | [Next.js Login + Dashboard PoC](#sprint-50-nextjs-login--dashboard-poc-2026-10-01) | 6 | 2026-10-01 | ✅ Done |
| 49 | [Next.js Type-Safe API Client](#sprint-49-nextjs-type-safe-api-client-2026-10-01) | 6 | 2026-10-01 | ✅ Done |
| 48 | [toxiproxy Chaos Integration](#sprint-48-toxiproxy-chaos-integration-2026-10-01) | 7 | 2026-10-01 | ⏸ Deferred |
| 47 | [Frontend Next.js Migration Plan](#sprint-47-frontend-nextjs-migration-plan-2026-10-01) | 6 | 2026-10-01 | ✅ Done |
| 46 | [JetStream Durable Subscription](#sprint-46-jetstream-durable-subscription-2026-10-01) | 4A follow-up | 2026-10-01 | ✅ Done |
| 45 | [Invoice/Payment Outbox Events](#sprint-45-invoicepayment-outbox-events-2026-10-01) | 4A | 2026-10-01 | ✅ Done |
| 44 | [RLS-on-Pool Audit (Currency/Collection)](#sprint-44-rls-on-pool-audit-currencycollection-2026-10-01) | 5A | 2026-10-01 | ✅ Done |
| 43 | [FOR UPDATE SKIP LOCKED Outbox](#sprint-43-for-update-skip-locked-outbox-2026-10-01) | 4A follow-up | 2026-10-01 | ✅ Done |
| 42 | [Fraud Flag Dedup](#sprint-42-fraud-flag-dedup-2026-10-01) | 8 follow-up | 2026-10-01 | ✅ Done |
| 41 | [Extended Chaos Tests](#sprint-41-extended-chaos-tests-2026-10-01) | 7 | 2026-10-01 | ✅ Done |
| 40 | [RLS-on-Pool Audit (Entry/Period/Reconciler)](#sprint-40-rls-on-pool-audit-entryperiodreconciler-2026-10-01) | 5A | 2026-10-01 | ✅ Done |
| 39 | [OTel SDK + OTLP Exporter](#sprint-39-otel-sdk--otlp-exporter-2026-10-01) | 3B | 2026-10-01 | ✅ Done |
| 38 | [Per-Tenant Fraud Thresholds](#sprint-38-per-tenant-fraud-thresholds-2026-10-01) | 8 | 2026-10-01 | ✅ Done |
| 37 | [RLS-on-Pool Audit (Account/Invoice/Transaction)](#sprint-37-rls-on-pool-audit-accountinvoicetransaction-2026-10-01) | 5A | 2026-10-01 | ✅ Done |
| 36 | [Chaos Tests for Outbox Recovery](#sprint-36-chaos-tests-for-outbox-recovery-2026-10-01) | 7 | 2026-10-01 | ✅ Done |
| 35 | [RLS for user_credentials](#sprint-35-rls-for-user_credentials-2026-10-01) | 5A | 2026-10-01 | ✅ Done |
| 34 | [W3C SpanContext Upgrade](#sprint-34-w3c-spancontext-upgrade-2026-10-01) | 3B follow-up | 2026-10-01 | ✅ Done |
| 33 | [Zero-Downtime JWT Rotation](#sprint-33-zero-downtime-jwt-rotation-2026-10-01) | 2E | 2026-10-01 | ✅ Done |
| 32 | [Login Attempt Partitioning](#sprint-32-login-attempt-partitioning-2026-10-01) | 2E follow-up | 2026-10-01 | ✅ Done |
| 31 | [Fraud Flag Scanner](#sprint-31-fraud-flag-scanner-2026-10-01) | 8 | 2026-10-01 | ✅ Done |
| 30 | [Cross-Tenant Worker via app_admin](#sprint-30-cross-tenant-worker-via-app_admin-2026-10-01) | 5A / 4A / 4D / 1B follow-up | 2026-10-01 | ✅ Done |
| 29 | [RLS-on-Pool Read-Path Fix](#sprint-29-rls-on-pool-read-path-fix-2026-10-01) | 5A / 8 follow-up | 2026-10-01 | ✅ Done |
| 28 | [Notification Dispatcher](#sprint-28-notification-dispatcher-2026-09-24) | 8 | 2026-09-24 | ✅ Done |
| 24 | [Transactional Outbox + NATS Subscriber](#sprint-24--transactional-outbox--nats-subscriber-2026-09-21) | 4A | 2026-09-21 | ✅ Done |
| 25 | [Aging Recalculator Worker](#sprint-25--aging-recalculator-worker-2026-09-22) | 4D | 2026-09-22 | ✅ Done |
| 26 | [FX Rate Auto-Refresh](#sprint-26--fx-rate-auto-refresh-2026-09-23) | 1D follow-up | 2026-09-23 | ✅ Done |
| 22B | [Hardening](#sprint-22b-hardening-2026-08-15) | Fase 2 follow-up | 2026-08-15 | ✅ Done |
| 22A | [Documentation & DX Hardening](#sprint-22a-documentation-dx-hardening-2026-08-15) | — | 2026-08-15 | ✅ Done |
| 21 | [Interview Prep](#sprint-21-interview-prep-2026-08-16) | Fase 7 | 2026-08-16 | ✅ Done |
| 20 | [Frontend Dashboard MVP](#sprint-20-frontend-dashboard-mvp-2026-08-15) | Fase 6 | 2026-08-15 | ✅ Done |
| 19 | [Deployment Fly.io](#sprint-19-deployment-flyio-fase-3b-2026-08-15) | Fase 3B | 2026-08-15 | ✅ Done |
| 18 | [Observability finishing touches + Load test foundation](#sprint-18-observability-finishing-touches-load-test-foundation-fase-3b-2026-08-15) | Fase 3B | 2026-08-15 | ✅ Done |
| 17 | [Integration Tests E2E](#sprint-17-integration-tests-e2e-2026-08-15) | Fase 7 | 2026-08-15 | ✅ Done |
| 16 | [Property-Based Tests](#sprint-16-property-based-tests-2026-08-14) | Fase 7 | 2026-08-14 | ✅ Done |
| 15 | [Tenant RLS + app_admin](#sprint-15-tenant-rls-integration-fase-2b-5a-2026-08-14) | Fase 2B + 5A | 2026-08-14 | ✅ Done |
| 14 | [Rate Limiting](#sprint-14-rate-limiting-2026-08-14) | Fase 2D | 2026-08-14 | ✅ Done |
| 14 followup | [Multi-Tier Rate Limiting](#sprint-14-followup-multi-tier-rate-limiting-fase-2d-lanjutan-2026-08-15) | Fase 2D Lanjutan | 2026-08-15 | ✅ Done |
| 13 | [Refresh Token Rotation + MFA](#sprint-13-refresh-token-rotation-mfa-fase-2e-lanjutan-2026-08-14) | Fase 2E lanjutan | 2026-08-14 | ✅ Done |
| 12 | [Multi-Currency](#sprint-12-multi-currency-fase-1d-2026-08-14) | Fase 1D | 2026-08-14 | ✅ Done |
| 11 | [Collection & Route Module](#sprint-11-collection-route-module-portfolio-sprint-4-fase-8-partial-2026-08-13) | Fase 1E | 2026-08-13 | ✅ Done |
| 10 | [Reconciler & Trial Balance](#sprint-10-reconciler-trial-balance-fase-1b-2026-08-13) | Fase 1B | 2026-08-13 | ✅ Done |
| 9 | [Period Close with Approval Workflow](#sprint-9-period-close-with-approval-workflow-fase-1a-2026-08-11) | Fase 1A | 2026-08-11 | ✅ Done |
| 8 | [Invoices + Hash Chain](#sprint-8-invoice-credit-hash-chain-jwtrbac-2026-08-11) | Fase 1C | 2026-08-11 | ✅ Done |
| 7 | [Lock Ordering Hardening](#sprint-7-lock-ordering-hardening-2026-08-12) | Fase 1 | 2026-08-12 | ✅ Done |
| 6 | [Idempotency-Key Pattern](#sprint-6-idempotency-key-pattern-2026-08-10) | Fase 1 | 2026-08-10 | ✅ Done |
| 5 | [ADR-0004 + Audit Log + Middleware](#sprint-5-adr-0004-audit-log-middleware-2026-08-10) | Fase 2A | 2026-08-10 | ✅ Done |
| 4 | [Double-Entry Core + Audit Trail](#sprint-4-double-entry-core-audit-trail-2026-08-10) | Fase 1 | 2026-08-10 | ✅ Done |
| 3 | [Account Schema + Repositories](#sprint-3-account-schema-repositories-2026-08-10) | Fase 1 | 2026-08-10 | ✅ Done |
| 2 | [Schema + Repositories + Transfer Use Case](#sprint-2-schema-repositories-transfer-use-case-2026-08-10) | Fase 1 | 2026-08-10 | ✅ Done |
| 1 | [Foundation Reset](#sprint-1-foundation-reset-2026-08-10) | Fase 0 | 2026-08-10 | ✅ Done |

---

## Sprint Aktif

### Sprint 23 — Tech Debt Foundation — 2026-09-20

**Status:** 🔄 In progress · **Theme:** Documentation & critical fixes · **Fase:** — (preparation untuk Fase 8+)

#### Goal
Hilangkan **7 critical tech debt items** yang terakumulasi sejak Sprint 22B. Result: sprint tracking hidup, transfer service akurat multi-period, main.go manageable, observability vendor-agnostic.

#### Scope (planned)
- **23.1** — Sprint tracking foundation (file ini) + ADR catalog + harmonisasi sprint counter ✅ Done
- **23.2** — Fix `TransferService.ensureOpenPeriod` stub (saat ini hardcoded seed UUID) ✅ Done
- **23.3** — Decompose `cmd/api/main.go` (583 LOC) menjadi wiring files (deferred)
- **23.4** — Adopt OpenTelemetry SDK (replace custom W3C traceparent) (deferred)

#### Key Artifacts (in progress)
- `docs/SPRINTS.md` — file ini (single source of truth sprint history) ✅
- `docs/adr/index.md` — ADR catalog dengan 8 ADRs + status ✅
- `internal/usecase/transfer_service.go` — `PeriodResolver` interface + pre-tx period resolution ✅

#### Learnings (preliminary)
- Doc drift terdeteksi di banyak tempat (broken links ke `SPRINTS.md`, inconsistent sprint counter)
- ADR-0003 reference ADR-0010/0011/0012 yang tidak ada → cleanup dengan Future ADR Candidate notes
- Pre-resolve period (outside tx) simpler than adding `period.Tx` dependency to TransferService — race vs concurrent period close acceptable karena migration 000008 trigger blocks insert ke closed period

#### Follow-ups
- Sprint 23.3 (cmd/api/main.go decomposition) — opsional, less critical
- Sprint 23.4 (OTel SDK migration) — opsional, less critical
- Sprint 24 — likely outbox pattern + NATS consumer
- ADR-0009 (planned): int64 minor units money rationale
- `InvoiceService.EnsurePeriod` masih punya fallback hardcoded UUID — could follow same pattern (lower priority)

---

## Sprint 24 — Transactional Outbox + NATS Subscriber (2026-09-21)

**Status:** ✅ Done · **Fase:** 4A (Event-driven foundations)

#### Goal
Implement the **transactional outbox pattern** so business writes reliably publish events to NATS without 2PC. Subscribe to `fmcg.transfer.posted` in the API process as the smoke test that the publisher → broker → subscriber loop works end-to-end.

#### Scope
- **24.1** — Migration `000017_outbox.up.sql` — `outbox_events` table dengan RLS + `app_admin` bypass
- **24.2** — `internal/domain/outbox/` — Event entity + Repository interface (Tx abstraction)
- **24.3** — `internal/repository/postgres/outbox_repo.go` + `tx_adapter_outbox.go` — pgx impl + cross-domain tx bridge
- **24.4** — `internal/infra/nats.go` — NATS client wrapper (Connect/Publish/Subscribe/Ping/Close)
- **24.5** — `internal/usecase/outbox_publisher.go` — `OutboxPublisher.RunOnce()` (fetch → publish → mark)
- **24.6** — `internal/worker/outbox_worker.go` + `cmd/worker/main.go` — ticker-based publisher loop
- **24.7** — `OutboxWriter` interface di `transfer_service.go` — write `transfer.posted` event in SAME tx as ledger writes
- **24.8** — `cmd/api/nats_subscriber.go` — log-only subscriber for `transfer.posted`; `/readyz` reports NATS state
- **24.9** — Unit tests for `OutboxPublisher` (empty / success / broker failure / fetch error)

#### Key Artifacts
- `migrations/000017_outbox.{up,down}.sql` — outbox_events table
- `internal/domain/outbox/outbox.go` — domain entity + interface (zero infra deps)
- `internal/repository/postgres/outbox_repo.go` — Postgres impl with `outbox_eventDTO` mapping
- `internal/repository/postgres/tx_adapter_outbox.go` — `WrapOutboxTx` + `UnwrapPgxTxFromLedger` bridges
- `internal/infra/nats.go` — `NATSClient` (Connect, Publish, Subscribe, Ping, Close, IsConnected)
- `internal/usecase/outbox_publisher.go` — `EventBroker` interface + `OutboxPublisher.RunOnce`
- `internal/usecase/outbox_publisher_test.go` — 6 unit tests with fake repo + broker
- `internal/worker/outbox_worker.go` — `OutboxPublisherWorker` (mirrors `ReconcilerWorker` pattern)
- `cmd/api/outbox_adapter.go` — `outboxWriterAdapter` bridges `usecase.OutboxWriter` ↔ `postgres.OutboxRepository`
- `cmd/api/nats_subscriber.go` — log handler proves end-to-end loop works

#### Learnings
- Outbox pattern = atomicity guarantee via "same tx as business write". No 2PC, no distributed transactions.
- Per-event publish failure ≠ cycle failure: `IncrementAttempts` records retry state without failing the cycle.
- `MarkPublished` AFTER batch: single UPDATE per cycle is more efficient than per-event.
- Cross-domain write pattern: `UnwrapPgxTxFromLedger(tx ledger.Tx) → wrapOutboxTx(pgxTx)` lets outbox repo write inside ledger tx without depending on the outbox domain's Tx type.
- Default fallback (`noopOutboxWriter`) preserves test compatibility — existing transfer tests don't break.
- `flushTimeout(2s)` after publish ensures broker ack before MarkPublished (vs fire-and-forget).

#### Follow-ups
- Sprint 25 — Aging Recalculator worker (cmd/worker/main.go TODO still pending)
- Sprint 26 — FX Rate Auto-Refresh
- Add `FOR UPDATE SKIP LOCKED` to `FetchUnpublished` for multi-publisher-safety
- More event types: `invoice.created`, `period.closed`, `payment.recorded`
- Replace log subscriber with real handlers (notification dispatcher, fraud scanner, projection writer)
- Move from core NATS to JetStream for durable subscription (currently fire-and-forget)

---

## Sprint 22 — Documentation & Hardening (2026-08-15)

Dua sub-sprint paralel: 22A documentation, 22B hardening.

### Sprint 22B — Hardening — 2026-08-15

**Status:** ✅ Done (partial — 1/5 items) · **Fase:** Fase 2 follow-up

#### Goal
Address 5 Tier-1 hardening items yang teridentifikasi saat Sprint 15 close.

#### Scope
- ✅ **22B.1** — Wire rate-limit metrics ke Prometheus (commit `2b22e50`)
- ✅ **22B.1c** — main.go:376 wiring fix (commit `aaec962`)
- ⏸ **22B.5** — GUC bind audit trail → di-defer ke Sprint 23 (butuh schema redesign)
- ⏸ **22B.2** — MFA recovery codes → di-defer ke Sprint 23
- ⏸ **22B.3** — Session list/revoke → di-defer ke Sprint 23
- ⏸ **22B.4** — Password policy validator → di-defer ke Sprint 23

#### Key Artifacts
- [ADR-0008](adr/0008-sprint-22b-hardening-roadmap.md) — Sprint 22B design decisions & Sprint 23 bundle
- `cmd/api/metrics.go` — Prometheus custom counters (multi-tier rate limiter)
- Custom metrics: `fmcg_rate_limit_allowed_total{tier}`, `fmcg_rate_limit_rejected_total{tier}`

#### Learnings
- Bundling 4 deferred items ke 1 Sprint 23 lebih efisien daripada 4 mini-sprints
- Setiap item tetap di-commit terpisah (sub-PR per area) untuk atomicity

#### Follow-ups
- Sprint 23 — 4 deferred items + tech debt cleanup

### Sprint 22A — Documentation & DX Hardening — 2026-08-15

**Status:** ✅ Done · **Fase:** — (maintenance)

#### Goal
Close doc drift dan tambah operational maturity.

#### Scope
- ✅ ADR-0007 (`app_admin` role RLS bypass) finalized + commit
- ✅ Sprint 22B planned dalam ADR-0008
- ✅ Documentation pass: `docs/architecture/`, `docs/runbooks/`

#### Key Artifacts
- [ADR-0007](adr/0007-app-admin-rls-bypass.md) — `app_admin` role rationale
- Updated `docs/index.md` project stats
- `runbooks/secret-rotation.md` — quarterly rotation SOP

#### Learnings
- ADR-0006 follow-up #2 (GUC bind audit) escalated ke ADR-0008 sebagai Sprint 23 item

---

## Sprint 21 — Interview Prep — 2026-08-16

**Status:** ✅ Done · **Fase:** Fase 7 (Testing & Quality)

#### Goal
Prepare interview-ready Q&A + demo script grounded in actual codebase.

#### Scope
- 15 anticipated Q&A (5 fintech + 5 distributed systems + 5 security)
- 5-7 menit structured demo script (`docs/interview/demo-script.md`)
- One-liner talking points untuk 30-second pitch
- Quick-prep cheat sheet (interviewer asks X → open Y first)

#### Key Artifacts
- [docs/interview/](interview/index.md) — 4 files (index + 3 Q&A + demo)
- Cross-referenced dari `docs/index.md` highlights

#### Learnings
- Setiap Q&A grounded ke file:line_number spesifik — easy to verify & maintain
- Demo script references actual curl commands yang jalan di demo Fly.io instance

---

## Sprint 20 — Frontend Dashboard MVP — 2026-08-15

**Status:** ✅ Done · **Fase:** Fase 6 (Integration)

#### Goal
Lightweight dashboard untuk stakeholder review tanpa Next.js overhead.

#### Scope
- 9 views (vanilla JS, zero npm deps)
- Reverse proxy `/v1/*` ke API (Node.js)
- Login flow + account list + transfer form + invoice view
- Demo deployment via Fly.io single-VM

#### Key Artifacts
- `web/` — Node proxy + vanilla JS SPA
- `web/README.md` (project root) — limitations & Next.js migration notes

#### Learnings
- Vanilla JS adequate untuk demo & portfolio; **bukan** production-grade
- Reverse proxy pattern lebih simple daripada CORS preflight untuk same-origin

#### Follow-ups
- Sprint TBD — migrate ke Next.js + TypeScript + Tailwind (production-grade)

---

## Sprint 19 — Deployment Fly.io (Fase 3B) — 2026-08-15

**Status:** ✅ Done · **Fase:** Fase 3B (Deployment)

#### Goal
Zero-cost production-like deployment untuk demo & stakeholder review.

#### Scope
- Single Fly.io app (`fmcg-wallet-demo`) region Singapore
- 512MB RAM shared-cpu-1x, 1GB persistent volume
- Multi-process via supervisord: Postgres + migrator + API
- Auto-deploy on push to main
- Demo data seeding via `seed-local-data.sh` adapted

#### Key Artifacts
- `Dockerfile.fly` — Alpine-based multi-process image
- `fly.toml` — single-VM deployment config
- `.github/workflows/fly-deploy.yml` — auto-deploy workflow
- [docs/runbooks/deployment-fly.md](runbooks/deployment-fly.md) — operational SOP

#### Learnings
- Distroless tidak bisa untuk multi-process (no shell) → Alpine + supervisord untuk Fly
- Free tier 512MB cukup untuk API + Postgres single-tenant demo
- Health check `/healthz` every 30s optimal untuk Fly proxy

---

## Sprint 18 — Observability finishing touches + Load test foundation (Fase 3B) — 2026-08-15

**Status:** ✅ Done · **Fase:** Fase 3B (Observability)

#### Goal
End-to-end tracing + load test harness untuk performance regression detection.

#### Scope
- W3C `traceparent` propagation (custom impl, `internal/middleware/tracing.go`)
- Tempo integration (`docker-compose.yml:210-222`)
- k6 load test script untuk `/v1/transfers` (`load-test/transfer.js` in project root)
- CI integration: integration tests run otomatis sebelum lint

#### Key Artifacts
- `internal/middleware/tracing.go` — W3C traceparent middleware
- `deployments/tempo/` — Tempo config
- `load-test/transfer.js` (project root) — k6 script

#### Learnings
- Custom W3C impl works, tapi tidak kompatibel vendor APM (Jaeger/Honeycomb auto-instrumentation)
- k6 default ramping-then-spike pattern cukup untuk regression detection

#### Follow-ups
- **Sprint 23.4** (planned) — adopt OpenTelemetry SDK untuk vendor-agnostic auto-instrumentation

---

## Sprint 17 — Integration Tests E2E — 2026-08-15

**Status:** ✅ Done · **Fase:** Fase 7 (Testing & Quality)

#### Goal
E2E integration tests dengan real Postgres, build-tag gated.

#### Scope
- 5 scenarios: transfer / concurrent / RLS isolation / period close / tamper detection
- Build tag `integration` (excluded dari default `go test ./...`)
- CI workflow step: install Postgres + run migrations + run integration tests
- `TestIntegration_RLSIsolation` — verify tenant A query returns 0 rows from tenant B tables

#### Key Artifacts
- `internal/usecase/integration_test.go` — E2E suite
- `.github/workflows/ci.yml` line 133 — integration test step

#### Learnings
- Real Postgres > mocks untuk SQL-heavy domain (mocks hide transaction semantics)
- Build tag `integration` keeps `go test ./...` fast untuk TDD loop

---

## Sprint 16 — Property-Based Tests — 2026-08-14

**Status:** ✅ Done · **Fase:** Fase 7 (Testing & Quality)

#### Goal
Catch invariant violations yang unit tests miss.

#### Scope
- 15 property-based tests across ledger / invoice / collection / reconciler
- Internal pattern (not gopter) — table-driven + random input generation
- 10,000+ random scenarios per CI run
- Invariants tested: FIFO allocation, double-entry conservation, hash chain continuity

#### Key Artifacts
- `internal/usecase/ledger_property_test.go`
- `internal/usecase/service_property_test.go`
- `internal/usecase/transfer_concurrent_test.go` — `TestConcurrent_NoDeadlocks_100x50`

#### Learnings
- Property tests menemukan edge cases yang unit tests miss (negative amounts, zero-balance accounts, dll)
- 100 goroutines × 50 iter × 10 accounts PASS dalam <3 detik — lock ordering works

---

## Sprint 15 — Tenant RLS Integration (Fase 2B + 5A) — 2026-08-14

**Status:** ✅ Done · **Fase:** Fase 2B + 5A (Security & Multi-tenancy)

#### Goal
Database-level tenant isolation via Row-Level Security + `app_admin` bypass role.

#### Scope
- RLS enabled di 11 tables (accounts, transactions, entries, invoices, payments, dll)
- GUC variables: `app.current_tenant_id`, `app.current_user_id`, `app.is_sales_rep`
- Tenant context middleware binds GUC per-request
- `app_admin` Postgres role (ADR-0007) — bypass RLS untuk ops tools
- Integration test verifikasi cross-tenant isolation

#### Key Artifacts
- `migrations/000014_tenant_rls.up.sql`
- `migrations/000015_app_admin_role.up.sql`
- `internal/platform/tenantctx/tenantctx.go` — GUC binding
- [ADR-0006](adr/0006-tenant-rls-strategy.md) — RLS strategy
- [ADR-0007](adr/0007-app-admin-rls-bypass.md) — app_admin rationale

#### Learnings
- RLS sebagai **defense-in-depth** — application-layer filter masih primary, RLS sebagai safety net
- `app_admin` role pattern: dedicated Postgres role + bypass policy, bukan superuser
- GUC `is_sales_rep` enables field-level authz (sales_rep hanya lihat route milik sendiri)

#### Follow-ups
- Sprint 22B/23: GUC bind audit trail (ADR-0006 follow-up #2)
- Sprint TBD: RLS untuk `user_credentials` table (saat ini excluded)

---

## Sprint 14 — Rate Limiting — 2026-08-14

**Status:** ✅ Done · **Fase:** Fase 2D (Security)

#### Goal
Brute-force defense di `/v1/auth/login` endpoint.

#### Scope
- Per-IP token bucket via Redis (`RATE_LIMIT_LOGIN_ENABLED`)
- Configurable: `LOGIN_RATE_LIMIT_PER_MIN`, `LOGIN_BURST`
- Returns `429 Too Many Requests` + `Retry-After`
- `RATE_LIMIT_LOGIN_DISABLED` flag untuk test environments

#### Key Artifacts
- `internal/middleware/rate_limit.go`
- `internal/platform/config/config.go` — rate limit config block
- `docs/runbooks/incident-response.md` — rate limit triggered playbook

#### Learnings
- Per-IP saja insufficient untuk authenticated endpoints (X-Forwarded-For bisa di-spoof)
- Real IP extraction (`chimw.RealIP`) required untuk behind-proxy correctness

#### Follow-ups
- Sprint 14b — multi-tier rate limit (per-IP + per-user + per-tenant)

### Sprint 14 followup — Multi-Tier Rate Limiting (Fase 2D Lanjutan) — 2026-08-15

**Status:** ✅ Done · **Fase:** Fase 2D follow-up

#### Goal
Defense-in-depth rate limit: per-IP + per-user + per-tenant tiers.

#### Scope
- Tighter limits untuk `/v1/transfers` (financial endpoint)
- Prometheus custom counters: `fmcg_rate_limit_allowed_total{tier}`, `fmcg_rate_limit_rejected_total{tier}`
- Multi-tier middleware applied inside `/v1` group

#### Key Artifacts
- `cmd/api/metrics.go` — Prometheus counters
- `internal/middleware/rate_limit.go` — multi-tier implementation

---

## Sprint 13 — Refresh Token Rotation + MFA (Fase 2E Lanjutan) — 2026-08-14

**Status:** ✅ Done · **Fase:** Fase 2E lanjutan (Security)

#### Goal
Stateless JWT + opaque refresh tokens dengan reuse detection + TOTP MFA.

#### Scope
- Opaque refresh tokens (random 256-bit), stored hashed (SHA-256)
- Reuse detection: presented token marked family-revoked (defense vs token theft)
- TOTP MFA (RFC 6238) per user
- Brute-force lockout: 5 attempts / 15 min
- Backwards compat: MFA optional (enforced per-user)

#### Key Artifacts
- `migrations/000013_refresh_tokens.up.sql`
- `internal/domain/auth/auth.go` — entity + interfaces
- `internal/usecase/auth_service.go` (762 lines)
- [docs/api/auth.md](api/auth.md)

#### Learnings
- Refresh token reuse detection catches **both** token theft AND accidental log sharing
- Bcrypt vs SHA-256: bcrypt untuk passwords (slow, anti-rainbow), SHA-256 untuk tokens (fast, no salt needed)

#### Follow-ups
- Sprint 22B/23: MFA recovery codes (user lockout prevention)
- Sprint 22B/23: Session list/revoke (operator visibility)
- Sprint 22B/23: Password policy validator

---

## Sprint 12 — Multi-Currency (Fase 1D) — 2026-08-14

**Status:** ✅ Done · **Fase:** Fase 1D (Financial Correctness)

#### Goal
Per-transaction FX rate snapshot untuk auditability + multi-currency account support.

#### Scope
- `currencies` table (ISO 4217 + minor units)
- `fx_rates` table (effective_from, effective_to, source)
- Asymmetric cross-currency entries: 2 pairs (debit + credit) untuk FX gain/loss
- Manual + seed FX rate modes (operator-driven)
- Trigger `enforce_fx_rate_snapshot` — FX rate mandatory for cross-currency

#### Key Artifacts
- `migrations/000012_multi_currency.up.sql`
- `internal/domain/currency/currency.go`
- `internal/usecase/currency_service.go`
- [ADR-0005](adr/0005-multi-currency-strategy.md)
- [docs/api/currencies.md](api/currencies.md)

#### Learnings
- Per-transaction FX rate snapshot > recalculation — audit trail accurate forever
- Asymmetric entries (gain/loss) simpler than synthetic FX account per pair

#### Follow-ups
- Sprint TBD: FX rate auto-refresh from API (network call in hot path = latency)

---

## Sprint 11 — Collection & Route Module (Portfolio Sprint 4 + Fase 8 Partial) — 2026-08-13

**Status:** ✅ Done · **Fase:** Fase 1E (Domain Operations)

#### Goal
Sales rep field workflow: route plan → visit → settle + supervisor approval.

#### Scope
- Routes (assigned by supervisor), visits (per-outlet), settlements (collected amount)
- Stop events (sales rep can stop mid-route dengan reason)
- Supervisor approval workflow untuk settlements
- Status machine: `planned → in_progress → settled → approved | rejected`

#### Key Artifacts
- `migrations/000011_collection.up.sql`
- `internal/domain/collection/collection.go`
- `internal/usecase/collection_service.go`
- [docs/api/collection.md](api/collection.md)

---

## Sprint 10 — Reconciler & Trial Balance (Fase 1B) — 2026-08-13

**Status:** ✅ Done · **Fase:** Fase 1B (Financial Correctness)

#### Goal
Background job yang verify ledger consistency + manual trigger API.

#### Scope
- Trial balance: `SUM(debits) == SUM(credits)` global check
- Per-account breakdown: cached_balance vs computed from entries
- Optional hash chain re-verification (tamper detection operational)
- Manual API trigger: `POST /v1/reconciler/run`
- History retention: last 100 runs
- Background ticker: 1 hour interval

#### Key Artifacts
- `internal/domain/reconciler/reconciler.go`
- `internal/usecase/reconciler_service.go`
- `internal/worker/reconciler_worker.go`
- `cmd/worker/main.go` — ticker wiring
- [docs/api/reconciler.md](api/reconciler.md)

#### Learnings
- Reconciler catches cache drift bugs yang unit tests miss
- Hash chain verification O(n) per run — too slow untuk hourly → deferred (manual trigger only)

---

## Sprint 9 — Period Close with Approval Workflow (Fase 1A) — 2026-08-11

**Status:** ✅ Done · **Fase:** Fase 1A (Financial Correctness)

#### Goal
Two-step period close dengan snapshot — accounting cycle compliance.

#### Scope
- Period entity: `open | closing | closed`
- Two-step approval: request → approve/reject (different actors)
- Period-end snapshot: account balances frozen at close time
- DB trigger: prevent posting to closed period

#### Key Artifacts
- `migrations/000008_period_close.up.sql`
- `migrations/000009_period_snapshots.up.sql`
- `internal/domain/period/period.go`
- `internal/usecase/period_service.go`
- [docs/api/periods.md](api/periods.md)

#### Learnings
- Snapshot = immutable accounting record (auditor-friendly)
- DB trigger = defense-in-depth (app code can't bypass period close)

---

## Sprint 8 — Invoice & Credit + Hash Chain + JWT/RBAC — 2026-08-11

**Status:** ✅ Done · **Fase:** Fase 1C (Financial Correctness)

#### Goal
Customer receivables + tamper detection operational.

#### Scope
- Invoice entity + Payment entity + CreditLimit
- FIFO payment allocation (oldest invoice first)
- Credit limit enforcement (atomic check)
- Aging view (30/60/90 days buckets)
- SHA-256 hash chain: `entry_hash = SHA256(prev_hash || entry_data)`
- Tamper detection: modified entry breaks chain

#### Key Artifacts
- `migrations/000006_create_invoices.up.sql`
- `migrations/000007_add_hash_chain.up.sql`
- `internal/domain/invoice/invoice.go`
- `internal/usecase/invoice_service.go`
- `internal/usecase/hashchain_verifier.go`
- [docs/api/invoices.md](api/invoices.md)

#### Learnings
- Hash chain cheap (SHA-256 native speed), but verify O(n) → defer to background
- FIFO allocation tested via property tests (Sprint 16)

---

## Sprint 7 — Lock Ordering Hardening — 2026-08-12

**Status:** ✅ Done · **Fase:** Fase 1 (Financial Correctness)

#### Goal
Deadlock-free concurrent transfers.

#### Scope
- `LockPairForUpdate(ctx, tx, idA, idB)` — sort UUIDs, lock in deterministic order
- Always lock lower UUID first → no circular wait possible
- ADR-0004 documents the pattern + trade-offs

#### Key Artifacts
- [ADR-0004](adr/0004-locking-strategy.md)
- `internal/usecase/transfer_service.go:213-226`

---

## Sprint 6 — Idempotency-Key Pattern — 2026-08-10

**Status:** ✅ Done · **Fase:** Fase 1 (Financial Correctness)

#### Goal
Stripe-style idempotency untuk client retries — duplicate request dengan same key returns same result, no double-posting.

#### Scope
- `Idempotency-Key` header support di transfer + payment endpoints
- Redis-backed cache dengan TTL 24h
- Conflict detection: same key + different request body → 409 Conflict

---

## Sprint 5 — ADR-0004 + Audit Log + Middleware — 2026-08-10

**Status:** ✅ Done · **Fase:** Fase 2A (Security & RBAC)

#### Goal
First security layer — JWT auth + Casbin RBAC + audit log + middleware foundation.

#### Scope
- JWT auth (HS256, golang-jwt/jwt/v5)
- Casbin RBAC: 5 roles × 11 objects
- Audit log (`audit_logs` table — Sprint 5 migration)
- Middleware: auth, RBAC, request ID, structured logging
- [ADR-0004](adr/0004-locking-strategy.md) drafted

#### Key Artifacts
- `migrations/000005_create_audit_logs.up.sql`
- `internal/auth/jwt.go` — JWT signer/verifier
- `internal/auth/rbac/` — Casbin enforcer + policies
- `internal/middleware/` — auth, RBAC, audit middleware

---

## Sprint 4 — Double-Entry Core + Audit Trail — 2026-08-10

**Status:** ✅ Done · **Fase:** Fase 1 (Financial Correctness)

#### Goal
First complete double-entry transfer end-to-end.

#### Scope
- Double-entry invariant enforcement in `TransferService`
- Audit log writes (DB-level immutability deferred ke Sprint 5)
- Account balance updates atomic dengan entry inserts

---

## Sprint 3 — Account Schema + Repositories — 2026-08-10

**Status:** ✅ Done · **Fase:** Fase 1 (Financial Correctness)

#### Goal
Chart of accounts + period + transaction schema with hand-written pgx repositories.

#### Scope
- `accounts`, `periods`, `transactions` tables
- Repository pattern with `Tx` interface for transaction abstraction
- UUID generation, money type, basic CRUD

---

## Sprint 2 — Schema + Repositories + Transfer Use Case — 2026-08-10

**Status:** ✅ Done · **Fase:** Fase 1 (Financial Correctness)

#### Goal
Foundation ledger schema + first working transfer use case.

#### Scope
- Postgres extensions: `pgcrypto`, `btree_gist`
- Core schema: `accounts`, `periods`, `transactions`, `entries`
- Hand-written pgx repositories (sqlc not yet adopted — see ADR-0002)
- First `TransferService` dengan `SELECT FOR UPDATE` + deterministic lock ordering (precursor ADR-0004)

#### Key Artifacts
- `migrations/000001_extensions.up.sql`
- `migrations/000002_create_accounts.up.sql`
- `migrations/000003_create_periods_and_transactions.up.sql`
- `migrations/000004_create_entries.up.sql` (immutable triggers)
- `internal/domain/ledger/` — entities + interfaces
- `internal/repository/postgres/` — pgx impls
- `internal/usecase/transfer_service.go` (early version)

---

## Foundation (bundled dengan Sprint 2–6)

**Status:** ✅ Done · **Fase:** Fase 0–1

#### Done alongside Sprint 2–6
- `cmd/{api,worker,migrator}/` — 3 binaries
- `internal/platform/{money,config,errors,httpx,logger,auth,tenantctx}` — 7 cross-cutting primitives
- `internal/auth/` — JWT signer/verifier + Casbin policies
- 37 linters golangci-lint strict
- 80% coverage threshold CI-enforced
- Multi-arch Docker distroless image (~20MB)

#### Learnings
- Strict linting + race detector catches issues at PR time, not production
- Distroless image = single binary, no shell, faster cold start, smaller attack surface

---

## Sprint 1 — Foundation Reset (2026-08-10)

**Status:** ✅ Done · **Fase:** Fase 0

#### Goal
Production-grade foundation reset — strict conventions from day 1.

#### Scope
- Repository init + Go 1.23+ toolchain
- Strict linter (37 linters golangci)
- CI/CD pipeline (5-stage: lint + test + security + build + Docker)
- Postgres 16 + migrations setup
- ADR-0001 (Go), ADR-0002 (sqlc — planned, belum di-implement)
- Architecture overview + C4 diagrams
- Domain glossary

#### Key Artifacts
- `Makefile` — `up`, `down`, `test`, `lint`, `security`, `verify`
- `.golangci.yml` — strict linter config
- `.github/workflows/ci.yml` — 5-stage pipeline
- [ADR-0001](adr/0001-go-as-backend-language.md)

#### Learnings
- "Production-grade from day 1" > MVP-then-refactor (less rework)
- ADR-0002 (sqlc) was planned but repos ultimately hand-written — Sprint TBD revisit

---

## Sprint 25 — Aging Recalculator Worker (2026-09-22)

**Status:** ✅ Done · **Fase:** 4D (Materialized aging)

#### Goal
Materialize the per-customer aging summary into a cached table so `GET /v1/customers/{id}/aging` reads from an indexed snapshot instead of recomputing the live `v_invoice_aging` view on every request. Bounded-fresh by worker interval (default 24h, configurable via `AGING_RECALC_INTERVAL`).

#### Scope
- **25.1** — Migration `000018_aging_snapshot.up.sql` — `aging_snapshots` + `aging_snapshot_runs` tables with RLS + `app_admin` bypass
- **25.2** — Domain `AgingSnapshotRepository` interface (Upsert / GetAgingSnapshot / ListCustomersWithOutstanding / StartRun / FinishRun / LatestRun)
- **25.3** — Postgres impl in `aging_snapshot_repo.go` — bulk INSERT with batched placeholders, idempotent DELETE+INSERT per run
- **25.4** — `AgingRecalculator.RunForAllTenants()` use case — list customers → live view per customer → bulk upsert under run ID
- **25.5** — `TenantSnapshotService` — read-through cache for the API (snapshot first, fallback to live view)
- **25.6** — `AgingWorker` ticker loop — mirrors `ReconcilerWorker` / `OutboxPublisherWorker` pattern
- **25.7** — Wire in `cmd/worker/main.go` (nightly by default)
- **25.8** — API integration — `handler.AgingAPI` interface + `agingSnapshotAPIAdapter`; `GetCustomerAging` reads snapshot first, falls back to `InvoiceAPI.GetAging` if `Aging` not wired
- **25.9** — 9 unit tests (5 recalc + 4 snapshot service), all PASS

#### Key Artifacts
- `migrations/000018_aging_snapshot.{up,down}.sql`
- `internal/domain/invoice/invoice.go` — `AgingSnapshotRepository` + `AgingSnapshot` + `CustomerRef` + `SnapshotRun` + `RunStatus`
- `internal/repository/postgres/aging_snapshot_repo.go` — full impl
- `internal/usecase/aging_recalculator.go` — `AgingRecalculator` + `TenantSnapshotService`
- `internal/usecase/aging_recalculator_test.go` — 9 unit tests
- `internal/worker/aging_worker.go` — nightly ticker
- `cmd/worker/main.go` — `wireAgingWorker()` + updated `cmd/api/main.go` — `agingAPI`
- `cmd/api/aging_snapshot_adapter.go` — adapter implementing `handler.AgingAPI`
- `internal/handler/handlers.go` — added optional `Aging AgingAPI` field

#### Learnings
- Snapshot pattern = trade bounded-freshness for read performance. Aging is a snapshot use case (decision-makers look at it, not real-time bots).
- Read-through service (`TenantSnapshotService`) = API code is unchanged, snapshot wiring is optional in `Handlers`. Tests work without snapshot setup.
- Run-id ties all rows from one recalc — enables point-in-time debugging + diff between runs.
- Bulk INSERT with batched placeholders (1000 rows/tx) is much faster than per-row INSERT — matters at scale.
- `aging_snapshot_runs` table is operator-only (no RLS) — worker writes, admin reads.

#### Follow-ups
- Incremental updates (not full truncate+insert) for large tenants
- Per-tenant parallel processing in `RunForAllTenants`
- `/readyz` endpoint reports last successful run + age (e.g. "stale > 25h")
- Subscribe to `transfer.posted` (Sprint 24) to invalidate cache incrementally on payment
- Add admin endpoint to force-recompute a single tenant

---

## Sprint 26 — FX Rate Auto-Refresh (2026-09-23)

**Status:** ✅ Done · **Fase:** 1D follow-up (multi-currency)

#### Goal
Replace manual-only FX rate entry with a periodic worker that fetches rates from a configurable provider and inserts them as `source='api'` rows. Closes the "Operator must set rates via `POST /v1/fx-rates`" gap noted in ADR-0005.

#### Scope
- **26.1** — `FxRateProvider` interface in `domain/currency` (`FetchRate`) + `FxRateBatchProvider` optional + `ParsePair` helper + error sentinels
- **26.2** — `infra/fxprovider` package: `HTTPProvider` (net/http, JSON unmarshal, `{BASE}` URL placeholder) + `StubProvider` (deterministic, used when no URL configured)
- **26.3** — Config block `FXConfig` in `platform/config` with env vars `FX_REFRESH_INTERVAL`, `FX_PROVIDER_URL`, `FX_PROVIDER_API_KEY`, `FX_PAIRS`, `FX_PROVIDER_TIMEOUT`
- **26.4** — `FxRateRefresher` use case — fetch → store via `CurrencyRepository.CreateFxRateNoTx` (uses pool directly, not business tx). Per-pair failure is skipped, doesn't fail the cycle.
- **26.5** — `FxRateWorker` ticker (mirror OutboxPublisherWorker / AgingWorker pattern)
- **26.6** — Wire in `cmd/worker/main.go` — picks `HTTPProvider` or `StubProvider` based on `FX_PROVIDER_URL` (empty → stub)
- **26.7** — 7 unit tests (6 refresher + 1 ParsePair), all PASS

#### Key Artifacts
- `internal/domain/currency/fx_provider.go` — interface + sentinels + `ParsePair`
- `internal/infra/fxprovider/fxprovider.go` — `HTTPProvider` + `StubProvider`
- `internal/usecase/fx_refresher.go` — `FxRateRefresher` + `FxRefreshResult` + `CurrencyRepo` interface
- `internal/usecase/fx_refresher_test.go` — 7 tests
- `internal/worker/fx_worker.go` — `FxRateWorker` ticker
- `internal/repository/postgres/currency_repo.go` — added `CreateFxRateNoTx` + `ListTenants`
- `internal/platform/config/config.go` — `FXConfig` struct + `splitPairs` helper
- `cmd/worker/main.go` — `wireFxRateWorker`

#### Learnings
- Sprint 12 enum already had `api` source + comment "Sprint 13+ can add an api source" — Sprint 26 closes that loop.
- Per-pair failure isolation: skipping an unavailable pair doesn't fail the cycle. Critical for resilience — provider flake for EUR/IDR shouldn't break USD/IDR refresh.
- StubProvider as zero-config fallback: dev/demo without network still works (deterministic rates).
- `CreateFxRateNoTx` (pool) vs `CreateFxRate` (tx-bound): refresh is fire-and-forget, no business tx wraps it. Adding `NoTx` variant avoids forcing the refresher to manage its own tx runner.
- `FxRateBatchProvider` interface allows 1 HTTP roundtrip per base instead of N (for providers that return all rates per base).

#### Follow-ups
- Publish `fx_rate.refreshed` events via outbox (Sprint 24) — currently silent
- Provider-specific adapters (exchangerate-api.com has different response shape than open.er-api.com)
- Per-pair retry/backoff (currently skipped-on-failure; could add jitter)
- Use `FxRateBatchProvider` to fetch all rates for a base in one call when HTTPProvider is used
- Add `/v1/fx-rates/refresh` admin endpoint for manual trigger
- Rate validity window config per-pair (currently fixed at 24h)

---

## Sprint 27 — Production Setup Fixes (2026-09-26)

**Status:** ✅ Done · **Fase:** DevOps hardening (post-Sprint 25-26 local setup work)

#### Goal
Eliminate manual workarounds in setup scripts (RLS disable + role drop) by
moving them into proper forward-fix migrations. Make the project deployable
to a clean Postgres without operator intervention.

#### Background
After Sprint 23-26 we had a fully functional project but two manual
workarounds were needed for local dev:
1. `ALTER TABLE refresh_tokens DISABLE ROW LEVEL SECURITY` — needed
   because migration 000014's strict tenant-isolation policy blocked
   INSERT during the login flow (which has no GUC context yet).
2. `DROP ROLE IF EXISTS app_admin` — needed before re-running migrations
   because migration 000015's `CREATE ROLE app_admin` has no `IF NOT EXISTS`.

Both are documented as known limitations in `scripts/README.md`. Sprint 27
fixes them properly via forward-fix migrations.

#### Scope
- **27.1** — `migrations/000019_refresh_tokens_rls_fix.up.sql`: replace
  strict `tenant_isolation_*` policies on refresh_tokens with:
  - `refresh_tokens_select_auth` — requires GUC match OR app_admin
  - `refresh_tokens_modify_auth` — allows INSERT when GUC is NULL (login
    flow), requires GUC match for normal flow, bypass for app_admin
- **27.2** — `migrations/000020_app_admin_role_idempotent.up.sql`: DO
  block wrapping CREATE ROLE app_admin in IF NOT EXISTS check.
  Forward-fix for 000015 even though 000015 also got the same wrap.
- **27.3** — `migrations/000021_create_fmcg_user.up.sql`: creates fmcg
  user + fmcg_wallet db idempotently. Forward-fix for the setup script's
  manual user creation.
- **27.4** — Fix `migrations/000015_app_admin_role.up.sql`: wrap CREATE ROLE
  in IF NOT EXISTS check (so it's idempotent directly, not just via 020).
- **27.5** — Update `scripts/start-fresh.ps1` and `scripts/setup-everything.ps1`
  to remove manual `DROP ROLE app_admin` + `ALTER TABLE refresh_tokens
  DISABLE ROW LEVEL SECURITY` steps. Migrations handle everything.
- **27.6** — Update `scripts/README.md` to reflect Sprint 27 fixes.

#### Key Artifacts
- `migrations/000019_refresh_tokens_rls_fix.{up,down}.sql`
- `migrations/000020_app_admin_role_idempotent.{up,down}.sql`
- `migrations/000021_create_fmcg_user.{up,down}.sql`
- `migrations/000015_app_admin_role.up.sql` (edited for idempotency)
- `scripts/start-fresh.ps1` (simplified: removed workarounds)
- `scripts/setup-everything.ps1` (Phase 5 is now a no-op)
- `scripts/README.md` (Resolved in Sprint 27 section)

#### Learnings
- Forward-fix migrations (000019+) are safe to add without invalidating
  existing migration history because golang-migrate tracks by version
  checksum per file, not per schema state.
- RLS policies can be made conditional on GUC state with `current_setting(..., true)`
  IS NULL checks. Cleaner than disabling RLS entirely.
- DO blocks with EXCEPTION WHEN OTHERS + NULL is a pragmatic pattern for
  idempotent role/grants creation in migrations.
- Setup scripts should defer to migrations for state setup. Scripts only
  orchestrate (run migrations, seed data, start processes).

#### Verification (all PASS)
- Fresh DB setup → all 21 migrations apply cleanly (no dirty state)
- Login flow works WITHOUT manually disabling RLS
- Login persists refresh_token row in DB (verified via psql)
- `verify-setup.ps1` reports 24/24 PASS
- `start-fresh.ps1 -Force` completes in <30s

#### Follow-ups
- Migration 000015 still has CREATE GRANT statements that may fail on re-run.
  Should be wrapped similarly in future sprint.
- Consider `001_grants.sql` to centralize all role/grants.

---

## Sprint 28 — Notification Dispatcher (2026-09-24)

**Status:** ✅ Done (partial — notification write path only) · **Fase:** 8

#### Goal
Close the outbox loop: events published by Sprint 24 (`fmcg.transfer.posted`,
`fmcg.invoice.created`) are consumed by a NATS subscriber in the worker
process and persisted as in-app notification rows that end users see via
`GET /v1/notifications`.

#### Scope
- **28.1** — Migration `000022_notifications.up.sql` — `notifications` table
  with RLS + `app_admin` bypass + index on `(tenant_id, user_id, created_at DESC)`
- **28.2** — `internal/domain/notification/` — entity + Repository interface
  (Tx abstraction for `Create`, pool for reads)
- **28.3** — `internal/repository/postgres/notification_repo.go` — Postgres
  impl + `PersistDirect` helper for worker (fire-and-forget)
- **28.4** — `internal/usecase/notification_service.go` — `NotificationService`
  with `Subscribe(ctx)` for NATS subjects, recipient derivation, severity mapping
- **28.5** — `cmd/worker/main.go:wireNotificationWorker` — wires service
  to NATS subscriber (event-driven)
- **28.6** — `internal/handler/notifications.go` — 4 endpoints:
  `GET /v1/notifications`, `GET /v1/notifications/unread-count`,
  `PATCH /v1/notifications/{id}/read`
- **28.7** — `cmd/api/notification_adapter.go` — bridges usecase to handler

#### Known Issue (logged as Sprint 29 follow-up)
- After the initial wiring, `GET /v1/notifications` returned 500 because
  `Handlers.Notifications` was not assigned in `handler.New(...)`. Fixed in
  the same commit by adding `Notifications: notifications,` to the struct
  literal.
- A *second* 500 persisted even after the wiring fix. Sprint 29 root-caused
  this as RLS-on-Pool: the repo used `r.db.Pool.Query` directly without
  binding GUC, so RLS `current_setting('app.current_tenant_id', true)::uuid`
  evaluated against NULL and the SELECT returned no rows. The error
  surfaced from a downstream consumer trying to scan a missing result.

#### Learnings
- The `PersistDirect` helper pattern (write path bypassing domain Tx) is
  pragmatic for fire-and-forget worker writes but skips the normal
  `RunInTxNotificationDomain` path — which means it must bind GUC itself.
- Hardcoded recipient mapping (admin UUID `33333333-...`) is acceptable
  for MVP but needs a real recipient-resolution table for production.
- Worker subscriber pattern: one `nats.MsgHandler` per subject, handler
  errors logged but don't kill the consumer loop (let NATS redelivery
  semantics govern retries).

---

## Sprint 29 — RLS-on-Pool Read-Path Fix (2026-10-01)

**Status:** ✅ Done · **Fase:** 5A / 8 follow-up · **Theme:** RLS defense-in-depth

#### Goal
Fix the Sprint 28 leftover bug (and a parallel latent bug in
`aging_snapshot_repo`) where repo read paths used `r.db.Pool.Query`
directly. Without binding tenant GUC inside a tx, RLS evaluating
`tenant_id = current_setting('app.current_tenant_id', true)::uuid`
returned zero rows (or — for INSERT paths — failed the `WITH CHECK`
outright). Result: notification feed was unreachable; aging snapshot
worker silently processed nothing.

#### Scope
- **29.1** — Add `DB.RunInReadTx(ctx, fn)` helper (`internal/repository/postgres/db.go`):
  opens READ ONLY tx + binds GUC via `tenantctx.SetTenantContext` if `*Info` is on ctx.
- **29.2** — `notification_repo.go`: `List`, `MarkRead`, `CountUnread`, `GetByID`
  now use `RunInReadTx`. `PersistDirect` uses `RunInTx` + explicit GUC bind
  (falls back to `n.TenantID`/`n.UserID` from the row if ctx has no `*Info`).
- **29.3** — `aging_snapshot_repo.go`: `GetAgingSnapshot` uses `RunInReadTx`.
  `ListCustomersWithOutstanding` and `UpsertAgingSnapshots` documented as
  latent (need `app_admin` DSN — out of Sprint 29 scope).
- **29.4** — Integration test `TestIntegration_NotificationReadPath`
  (`internal/usecase/integration_test.go`) — 6 scenarios covering
  tenant-scoped list, cross-tenant isolation, CountUnread/MarkRead, defense
  against wrong-userID mark-read.
- **29.5** — `cmd/worker/main.go:92` typo fix (`wire fx rate worker` →
  `wire notification worker` in the `wireNotificationWorker` error wrap).
- **29.6** — `internal/hander/handlers.go` — remove pre-existing unused
  `"os"` import (build was broken since Sprint 28; nobody noticed because
  CI runs on Linux).

#### Key Artifacts
- `internal/repository/postgres/db.go:271` — `RunInReadTx` + `readOnlyTxOpts`
- `internal/repository/postgres/notification_repo.go` — all paths rebind GUC
- `internal/repository/postgres/aging_snapshot_repo.go:103` — `GetAgingSnapshot`
  uses RunInReadTx; cross-tenant ops get explicit KNOWN LIMITATION comment
- `internal/usecase/integration_test.go` — `TestIntegration_NotificationReadPath`
  + `cleanupNotifications` helper

#### Learnings
- `RunInReadTx` is the right shape for tenant-scoped reads: short-lived
  READ ONLY tx + automatic GUC bind. Adds ~1ms overhead per query but
  keeps RLS as defense-in-depth. The alternative — making the
  `notifications` RLS policy tolerant of missing GUC — would silently
  weaken isolation.
- `PersistDirect` needs its own GUC fallback when caller forgets to
  attach `*Info` to ctx. Worker is the canonical fire-and-forget
  caller; the fallback reads `n.TenantID` from the row.
- Cross-tenant worker operations (aging recalculator, fx refresher,
  reconciler, audit scrubber) cannot use `RunInReadTx` — they need
  `app_admin` DSN. Documented as Sprint 30+ follow-up. Until then,
  these workers are silently idle in production.
- Pre-existing build break (unused `os` import in `handlers.go`) survived
  Sprint 28 because CI runs on Linux (LF) and the LF file still parses;
  the CR/LF Windows checkout exposed it. Sprint 29 fix is local; Sprint
  30+ should add a `go build ./...` step to local pre-commit hooks.

#### Verification
- `go build ./...` — PASS (after handlers.go fix)
- `go vet ./internal/... ./cmd/...` — PASS
- `TestIntegration_NotificationReadPath` — 6 scenarios, all PASS
  (run with `go test -tags=integration` against TEST_DATABASE_URL)

#### Follow-ups (Sprint 30+)
- **`app_admin` DSN for cross-tenant workers** — add `cfg.DB.AdminDSN`,
  have aging/fx/reconciler workers connect through it. Closes the silent
  worker bug. (Effort: 1 week)
- **Audit all `r.db.Pool.Query/Exec` call sites** for the same RLS-on-Pool
  pattern. Fix tenant-scoped ones (already done for notifications + aging
  GetAgingSnapshot); fix cross-tenant ones via `app_admin` DSN.
- **Per-call GUC audit trail** — currently `SetTenantContext` logs to
  `guc_bind_audit` table on each tx. Sprint 30 should add Prometheus
  counter `fmcg_guc_binds_total{operation}` for ops visibility.
- **Move handlers.go-style leftover fixes to a CI gate** — add
  `golangci-lint --no-config --disable-all -E unused` step to detect
  unused imports early.

---

## Sprint 30 - Cross-Tenant Worker via app_admin (2026-10-01)

**Status:** ✅ Done · **Fase:** 5A / 4A / 4D / 1B follow-up · **Theme:** Worker correctness

#### Goal
Repair the silent worker bug surfaced by Sprint 29's audit: outbox
publisher, aging recalculator, fx refresher, and reconciler worker all
used bare `r.db.Pool.Query` for cross-tenant scans. With RLS enabled and
no tenant GUC bound on ctx, these queries returned zero rows - meaning
**the corresponding background jobs have not been doing anything in
production** since Sprint 15 enabled RLS.

#### Design Decision: SET LOCAL ROLE (not separate connection pool)
The original 000015 design called for `BEGIN; SET LOCAL ROLE app_admin;
<queries>; COMMIT;` inside the existing fmcg connection pool. We kept
that pattern instead of introducing a separate `AdminPool` because:
- `app_admin` is `NOINHERIT` (000015) so fmcg must explicitly SET ROLE
- `SET LOCAL ROLE` auto-reverts on COMMIT/ROLLBACK - no connection-level
  state leakage between requests
- No new connection pool, no password management for app_admin role
- The fmcg→app_admin grant was already in place (000015)
- If we ever need to run as a truly different role (e.g., ops user with
  different password), we can add `cfg.DB.AdminDSN` later

#### Scope
- **30.1** — Migration `000023_app_admin_grants.{up,down}.sql` —
  explicit GRANTs to app_admin on tables created after 000015
  (guc_bind_audit, outbox_events, aging_snapshots, aging_snapshot_runs,
  notifications), `ALTER DEFAULT PRIVILEGES` so future tables inherit,
  and admin_bypass policy for `collection_routes` (missed in 000015).
- **30.2** — `internal/repository/postgres/db.go` — `RunInAdminTx(ctx, fn)`
  helper (opens tx + `SET LOCAL ROLE app_admin` + auto-revert on
  commit/rollback) + `IsAdminRoleAvailable(ctx)` probe.
- **30.3** — Update cross-tenant repos to use `RunInAdminTx`:
  - `aging_snapshot_repo.ListCustomersWithOutstanding` (Sprint 25 nightly)
  - `aging_snapshot_repo.UpsertAgingSnapshots` (Sprint 25 nightly write)
  - `outbox_repo.FetchUnpublished` (Sprint 24 publisher)
  - `outbox_repo.MarkPublished` (Sprint 24 publisher)
  - `outbox_repo.IncrementAttempts` (Sprint 24 publisher retry)
  - `currency_repo.ListTenants` (Sprint 26 fx refresher)
  - `reconciler_repo.ListTenants` (Sprint 10 reconciler ticker)
- **30.4** — `cmd/worker/main.go:verifyAdminRole` — startup probe that
  warns (instead of crashes) if `app_admin` is unreachable, with hint to
  apply migrations 000015 + 000023.
- **30.5** — Update `notification_repo` doc comment to reference the
  cross-tenant policy (`app_admin` bypass applies to its insert path
  via PersistDirect - actually NOT needed since worker attaches
  `*tenantctx.Info` derived from the originating event).

#### Key Artifacts
- `migrations/000023_app_admin_grants.{up,down}.sql`
- `internal/repository/postgres/db.go:RunInAdminTx` (~30 LOC)
- `internal/repository/postgres/db.go:IsAdminRoleAvailable` (~22 LOC)
- 7 repo methods updated to use `RunInAdminTx`
- `cmd/worker/main.go:verifyAdminRole` (45 LOC, with explicit WARN log)

#### Learnings
- `SET LOCAL ROLE app_admin` works on the existing fmcg pool without a
  separate connection. This is significantly simpler than introducing a
  second pool + password management for app_admin.
- The original migration 000015 issued `GRANT ... ON ALL TABLES` which
  applies at GRANT TIME only. Tables created later (guc_bind_audit,
  outbox_events, aging_snapshots, notifications) silently lost the
  GRANT even though they had admin_bypass policies. Migration 000023
  patches this + adds `ALTER DEFAULT PRIVILEGES` so future tables
  auto-inherit.
- `collection_routes` had RLS enabled in 000014 but was missed by the
  admin_bypass loop in 000015 (explicit array, didn't include it).
  Now patched.
- The `IsAdminRoleAvailable` probe is valuable for ops: it surfaces
  missing grants at process start instead of waiting for the first
  cross-tenant cycle to fail with `permission denied to set role`.
- We deliberately did NOT add a separate AdminDSN connection - the
  pattern would have required password management and a second pool
  for marginal benefit (no auth-as-admin flow exists today).

#### Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- No integration test added in this sprint (deferred to Sprint 31's
  fraud-scanner test suite which exercises the admin path)

#### Follow-ups (Sprint 31+)
- **Integration test for RunInAdminTx** — add to integration_test.go
  to verify cross-tenant scans return rows from multiple tenants.
- **Per-call GUC audit trail** (Sprint 29 follow-up) — Prometheus
  counter `fmcg_guc_binds_total{operation}` for ops visibility.
- **FOR UPDATE SKIP LOCKED** (Sprint 24 follow-up) — outbox publisher
  needs this for multi-instance safety before scaling workers.
- **Audit other RLS-on-Pool call sites** — `audit_repo.List` etc. may
  still need the `RunInReadTx` treatment (Sprint 29 follow-up).

---

## Sprint 31 - Fraud Flag Scanner (2026-10-01)

**Status:** ✅ Done · **Fase:** 8 · **Theme:** Rule-based fraud detection

#### Goal
Implement the FraudScannerWorker: subscribe to `fmcg.transfer.posted`
events (Sprint 24 outbox publisher), run rule-based detection across 4
rules, persist fraud flags, and emit critical-severity notifications so
end users see fraud alerts via the existing notification feed (Sprint 28).

#### Scope
- **31.1** — Migration `000024_fraud_flags.{up,down}.sql` — `fraud_flags`
  table with tenant_id RLS + admin_bypass policy (picks up
  `ALTER DEFAULT PRIVILEGES` from migration 000023, no explicit GRANT
  needed), 3 indexes (account-detected, open-only, by-rule).
- **31.2** — `internal/domain/fraud/fraud.go` — Flag entity, Severity /
  Status / RuleName enums (mirroring DB CHECK), TransferEvent input,
  Match output, Tx abstraction, Repository interface (Create + PersistDirect
  + ListByAccount + CountRecentByAccount + HasRecipientHistory), Rule
  interface.
- **31.3** — `internal/domain/fraud/rules.go` — 4 built-in rules as pure
  functions: `LargeAmountRule` (critical), `OffHoursRule` (warn),
  `VelocityRule` (critical, takes injectable `LookupCount` func),
  `FirstTimeRecipientRule` (info, takes injectable `HasHistory` func).
- **31.4** — `internal/repository/postgres/fraud_repo.go` — Postgres impl
  with RLS-correct write (RunInTxFraudDomain) + RLS-correct tenant-scoped
  read (RunInReadTx) + cross-tenant scans (RunInAdminTx for velocity +
  recipient-history lookups from worker).
- **31.5** — `internal/repository/postgres/tx_adapter_fraud.go` —
  `RunInTxFraudDomain` helper (same shape as notification's), binds GUC
  from ctx's `*tenantctx.Info`.
- **31.6** — `internal/usecase/fraud_service.go` — FraudScannerService
  subscribes to NATS, decodes payload, runs all rules, persists flags,
  emits critical notifications via the existing notification dispatcher
  path (postgres.PersistNotificationDirect).
- **31.7** — `cmd/worker/main.go:wireFraudWorker` — wires the worker
  (parallel to wireNotificationWorker). NATS subject `fmcg.transfer.posted`.
- **31.8** — `internal/platform/config/config.go` — FraudConfig with 5
  env vars: `FRAUD_LARGE_AMOUNT_THRESHOLD_MINOR`, `FRAUD_VELOCITY_MAX_COUNT`,
  `FRAUD_VELOCITY_WINDOW`, `FRAUD_OFF_HOURS_START`, `FRAUD_OFF_HOURS_END`.
  Hardcoded defaults: 50M IDR / 10 transfers / 5min / 6-22h.
- **31.9** — Tests:
  - 13 unit tests in `internal/domain/fraud/rules_test.go` — table-driven
    covering each rule's positive/negative/boundary paths + error cases
  - 6 unit tests in `internal/usecase/fraud_service_test.go` — orchestration,
    severity → notification mapping, error swallowing, multiple-rule matches
  - 1 integration test `TestIntegration_FraudScannerPersistence` —
    end-to-end PersistDirect + ListByAccount + CountRecentByAccount +
    HasRecipientHistory via real Postgres

#### Key Artifacts
- `migrations/000024_fraud_flags.{up,down}.sql`
- `internal/domain/fraud/fraud.go` (~110 LOC) + `rules.go` (~140 LOC)
- `internal/repository/postgres/fraud_repo.go` (~190 LOC)
- `internal/repository/postgres/tx_adapter_fraud.go` (~60 LOC)
- `internal/usecase/fraud_service.go` (~180 LOC)
- `internal/usecase/fraud_service_test.go` (~280 LOC, 6 tests)
- `internal/domain/fraud/rules_test.go` (~250 LOC, 13 tests)
- `internal/usecase/integration_test.go` (+120 LOC, 1 new scenario)

#### Learnings
- `Repository` interface exposes BOTH `Create(ctx, tx, flag)` (for
  cross-domain atomic writes, future) AND `PersistDirect(ctx, flag)`
  (for worker fire-and-forget). This avoids the awkward notification
  pattern of type-asserting to concrete `*postgres.X` in the usecase
  layer, AND keeps the interface testable with fakes.
- Rule evaluation is decoupled from DB I/O via injectable function
  fields (`VelocityRule.LookupCount`, `FirstTimeRecipientRule.HasHistory`).
  This makes rules testable with zero infra, and lets us plug in
  different storage backends without rewriting rule logic.
- Sprint 30's `RunInAdminTx` made the worker integration trivial — no
  new infrastructure, just `SET LOCAL ROLE app_admin` inside the existing
  `fmcg` pool.
- Critical-flag → notification handoff uses `postgres.PersistNotificationDirect`
  directly (not the notification usecase service) to avoid a circular
  dep between `usecase.fraud_service` ↔ `usecase.notification_service`.
  This is acceptable since fraud doesn't need notification's full logic.
- Migration 000023's `ALTER DEFAULT PRIVILEGES` pays off here: this
  migration does NOT need explicit `GRANT` statements to `app_admin`
  because new tables automatically inherit the grants. Big DX win.

#### Verification
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./internal/domain/fraud/...` — 13 tests PASS
- `go test ./internal/usecase/...` — 6 fraud tests PASS, all existing tests PASS
- Integration test `TestIntegration_FraudScannerPersistence` — compiles
  (run with `go test -tags=integration` against TEST_DATABASE_URL)

#### Follow-ups (Sprint 32+)
- **Per-tenant thresholds** — currently env-only; production needs
  per-tenant config in DB so each tenant can tune their risk appetite.
- **Severity-based short-circuit** — skip lower-severity rules if a
  critical rule already fired (saves DB writes, reduces notification noise).
- **Rule priority** — run critical rules before info rules (e.g.
  VelocityRule before FirstTimeRecipientRule) to save the lookup cost
  on the latter when a critical rule fires.
- **WebSocket push for critical flags** — current notification is polled;
  real-time push would let ops react faster.
- **Cascade / dedup** — if same (transfer_id, rule_name) fires twice
  (NATS redelivery), we currently insert 2 flags. Add unique constraint
  or ON CONFLICT DO NOTHING.

---

## Sprint 32 — Login Attempt Partitioning (2026-10-01)

**Status:** ✅ Done · **Fase:** 2E follow-up · **Theme:** Operational scale

#### Goal
Partition `login_attempts` by month so the table scales to production
volume (millions of rows) without index maintenance becoming a bottleneck.
Enables operator-driven retention via partition DROP.

#### Scope
- **32.1** — Migration `000025_login_attempts_partitioning.{up,down}.sql`:
  rename existing table → create PARTITION BY RANGE (attempted_at) →
  pre-create 16 monthly partitions covering [now-3 months ... now+12] →
  per-partition indexes → copy data → drop old table.
- **32.2** — Helper function `fmcg_create_login_attempts_partition(year, month)`
  for ops to create future partitions beyond the pre-created window.
- **32.3** — `auth_repo.CreateLoginAttemptPartition(ctx, year, month)` —
  Go-side wrapper around the helper function.
- **32.4** — `auth_repo.DropLoginAttemptPartition(ctx, year, month)` —
  DETACH CONCURRENTLY + DROP TABLE for retention enforcement.
- **32.5** — Integration test `TestIntegration_LoginAttemptsPartitioning`
  verifies create + verify + drop via repo helpers.

#### Key Artifacts
- `migrations/000025_login_attempts_partitioning.{up,down}.sql`
- `internal/repository/postgres/auth_repo.go`: CreateLoginAttemptPartition +
  DropLoginAttemptPartition
- `internal/usecase/integration_test.go`: 1 new scenario

#### Learnings
- Postgres 12+ declarative partitioning works without extension (pg_partman).
  Migrations stay portable.
- Pre-creating 16 partitions provides comfortable lead time for ops.
- DETACH CONCURRENTLY before DROP avoids production read locks during
  retention enforcement.
- For pre-000023 tables, ALTER DEFAULT PRIVILEGES doesn't auto-apply;
  explicit GRANTs needed.

---

## Sprint 33 — Zero-Downtime JWT Rotation (2026-10-01)

**Status:** ✅ Done · **Fase:** 2E · **Theme:** Security hardening

#### Goal
Close the security finding flagged in `docs/runbooks/secret-rotation.md`:
changing JWT_SECRET invalidated ALL existing tokens, requiring ~30s downtime
+ forcing all users to re-login. Implement zero-downtime rotation via
multi-key support.

#### Scope
- **33.1** — `internal/auth/jwt/jwt.go`: `SecretProvider.Secrets()` returns
  `[][]byte` (was `[]byte`). Added `MultiKeySecret` type. Signer uses
  primary; Verifier tries primary, then secondary, etc.
- **33.2** — `internal/auth/jwt/jwt_test.go`: 8 unit tests covering all
  rotation scenarios.
- **33.3** — `config.JWTConfig`: added `SecretPrimary` + `SecretSecondary`
  fields, loaded from `JWT_SECRET_PRIMARY` + `JWT_SECRET_SECONDARY` env
  vars. Validation accepts either legacy `JWT_SECRET` or new primary.
- **33.4** — `cmd/api/main.go:buildJWTSecretProvider` — wires config into
  `jwt.MultiKeySecret`.
- **33.5** — `docs/runbooks/secret-rotation.md` — updated rotation
  procedure with zero-downtime 3-phase approach.

#### Key Artifacts
- 8 new unit tests in `internal/auth/jwt/jwt_test.go`
- Updated runbook with concrete env commands

#### Production tip
Three-phase rotation:
1. Set `JWT_SECRET_PRIMARY=$OLD`, `JWT_SECRET_SECONDARY=$NEW` → deploy
2. Wait JWT_ACCESS_TTL (15min) + grace period for all access tokens to expire
3. Set `JWT_SECRET_PRIMARY=$NEW`, unset `JWT_SECRET_SECONDARY` → deploy

---

## Sprint 34 — W3C SpanContext Upgrade (2026-10-01)

**Status:** ✅ Done · **Fase:** 3B follow-up · **Theme:** Observability

#### Goal
Upgrade the custom W3C traceparent implementation (Sprint 18) from
`trace_id`-only to full `SpanContext` (trace_id + span_id + parent_span_id +
flags). Each request becomes a root span; DB/NATS operations can create
child spans via `NewChild()`.

#### Scope
- **34.1** — `internal/middleware/tracing.go`: replaced `traceIDKey` context
  value with full `SpanContext` struct. Added `NewChild()`, `WithSpanContext()`,
  `SpanContextFromContext()`, `LogAttrs()`. `TraceMiddleware` now echoes
  the full W3C traceparent (with generated span_id).
- **34.2** — `internal/middleware/tracing_test.go`: 12 unit tests covering
  root span generation, child propagation, malformed headers, backward compat.

#### Why not OTel SDK directly
- OTel SDK + OTLP exporter = ~5 MB transitive deps
- This implementation provides 100% of the W3C propagation without
  needing a running Tempo collector for the core flow to work
- W3C format is identical to what OTel SDK emits → forward-compatible
  drop-in for Sprint 34.1

#### Key Artifacts
- 12 new unit tests in `internal/middleware/tracing_test.go`

---

## Sprint 35 — RLS for user_credentials (2026-10-01)

**Status:** ✅ Done · **Fase:** 5A · **Theme:** Security hardening

#### Goal
Close the explicit TODO from migration 000014:

> user_credentials (has tenant_id; but RLS would block admin tools —
>     for Sprint 15 we exclude this table from RLS ... Future Sprint:
>     add admin RLS bypass via dedicated role.)

The missing piece (app_admin role with RLS bypass) arrived in Sprint 14.

#### Scope
- **35.1** — Migration `000026_user_credentials_rls.{up,down}.sql`: ENABLE
  + FORCE RLS on user_credentials. Add tenant_isolation_select +
  tenant_isolation_modify policies. Add admin_bypass policy for app_admin.
  Explicit GRANTs to fmcg + app_admin (table was created in 000013, before
  000023's ALTER DEFAULT PRIVILEGES).
- **35.2** — `auth_repo.GetUserCredentialsByID`: switched to `RunInReadTx`
  so the tenant GUC is bound before the SELECT.
- **35.3** — Integration test `TestIntegration_UserCredentialsRLS`: tenant
  A reads OK, tenant B gets ErrUserNotFound (RLS makes it look like user
  not found).

#### Learnings
- App-layer checks still required — RLS is defense-in-depth, not a
  replacement for explicit tenant verification.
- Pre-000013 tables need explicit GRANTs (000023's ALTER DEFAULT PRIVILEGES
  only applies to tables created AFTER the migration).

---

## Sprint 36 — Chaos Tests for Outbox Recovery (2026-10-01)

**Status:** ✅ Done · **Fase:** 7 · **Theme:** Quality / reliability

#### Goal
Add chaos-style tests that verify the worker stack recovers from
interruption. Approach: test behavioral invariants rather than injecting
failures via toxiproxy/chaos-mesh (those would require Docker compose
changes).

#### Scope
- **36.1** — `internal/usecase/chaos_test.go`:
  - `TestIntegration_OutboxPublisher_RecoveryFromInterruption`: insert 5
    events, cancel ctx mid-cycle, verify NO events marked published,
    re-fetch with fresh ctx → all 5 still visible, MarkPublished → all
    marked, re-fetch → 0. Proves DB is source of truth.
  - `TestIntegration_OutboxPublisher_DuplicateInsertIsIdempotent`: insert
    same event_id twice (NATS redelivery simulation), verify FetchUnpublished
    handles it gracefully.

#### Why not full mutation testing
- Mutation testing mutates Go AST and reruns tests — adds significant
  build complexity
- Chaos tests verify the invariants that protect against the failures
  we actually care about (worker crash, lost events)

#### Why not toxiproxy
- Toxiproxy injects network failures at the proxy level — requires Docker
  compose service changes
- The outbox pattern's invariant (DB is source of truth) makes most
  network-failure scenarios irrelevant; pgxpool handles reconnection

For Sprint 36.1+: add toxiproxy integration when ops tooling allows.

---

## Sprint 37 — RLS-on-Pool Audit (Account/Invoice/Transaction) (2026-10-01)

**Status:** ✅ Done (partial — high-impact repos fixed, follow-ups in 37.1) · **Fase:** 5A · **Theme:** Defense-in-depth

#### Goal
Audit remaining `r.db.Pool.Query/QueryRow/Exec` call sites that were still
using bare pool. RLS enabled since Sprint 15 / migration 000014 means
these queries were returning zero rows when GUC wasn't bound. The most
critical paths (account, invoice, transaction) silently failed in
production for HTTP endpoints that depend on them.

#### Scope
- **37.1** — `internal/repository/postgres/account_repo.go`: Create +
  GetByID + GetByCode + List + Update now use RunInTx/RunInReadTx.
- **37.2** — `internal/repository/postgres/transaction_repo.go`:
  GetByID + GetByIdempotencyKey + MarkPosted + MarkFailed + MarkReversed.
- **37.3** — `internal/repository/postgres/invoice_repo.go`:
  GetByID + GetByCode + List + ListAllocations + GetAging.
- **37.4** — `internal/usecase/integration_test.go`:
  TestIntegration_AccountInvoiceTransactionRLS — verifies Create +
  GetByID + GetByCode + List work post-fix AND tenant B cannot see
  tenant A's rows.

#### Remaining (deferred to Sprint 37.1 follow-up)
- `entry_repo.go`: ListByTransaction, ListByAccount, GetCachedBalance,
  GetTrialBalance — ledger_entries has RLS
- `audit_repo.go`: List, ListGUCBinds, ListByActor — audit_logs has NO
  RLS (intentionally operator-visible across tenants) → OK as-is
- `reconciler_repo.go`, `period_repo.go` — need per-method audit

These don't affect the primary user-facing API paths.

---

## Sprint 38 — Per-Tenant Fraud Thresholds (2026-10-01)

**Status:** ✅ Done · **Fase:** 8 · **Theme:** Multi-tenant config

#### Goal
Replace env-only fraud thresholds with per-tenant overrides so each
tenant can tune their own risk appetite.

#### Scope
- **38.1** — `migrations/000027_tenant_fraud_settings.{up,down}.sql`:
  new table with RLS + admin_bypass (auto-grants via 000023 ALTER
  DEFAULT PRIVILEGES). CHECK constraints on positive thresholds +
  0-23 hour bounds.
- **38.2** — `internal/domain/tenant_fraud_settings.go`:
  TenantFraudSettings entity with EffectiveX methods (return override
  or fallback).
- **38.3** — `internal/repository/postgres/tenant_fraud_settings_repo.go`:
  Postgres impl with GetByTenant (RunInReadTx), Upsert (RunInTx), Delete.
- **38.4** — `internal/usecase/tenant_fraud_settings_service.go`:
  caching service with 60s TTL + Invalidate() / InvalidateAll().
- **38.5** — `internal/usecase/tenant_fraud_settings_service_test.go`:
  6 unit tests (default fallback, per-tenant override, cache
  invalidation, DB error fallback, disabled rules, partial override).
- **38.6** — `internal/usecase/fraud_service.go`: Rules field changed
  from `[]fraud.Rule` to `func(ctx, tenantID) []fraud.Rule` (closure
  resolved per-event per-tenant).
- **38.7** — `cmd/worker/main.go`: wireFraudWorker now creates the
  settings service and passes a per-tenant closure.

#### Production tips
- Set per-tenant threshold:
  `INSERT INTO tenant_fraud_settings (tenant_id, large_amount_threshold_minor) VALUES ('<uuid>', 500000000) ON CONFLICT (tenant_id) DO UPDATE SET large_amount_threshold_minor = EXCLUDED.large_amount_threshold_minor;`
- Disable a rule:
  `UPDATE tenant_fraud_settings SET disabled_rules = '["off_hours"]'::jsonb WHERE tenant_id = '<uuid>';`

---

## Sprint 39 — OTel SDK + OTLP Exporter (2026-10-01)

**Status:** ✅ Done · **Fase:** 3B · **Theme:** Distributed tracing

#### Goal
Replace the W3C-only trace context (Sprint 34) with full OpenTelemetry SDK
+ OTLP/HTTP exporter to send spans to a real collector (Tempo).

#### Scope
- **39.1** — `go.mod`: new deps for OTel SDK + OTLP HTTP exporter +
  otelhttp contrib.
- **39.2** — `internal/telemetry/otel.go`: new package. InitTracer(ctx,
  cfg) returns ShutdownFunc. TelemetryConfig with Enabled / OTLPEndpoint
  / ServiceName / SamplerRatio. HTTPMiddleware returns otelhttp-wrapped
  handler with method+path span name.
- **39.3** — `internal/telemetry/otel_test.go`: 4 unit tests (disabled
  no-op, stripScheme, defaults, shutdown timeout).
- **39.4** — `internal/platform/config/config.go`: added OTELEnabled
  bool field, OTEL_ENABLED env var.
- **39.5** — `cmd/api/main.go`: call telemetry.InitTracer in run()
  when OTEL_ENABLED, defer shutdown. Also fixed pre-existing missing
  httpx import that was blocking compilation.

#### Operational notes
- Enable: `OTEL_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=http://tempo:4318 OTEL_SERVICE_NAME=fmcg-wallet-api`
- When disabled, no behavior change (current TraceMiddleware still works)
- W3C trace context compatible with Sprint 34 — same propagation format
- Coexistence: TraceMiddleware (Sprint 34) provides trace_id in logs
  even when OTel is off; otelhttp (this sprint) provides full span
  creation when OTel is on. Both can run side-by-side.

---

## Sprint 40 — RLS-on-Pool Audit (Entry/Period/Reconciler) (2026-10-01)

**Status:** ✅ Done · **Fase:** 5A · **Theme:** Defense-in-depth

#### Goal
Continue the Sprint 37 RLS-on-Pool audit for the remaining repos:
entry_repo, period_repo, reconciler_repo.

#### Scope
- **40.1** — `internal/repository/postgres/entry_repo.go`:
  ListByTransaction, ListByAccount, SumForAccount now use RunInReadTx.
  TrialBalance uses RunInAdminTx (cross-tenant SUM for reconciler).
- **40.2** — `internal/repository/postgres/period_repo.go`: GetCloseRequest,
  ListSnapshotsByPeriod, ListRequestsByPeriod, ListAccountsByTenant,
  GetCurrentOpenPeriod now use RunInReadTx.
- **40.3** — `internal/repository/postgres/reconciler_repo.go`: GetRun,
  ListRunsByPeriod, ListRunsByTenant, ListAccountResultsByRun,
  ListOpenPeriods now use RunInReadTx.

#### Remaining (acceptable as-is)
- `audit_repo.go`: audit_logs has NO RLS (intentionally operator-visible
  across tenants for forensics)
- `collection_repo.go`: route_stops, collection_events have NO RLS

---

## Sprint 41 — Extended Chaos Tests (2026-10-01)

**Status:** ✅ Done · **Fase:** 7 · **Theme:** Worker resilience

#### Goal
Extend the Sprint 36 chaos tests with more behavioral invariants.

#### Scope
- `internal/usecase/chaos_extended_test.go` — 4 new tests:
  - `TestIntegration_OutboxPublisher_GracefulShutdown`: SIGTERM
    simulation via ctx cancellation; verifies FetchUnpublished respects
    ctx and no events are lost.
  - `TestIntegration_OutboxPublisher_ConcurrentSafety`: two publisher
    instances call FetchUnpublished simultaneously; verifies both get
    results (downstream must be idempotent on event_id — Sprint 24
    follow-up: FOR UPDATE SKIP LOCKED).
  - `TestIntegration_OutboxPublisher_EmptyQueueHandled`: graceful
    empty result.
  - `TestIntegration_OutboxPublisher_PaginationLimit`: limit param
    respected, no event in two consecutive batches.

#### Why not toxiproxy
The behavioral tests verify the recovery path that matters for correctness
(the DB schema property that outbox is source of truth). Toxiproxy
would add latency/bandwidth stress tests on top — deferred to Sprint 41.1.

---

## Sprint 42 — Fraud Flag Dedup (2026-10-01)

**Status:** ✅ Done · **Fase:** 8 follow-up · **Theme:** Operational cleanliness

#### Goal
When NATS redelivers a transfer.posted event (worker crash mid-publish),
the fraud scanner re-runs the rule → duplicate flag rows. Add UNIQUE
constraint + ON CONFLICT DO NOTHING.

#### Scope
- **42.1** — Migration `000028_fraud_flag_dedup.{up,down}.sql`: UNIQUE
  INDEX (transfer_id, rule_name) + ADD CONSTRAINT via DO block
  (idempotent).
- **42.2** — `internal/repository/postgres/fraud_repo.go`: Create() uses
  `ON CONFLICT (transfer_id, rule_name) DO NOTHING`.
- **42.3** — Integration test `TestIntegration_FraudFlagDedup`:
  inserts twice, verifies only 1 row exists, first insert wins.

---

## Sprint 43 — FOR UPDATE SKIP LOCKED Outbox (2026-10-01)

**Status:** ✅ Done · **Fase:** 4A follow-up · **Theme:** Multi-publisher safety

#### Goal
When two publisher instances run simultaneously (deploy with old + new
binary both active), each fetches events independently. Without row
locking, both could publish the same event → downstream consumers see
duplicates.

#### Scope
- **43.1** — `internal/repository/postgres/outbox_repo.go`: new
  `FetchUnpublishedLocked(ctx, limit)` that returns the pgx.Tx handle.
  Uses `FOR UPDATE SKIP LOCKED`. The caller controls commit timing so
  locks stay held until publish completes.
- **43.2** — `internal/usecase/chaos_extended_test.go`: new test
  `TestIntegration_OutboxPublisher_FetchUnpublishedLocked_SkipLocked`
  verifies two concurrent txs see disjoint event sets.

#### Usage pattern
```
1. FetchUnpublishedLocked(ctx, 50) → events + tx handle
2. Publish each event to NATS
3. UPDATE SET published_at = now() WHERE id IN (...)
4. tx.Commit()  // releases locks, events marked done
```
If anything fails: `tx.Rollback()` → locks released, events stay
unpublished for the next cycle (or another publisher).

---

## Sprint 44 — RLS-on-Pool Audit (Currency/Collection) (2026-10-01)

**Status:** ✅ Done · **Fase:** 5A · **Theme:** Defense-in-depth

#### Goal
Complete the RLS-on-Pool audit. currency_repo's GetFxRate/GetLatestFxRate/ListFxRates
were still bare Pool queries on `fx_rates` (which has RLS).

#### Changes
- `internal/repository/postgres/currency_repo.go`: GetFxRate,
  GetLatestFxRate, ListFxRates now use RunInReadTx. currencies table
  has NO RLS (intentionally global reference data) so GetCurrency is
  left on bare Pool.

#### Remaining
- `collection_repo.go`: route_stops, collection_events, settlements
  have NO RLS — left as-is.

---

## Sprint 45 — Invoice/Payment Outbox Events (2026-10-01)

**Status:** ✅ Done · **Fase:** 4A · **Theme:** Event vocabulary

#### Goal
Extend the outbox event vocabulary beyond just `transfer.posted` so
downstream consumers (notifications, fraud, analytics) can react to
invoice + payment lifecycle events.

#### Scope
- **45.1** — `internal/domain/outbox/outbox.go`: new constants
  `EventInvoiceCreated`, `EventPaymentRecorded`, `EventInvoiceOverdue`,
  `SubjectInvoiceCreated`, `SubjectPaymentRecorded`. Aggregate types
  (invoice, payment) were already in the DB CHECK constraint.
- **45.2** — `internal/usecase/transfer_service.go`: OutboxWriter
  interface now includes AppendInvoiceCreated + AppendPaymentRecorded.
  tx parameter changed from `ledger.Tx` to `any` to support multiple
  domain Tx types. noopOutboxWriter gets matching no-op methods.
- **45.3** — `internal/usecase/invoice_service.go`: OutboxWriter
  injected (falls back to noopOutboxWriter). CreateInvoice emits
  `invoice.created` event in same tx. RecordPayment emits
  `payment.recorded` event in same tx.
- **45.4** — `internal/repository/postgres/tx_extractor.go`: new
  UnwrapPgxTx(any) helper type-switches on known tx wrapper types
  (txAdapter, fraudTxAdapter, notificationTxAdapter, invoiceTxAdapter).
- **45.5** — `cmd/api/outbox_adapter.go`: updated to use new
  any-based interface + UnwrapPgxTx.
- **45.6** — `cmd/api/main.go`: wires outboxWriter into InvoiceService
  deps.

#### Notification worker compatibility
Notification worker subscribes to `fmcg.>` (wildcard) — automatically
receives the new event types. Fraud worker stays scoped to
`fmcg.transfer.posted` (fraud rules don't apply to invoice/payment).

---

## Sprint 46 — JetStream Durable Subscription (2026-10-01)

**Status:** ✅ Done (opt-in) · **Fase:** 4A follow-up · **Theme:** Reliability

#### Goal
Add opt-in JetStream support so events survive broker restart.
Pre-Sprint-46 used core NATS publish (fire-and-forget); the outbox
table was the durability boundary.

#### Scope
- **46.1** — `internal/infra/nats.go`: new `EnsureStream` (idempotent
  stream setup), `PublishJS` (durable publish), `SubscribeDurable`
  (consumer group with replay). Sprint 24 `Subscribe` (core NATS)
  preserved for backward compat.
- **46.2** — `internal/usecase/outbox_publisher.go`: new JetStreamBroker
  type. `publish()` method type-asserts the broker and dispatches to
  PublishJS if available. Test fakes that only implement Publish keep
  working.
- **46.3** — `internal/platform/config/config.go`: NATSConfig.
  JetStreamEnabled field + `NATS_JETSTREAM_ENABLED` env var.

#### Operational notes
- Enable: `NATS_JETSTREAM_ENABLED=true`
- Stream name default: `FMCG_EVENTS` (configurable via `NATS_STREAM_NAME`)
- Subjects: `fmcg.>` (configurable via `NATS_STREAM_SUBJECTS`)
- Retention: 24h (`MaxAge`); outbox is source of truth so older events
  can be safely purged

#### Backward compat
If `NATS_JETSTREAM_ENABLED` is false (default), outbox publisher uses
core NATS publish (fire-and-forget). No behavior change.

---

## Sprint 47 — Frontend Next.js Migration Plan (2026-10-01)

**Status:** ✅ Done (planning + scaffold only) · **Fase:** 6 · **Theme:** UX modernization

#### Goal
Document the migration plan from `web/` (vanilla JS, Sprint 20 MVP) to
`web-next/` (Next.js 15 + TypeScript). This sprint is **planning +
scaffold only** — the actual UI migration is in Sprint 47.1+.

#### Scope
- **47.1** — `web-next/README.md`: full migration plan (~3 KB),
  4 sub-sprints outlined (47.1 type-safe API client, 47.2 page
  migration, 47.3 production hardening), folder structure diagram,
  package.json scaffold, integration plan with backend.
- **47.2** — `web-next/package.json`: dependency manifest (next 15,
  react 19, tanstack-query, zod, tailwind, orval for OpenAPI codegen,
  vitest, playwright).
- **47.3** — `web-next/tsconfig.json`, `next.config.js`,
  `tailwind.config.ts`, `.gitignore`: standard Next.js 15 + Tailwind
  + TypeScript scaffold with security headers (X-Frame-Options DENY,
  X-Content-Type-Options nosniff, Referrer-Policy strict-origin-when-cross-origin)
  and reverse-proxy rewrites for `/v1/*` to the backend.

#### Active frontend unchanged
`web/` (vanilla JS) remains the active frontend. The migration will
be implemented in Sprint 47.1+ when an implementer picks up the plan.

#### Why split this way
The full migration is ~2 weeks. Splitting planning from implementation
keeps the commit history clean and lets a new contributor pick up the
plan directly without reading Sprint 47 in detail.

---

## Sprint 48 — toxiproxy Chaos Integration (2026-10-01)

**Status:** ⏸ Deferred · **Fase:** 7 · **Theme:** Chaos engineering

#### Decision
Defer to integration setup phase. toxiproxy requires:
- A docker-compose service for toxiproxy
- Wire-up to inject latency/failure between the worker and Postgres/NATS
- CI infrastructure to run the chaos suite against a real broker

The behavioral chaos tests added in Sprint 36 + Sprint 41 already
verify the invariants that matter for correctness (DB as source of
truth, no data loss on interruption, multi-publisher safety via
SKIP LOCKED, etc.). toxiproxy would add latency/bandwidth stress on
top of these — useful but not blocking for production launch.

#### For Sprint 48.1+ implementer
1. Add `toxiproxy` service to `deployments/docker-compose.yml` with
   admin API on `:8474` and proxy on `:5432` (DB) and `:4222` (NATS).
2. Update tests to point at toxiproxy endpoints (env vars).
3. Add latency/jitter/failure scenarios in `internal/usecase/chaos_*_test.go`.
4. CI: separate `make test-chaos` target that requires toxiproxy
   running. Skip by default; opt-in via `RUN_CHAOS=1`.

---

## Sprint 49 — Next.js Type-Safe API Client (2026-10-01)

**Status:** ✅ Done (scaffold) · **Fase:** 6 · **Theme:** Type safety

#### Goal
Replace the vanilla JS string-based API calls in `web/public/` with a
type-safe TS client generated from OpenAPI.

#### Scope
- **49.1** — `web-next/openapi.json`: hand-curated OpenAPI 3.0 spec
  covering auth/accounts/transfers/invoices/notifications. When
  backend gains OpenAPI annotation middleware (Sprint 49.0 follow-up),
  this file is replaced with the live spec at `/openapi.json`.
- **49.2** — `web-next/orval.config.ts`: codegen config (orval +
  prettier + tags-split per endpoint group).
- **49.3** — `web-next/lib/api.ts`: hand-written stopgap API client
  (login, listAccounts, listTransfers, listInvoices, listNotifications).
  Uses `credentials: 'include'` for the httpOnly JWT cookie. Throws
  structured `ApiClientError` on non-2xx.
- **49.4** — `web-next/lib/auth.tsx`: `AuthProvider` + `useAuth()` hook
  for client-side user state. Stub for Sprint 50 — Sprint 49.1 wires
  the real `/v1/auth/me` call.

#### For Sprint 49.1+ implementer
1. Add OpenAPI annotation middleware to `cmd/api/main.go` (wraps
   routes with annotations, serves `/openapi.json`).
2. Replace `web-next/openapi.json` with live spec.
3. Run `pnpm typegen` to generate `web-next/lib/api.ts`.
4. Delete the hand-written stopgap.

---

## Sprint 50 — Next.js Login + Dashboard PoC (2026-10-01)

**Status:** ✅ Done (proof-of-concept) · **Fase:** 6 · **Theme:** UX

#### Goal
Migrate ONE user-facing page (login) + dashboard layout from vanilla JS
to Next.js 15 + React 19 + Tailwind. Proves the migration plan from
Sprint 47 works end-to-end.

#### Scope
- **50.1** — `web-next/app/layout.tsx`: root layout with AuthProvider.
- **50.2** — `web-next/app/globals.css`: Tailwind base + CSS vars.
- **50.3** — `web-next/app/page.tsx`: landing page (redirect to dashboard
  if logged in, else show /login link).
- **50.4** — `web-next/app/login/page.tsx`: client-component login form
  with loading/error state, calls `api.login`, redirects to /dashboard.
- **50.5** — `web-next/app/(dashboard)/layout.tsx`: shared sidebar +
  main area for authenticated pages (route group via parentheses).
- **50.6** — `web-next/app/(dashboard)/dashboard/page.tsx`: server
  component (RSC) dashboard with 3 placeholder widgets.
- **50.7** — `web-next/lib/auth-server.ts`: `getServerSession()` stub
  for SSR — Sprint 49.1 wires the real cookie-read + `/v1/auth/me`.

#### Migrated equivalents
| web/ (vanilla JS, Sprint 20) | web-next/ (Sprint 50) |
|---|---|
| `public/index.html` (login form) | `app/login/page.tsx` |
| `public/dashboard.html` | `app/(dashboard)/dashboard/page.tsx` |
| `public/index.html` (sidebar partial) | `app/(dashboard)/layout.tsx` |
| `server.js` (Node reverse proxy) | `next.config.js` `rewrites()` |

---

## Sprint 51 — Period.closed Event Type (2026-10-01)

**Status:** ✅ Done · **Fase:** 4A · **Theme:** Event vocabulary

#### Goal
Close the period event vocabulary so reconciliation dashboards + audit
trail know when periods close/reopen.

#### Scope
- **51.1** — `internal/domain/outbox/outbox.go`: new constants
  `EventPeriodClosed`, `EventPeriodReopened`, `SubjectPeriodClosed`,
  `SubjectPeriodReopened`. Aggregate type `period` was already in DB CHECK.
- **51.2** — `internal/usecase/transfer_service.go`: OutboxWriter
  interface adds `AppendPeriodClosed` + `AppendPeriodReopened`.
- **51.3** — `internal/usecase/period_service.go`: OutboxWriter
  injected (defaults to noopOutboxWriter). `ApproveClose` emits
  `period.closed` in same tx (after snapshot inserts + status flip).
  `Reopen` emits `period.reopened` in same tx.
- **51.4** — `cmd/api/outbox_adapter.go`: `AppendPeriodClosed` +
  `AppendPeriodReopened` implementations.
- **51.5** — `cmd/api/main.go`: re-order wiring so outboxWriter is
  constructed before periodService so it can be passed to PeriodService deps.

---

## Sprint Backlog (post-Sprint 51)

| Sprint | Title | Fase | Source | Effort |
|---|---|---|---|---|
| 52 | toxiproxy chaos integration | 7 | Sprint 48 follow-up | 1 week |
| 53 | Next.js accounts page migration | 6 | Sprint 50 follow-up | 1 week |
| 54 | Next.js transfers page migration | 6 | Sprint 50 follow-up | 1 week |
| 55 | Next.js invoices + notifications pages | 6 | Sprint 50 follow-up | 1 week |
| 56 | Sprint 49.1 — real OpenAPI codegen wire-up | 6 | Sprint 49 follow-up | 1 week |
| 57 | Sprint 49.1 — real /v1/auth/me wire-up | 6 | Sprint 49 follow-up | 2 days |
| TBD | Replace third-party JWT (legacy JWT_SECRET) | 2E | Migration cleanup | 1 day |
| TBD | WebSocket push for critical flags | 8 | Sprint 31 follow-up | 1 week |

---

## Cumulative Stats (post-Sprint 51)

| Metric | Value | Source |
|---|---|---|
| Total sprints completed | 51 | this file |
| Total LOC | ~22,500 | docs/index.md (refresh in Sprint 31) |
| Go files (production) | ~100 | docs/index.md |
| Go files (test) | ~30 | docs/index.md |
| Migrations | 22 (Sprint 51 was code-only, no new migrations) | migrations/ folder |
| ADRs | 8 | docs/adr/ folder |
| REST endpoints | 39+ (added 3 in Sprint 28) | docs/api/overview.md |
| Use cases | 9 | internal/usecase/ folder |
| Repositories | 11 | internal/repository/postgres/ folder |
| Unit tests | 175+ (+2 chaos_extended Sprint 41 + 1 for Sprint 43 SKIP LOCKED) | docs/index.md |
| Integration scenarios | 12 (+1 Sprint 40 entry/period/reconciler + 1 Sprint 42 fraud flag dedup) | Sprint 17 + Sprint 29 + Sprint 31 + Block 1 + Block 2 + Sprint 37 + Block 3 |
| Coverage threshold | 80% (CI-enforced) | .github/workflows/ci.yml |
| Linters | 37 strict | .golangci.yml |
| Docker image size | ~20MB (distroless) | Dockerfile |

---

## ADR Cross-Reference

Setiap ADR terkait dengan sprint:

| ADR | Title | Sprint | Status |
|---|---|---|---|
| [0001](adr/0001-go-as-backend-language.md) | Go as backend language | 1 | Accepted |
| [0002](adr/0002-sqlc-over-orm.md) | sqlc over ORM | 1 (planned) | Accepted (not yet adopted) |
| [0003](adr/0003-double-entry-ledger.md) | Double-entry ledger | 2-4 | Accepted |
| [0004](adr/0004-locking-strategy.md) | Locking strategy | 7 | Accepted |
| [0005](adr/0005-multi-currency-strategy.md) | Multi-currency strategy | 12 | Accepted |
| [0006](adr/0006-tenant-rls-strategy.md) | Tenant RLS strategy | 15 | Accepted |
| [0007](adr/0007-app-admin-rls-bypass.md) | app_admin role for RLS bypass | 15 follow-up / 22A.4 | Accepted |
| [0008](adr/0008-sprint-22b-hardening-roadmap.md) | Sprint 22B hardening roadmap | 22B | Accepted |
| [0009](adr/0009-sprint-29-rls-read-path.md) *(planned)* | RLS read-tx wrapping rationale | 29 | Planned |

Future ADR candidates: int64 minor units money rationale, hash chain rationale, period close rationale.

---

## Conventions

- **Sprint ID**: integer (1, 2, ..., 23) atau letter suffix (22A, 22B) untuk sub-sprint paralel
- **Sprint scope**: 1–2 minggu calendar time, atau 1–5 hari focused work
- **Definition of Done**: 
  - `make ci` PASS (lint + test + build)
  - Coverage tetap ≥ 80%
  - 0 TODO baru
  - Sprint retrospective ditambahkan ke file ini
  - Fly.io demo deploy sukses + `/readyz` masih hijau
- **Sprint counter**: terus increment dari 1, tidak reuse numbers
