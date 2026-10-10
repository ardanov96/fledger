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
- **Sales rep** yang collect tagihan di lapangan (*canvassing*)
- **Finance HQ** yang butuh visibility real-time tanpa nunggu laporan akhir bulan

**Masalah yang dipecahkan:**

1. Sales rep telat setor → tidak ada audit trail yang trustworthy
2. Rekonsiliasi manual → lambat & rawan *human error*
3. Tidak ada *real-time visibility* → finance HQ buta sampai akhir bulan

**Solusi teknis:**

- **Double-entry ledger** (immutable entries) — audit-ready
- **`SELECT ... FOR UPDATE`** dengan deterministic lock ordering — concurrency safety tanpa deadlock
- **Cryptographic Hash Chaining (SHA-256)** — mendeteksi manipulasi database per baris mutasi
- **Background workers & Reconciler** — verifikasi otomatis neraca saldo & aging schedule
- **Multi-tenancy isolation** — isolasi data menggunakan PostgreSQL Row-Level Security (RLS)
- **Full observability stack** — Prometheus + Loki + Tempo + Grafana dengan W3C `traceparent`

---

## 🧭 9 Modul Utama Sistem (Web Dashboard & Core Engine)

Sistem FMCG Wallet mencakup **9 modul terpadu** yang saling terhubung untuk menjamin keabsahan transaksi finansial dari lapangan hingga tutup buku pusat:

