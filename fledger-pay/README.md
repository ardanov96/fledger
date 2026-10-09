# Fledger Pay — B2B Payment Gateway & Auto-Settlement Engine

> **Service**: `fledger-pay`
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)
> **Port**: `:8083`
> **Status**: ✅ **Production-Ready (Sprint 1–5 selesai, 100% test pass)**
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router + CORS, zero npm dependencies

---

## 🎯 Tanggung Jawab Layanan

1. **Multi-Bank Virtual Account & QRIS Generator** — Menerbitkan nomor VA unik (BCA, Mandiri, BRI, BNI, Permata) dan QRIS dinamis EMVCo-compliant per faktur/Surat Jalan.
2. **Webhook Callback Ingestion** — Menerima konfirmasi pembayaran seketika dari Payment Gateway (Midtrans, Xendit) atau simulator bank dengan validasi tanda tangan **HMAC SHA-256/512**.
3. **Idempotency Guard** — Menjamin tidak ada *double-crediting* meskipun gateway mengirim callback berulang kali (cek `external_reference` + `idempotency_key`).
4. **Automated Settlement Bridge ke Fledger Core** — Outbox pattern transaksional: setiap pelunasan otomatis memicu **double-entry** (Debit Kas Bank, Kredit Piutang Usaha) via `POST /v1/transfers`, lalu tandai invoice `PAID` via `POST /v1/invoices/:id/pay`.
5. **Interactive Payment Simulator** — Web portal di `:8083/` (zero-npm) untuk demo dan uji E2E sandbox.

---

## 📅 Sprint Progress

| Sprint | Status | Highlight |
|---|---|---|
| 1 — Pondasi DB & Health Probes | ✅ | 7 tabel + `pay_schema_migrations`; `/healthz` + `/readyz` (DB ping) |
| 2 — Payment Request + VA + QRIS | ✅ | Generator VA per-bank + QRIS EMVCo dengan CRC-16/CCITT |
| 3 — Webhook Ingestion + HMAC + Idempotency | ✅ | Midtrans (SHA-512), Xendit (token), Direct; idempotency tested |
| 4 — Fledger Core HTTP + Outbox Worker | ✅ | Retry exponential backoff 3s→30min, FOR UPDATE SKIP LOCKED |
| 5 — Web Simulator + E2E | ✅ | Portal HTML/CSS/JS serve via `embed.FS`; e2e-flow.ps1 PASSED |

---

## 🤖 Panduan untuk AI Coding Agent / Developer

Baca dokumen di folder `docs/`:
- 📄 [AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md) — Blueprint eksekusi.
- 📄 [ROADMAP-PAY.md](docs/ROADMAP-PAY.md) — Rincian 5 sprint + DoD.
- 📄 [DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql) — DDL lengkap.
- 📄 [API-SPECIFICATION.md](docs/API-SPECIFICATION.md) — Kontrak REST + webhook.
- 📄 [SIMULATOR-AND-SETTLEMENT-GUIDE.md](docs/SIMULATOR-AND-SETTLEMENT-GUIDE.md) — Alur sandbox.

---

## ⚙️ Konfigurasi `.env`

```ini
APP_ENV=development
PORT=8083
DATABASE_URL=postgres://fmcg:fmcg_dev_password@localhost:5432/fledger_pay?sslmode=disable
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
JWT_SECRET=super-secret-fledger-pay-jwt-key-minimum-32-chars!
WEBHOOK_SECRET=dev-webhook-secret-key-midtrans-xendit
OUTBOX_POLL_INTERVAL=3s
OUTBOX_MAX_ATTEMPTS=10
```

---

## 🚀 Quick Start

```powershell
# 1. Setup database (opsional, script `migrations/000001_init_pay.sql` bisa dijalankan manual)
psql -U fmcg -d fledger_pay -f src/migrations/000001_init_pay.sql

# 2. Jalankan unit + integration tests
.\src\scripts\run-tests.ps1

# 3. Jalankan API server
.\src\scripts\start-api.cmd

# 4. (terminal lain) E2E smoke test
.\src\scripts\e2e-flow.ps1

# 5. Buka web simulator
#    http://localhost:8083/
```

