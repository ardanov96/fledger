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
├── fledger-dunning/               <-- Layanan Otomasi Penagihan Piutang (AR Dunning) & WhatsApp Gateway
│   ├── docs/                      <-- Blueprint eksekusi AI Agent, skema DB & spesifikasi API
│   │   ├── AGENT-EXECUTION-BRIEF.md  <-- Blueprint siap eksekusi untuk AI Coding Agent
│   │   ├── DATABASE-SCHEMA.sql       <-- Skrip DDL PostgreSQL 16
│   │   ├── API-SPECIFICATION.md      <-- Spesifikasi REST API
│   │   ├── ROADMAP-DUNNING.md        <-- Sprint roadmap & Definition of Done
│   │   └── WHATSAPP-CADENCE-AND-STATEMENT-GUIDE.md <-- Panduan dunning & anti-ban
│   ├── .env.example               <-- Template konfigurasi lingkungan
│   └── README.md                  <-- Onboarding guide Fledger Dunning
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

### 6. 🔔 `fledger-dunning` (Automated AR Dunning & WhatsApp Gateway)
* **Status**: **Siap Dieksekusi oleh AI Agent (Phase 4 Milestone 1)**
* **Teknologi**: Go 1.23+, PostgreSQL 16, Chi Router, WhatsApp Engine, PDF e-Statement
* **Tanggung Jawab**:
  - Penagihan bertingkat otomatis 5 tahap: H-3 (Pengingat ramah), Hari H, H+3, H+7, dan H+14 eskalasi penangguhan pesanan.
  - Deep-link pembayaran 1-klik terintegrasi langsung ke `fledger-pay` (QRIS & Virtual Account).
  - **Self-Healing Webhook Loop**: Otomatis membatalkan seluruh sisa antrian dunning seketika saat faktur lunas di `fledger-pay`.
  - **Anti-Ban Jitter Engine**: Delay acak 3–8 detik & variasi teks (*spintax*) untuk melindungi nomor WhatsApp resmi.
  - Render PDF Rekening Koran Toko bulanan (e-Statement) otomatis setiap tanggal 1 awal bulan.
* 👉 **[Buka Blueprint Eksekusi AI Agent fledger-dunning](fledger-dunning/docs/AGENT-EXECUTION-BRIEF.md)**
* 👉 **[Buka Panduan fledger-dunning](fledger-dunning/README.md)**

---

## 🤖 Panduan Khusus untuk AI Coding Agent

Jika Anda adalah AI Agent yang ditugaskan untuk mengerjakan project lanjutan di repositori ini:

1. **Fokus Ruang Kerja**:
   - Fitur akuntansi atau core wallet: buka dan kerjakan di dalam folder [`fledger-core/`](fledger-core/).
   - Fitur logistik armada atau POD: buka dan kerjakan di dalam folder [`fledger-fleet/`](fledger-fleet/).
   - Fitur payment gateway atau auto-settlement: buka dan kerjakan di dalam folder [`fledger-pay/`](fledger-pay/).
   - Fitur SFA, kunjungan toko GPS, atau cash collection: buka dan kerjakan di dalam folder [`fledger-force/`](fledger-force/).
   - Fitur OMS, katalog grosir, atau Hard Credit Gate: buka dan kerjakan di dalam folder [`fledger-order/`](fledger-order/).
   - Fitur dunning WA, rekening koran e-Statement, atau penagihan AR: buka dan kerjakan di dalam folder [`fledger-dunning/`](fledger-dunning/).