| Modul | Deskripsi & Nilai Bisnis | Mekanisme Teknis Kunci |
|---|---|---|
| **[1. Dashboard](docs/modules/README.md#1--modul-dashboard-executive-summary--real-time-kpi)** | Ringkasan KPI eksekutif, total kas aset, piutang berjalan, net balance, dan alert tagihan terbuka. | Real-time agregasi saldo akun, preview umur piutang |
| **[2. Accounts](docs/modules/README.md#2--modul-accounts-bagan-akun--chart-of-accounts)** | Bagan akun (*Chart of Accounts*) untuk kasir gudang, tas kas salesman, piutang toko, dll. | 9 tipe akun (`cash`, `sales_rep`, `customer`, `hq`, dll.), isolasi multi-tenant |
| **[3. Transfers](docs/modules/README.md#3--modul-transfers-mutasi-dana--double-entry-ledger)** | Mutasi saldo antar akun & penyetoran tagihan salesman ke HQ (*Settlement*). | Double-entry invariant, `SELECT FOR UPDATE`, SHA-256 hash chain, Idempotency-Key |
| **[4. Invoices](docs/modules/README.md#4--modul-invoices-faktur-penjualan--piutang-dagang)** | Faktur penjualan B2B, pelunasan pembayaran cicil/lunas, dan kontrol credit limit toko. | State machine (`open` ➔ `partial` ➔ `paid` / `overdue`), validasi credit limit |
| **[5. Aging](docs/modules/README.md#5--modul-aging-jadwal-umur-piutang--ar-aging-schedule)** | Jadwal umur piutang (*AR Aging Schedule*) untuk mitigasi risiko kredit macet. | 4 bucket (0–30, 31–60, 61–90, >90 hari / Bad Debt) |
| **[6. Periods](docs/modules/README.md#6--modul-periods-periode-akuntansi--tutup-buku)** | Siklus tutup buku bulanan (*Month-End Close*) untuk mencegah transaksi susulan masa lalu. | Maker-checker two-step approval (`Request` ➔ `Approve`) + *frozen balance snapshot* |
| **[7. Reconciler](docs/modules/README.md#7--modul-reconciler-rekonsiliasi-buku-besar--audit-kriptografi)** | Validasi kepatuhan neraca saldo (Debit = Kredit) & audit integritas mutasi database. | Trial balance check (`BALANCED`/`IMBALANCED`) + audit SHA-256 (`TAMPERED`) |
| **[8. Currencies & FX](docs/modules/README.md#8--modul-currencies--fx-master-mata-uang--konversi-valas)** | Master valuta asing dan kalkulator simulasi kurs konversi valas interaktif. | Base currency `IDR`, fixed rate snapshot per transaksi, tombol swap & preset chips |
| **[9. Audit Log](docs/modules/README.md#9--modul-audit-audit-log-trail-worm-style)** | Jejak rekam aktivitas keamanan dan kepatuhan finansial anti-hapus (WORM-style). | Immutable append-only audit log, zero update/delete, IP & User-Agent capture |

> 📖 **Dokumentasi modul lengkap, diagram arsitektur, dan alur operasional dapat dibaca di [Panduan Modul](docs/modules/README.md).**

---

## 🧩 Ekosistem & Roadmap Microservices (FLEDGER OS)

Sistem ini merupakan fondasi inti (**Fledger Core**) dari ekosistem **FLEDGER OS** (*The Ledger-First Operating System for Distribution & Supply Chain*). Dibangun dengan Clean Architecture & Domain-Driven Design (DDD), seluruh 6 microservice kini telah selesai (**100% Production-Ready**) dan terhubung via API Gateway & Docker Compose:

- **🟢 Fledger Core** *(Port :8081 / Gateway: `/core/`)*: Core Financial Ledger, Double-Entry Engine, AR Aging Schedule, SHA-256 Hash Chains, dan Settlement Vault.
- **🚚 Fledger Fleet** *(Port :8082 / Gateway: `/fleet/`)*: Manajemen logistik armada, dispatching trip, Surat Jalan (DO), dan Proof of Delivery (POD) digital touch canvas ([GitHub](https://github.com/ardanov96/fledger-fleet)).
- **💳 Fledger Pay** *(Port :8083 / Gateway: `/pay/`)*: B2B Payment Gateway Adapter untuk integrasi Virtual Account (BCA, Mandiri, BRI) & QRIS Dinamis B2B dengan auto-settlement webhook ([GitHub](https://github.com/ardanov96/fledger-pay)).
- **📱 Fledger Force** *(Port :8084 / Gateway: `/force/`)*: Sales Force Automation (SFA) PWA, verifikasi GPS Haversine radius 100m, Mobile Cash Collection, dan Anti-Cash Kitting Daily Settlement ([GitHub](https://github.com/ardanov96/fledger-force)).
- **📦 Fledger Order** *(Port :8085 / Gateway: `/order/`)*: B2B Order Management System (OMS), katalog grosir multi-tier, alokasi stok gudang, dan Hard Credit Limit Gate ([GitHub](https://github.com/ardanov96/fledger-order)).
- **🔔 Fledger Dunning** *(Port :8086 / Gateway: `/dunning/`)*: Otomasi penagihan piutang via WhatsApp 5-tahap cadence, anti-ban jitter engine, PDF e-Statement generator, dan self-healing webhook loop ([GitHub](https://github.com/ardanov96/fledger-dunning)).

> 🗺️ **Dokumentasi lengkap arsitektur microservices, solusi 5 masalah industri FMCG, kontrak integrasi REST/outbox, dan orkestrasi master compose dapat dibaca di [Roadmap Microservices Fledger OS](docs/architecture/roadmap-microservices.md) dan [Master Monorepo Guide](../README.md).**

---

## 🛠️ Tech Stack

| Layer | Pilihan | Kenapa |
|---|---|---|
| Bahasa | **Go 1.23+** | Concurrency, performance, single static binary |
| HTTP Router | **chi** | Idiomatic, ringan, middleware-rich |
| Database | **PostgreSQL 16** + pgx + sqlc | SQL-first, locking-aware, audit-friendly, Row-Level Security |
| Cache/Broker | **Redis 7** | Cache + asynq broker |
| Event Broker | **NATS JetStream** | Lightweight, durable, replay-able messaging |
| Auth & Security | **JWT + Refresh Token Rotation** + bcrypt | Stateless, defense vs token theft, multi-tenant RBAC |
| Background Job | **asynq** + Ticker Worker | Retry-capable, Redis-backed |
| Validation | **go-playground/validator** | Standar industri Go |
| Logging | **log/slog** (stdlib) | Zero-dep, structured JSON |
| Metrics | **Prometheus** | Standar industri monitoring |
| Tracing | **OpenTelemetry + Tempo** | W3C `traceparent` vendor-neutral |
| Frontend | **Single Page App (SPA)** | Zero-bundle, lightning fast, lightweight Vanilla JS |
| UI | **Custom Responsive CSS** | Clean modern design system, interactive states, zero npm bloat |
| Deployment | **Docker distroless** + VPS | Small image (~20MB) |

---

## 🚀 Quick Start

### Prerequisites

- Go 1.23+
- Docker + Docker Compose
- Node.js 20+ (untuk frontend dev proxy)
- PostgreSQL 16/17 (jika menjalankan lokal)

### 1. Setup & Jalankan Stack Backend

```bash
# Clone repository
git clone https://github.com/ardanov96/fledger.git
cd fledger

# Salin konfigurasi environment
cp .env.example .env
# Edit .env — minimal: set JWT_SECRET (32+ karakter)

# Jalankan database & dependensi via Docker Compose
docker compose up -d

# Jalankan migrasi database
go run ./cmd/migrator up
# Atau: make migrate-up

# (Opsional) Jalankan data seeder demo yang saling terhubung
go run ./scripts/seed-rich-data.go
# Atau via SQL: psql -U fmcg -d fmcg_wallet -f scripts/seed-rich-connected-data.sql

# Jalankan API server (port 8081 / 8080)
go run ./cmd/api
# Atau: make run-api
```

### 2. Jalankan Frontend Web Dashboard

Dashboard web dijalankan melalui server reverse-proxy Node.js yang ringan (zero npm dependencies):

```bash
cd web
npm start
# Dashboard aktif di http://localhost:3000
```

### 3. Akun Login Demo

Setelah melakukan *seeding data*, gunakan kredensial berikut untuk masuk ke dashboard:
- **Tenant ID**: `00000000-0000-0000-0000-000000000001`
- **Username (Admin HQ)**: `33333333-3333-3333-3333-333333333333`
- **Password**: `DemoTest1234!`

Endpoints Healthcheck & Metrics:
- `GET /healthz` — Liveness probe
- `GET /readyz` — Readiness probe (cek DB/Redis/NATS)
- `GET /version` — Informasi versi & build
- `GET /metrics` — Prometheus metrics

---

## 📁 Struktur Direktori

```
fledger/
├── cmd/
│   ├── api/         # HTTP API server
│   ├── worker/      # Background job runner (asynq, outbox publisher)
│   └── migrator/    # CLI untuk database migrations
├── internal/
│   ├── platform/    # Cross-cutting: money, config, logger, errors, httpx, tenantctx
│   ├── domain/      # Pure business logic & interfaces (zero infra dependencies)
│   │   ├── ledger/  # Account, Entry, Transaction
│   │   ├── audit/   # WORM Audit trail
│   │   ├── invoice/ # Invoice, Payment & Credit Management
│   │   ├── period/  # Period close workflow & snapshots
│   │   ├── reconciler/ # Trial balance & hash chain verifier
│   │   ├── currency/# Multi-currency registry & FX conversions
│   │   └── auth/    # Token lifecycle, refresh rotation, RBAC
│   ├── usecase/     # Application services & business orchestration
│   ├── repository/  # PostgreSQL pgx adapters (RLS-enforced)
│   ├── handler/     # HTTP handlers & DTO mapping
│   ├── middleware/  # Auth, RBAC, tenant context, audit, CORS, rate-limiting
│   └── worker/      # Background tickers & workers
├── migrations/      # 16 versioned SQL migration files
├── web/             # Single Page Application Dashboard
│   ├── server.js    # Zero-dependency Node static & reverse-proxy server
│   └── public/      # index.html, styles.css, app.js
├── docs/            # Dokumentasi lengkap
│   ├── modules/     # Panduan 9 modul dashboard & relasi data
│   ├── architecture/# C4 diagrams & sequence flows
│   ├── adr/         # 8 Architecture Decision Records
│   ├── api/         # Endpoint API documentation
│   └── SPRINTS.md   # Catatan perkembangan sprint
├── scripts/         # Utilitas database seeder & scripts pengujian
├── Makefile
├── Dockerfile
└── README.md
```

---

## 🧪 Pengujian & Kualitas Kode (Quality & Testing)

```bash
make test              # Menjalankan unit tests dengan race detector
make test-cover        # Menjalankan tests & mencetak laporan coverage
make test-cover-check  # Memastikan threshold coverage minimum tercapai
make lint              # golangci-lint strict check
make security          # govulncheck + gosec scanning
make verify            # Verifikasi lengkap: fmt + vet + lint + test
```

**Statistik Pengujian:**
- **120+ unit tests** di berbagai domain use case
- **15 property-based tests** memvalidasi *invariants* (kekekalan saldo, keutuhan hash chain)
- **5 skenario integration test E2E** dengan database PostgreSQL riil
- `go test -race` lulus tanpa race condition

---

## 🤝 Keunggulan untuk Evaluasi Teknis / Interview

Arsitektur project ini sengaja dirancang dengan standar *production-grade* yang memiliki pertahanan teknis kuat:

- **Money Type Khusus** (`int64` minor units, melarang penggunaan `float64`) untuk menjamin kepatuhan akuntansi tanpa masalah pembulatan desimal.
- **Double-Entry Bookkeeping** dengan pembuktian matematis Debit = Kredit di setiap baris mutasi.
- **`SELECT ... FOR UPDATE` Deterministic Lock Ordering** mencegah *deadlock* saat multi-user bertransaksi pada akun yang sama.
- **Cryptographic Hash Chain (SHA-256)** per mutasi buku besar untuk *tamper-evident audit*.
- **PostgreSQL Row-Level Security (RLS)** untuk isolasi data multi-tenant di level basis data.
- **Two-Step Approval Tutup Buku (Maker-Checker)** dengan pembekuan saldo permanen (*Period Snapshot*).
- **Opaque Refresh Token Rotation** dengan deteksi penyalahgunaan token otomatis (*token theft detection*).

---

## 📜 Lisensi

Didistribusikan di bawah lisensi MIT. Lihat [LICENSE](LICENSE) untuk informasi lebih lanjut.

---

**Status:** Active development — Production-Grade Architecture  
**Repository:** [https://github.com/ardanov96/fledger](https://github.com/ardanov96/fledger)
