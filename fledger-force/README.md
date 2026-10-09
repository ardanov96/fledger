# Fledger Force — Sales Force Automation & Anti-Cash Kitting Engine

> **Service**: `fledger-force`
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)
> **Port**: `:8084`
> **Status**: ✅ **Production-Ready (Sprint 1–5 selesai, 100% test pass)**
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router + CORS, Geofencing Haversine, embedded PWA

---

## 🎯 Tanggung Jawab Layanan

1. **Rute Kunjungan Terverifikasi GPS (*Geofenced Beat Plan*)** — Memastikan salesman berada dalam radius 100m dari toko saat check-in menggunakan rumus **Haversine**.
2. **Mobile Cash Collection & Tanda Terima Digital** — Menerbitkan kwitansi resmi `RCP-YYYYMM-XXXXX` instan + payload notifikasi WhatsApp ke pemilik toko.
3. **Pencegahan Fraud Uang Kas (*Anti-Cash Kitting*)** — Setiap pelunasan langsung memicu mutasi **double-entry** di Fledger Core:
   $$\text{Debit: Account:Salesman-Wallet} \quad | \quad \text{Credit: Account:Customer-AR}$$
4. **Daily Settlement Lock (Tutup Kasir Sore Hari)** — Kasir memvalidasi setoran fisik, reset saldo salesman ke 0, lock salesman jika ada **discrepancy**:
   $$\text{Debit: Account:HQ-Cash} \quad | \quad \text{Credit: Account:Salesman-Wallet}$$
5. **Interactive PWA Portal** — Vanilla JS portal mobile-friendly untuk salesman + kasir (zero npm), di-embed via `//go:embed all:files`.

---

## 📅 Sprint Progress

| Sprint | Status | Highlight |
|---|---|---|
| 1 — Pondasi DB & Health | ✅ | 9 tabel + `force_schema_migrations`; `/healthz` + `/readyz` (DB ping) |
| 2 — Rep/Store/Beat Plan/Check-In | ✅ | Master CRUD + Haversine geofencing + out-of-radius audit |
| 3 — Cash Collection Engine | ✅ | Receipt `RCP-YYYYMM-XXXXX` + WA payload + `max_cash_limit` guard (422) |
| 4 — EOD Settlement + Outbox | ✅ | Inquiry + settlement + discrepancy lock + outbox → Core |
| 5 — PWA + E2E | ✅ | Embedded PWA + `e2e-flow.ps1` 100% PASSED (11 steps) |

---

## 🤖 Panduan untuk AI Coding Agent / Developer

Baca dokumen di folder `docs/`:
- 📄 [AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md) — Blueprint eksekusi.
- 📄 [ROADMAP-PAY.md](docs/ROADMAP-PAY.md) — Rincian 5 sprint + DoD.
- 📄 [DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql) — DDL lengkap.
- 📄 [API-SPECIFICATION.md](docs/API-SPECIFICATION.md) — Kontrak REST + webhook.
- 📄 [SIMULATOR-AND-SETTLEMENT-GUIDE.md](docs/SIMULATOR-AND-SETTLEMENT-GUIDE.md) — Alur operasional + rekonsiliasi.

---

## ⚙️ Konfigurasi `.env`

```ini
APP_ENV=development
PORT=8084
DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_force?sslmode=disable
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
FLEDGER_CORE_API_KEY=dev-force-key-fledger-2026
JWT_SECRET=super-secret-fledger-force-jwt-key-minimum-32-chars!
TOKEN_TTL=24h
OUTBOX_POLL_INTERVAL=3s
OUTBOX_REQUEST_TIMEOUT=5s
OUTBOX_MAX_ATTEMPTS=10
```

---

## 🚀 Quick Start

```powershell
# 1. Setup database (opsional, script `migrations/000001_init_force.sql` bisa dijalankan manual)
psql -U fmcg -d fledger_force -f src/migrations/000001_init_force.sql

# 2. Jalankan unit + integration tests
.\src\scripts\run-tests.ps1

# 3. Jalankan API server
.\src\scripts\start-api.cmd

# 4. (terminal lain) E2E smoke test
.\src\scripts\e2e-flow.ps1

# 5. Buka PWA di browser
#    http://localhost:8084/
```