2. **Pedoman Teknis Layanan**:
   - **Fledger Dunning**: Baca spesifikasi di [`fledger-dunning/docs/AGENT-EXECUTION-BRIEF.md`](fledger-dunning/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-dunning/docs/DATABASE-SCHEMA.sql`](fledger-dunning/docs/DATABASE-SCHEMA.sql).
   - **Fledger Order**: Baca spesifikasi di [`fledger-order/docs/AGENT-EXECUTION-BRIEF.md`](fledger-order/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-order/docs/DATABASE-SCHEMA.sql`](fledger-order/docs/DATABASE-SCHEMA.sql).
   - **Fledger Force**: Baca spesifikasi di [`fledger-force/docs/AGENT-EXECUTION-BRIEF.md`](fledger-force/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-force/docs/DATABASE-SCHEMA.sql`](fledger-force/docs/DATABASE-SCHEMA.sql).
   - **Fledger Pay**: Baca spesifikasi di [`fledger-pay/docs/AGENT-EXECUTION-BRIEF.md`](fledger-pay/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-pay/docs/DATABASE-SCHEMA.sql`](fledger-pay/docs/DATABASE-SCHEMA.sql).
   - **Fledger Fleet**: Baca spesifikasi di [`fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md`](fledger-fleet/docs/AGENT-EXECUTION-BRIEF.md) dan terapkan skema dari [`fledger-fleet/docs/DATABASE-SCHEMA.sql`](fledger-fleet/docs/DATABASE-SCHEMA.sql).
3. **Peta Roadmap Global**:
   - Pelajari visi makro dan 5 masalah industri FMCG Indonesia di [`fledger-core/docs/architecture/roadmap-microservices.md`](fledger-core/docs/architecture/roadmap-microservices.md).

---

---

## 🚀 Menjalankan Seluruh Ekosistem Microservices (Master Orchestration)

Ekosistem Fledger OS kini dilengkapi dengan **Master Docker Compose** dan **Unified API Gateway** untuk menyalakan seluruh 6 microservice secara terpadu:

### Opsi A: Full Container Mode (Infrastruktur + 6 Microservices + Gateway)
```bash
# Menyalakan PostgreSQL cluster, Redis, NATS, 6 microservices, dan API Gateway (:80)
docker compose up -d

# Periksa status seluruh container
docker compose ps

# Matikan seluruh ekosistem secara rapi
docker compose down
```

### Opsi B: Hybrid / Local Host Mode (Infrastruktur di Docker, Kode di Host)
```bash
# 1. Nyalakan infrastruktur saja (Postgres multi-db, Redis, NATS)
docker compose up -d postgres redis nats

# 2. Nyalakan ke-6 service Go secara background di Windows
powershell -ExecutionPolicy Bypass -File .\scripts\start-all-local.ps1

# 3. Untuk mematikan seluruh service lokal
powershell -ExecutionPolicy Bypass -File .\scripts\stop-all-local.ps1
```

---

## 🌐 Unified Port & Web Portal Directory

Akses langsung seluruh modul operasional Fledger OS:

| Layanan / Modul | Port Langsung | Path Gateway (:80) | Tanggung Jawab & Web UI |
|---|---|---|---|
| **🌐 API Gateway Hub** | `http://localhost:80` | `/` | Central Portal Control Plane & Service Directory |
| **🟢 Fledger Core** | `http://localhost:8081` | `/core/` | Core Ledger, AR Aging & Reconciler (`/web`) |
| **🚚 Fledger Fleet** | `http://localhost:8082` | `/fleet/` | Logistik Armada & Digital POD Canvas (`/web`) |
| **💳 Fledger Pay** | `http://localhost:8083` | `/pay/` | Payment Sandbox VA & QRIS Simulator (`/web`) |
| **📱 Fledger Force** | `http://localhost:8084` | `/force/` | Sales Force Automation PWA & GPS Check-In (`/web`) |
| **📦 Fledger Order** | `http://localhost:8085` | `/order/` | B2B OMS & Hard Credit Gate (`/web`) |
| **🔔 Fledger Dunning** | `http://localhost:8086` | `/dunning/` | AR Dunning WhatsApp & PDF e-Statement (`/web`) |

---

## 🧪 "The Golden FMCG Flow" — Master E2E Ecosystem Test

Untuk menguji integritas siklus hidup distribusi B2B dari hulu ke hilir (Order ➡️ Credit Check Core ➡️ Dispatch Fleet ➡️ Digital POD ➡️ Faktur Bersih ➡️ Dunning 5-Tahap ➡️ Pembayaran QRIS/VA ➡️ Self-Healing Pembatalan ➡️ Jurnal Finansial Seimbang):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test-ecosystem-e2e.ps1
```
