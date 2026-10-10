# Fledger Fleet — Fleet Logistics & Proof of Delivery Engine

> **Service**: `fledger-fleet`  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for Distribution & Supply Chain*)  
> **Port**: `:8082`  
> **Gateway Route**: `http://localhost:80/fleet/`  
> **Status**: ✅ **Production-Ready (Sprint 1–5 selesai, 100% test pass)**  
> **Standalone Repository**: [ardanov96/fledger-fleet](https://github.com/ardanov96/fledger-fleet)

---

## 🎯 Tentang Service Ini

**Fledger Fleet** adalah microservice pengelola operasional logistik, armada pengiriman, Surat Jalan (*Delivery Order / DO*), dan **Digital Proof of Delivery (POD)** dalam ekosistem **FLEDGER OS**.

Service ini dirancang untuk menyelesaikan sengketa barang rusak antara supir, toko, dan salesman dengan cara:
1. Menangkap bukti penerimaan fisik di tempat (tanda tangan digital penerima & foto barang rusak).
2. Menghitung otomatis nominal bersih barang yang benar-benar diterima toko.
3. Mengirimkan sinyal penyelesaian pengantaran secara otomatis ke **Fledger Core API** (`POST /v1/invoices`) agar faktur resmi langsung terbit dengan nominal yang tepat tanpa perlu revisi nota manual.

---

## ⚙️ Status Implementasi (per Sprint)

| Sprint | Status | Highlight |
|---|---|---|
| 1 — Setup & DB | ✅ | Schema lengkap, seeder demo, health check |
| 2 — Master + Trip Dispatcher | ✅ | Validasi ON_TRIP + kapasitas muatan |
| 3 — DO + Digital POD | ✅ | Validasi `qty_rejected > 0` ⇒ wajib foto & `rejection_reason` |
| 4 — Fledger Core sync | ✅ | Outbox pattern + exponential backoff + `Idempotency-Key: <DO UUID>` |
| 5 — Verifikasi E2E | ✅ | Unit tests + integration tests + E2E script |

---

## 🤖 Panduan untuk AI Coding Agent / Developer

Jika Anda adalah AI Agent yang ditugaskan untuk mengerjakan atau melanjutkan service ini:

1. **Buka & Baca Dokumen Spesifikasi Utama**:  
   👉 [`docs/AGENT-EXECUTION-BRIEF.md`](docs/AGENT-EXECUTION-BRIEF.md)
2. **Skema Basis Data (DDL)**:  
   👉 [`docs/DATABASE-SCHEMA.sql`](docs/DATABASE-SCHEMA.sql)
3. **Kontrak Endpoint REST API**:  
   👉 [`docs/API-SPECIFICATION.md`](docs/API-SPECIFICATION.md)
4. **Roadmap & Target Sprint**:  
   👉 [`docs/ROADMAP-FLEET.md`](docs/ROADMAP-FLEET.md)

---

## ⚙️ Konfigurasi Lingkungan (`.env`)

Salin berkas template `.env.example` menjadi `.env`:

```bash
cp .env.example .env
```

Variabel konfigurasi yang dibutuhkan:
```ini
PORT=8082
DATABASE_URL=postgres://postgres:postgres@localhost:5432/fledger_fleet?sslmode=disable
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
FLEDGER_CORE_API_KEY=fledger-fleet-service-key
JWT_SECRET=super-secret-key-32-characters-minimum-fleet
TOKEN_EXPIRY_HOURS=24
OUTBOX_POLL_INTERVAL_SEC=15
OUTBOX_REQUEST_TIMEOUT_SEC=10
OUTBOX_MAX_ATTEMPTS=8
```

---

## 🚀 Quick Start

### Opsi A: Menjalankan via Local Host
```powershell
# 1. Setup database (membuat DB, menjalankan semua migrasi, seeder)
.\src\scripts\setup-db.cmd

# 2. Jalankan unit + integration tests
.\src\scripts\run-tests.ps1

# 3. Jalankan API server
.\src\scripts\start-api.cmd

# 4. (terminal lain) Jalankan E2E smoke test
.\src\scripts\e2e-flow.ps1

# 5. Akses Web Portal
#    http://localhost:8082/ atau via Gateway http://localhost:80/fleet/
```

### Opsi B: Menjalankan via Docker Standalone
```bash
# Build & jalankan image Docker service
docker build -t fledger-fleet:latest .
docker run -d --name fledger-fleet -p 8082:8082 --env-file .env fledger-fleet:latest
```

### Opsi C: Menjalankan via Master Docker Compose (Ekosistem Penuh)
```bash
# Dari root monorepo:
docker compose up -d fledger-fleet
```

API server listen di `http://localhost:8082`.  
Login dev: `POST /v1/dev/login?tenant_id=00000000-0000-0000-0000-000000000001` (hanya di `APP_ENV=development`).

---

## 📁 Struktur Folder Proyek

```text
fledger-fleet/
├── docs/                                       <-- Spesifikasi, schema, roadmap
│   ├── AGENT-EXECUTION-BRIEF.md
│   ├── DATABASE-SCHEMA.sql
│   ├── API-SPECIFICATION.md
│   └── ROADMAP-FLEET.md
├── src/
│   ├── go.mod
│   ├── cmd/
│   │   ├── api/                                <-- entrypoint HTTP server
│   │   └── migrator/                           <-- CLI migrasi SQL
│   ├── internal/
│   │   ├── config/                             <-- env loader + validation
│   │   ├── platform/                           <-- errors, httpx, log
│   │   ├── auth/jwt/                           <-- HS256 sign/verify
│   │   ├── middleware/                         <-- RequireAuth, RequireScope
│   │   ├── domain/                             <-- vehicle, driver, trip, DO, pod, outbox
│   │   ├── repository/postgres/                <-- pgx-based repos
│   │   ├── integration/coreclient/             <-- Fledger Core HTTP adapter
│   │   ├── usecase/                            <-- application services
│   │   ├── handler/                            <-- REST handlers
│   │   └── integration/                        <-- E2E + unit tests
│   ├── migrations/                             <-- SQL migration files
│   │   ├── 000001_init_fleet.sql
│   │   ├── 000002_seed_demo.sql
│   │   ├── 000003_audit_trail.sql
│   │   └── 000004_audit_actor_text.sql
│   └── scripts/                                <-- helper scripts
│       ├── setup-db.cmd
│       ├── start-api.cmd
│       ├── run-tests.ps1
│       └── e2e-flow.ps1
├── .env.example
└── README.md
```

---

## 🧪 Pengujian

| Jenis Test | Lokasi | Perintah |
|---|---|---|
| Unit tests | `internal/{auth,config,domain,usecase}/` | `go test ./internal/...` |
| Integration (DB + Core stub) | `internal/integration/` | `go test ./internal/integration/...` |
| E2E (live HTTP) | `scripts/e2e-flow.ps1` | `.\src\scripts\e2e-flow.ps1` |

Coverage saat ini (Sprint 5):
- `auth/jwt`: **85.7%**
- `config`: **57.1%**
- `integration/coreclient` (Fledger Core adapter): **64.7%**
- E2E: full happy-path + 1 negative path (POD tanpa foto)

---

## 🛡 Guardrails Terpenuhi

- ✅ **Nominal Finansial**: Semua nominal disimpan sebagai `BIGINT` minor units (Rupiah integer, tanpa floating-point).
- ✅ **Zero-Cost**: PostgreSQL lokal, Leaflet/OpenStreetMap siap di-attach via `destination_lat/lng` & `delivered_lat/lng`.
- ✅ **Audit Trail**: Tabel `fleet_do_status_history` + kolom `actor_id` di DO/Trip/Vehicle (Sprint guardrail §7.3).
- ✅ **Idempotency**: Setiap POST `/v1/invoices` membawa `Idempotency-Key: <DO UUID>` (Sprint 4 / §5.1 brief).
- ✅ **Outbox Pattern**: POD disimpan lokal dulu, lalu di-retry ke Core oleh background worker (exponential backoff 30s→30min).

---

## 🧩 Integrasi dengan Fledger Core

Lihat [`docs/API-SPECIFICATION.md`](docs/API-SPECIFICATION.md) untuk kontrak lengkap.

### Contoh: Fledger Core invoice payload
```json
POST /v1/invoices HTTP/1.1
Host: localhost:8081
Authorization: Bearer <FLEDGER_CORE_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Idempotency-Key: <DO_UUID>

{
  "customer_id":  "cccccccc-0001-0000-0000-000000000001",
  "code":         "INV-DO-DO-202610-0089",
  "amount_minor": 4000000,
  "due_date":     "2026-10-22",
  "description":  "Generated from DO-202610-0089 (delivered 8, rejected 2) by Ibu Siti"
}
```

> Catatan: Kontrak payload `POST /v1/invoices` di Fledger Core nyata (lihat `fledger-core/internal/handler/dto.go::CreateInvoiceRequest`) menggunakan field `code`/`amount_minor`/`due_date` (YYYY-MM-DD) — bukan `invoice_number`/`amount`/`due_date` (RFC3339) seperti yang dicontohkan di brief. Implementasi di repo ini mengikuti kontrak Core yang sebenarnya.