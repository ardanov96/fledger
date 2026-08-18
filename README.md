# FMCG Wallet — Hybrid Wallet Backend (Production-Grade)

[![CI](https://github.com/runut/fmcg-wallet/actions/workflows/ci.yml/badge.svg)](https://github.com/runut/fmcg-wallet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/runut/fmcg-wallet)](https://goreportcard.com/report/github.com/runut/fmcg-wallet)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)

> Sistem hybrid wallet **FMCG/F&B** Indonesia — double-entry ledger, receivables, collection routes, dan net-off calculation, dibangun dengan standar **production-grade** (bukan MVP).

---

## 🎯 Apa Ini?

Backend wallet yang dirancang untuk distributor FMCG / F&B Indonesia yang punya:

- **Banyak outlet/retailer** — yang sering telat setor ke HQ
- **Sales rep** yang collect tagihan di lapangan
- **Finance HQ** yang butuh visibility real-time tanpa nunggu laporan akhir bulan

**Masalah yang dipecahkan:**

1. Sales rep telat setor → gak ada audit trail yang trustworthy
2. Reconciliation manual → lambat & rawan human error
3. Gak ada real-time visibility → finance HQ buta sampai akhir bulan

**Solusi teknis:**

- Double-entry ledger (immutable entries) — audit-ready
- `SELECT ... FOR UPDATE` untuk concurrency safety
- Background workers (asynq) untuk aging & reconciliation
- Real-time WebSocket untuk dashboard updates
- Full observability stack (Prometheus + Loki + Tempo + Grafana)
- Multi-tenancy ready (Fase 5)

---

## �️ Tech Stack

| Layer | Pilihan | Kenapa |
|---|---|---|
| Bahasa | **Go 1.23+** | Concurrency, performance, single binary |
| HTTP Router | **chi** | Idiomatic, ringan, middleware-rich |
| Database | **PostgreSQL 16** + pgx + sqlc | SQL-first, locking-aware, audit-friendly |
| Cache/Broker | **Redis 7** | Cache + asynq broker |
| Event Broker | **NATS JetStream** | Lightweight, durable, replay-able |
| Auth | **JWT** + bcrypt | Stateless, scalable |
| Background Job | **asynq** | Retry-capable, Redis-backed |
| Validation | **go-playground/validator** | Standard |
| Logging | **log/slog** (stdlib) | Zero-dep, structured |
| Metrics | **Prometheus** | Standard industri |
| Tracing | **OpenTelemetry + Tempo** | Vendor-neutral |
| Frontend | **Next.js 15** + TypeScript | Modern, fast |
| UI | **Tailwind + shadcn/ui** | Cepat, rapi |
| Deployment | **Docker distroless** + VPS | Small image (~20MB) |

---

## 🚀 Quick Start

### Prerequisites

- Go 1.23+
- Docker + Docker Compose
- Node.js 20+ (untuk frontend)
- Make

### Setup

```bash
# Clone
git clone https://github.com/runut/fmcg-wallet.git
cd fmcg-wallet

# Copy env template
cp .env.example .env
# Edit .env — minimal: set JWT_SECRET (32+ chars)

# Start full local stack (postgres, redis, nats, observability)
make up

# Verify services
make ps

# Run migrations
make migrate-up

# (Optional) Seed demo data for E2E smoke test
make seed-local

# Run API
make run-api
```

### Frontend (Web Dashboard)

The lightweight web dashboard lives in `web/` (zero-dependency Node server
that serves a static SPA from `web/public/` and reverse-proxies `/v1/*` to
the API).

```bash
cd web
npm install
npm start
# Dashboard now at http://localhost:3000
```

If you prefer to skip the web UI entirely, you can interact with the API
via `curl` against `http://localhost:8080`.

API sekarang hidup di `http://localhost:8080`:

- `GET /healthz` — liveness probe
- `GET /readyz` — readiness probe (cek DB/Redis/NATS)
- `GET /version` — build info
- `GET /metrics` — Prometheus metrics
- `GET /v1/ping` — hello world
- `GET http://localhost:3000` — Grafana (admin/admin)
- `GET http://localhost:9090` — Prometheus
- `GET http://localhost:8025` — MailHog

### Teardown

```bash
make down            # stop containers, keep data
make down-volumes    # stop AND delete all data
```

---

## 📁 Project Structure

```
fmcg-wallet/
├── cmd/
│   ├── api/         # HTTP API server
│   ├── worker/      # Background job runner (asynq, outbox publisher)
│   └── migrator/    # CLI untuk database migrations
├── internal/
│   ├── platform/    # Cross-cutting: money, config, logger, errors, httpx
│   ├── domain/      # Business logic (pure, no infra deps)
│   │   ├── ledger/  # Account, Entry, Transaction interfaces
│   │   ├── audit/   # Audit trail
│   │   ├── invoice/ # Invoice & Credit Management
│   │   ├── period/  # Period close workflow
│   │   └── reconciler/ # Trial balance reconciler (Fase 1B)
│   ├── usecase/     # Application orchestration
│   ├── repository/  # Postgres implementations
│   ├── handler/     # HTTP handlers
│   ├── middleware/  # Auth, RBAC, audit, CORS
│   ├── auth/        # JWT + Casbin RBAC
│   ├── infra/       # External service clients
│   └── worker/      # Background workers (Reconciler ticker)
├── migrations/      # Versioned SQL migrations (10 files)
├── deployments/     # docker-compose, observability configs
├── docs/
│   ├── adr/         # Architecture Decision Records (4 ADRs)
│   ├── domain/      # Business glossary
│   ├── diagrams/    # C4 model (mermaid)
│   ├── runbooks/    # Operational playbooks
│   ├── interview/   # Q&A untuk interview
│   └── SPRINTS.md   # Sprint-by-sprint summary
├── scripts/         # Dev utilities
├── .github/         # CI/CD workflows
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── README.md
```

---

## 📚 Dokumentasi

- **[Sprint Summary](docs/SPRINTS.md)** — Sprint-by-sprint delivery log (Sprint 1-10 done)
- **[Roadmap Production-Grade](../roadmap-production-grade.md)** — 30-week execution plan (Fase 0-9 + 10)
- **[Tech Stack Detail](../tech-stack-fmcg-wallet-portfolio.md)** — Sprint-by-sprint technical detail
- **[Modules & Features](../modules-features-fmcg-wallet-portfolio.md)** — Domain modules & entities
- **[ADRs](docs/adr/)** — Kenapa kita pilih X bukan Y (4 ADRs)
- **[Domain Glossary](docs/domain/glossary.md)** — Istilah bisnis & teknis
- **[Contributing](CONTRIBUTING.md)** — Coding standards, workflow, DoD

---

## 🛣️ Status Roadmap

| Fase | Minggu | Status |
|---|---|---|
| 0 — Foundation Reset | 1-2 | ✅ Done (Sprint 1) |
| 1 — Financial Correctness | 3-5 | 🔄 In progress (1A/1B/1C done, 1D pending) |
| 2 — Security & RBAC | 6-8 | 🔄 In progress (2A RBAC, 2C Audit log, 2E JWT done) |
| 3 — Observability | 9-10 | 🔄 Infra-ready (3A done, 3B/C pending) |
| 4 — Scalability | 11-14 | 🔄 Partial (4B NATS infra ready) |
| 5 — Multi-Tenancy | 15-17 | 🔄 Partial (5A `tenant_id` di 10 tabel, RLS pending) |
| 6 — Integration | 18-20 | 🔄 Partial (6D API versioning done) |
| 7 — Testing & Quality | 21-23 | 🔄 Partial (7A property-based partial, 87+ unit tests) |
| 8 — Domain Enrichment | 24-26 | ⏳ Pending |
| 9 — Ops Excellence | 27-29 | ⏳ Pending |
| 10 — Differentiator | 30+ | � Ongoing |

**Progress: ~10/30 sprints complete (~33%)** setelah Sprint 10 (Reconciler & Trial Balance).

Lihat `docs/SPRINTS.md` untuk detail per-sprint, atau `roadmap-production-grade.md` di parent directory untuk fase-level breakdown.

---

## ✅ Apa yang Sudah Jadi (Sprint 1-10)

**Foundation (Sprint 1-2):**
- Repo, toolchain (Go 1.23+), strict linter, CI/CD (5-stage), Docker multi-stage distroless
- Postgres migrations, double-entry ledger dengan immutability triggers
- `TransferService` dengan `SELECT FOR UPDATE` deterministic lock ordering (ADR-0004)

**HTTP API + Security (Sprint 3-8):**
- 25 REST endpoints: accounts, transfers, invoices, payments, credit-limit, aging, period close, reconciler
- `Idempotency-Key` header pattern (Stripe-style)
- JWT auth (HS256, golang-jwt/jwt/v5) + Casbin RBAC dengan 5 role + 11 object
- Audit log (DB-level immutability + Postgres-backed)
- Invoice + Credit Management (atomic credit-limit, FIFO/manual payment, aging view)
- Hash chain tamper detection (SHA-256, blockchain-like audit)
- Period close two-step approval workflow + period-end snapshots

**Reconciler (Sprint 10):**
- Trial balance background job (ticker-based, 1h interval)
- Manual API trigger + per-account breakdown + history retention
- Optional hash-chain verification untuk tamper detection operational

**Observability (Fase 3A):**
- Prometheus + Loki + Tempo + Grafana (System Health dashboard)

---

## 🧪 Testing

```bash
make test              # run all tests with race detector
make test-cover        # run tests + open coverage report
make test-cover-check  # enforce 80% threshold
make lint              # golangci-lint strict
make security          # govulncheck + gosec
make verify            # full verification: fmt + vet + lint + test-cover
```

**Test stats (post-Sprint 10):**
- **87+ unit tests** di 24 test files
- Coverage: money, transfer (single + concurrent + property-based), invoice (14 scenarios), hashchain (7 scenarios), period (11 scenarios), reconciler (12 scenarios), audit, handler, middleware
- `go test -race` PASS on Linux CI

---

## 🐳 Deployment

Production deployment pakai single VPS (Hetzner CPX11, ~€3.85/bln) + Cloudflare. Estimated total: **Rp 60-80rb/bulan** untuk 12 bulan pertama.

Lihat `roadmap-production-grade.md` section "Deployment Architecture" untuk detail lengkap.

### Quick deploy (CI/CD)

Setiap push ke `main` akan:
1. Run CI (lint + test + security + build)
2. Build multi-arch Docker image (linux/amd64 + linux/arm64)
3. Push ke GitHub Container Registry (`ghcr.io/runut/fmcg-wallet`)

Manual deploy:
```bash
# On VPS
docker pull ghcr.io/runut/fmcg-wallet:latest
docker compose -f docker-compose.prod.yml up -d
```

---

## 🤝 Untuk Interviewer

Project ini sengaja dibangun untuk **defensibility di interview**. Beberapa highlight:

- **Money type** (`int64` minor units, larang `float64`) — instant credibility
- **Double-entry ledger** — domain knowledge yang finance-specific
- **`SELECT ... FOR UPDATE`** — concurrency safety dijelaskan dengan trade-off
- **Transaction abstraction** (`Tx` interface) — clean architecture yang bisa di-unit-test
- **Typed errors** — HTTP status mapping yang konsisten
- **Hash chain** ✅ implemented (Fase 1C) — tamper detection operational
- **Trial balance reconciler** ✅ implemented (Fase 1B) — background job + manual API + per-account breakdown
- **Period close with approval workflow** ✅ implemented (Fase 1A) — two-step + snapshot
- **Casbin RBAC with domains** ✅ implemented (Fase 2A) — multi-tenant role scoping
- **JWT + SecretProvider** ✅ implemented (Fase 2E core) — Vault-ready
- **Outbox pattern** (planned Fase 4A) — distributed system thinking
- **Multi-tenancy** (in progress Fase 5) — `tenant_id` di 10 tabel, RLS menyusul

Lihat `docs/SPRINTS.md` untuk detail delivery per sprint, atau `docs/interview/` (di-update per fase) untuk Q&A yang sudah disiapkan.

---

## 📜 License

MIT — see [LICENSE](LICENSE).

---

## 🙏 Acknowledgments

- **Stack** — Inspired by best practices from various production systems
- **Domain knowledge** — SAK EMKM (Indonesia), double-entry accounting fundamentals
- **Tools** — Go, Postgres, NATS, Grafana — all open source, all amazing

---

**Maintainer:** [@runut](https://github.com/runut)
**Status:** Active development — Sprint 10 (Fase 1B Reconciler done)
**Last updated:** 2026-08-13
