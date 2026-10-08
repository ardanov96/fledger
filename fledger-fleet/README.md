# Fledger Fleet — Fleet Logistics & Proof of Delivery Engine

> **Service**: `fledger-fleet`  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for Distribution & Supply Chain*)  
> **Status**: Ready for AI Agent / Engineer Execution

---

## 🎯 Tentang Service Ini

**Fledger Fleet** adalah microservice pengelola operasional logistik, armada pengiriman, Surat Jalan (*Delivery Order / DO*), dan **Digital Proof of Delivery (POD)** dalam ekosistem **FLEDGER OS**.

Service ini dirancang untuk menyelesaikan sengketa barang rusak antara supir, toko, dan salesman dengan cara:
1. Menangkap bukti penerimaan fisik di tempat (tanda tangan digital penerima & foto barang rusak).
2. Menghitung otomatis nominal bersih barang yang benar-benar diterima toko.
3. Mengirimkan sinyal penyelesaian pengantaran secara otomatis ke **Fledger Core API** (`POST /v1/invoices`) agar faktur resmi langsung terbit dengan nominal yang tepat tanpa perlu revisi nota manual.

---

## 🤖 Panduan untuk AI Coding Agent

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
JWT_SECRET=super-secret-key-32-characters-minimum
```

---

## 📁 Struktur Folder Proyek

```text
fledger-fleet/
├── docs/                           <-- Seluruh dokumen spesifikasi, schema, & roadmap
│   ├── AGENT-EXECUTION-BRIEF.md    <-- Blueprint instruksi eksekusi AI Agent
│   ├── DATABASE-SCHEMA.sql         <-- Skrip DDL PostgreSQL 16
│   ├── API-SPECIFICATION.md        <-- Spesifikasi REST API & Fledger Core client
│   └── ROADMAP-FLEET.md            <-- Sprint 1 s/d 5 roadmap & Definition of Done
├── src/                            <-- Tempat kode sumber implementasi backend
├── .env.example                    <-- Template environment variables
└── README.md                       <-- Berkas ini
```
