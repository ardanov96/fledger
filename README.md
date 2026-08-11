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

## 🏗️ Tech Stack

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

# Run migrations (when migrator is implemented in Fase 0)
make migrate-up

# Run API
make run-api
```

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
│   │   └── ledger/  # Account, Entry, Transaction interfaces
│   ├── usecase/     # Application orchestration (Fase 0+)
│   ├── repository/  # Postgres implementations (sqlc-generated)
│   ├── handler/     # HTTP handlers (Fase 0+)
│   ├── middleware/  # Auth, rate limit, CORS (Fase 0+)
│   └── infra/       # External service clients (Fase 0+)
├── migrations/      # Versioned SQL migrations
├── deployments/     # docker-compose, observability configs
├── docs/
│   ├── adr/         # Architecture Decision Records
│   ├── domain/      # Business glossary
│   ├── diagrams/    # C4 model (mermaid)
│   ├── runbooks/    # Operational playbooks
│   └── interview/   # Q&A untuk interview
├── web/             # Next.js frontend
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

- **[Roadmap Production-Grade](../roadmap-production-grade.md)** — 30-week execution plan (Fase 0-9 + 10)
- **[Tech Stack Detail](../tech-stack-fmcg-wallet-portfolio.md)** — Sprint-by-sprint technical detail
- **[Modules & Features](../modules-features-fmcg-wallet-portfolio.md)** — Domain modules & entities
- **[ADRs](docs/adr/)** — Kenapa kita pilih X bukan Y
- **[Domain Glossary](docs/domain/glossary.md)** — Istilah bisnis & teknis
- **[Contributing](CONTRIBUTING.md)** — Coding standards, workflow, DoD

---

## 🛣️ Status Roadmap

| Fase | Minggu | Status |
|---|---|---|
| 0 — Foundation Reset | 1-2 | 🚧 In progress |
| 1 — Financial Correctness | 3-5 | ⏳ Pending |
| 2 — Security & RBAC | 6-8 | ⏳ Pending |
| 3 — Observability | 9-10 | ⏳ Pending |
| 4 — Scalability | 11-14 | ⏳ Pending |
| 5 — Multi-Tenancy | 15-17 | ⏳ Pending |
| 6 — Integration | 18-20 | ⏳ Pending |
| 7 — Testing & Quality | 21-23 | ⏳ Pending |
| 8 — Domain Enrichment | 24-26 | ⏳ Pending |
| 9 — Ops Excellence | 27-29 | ⏳ Pending |
| 10 — Differentiator | 30+ | ⏳ Ongoing |

Lihat `roadmap-production-grade.md` di parent directory untuk detail lengkap.

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
- **Hash chain (planned Fase 1C)** — wow factor untuk audit
- **Outbox pattern (planned Fase 4A)** — distributed system thinking
- **Multi-tenancy (planned Fase 5)** — SaaS-ready architecture

Lihat `docs/interview/` (di-update per fase) untuk Q&A yang sudah disiapkan.

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
**Status:** Active development — Fase 0 (Foundation)
**Last updated:** 2026-08-10
