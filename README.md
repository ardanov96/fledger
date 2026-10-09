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
│   ├── src/                       <-- Source code backend Fledger Fleet (Go Chi, :8082)
│   ├── web/                       <-- Web Portal Mode Supir POD & Dispatcher Hub
│   ├── .env.example               <-- Template konfigurasi lingkungan
│   └── README.md                  <-- Onboarding guide Fledger Fleet
│
├── fledger-pay/                   <-- Layanan B2B Payment Gateway & Auto-Settlement Engine
│   ├── docs/                      <-- Blueprint eksekusi AI Agent, skema DB & spesifikasi API
│   ├── src/                       <-- Source code backend Fledger Pay (Go Chi, :8083)
│   ├── web/                       <-- Web Simulator Sandbox VA & QRIS
│   ├── .env.example               <-- Template konfigurasi lingkungan
│   └── README.md                  <-- Onboarding guide Fledger Pay
│
├── fledger-force/                 <-- Layanan Sales Force Automation (SFA) & Anti-Cash Kitting
│   ├── docs/                      <-- Blueprint eksekusi AI Agent, skema DB & spesifikasi API
│   │   ├── AGENT-EXECUTION-BRIEF.md  <-- Blueprint siap eksekusi untuk AI Coding Agent
│   │   ├── DATABASE-SCHEMA.sql       <-- Skrip DDL PostgreSQL 16
│   │   ├── API-SPECIFICATION.md      <-- Spesifikasi REST API
│   │   ├── ROADMAP-FORCE.md          <-- Sprint roadmap & Definition of Done
│   │   └── CASH-COLLECTION-AND-EOD-GUIDE.md <-- Panduan cash collection & EOD kasir
│   ├── .env.example               <-- Template konfigurasi lingkungan
│   └── README.md                  <-- Onboarding guide Fledger Force
│
├── fledger-order/                 <-- Layanan B2B Order Management System (OMS) & Hard Credit Gate
│   ├── docs/                      <-- Blueprint eksekusi AI Agent, skema DB & spesifikasi API
│   │   ├── AGENT-EXECUTION-BRIEF.md  <-- Blueprint siap eksekusi untuk AI Coding Agent
│   │   ├── DATABASE-SCHEMA.sql       <-- Skrip DDL PostgreSQL 16
│   │   ├── API-SPECIFICATION.md      <-- Spesifikasi REST API
│   │   ├── ROADMAP-ORDER.md          <-- Sprint roadmap & Definition of Done
│   │   └── HARD-CREDIT-GATE-AND-FLEET-BRIDGE-GUIDE.md <-- Panduan credit gate & fleet bridge
│   ├── .env.example               <-- Template konfigurasi lingkungan
│   └── README.md                  <-- Onboarding guide Fledger Order
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
* **Status**: **Production-Ready** (Sprint 1–5 Selesai & Web Portal Live)
* **Teknologi**: Go 1.25, Chi Router, PostgreSQL 16, HTML5 Touch Canvas POD
* **Tanggung Jawab**:
  - Manajemen armada (*vehicles*) dan supir (*drivers*).
  - Perencanaan rute jalan harian (*Trip Dispatching*).
  - Penerbitan Surat Jalan (*Delivery Order / DO*).
  - **Digital Proof of Delivery (POD)**: Tangkap foto barang rusak/retur dan tanda tangan digital penerima di toko.
  - **Auto-Invoicing Bridge**: Otomatis memicu penerbitan faktur bersih di `fledger-core` (`POST /v1/invoices`) setelah pengiriman selesai.
  - Standalone repositori: [ardanov96/fledger-fleet](https://github.com/ardanov96/fledger-fleet)
* 👉 **[Buka Panduan fledger-fleet](fledger-fleet/README.md)**

---

### 3. 💳 `fledger-pay` (B2B Payment Gateway & Auto-Settlement Engine)
* **Status**: **Production-Ready** (Sprint 1–5 Selesai & Web Simulator Live)
* **Teknologi**: Go 1.23+, PostgreSQL 16, Chi Router, Embedded Web Simulator
* **Tanggung Jawab**:
  - Multi-bank Virtual Account (BCA, Mandiri, BRI, BNI) & Dynamic B2B QRIS.
  - Webhook callback ingestion dengan validasi tanda tangan HMAC SHA-256 & anti double-crediting.
  - Auto-settlement bridge ke `fledger-core` (`POST /v1/transfers` & `POST /v1/invoices/:id/pay`).
  - Interactive payment sandbox simulator UI.
  - Standalone repositori: [ardanov96/fledger-pay](https://github.com/ardanov96/fledger-pay)
* 👉 **[Buka Panduan fledger-pay](fledger-pay/README.md)**

---

### 4. 📱 `fledger-force` (Sales Force Automation & Anti-Cash Kitting Engine)
* **Status**: **Tahap Pengembangan oleh AI Agent (Phase 3 Milestone 1)**
* **Teknologi**: Go 1.23+, PostgreSQL 16, Geofencing Haversine, PWA Mobile UI
* **Tanggung Jawab**:
  - Rute kunjungan harian (*Geofenced Beat Plan*) dengan verifikasi GPS radius 100m.
  - Mobile Cash Collection & tanda terima WhatsApp instan ke toko.
  - Anti-Cash Kitting liability transfer: memindahkan beban kas dari piutang toko ke wallet salesman di `fledger-core`.
  - Daily Settlement Lock: meja setor kasir gudang di sore hari untuk membuka kunci operasional esok hari.
* 👉 **[Buka Blueprint Eksekusi AI Agent fledger-force](fledger-force/docs/AGENT-EXECUTION-BRIEF.md)**
* 👉 **[Buka Panduan fledger-force](fledger-force/README.md)**

---

### 5. 📦 `fledger-order` (B2B Order Management System & Hard Credit Gate)
* **Status**: **Siap Dieksekusi oleh AI Agent (Phase 3 Milestone 2)**
* **Teknologi**: Go 1.23+, PostgreSQL 16, Chi Router, B2B Web Ordering Portal
* **Tanggung Jawab**:
  - Master katalog SKU produk FMCG & penetapan harga grosir bertingkat (*wholesale pricing tiers*).
  - Alokasi & penguncian stok fisik gudang (*inventory stock reservation*) untuk mencegah overselling.
  - **Hard Credit Gate (Pre-Flight Check ke Fledger Core)**: Blokir pesanan otomatis jika melanggar plafon piutang atau menunggak faktur $>30$ hari (`CREDIT_BLOCKED`).
  - **Fleet Dispatching Bridge**: Otomatis menerbitkan Surat Jalan (DO) di `fledger-fleet` dengan akumulasi tonase berat muatan dalam kilogram.
* 👉 **[Buka Blueprint Eksekusi AI Agent fledger-order](fledger-order/docs/AGENT-EXECUTION-BRIEF.md)**
* 👉 **[Buka Panduan fledger-order](fledger-order/README.md)**

---

## 🤖 Panduan Khusus untuk AI Coding Agent

Jika Anda adalah AI Agent yang ditugaskan untuk mengerjakan project lanjutan di repositori ini:

1. **Fokus Ruang Kerja**:
   - Fitur akuntansi atau core wallet: buka dan kerjakan di dalam folder [`fledger-core/`](fledger-core/).
   - Fitur logistik armada atau POD: buka dan kerjakan di dalam folder [`fledger-fleet/`](fledger-fleet/).
   - Fitur payment gateway atau auto-settlement: buka dan kerjakan di dalam folder [`fledger-pay/`](fledger-pay/).
   - Fitur SFA, kunjungan toko GPS, atau cash collection: buka dan kerjakan di dalam folder [`fledger-force/`](fledger-force/).
   - Fitur OMS, katalog grosir, atau Hard Credit Gate: buka dan kerjakan di dalam folder [`fledger-order/`](fledger-order/).
2. **Pedoman Teknis Layanan**:
   - **Fledger Order**: Baca spesifikasi di [`fledger-order/docs/AGENT-EXECUTION-BRIEF.md`](fledger-order/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-order/docs/DATABASE-SCHEMA.sql`](fledger-order/docs/DATABASE-SCHEMA.sql).
   - **Fledger Force**: Baca spesifikasi di [`fledger-force/docs/AGENT-EXECUTION-BRIEF.md`](fledger-force/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-force/docs/DATABASE-SCHEMA.sql`](fledger-force/docs/DATABASE-SCHEMA.sql).
   - **Fledger Pay**: Baca spesifikasi di [`fledger-pay/docs/AGENT-EXECUTION-BRIEF.md`](fledger-pay/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-pay/docs/DATABASE-SCHEMA.sql`](fledger-pay/docs/DATABASE-SCHEMA.sql).
   - **Fledger Fleet**: Baca spesifikasi di [`fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md`](fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-fleet/docs/DATABASE-SCHEMA.sql`](fledger-fleet/docs/DATABASE-SCHEMA.sql).
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
