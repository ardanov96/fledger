# FLEDGER OS — The Ledger-First Operating System

> **The Ledger-First Operating System for Distribution, Fleet Logistics & B2B Supply Chain**  
> *Sistem operasi distribusi B2B Indonesia yang menyatukan logistik fisik, penagihan lapangan, dan pembukuan finansial double-entry dalam satu ekosistem terpadu.*

---

## 🧭 Struktur Monorepo Proyek

Repositori ini mengadopsi struktur monorepo modular yang memisahkan masing-masing bounded context layanan mikro (*microservices*):

```text
fledger/
├── fledger-core/                  <-- Core Financial Ledger & Settlement Engine (FMCG Wallet)
│   ├── cmd/                       <-- Go entrypoints (cmd/api, cmd/migrator, dll.)
│   ├── internal/                  <-- Clean architecture (domain, usecase, repo, handler)
│   ├── migrations/                <-- 30 PostgreSQL migrations (RLS, hash chains, audit)
│   ├── web/                       <-- Frontend Dashboard 9 views (Vanilla JS, zero npm)
│   ├── web-next/                  <-- Frontend Next.js client
│   ├── docs/                      <-- Dokumentasi modul, ADR, API reference, runbooks
│   ├── Makefile                   <-- Build automation & test targets
│   ├── go.mod                     <-- Go 1.23+ module
│   └── README.md                  <-- Panduan lengkap operasional Core Ledger
│
├── fledger-fleet/                 <-- Layanan Logistik, Dispatching & Digital Proof of Delivery (POD)
│   ├── docs/                      <-- Dokumen spesifikasi eksekusi AI Agent & skema database
│   │   ├── AGENT-EXECUTION-BRIEF.md  <-- Blueprint siap eksekusi untuk AI Coding Agent
│   │   ├── DATABASE-SCHEMA.sql       <-- Skrip DDL PostgreSQL 16
│   │   ├── API-SPECIFICATION.md      <-- Spesifikasi REST API & Fledger Core client
│   │   └── ROADMAP-FLEET.md          <-- Sprint roadmap & Definition of Done
│   ├── src/                       <-- Tempat source code backend Fledger Fleet
│   ├── .env.example               <-- Template konfigurasi lingkungan
│   └── README.md                  <-- Onboarding guide Fledger Fleet
│
└── README.md                      <-- Berkas ini (Monorepo Master Guide)
```

---

## 📦 Layanan dalam Ekosistem

### 1. 🟢 `fledger-core` (Core Financial Ledger & Settlement Engine)
* **Status**: **Production-Ready** (Sprint 1–22B Selesai)
* **Teknologi**: Go 1.23, PostgreSQL 16, Redis 7, NATS JetStream
* **Tanggung Jawab**:
  - Double-entry immutable ledger (debit = kredit).
  - 9 Modul Web Dashboard (Dashboard, Accounts, Transfers, Invoices, Aging, Periods, Reconciler, Currencies & FX, Audit Log).
  - AR Aging schedule (0–30, 31–60, 61–90, >90 hari).
  - Cryptographic hash chaining (SHA-256) per baris mutasi.
  - Penutupan periode akuntansi bulanan (*Two-step Maker-Checker approval*).
  - Row-Level Security (PostgreSQL RLS) untuk multi-tenant isolation.
* 👉 **[Buka Dokumentasi & Panduan fledger-core](fledger-core/README.md)**

---

### 2. 🚚 `fledger-fleet` (Fleet Logistics & Proof of Delivery Engine)
* **Status**: **Tahap Pengembangan / Siap Dieksekusi oleh AI Agent**
* **Teknologi**: Node/Go Backend + PostgreSQL 16
* **Tanggung Jawab**:
  - Manajemen armada (*vehicles*) dan supir (*drivers*).
  - Perencanaan rute jalan harian (*Trip Dispatching*).
  - Penerbitan Surat Jalan (*Delivery Order / DO*).
  - **Digital Proof of Delivery (POD)**: Tangkap foto barang rusak/retur dan tanda tangan digital penerima di toko.
  - **Auto-Invoicing Bridge**: Otomatis memicu penerbitan faktur bersih di `fledger-core` (`POST /v1/invoices`) setelah pengiriman selesai.
* 👉 **[Buka Blueprint Eksekusi AI Agent](fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md)**
* 👉 **[Buka Panduan fledger-fleet](fledger-fleet/README.md)**

---

## 🤖 Panduan Khusus untuk AI Coding Agent

Jika Anda adalah AI Agent yang ditugaskan untuk mengerjakan project lanjutan di repositori ini:

1. **Fokus Ruang Kerja**:
   - Jika mengerjakan fitur akuntansi atau core wallet: buka dan kerjakan di dalam folder [`fledger-core/`](fledger-core/).
   - Jika mengerjakan fitur armada, pengiriman, dan digital POD: buka dan kerjakan di dalam folder [`fledger-fleet/`](fledger-fleet/).
2. **Pedoman Teknis Fleet**:
   - Baca seluruh spesifikasi di [`fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md`](fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md).
   - Terapkan skema database dari [`fledger-fleet/docs/DATABASE-SCHEMA.sql`](fledger-fleet/docs/DATABASE-SCHEMA.sql).
   - Pastikan setiap request ke `fledger-core` menggunakan `Idempotency-Key` bernilai UUID unik Surat Jalan.
3. **Peta Roadmap Global**:
   - Pelajari visi makro dan 5 masalah industri FMCG Indonesia di [`fledger-core/docs/architecture/roadmap-microservices.md`](fledger-core/docs/architecture/roadmap-microservices.md).

---

## 🚀 Quick Start (Menjalankan Fledger Core)

```bash
# 1. Pindah ke direktori fledger-core
cd fledger-core

# 2. Setup environment
cp .env.example .env

# 3. Jalankan infrastruktur & migrasi
make up
make migrate-up

# 4. Jalankan Core API
go run ./cmd/api

# 5. Jalankan Web Dashboard (di terminal lain)
node web/server.js
```

Akses Web Dashboard di browser: `http://localhost:3000`  
Kredensial Login Demo: `demo-user` / `demo-password`
