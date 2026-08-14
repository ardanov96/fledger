# FMCG Wallet

> **Production-grade hybrid wallet backend** untuk distributor FMCG/F&B Indonesia — double-entry ledger, receivables, collection routes, dan real-time net-off calculation.

[![CI](https://github.com/runut/fmcg-wallet/actions/workflows/ci.yml/badge.svg)](https://github.com/runut/fmcg-wallet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/runut/fmcg-wallet)](https://goreportcard.com/report/github.com/runut/fmcg-wallet)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go 1.23+](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)

[:fontawesome-brands-github: View on GitHub](https://github.com/runut/fmcg-wallet){ .md-button .md-button--primary }
[Get Started](architecture/overview.md){ .md-button }

---

## 🎯 What is this?

Backend wallet untuk distributor **FMCG/F&B Indonesia** yang menghadapi masalah nyata:

- **Banyak outlet/retailer** — yang sering telat setor ke HQ
- **Sales rep** yang collect tagihan keliling dari pintu ke pintu
- **Finance HQ** yang butuh **real-time visibility**, gak mungkin nunggu laporan akhir bulan

### Masalah yang diselesaikan

1. Sales rep telat setor → tidak ada audit trail yang trustworthy
2. Reconciliation manual → lambat & rawan human error
3. Tidak ada real-time visibility → finance HQ buta sampai akhir bulan

### Solusi teknis

- **Double-entry ledger** dengan immutable entries — audit-ready
- **`SELECT ... FOR UPDATE`** dengan deterministic lock ordering — concurrency-safe
- **Background workers** untuk aging calculation & reconciliation
- **JWT + opaque refresh tokens** dengan reuse-detection — defense vs token theft
- **TOTP MFA** (RFC 6238) + bcrypt + brute-force protection
- **Postgres Row-Level Security** untuk tenant isolation
- **Full observability stack** (Prometheus + Loki + Tempo + Grafana)

---

## 🏗️ Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| Bahasa | **Go 1.23+** | Concurrency, performance, single binary, easy deploy |
| HTTP Router | **chi** | Idiomatic, ringan, middleware ecosystem yang matang |
| Database | **PostgreSQL 16** + pgx | SQL-first, locking-aware, audit-friendly |
| Cache/Broker | Redis 7 | Cache + future asynq broker |
| Auth | JWT (HS256) + bcrypt + opaque refresh | Stateless + brute-force safe |
| Validation | go-playground/validator | De-facto standard |
| Logging | log/slog (stdlib) | Zero-dep, structured |
| Metrics | Prometheus | Standard industri |
| Tracing | W3C traceparent + Tempo | Vendor-neutral, lightweight |
| Deployment | Docker distroless (~20MB) | Small image, fast cold start |

---

## ✨ Highlights (Sprint 1-18)

Production-grade backend yang sudah include:

- **Sprint 2-4**: Core ledger (double-entry + idempotency + audit)
- **Sprint 8**: Invoice + credit management + hash chain tamper detection
- **Sprint 9**: Period close with two-step approval + snapshots
- **Sprint 10**: Reconciler background job (trial balance + hash chain check)
- **Sprint 11**: Collection routes (sales rep field workflow)
- **Sprint 12**: Multi-currency dengan FX rate snapshot
- **Sprint 13**: Refresh token rotation + TOTP MFA + brute force protection
- **Sprint 14**: Rate limiting (token bucket)
- **Sprint 15**: Field-level authz + tenant RLS
- **Sprint 18**: W3C trace propagation + k6 load test

Lihat [Sprint Log](SPRINTS.md) untuk timeline lengkap.

---

## 🚀 Quick Start

### Prerequisites

- Go 1.23+
- Docker + Docker Compose
- Make

### Setup

```bash
git clone https://github.com/runut/fmcg-wallet.git
cd fmcg-wallet

cp .env.example .env
# Edit .env — minimal: set JWT_SECRET (32+ chars)

# Start full local stack (postgres, redis, nats, observability)
make up

# Run migrations
make migrate-up

# Run API
make run-api
```

API listen di `http://localhost:8080`:

- `GET /healthz` — liveness probe
- `GET /readyz` — readiness probe (cek DB/Redis/NATS)
- `GET /version` — build info
- `GET /metrics` — Prometheus metrics
- `GET /v1/ping` — hello world
- `GET /v1/accounts` — list accounts (auth required)
- `GET http://localhost:3000` — Grafana (admin/admin)
- `GET http://localhost:9090` — Prometheus

Lihat [API Reference](api/overview.md) untuk endpoint lengkap.

### Testing

```bash
make test            # semua test dengan race detector
make test-cover      # dengan coverage report
make test-cover-check # enforce 80% coverage threshold
make lint            # golangci-lint (strict)
make security        # govulncheck + gosec
make verify          # full check (fmt + vet + lint + test + coverage)
```

---

## 📚 Documentation Sections

<div class="grid cards" markdown>

-   :material-architecture: **[Architecture](architecture/overview.md)**

    C4 diagrams, sequence flows, tech stack rationale

-   :material-file-document-multiple: **[ADRs](adr/index.md)**

    5 architectural decision records with trade-offs

-   :material-api: **[API Reference](api/overview.md)**

    All 36+ endpoints with request/response examples

-   :material-run-fast: **[Runbooks](runbooks/integration-tests.md)**

    Production operations: backup, rotation, observability, incident

-   :material-school: **[Interview Prep](interview/index.md)**

    Q&A for fintech / distributed systems / security interviews

-   :material-rocket-launch: **[Sprint Log](SPRINTS.md)**

    Complete delivery timeline from Sprint 1 to current

</div>

---

## � Project Stats (post-Sprint 18)

| Metric | Count |
|---|---|
| Go files (production) | ~100 |
| Go files (test) | ~30 |
| Total lines of code | ~17,000 |
| Migrations | 14 |
| ADRs | 5 |
| REST endpoints | 36+ |
| Use cases | 9 |
| Repositories | 11 |
| Unit tests | 117+ (incl. 6 property-based) |
| Build | `go build ./...` PASS |
| Race detector | `go test -race` PASS |

---

## 🛡️ License

MIT — lihat [LICENSE](https://github.com/runut/fmcg-wallet/blob/main/LICENSE) untuk detail.

## 🤝 Contributing

Lihat [CONTRIBUTING.md](https://github.com/runut/fmcg-wallet/blob/main/CONTRIBUTING.md) untuk development setup, coding standards, dan Definition of Done.
