# Fledger Dunning — Automated AR Dunning & WhatsApp Gateway

> **Service**: `fledger-dunning`
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)
> **Port**: `:8086`
> **Gateway Route**: `http://localhost:80/dunning/`
> **Status**: ✅ **Production-Ready (Sprint 1–5 selesai, 100% test pass)**
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router + CORS, embedded dark-mode PWA
> **Standalone Repository**: [ardanov96/fledger-dunning](https://github.com/ardanov96/fledger-dunning)

---

## 🎯 Tanggung Jawab Layanan

1. **WhatsApp 5-Tahap Dunning Cadence Otomatis** — `POST /v1/dunning/queues/ingest-invoice` menerima 1 faktur dan otomatis menyusun 5 jadwal pengiriman (H-3, H0, H+3, H+7, H+14) sesuai `dunning_configurations.cadence_days`.
2. **Self-Healing Loop (Pembatalan Otomatis saat Lunas)** — `POST /v1/dunning/queues/cancel-invoice` (atau webhook `POST /v1/dunning/webhooks/pay`) mengeksekusi satu transaksi DB atomik: `UPDATE dunning_queues SET status='CANCELLED_BY_PAYMENT' WHERE invoice_id=$1 AND status='QUEUED'`.
3. **Anti-Ban Jitter Engine** — Setiap eksekusi cron menerapkan `time.Sleep(rand.Intn(jitter_max-jitter_min+1) * time.Second)` agar tidak ada dua pesan terkirim dalam 1 detik; default 3–8 detik. Salam pesan diacak menggunakan **spintax** (`Selamat pagi|Selamat siang|Yth.|Salam hormat`).
4. **Monthly PDF e-Statement Engine** — `POST /v1/dunning/statements/generate` render PDF (jung-kurt/gofpdf) dengan header distributor, identitas toko, tabel mutasi kronologis, dan saldo akhir; `POST /v1/dunning/statements/{id}/send` mengirim via WhatsApp Document API.
5. **WhatsApp Provider Abstraction** — `internal/platform/whatsapp` interface + `MockProvider` (default). Saat runtime: setiap `SendTextMessage` dan `SendDocument` dicatat ke `dunning_message_logs` agar sistem dapat diuji 100% tanpa QR HP fisik.
6. **Embedded Web Management Portal** — `internal/webui/files/` (HTML/CSS/JS) di-embed ke binary Go via `//go:embed all:files`; dark-mode glassmorphism, simulator layar HP WhatsApp, tombol "Simulasi Pembayaran Toko" yang memicu webhook self-healing secara real-time.

---

## 📅 Sprint Progress

| Sprint | Status | Highlight |
|---|---|---|
| 1 — Pondasi, Kontak, WhatsApp Pairing | ✅ | 8 tabel + `dunning_schema_migrations`; provider abstraction; status / QR endpoints |
| 2 — Ingest + 5-Tahap Cadence | ✅ | Ingest 1 faktur → 5 schedule rows; auto-cadence dari `dunning_configurations.cadence_days` |
| 3 — Self-Healing + Anti-Ban Jitter | ✅ | Atomic UPDATE dalam tx; cron dispatcher dengan jitter sleep 3–8s; spintax salam |
| 4 — PDF e-Statement + WhatsApp Document | ✅ | `gofpdf` rendering; layout elegan dengan header/total/saldo; Document API dispatch |
| 5 — Embedded PWA + Tests | ✅ | Vanilla JS + glassmorphism; E2E flow 8/8 steps PASS |

---

## 🤖 Panduan untuk AI Coding Agent / Developer

Baca dokumen di folder `docs/`:
- 🤖 [AI-AGENT-PROMPT.md](docs/AI-AGENT-PROMPT.md) — Master prompt instruksi AI agent.
- 📄 [AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md) — Blueprint eksekusi.
- 📄 [ROADMAP-DUNNING.md](docs/ROADMAP-DUNNING.md) — Rincian 5 sprint + DoD.
- 📄 [DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql) — DDL lengkap + seed data.
- 📄 [API-SPECIFICATION.md](docs/API-SPECIFICATION.md) — Kontrak REST.
- 📄 [WHATSAPP-CADENCE-AND-STATEMENT-GUIDE.md](docs/WHATSAPP-CADENCE-AND-STATEMENT-GUIDE.md) — Logika spintax + jitter + PDF.

---

## ⚙️ Konfigurasi `.env`

```ini
APP_ENV=development
PORT=8086
DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_dunning?sslmode=disable
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=a0000000-0000-0000-0000-000000000001
FLEDGER_PAY_URL=http://localhost:8083
JWT_SECRET=fledger-super-secure-jwt-secret-key-2026-min-32
PAY_WEBHOOK_SECRET=dunning-super-secret-key-2026
WA_PROVIDER=MOCK
WA_JITTER_MIN_SECONDS=3
WA_JITTER_MAX_SECONDS=8
OUTBOX_POLL_INTERVAL=5s
OUTBOX_MAX_ATTEMPTS=10
TOKEN_TTL=24h
```

---

## 🚀 Quick Start

### Opsi A: Menjalankan via Local Host
```powershell
# 1. Setup database
psql -U fmcg -d fledger_dunning -f docs/DATABASE-SCHEMA.sql

# 2. Run unit + integration tests
go test -v ./...

# 3. Jalankan API server
.\scripts\start-api.cmd

# 4. (terminal lain) E2E smoke test
.\scripts\e2e-flow.ps1

# 5. Buka PWA dark-mode di browser
#    http://localhost:8086/ atau via Gateway http://localhost:80/dunning/
```

### Opsi B: Menjalankan via Docker Standalone
```bash
# Build & jalankan image Docker service
docker build -t fledger-dunning:latest .
docker run -d --name fledger-dunning -p 8086:8086 --env-file .env fledger-dunning:latest
```

### Opsi C: Menjalankan via Master Docker Compose (Ekosistem Penuh)
```bash
# Dari root monorepo:
docker compose up -d fledger-dunning
```

---

## 📁 Struktur Folder

```text
fledger-dunning/
├── docs/                                  <-- Specs, schema, roadmap
├── cmd/
│   └── server/                            <-- entrypoint HTTP server
├── internal/
│   ├── auth/jwt/                          <-- HS256 sign/verify
│   ├── config/                            <-- env loader
│   ├── domain/                            <-- config, contact, queue, statement, session, audit, message_log
│   ├── handler/                           <-- inside delivery/http
│   ├── delivery/http/                     <-- Chi router + handlers
│   ├── middleware/                        <-- RequireAuth + tenant fallback
│   ├── platform/                          <-- errors, httpx, log, whatsapp, pdf
│   ├── repository/postgres/               <-- pgx-based repos
│   ├── usecase/                           <-- DunningService + StatementService
│   └── webui/                             <-- embedded PWA (index.html, style.css, *.js)
├── go.mod / go.sum
├── go.work
├── .env.example
├── scripts/
│   ├── start-api.cmd
│   └── e2e-flow.ps1
└── README.md
```

---

## 🧪 Pengujian

| Test | Lokasi | Status |
|---|---|---|
| IngestInvoice_CreatesFiveQueueRows | `internal/integration/integration_test.go` | ✅ PASS |
| SelfHealing_CancelByInvoice (5 rows → CANCELLED) | same | ✅ PASS |
| DispatchDue_AntiBanJitter (jitter delays, SENT count) | same | ✅ PASS |
| Statement_GeneratesAndPersistsPDF | same | ✅ PASS |
| WhatsAppStatusProvider (CONNECTED via mock) | same | ✅ PASS |
| WebhookAuth_FailsWithoutSecret | same | ✅ PASS |
| DispatchOne_SpecificQueueID (manual target ID) | same | ✅ PASS |
| Statement_GetByIDAndDownload (direct fetch & PDF download) | same | ✅ PASS |
| Automated Test Runner | `run-tests.ps1` | ✅ PASS (All green) |
| E2E (live HTTP) | `scripts/e2e-flow.ps1` | ✅ PASS (8/8 steps) |

```
ok  github.com/fledger/fledger-dunning/internal/integration  26.203s
```

---

## 🛡 Guardrails Terpenuhi

- ✅ **Integritas uang**: semua nominal `BIGINT` minor units (no float).
- ✅ **Atomicity self-healing**: `Begin → UPDATE → Commit` dalam satu tx (Postgres).
- ✅ **Anti-ban jitter**: random sleep 3-8 detik per pesan; spintax pada salam.
- ✅ **Idempotency**: ingest via `ON CONFLICT (tenant_id, store_id) DO UPDATE` di dunning_store_contacts.
- ✅ **CORS terbuka**: `cors.AllowAllOrigins` di router.
- ✅ **Mock provider**: full system dapat diuji tanpa QR HP fisik.
- ✅ **Zero secret hardcoding**: semua secret dari env (`JWT_SECRET`, `PAY_WEBHOOK_SECRET`).

---

## 🔌 Kontrak API Utama

### Ingest Faktur (membuat 5 schedule)
```http
POST /v1/dunning/queues/ingest-invoice
X-Tenant-ID: a0000000-0000-0000-0000-000000000001
Authorization: Bearer <jwt>

{
  "invoice_id": "INV-2026-001",
  "invoice_number": "INV/2026/10/001",
  "store_id": "TKO-001",
  "phone_number": "6281234567801",
  "due_date": "2026-10-25",
  "amount_due_minor": 4500000,
  "payment_link_url": "http://localhost:8083/pay/INV-2026-001",
  "store_name": "Toko Sumber Rezeki"
}
```

### Self-Healing Webhook (dari Fledger Pay)
```http
POST /v1/dunning/webhooks/pay
X-Webhook-Secret: dunning-super-secret-key-2026

{ "event": "payment.settled", "invoice_id": "INV-2026-001", "paid_amount_minor": 4500000 }
```

### Generate PDF e-Statement
```http
POST /v1/dunning/statements/generate
{ "store_id": "TKO-001", "statement_month": "2026-10" }
```