---

## 📁 Struktur Folder

```text
fledger-pay/
├── docs/                                 <-- Specs, schema, roadmap
├── src/
│   ├── cmd/
│   │   ├── api/                          <-- HTTP server
│   │   └── migrator/                     <-- SQL migrator CLI
│   ├── internal/
│   │   ├── auth/jwt/                     <-- HS256 sign/verify
│   │   ├── config/                       <-- env loader
│   │   ├── domain/                       <-- payment, va, qris, transaction, outbox, audit
│   │   ├── handler/                      <-- REST + webhook
│   │   ├── integration/coreclient/       <-- Fledger Core HTTP client
│   │   ├── middleware/                   <-- RequireAuth
│   │   ├── platform/                     <-- errors, httpx, log
│   │   ├── repository/postgres/          <-- pgx-based repos
│   │   ├── usecase/                      <-- business services + outbox worker
│   │   ├── webhook/                      <-- HMAC SHA-256/512 verifier
│   │   └── webui/                        <-- embedded static files (HTML/CSS/JS)
│   ├── migrations/
│   │   └── 000001_init_pay.sql
│   ├── go.mod / go.sum
│   └── scripts/                          <-- helper scripts (cmd)
├── web/                                  <-- source for embedded portal
│   ├── index.html
│   ├── style.css
│   ├── pay-client.js
│   └── app.js
├── .env.example
└── README.md
```

---

## 🧪 Pengujian

| Test | Lokasi | Status |
|---|---|---|
| HMAC signature (Midtrans SHA-512, generic SHA-256, Xendit token) | `internal/webhook/signature_test.go` | ✅ PASS |
| Full happy path: create PR → settle → outbox → Core | `internal/integration/integration_test.go` | ✅ PASS |
| Idempotency replay | same | ✅ PASS |
| Outbox retry when Core offline | same | ✅ PASS |
| VA generator (stable, unique) | same | ✅ PASS |
| QRIS EMVCo structure (CRC, tag 53, 58) | same | ✅ PASS |
| E2E (live HTTP) | `scripts/e2e-flow.ps1` | ✅ PASS |

**Coverage**:
- webhook package: HMAC + CRC-16 known-vector test.
- integration: 6 tests, all run against real PostgreSQL + stubbed Fledger Core.

```
==> internal/webhook  [OK]
==> internal/integration  [OK]
```

---

## 🛡 Guardrails Terpenuhi

- ✅ **Integritas uang**: semua nominal `BIGINT` minor units (Rupiah integer, tanpa floating-point).
- ✅ **Idempotency wajib**: `pay_transactions.external_reference` UNIQUE + outbox event dedup; replay returns 200 OK with `idempotent_replay=true`.
- ✅ **Outbox pattern transaksional**: POD + outbox event ditulis dalam satu DB tx; Core call di-defer ke background worker.
- ✅ **CORS terbuka**: `cors.AllowAllOrigins` mounted di root.
- ✅ **Zero Secret Hardcoding**: `JWT_SECRET`, `WEBHOOK_SECRET`, `FLEDGER_CORE_URL` semua via env.

---

## 🔌 Integrasi dengan Fledger Core

Kontrak yang dipakai mengikuti **Fledger Core yang sebenarnya** (bukan contoh di brief):

```
POST /v1/transfers
Idempotency-Key: <pay_transaction_id>
{
  "from_account_id": "ACC_CUSTOMER_AR",
  "to_account_id":   "ACC_BANK_BCA",
  "amount_minor":    4000000,
  "currency":        "IDR",
  "description":     "Auto-settlement via VA_BCA (PAY-202610-00891)"
}

POST /v1/invoices/<invoice_id>/pay
Idempotency-Key: <pay_transaction_id>
```