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
| 23 | [Tech Debt Foundation](#sprint-23-tech-debt-foundation-2026-09-20) | — | 2026-09-20 | ✅ Done |
| 24 | [Transactional Outbox + NATS Subscriber](#sprint-24--transactional-outbox--nats-subscriber-2026-09-21) | 4A | 2026-09-21 | ✅ Done |
| 25 | [Aging Recalculator Worker](#sprint-25--aging-recalculator-worker-2026-09-22) | 4D | 2026-09-22 | ✅ Done |
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

## Sprint Backlog (Planned)

| Sprint | Title | Fase | Source | Effort |
|---|---|---|---|---|
| 26 | FX Rate Auto-Refresh | Fase 1D follow-up | ADR-0005 follow-up | 1 week |
| 27 | Notification Dispatcher | Fase 8 | cmd/worker/main.go:71 TODO + Sprint 24 subscriber | 1 week |
| 28 | Fraud Flag Scanner | Fase 8 | cmd/worker/main.go:72 TODO + Sprint 24 subscriber | 1 week |
| 29 | Login Attempt Partitioning | Fase 2B | Migration 000013 follow-up | 2 days |
| 30 | Secret Rotation Enforcement | Fase 2E | runbooks/secret-rotation.md TODO | 2 days |
| 31 | OTel SDK Migration | Fase 3B | Sprint 18 follow-up | 3 days |
| 32 | RLS for `user_credentials` | Fase 5A | ADR-0006:78 TODO | 2 days |
| 33 | Mutation Testing + Chaos | Fase 7 | Roadmap | 1 week |
| TBD | Frontend Next.js Migration | Fase 6 | `web/README.md` limitations | 2 weeks |
| TBD | FOR UPDATE SKIP LOCKED di outbox FetchUnpublished | 4A follow-up | Sprint 24 follow-up | 1 day |
| TBD | More event types (invoice.created, period.closed, payment.recorded) | 4A | Sprint 24 follow-up | 1 week |
| TBD | JetStream migration untuk durable subscription | 4A | Sprint 24 follow-up | 1 week |

---

## Cumulative Stats (post-Sprint 22B)

| Metric | Value | Source |
|---|---|---|
| Total sprints completed | 22B | this file |
| Total LOC | ~18,000 | docs/index.md |
| Go files (production) | ~100 | docs/index.md |
| Go files (test) | ~30 | docs/index.md |
| Migrations | 16 | migrations/ folder |
| ADRs | 8 | docs/adr/ folder |
| REST endpoints | 36+ | docs/api/overview.md |
| Use cases | 9 | internal/usecase/ folder |
| Repositories | 11 | internal/repository/postgres/ folder |
| Unit tests | 120+ (incl. 15 property-based) | docs/index.md |
| Integration scenarios | 5 (build tag `integration`) | Sprint 17 |
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