---

## 📁 Struktur Folder

```text
fledger-force/
├── docs/                                 <-- Specs, schema, roadmap
├── src/
│   ├── cmd/
│   │   ├── api/                          <-- HTTP server
│   │   └── migrator/                     <-- SQL migrator CLI
│   ├── internal/
│   │   ├── auth/jwt/                     <-- HS256 sign/verify
│   │   ├── config/                       <-- env loader
│   │   ├── domain/                       <-- rep, store, beatplan, visit, collection, settlement, outbox, audit
│   │   ├── handler/                      <-- REST handlers
│   │   ├── integration/coreclient/       <-- Fledger Core HTTP client
│   │   ├── middleware/                   <-- RequireAuth
│   │   ├── platform/                     <-- errors, httpx, log, geo (Haversine)
│   │   ├── repository/postgres/          <-- pgx-based repos
│   │   ├── usecase/                      <-- business services + outbox worker
│   │   └── webui/                        <-- embedded static PWA
│   ├── migrations/
│   │   └── 000001_init_force.sql
│   ├── go.mod / go.sum
│   └── scripts/                          <-- helper scripts (cmd)
├── web/                                  <-- source for PWA
│   ├── index.html
│   ├── style.css
│   ├── force-client.js
│   └── app.js
├── .env.example
└── README.md
```

---

## 🧪 Pengujian

| Test | Lokasi | Status |
|---|---|---|
| Haversine (Monas reference, within/outside radius) | `internal/platform/geo/haversine_test.go` | ✅ PASS |
| Happy path: rep → store → beat plan → check-in → collect → outbox → EOD | `internal/integration/integration_test.go` | ✅ PASS |
| Geofence out-of-radius flagged + audit row | same | ✅ PASS |
| `max_cash_limit` guard returns 422 | same | ✅ PASS |
| EOD discrepancy locks rep with `SETTLEMENT_LOCKED` | same | ✅ PASS |
| Receipt format `RCP-YYYYMM-XXXXX` | same | ✅ PASS |
| EOD format `EOD-YYYYMM-XXXXX` | same | ✅ PASS |
| WA receipt payload includes receipt + amount | same | ✅ PASS |
| E2E (live HTTP) | `scripts/e2e-flow.ps1` | ✅ PASS (11/11) |

**Coverage**:
- `internal/platform/geo`: 3 tests (Haversine + bounds)
- `internal/integration`: 6 tests covering full rep→store→visit→collect→EOD flow

```
==> internal/platform/geo    [OK]
==> internal/integration    [OK]
```

---

## 🛡 Guardrails Terpenuhi

- ✅ **Integritas uang**: semua nominal `BIGINT` minor units (Rp integer, tanpa floating-point).
- ✅ **Geofence Haversine**: rumus matematika sesuai brief §3 — toko > 100m ditandai `geofence_verified=false` + audit row.
- ✅ **Plafon kas**: `max_cash_limit` enforced, tolak dengan HTTP 422.
- ✅ **Idempotency**: `pay_transactions` UNIQUE; outbox retries dengan exponential backoff 3s → 30min.
- ✅ **Outbox transaksional**: insert bayar + insert outbox event dalam satu DB tx.
- ✅ **Discrepancy lock**: EOD dengan selisih → salesman `SETTLEMENT_LOCKED` + audit.
- ✅ **CORS terbuka**: `cors.AllowAllOrigins` mounted di root.
- ✅ **Zero Secret Hardcoding**: semua kunci dari env.

---

## 🔌 Integrasi dengan Fledger Core

Kontrak `POST /v1/transfers` mengikuti **Fledger Core sebenarnya**:

```http
POST /v1/transfers
Idempotency-Key: <pay_transaction_id>

{
  "from_account_id": "ACC_CUSTOMER_AR",
  "to_account_id":   "ACC_SALES_WALLET_BUDI",
  "amount_minor":    2500000,
  "currency":        "IDR",
  "description":     "Pembayaran kas lapangan Toko Sumber Rezeki (RCP-202610-516C6)"
}
```