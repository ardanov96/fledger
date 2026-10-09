# FLEDGER ORDER — AI Agent Execution Brief & Technical Blueprint

> **Parent Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for Distribution & Supply Chain*)  
> **Service Name**: **Fledger Order** (`fledger-order`)  
> **Default Port**: `:8085`  
> **Target Audience**: Autonomous AI Coding Agent / Lead Software Engineer  
> **Goal**: Membangun dan mengoperasikan microservice **Fledger Order** sebagai engine *B2B Order Management System (OMS)*, *Multi-Tier FMCG Catalog*, *Warehouse Stock Reservation*, *Hard Credit Gate (Anti-Credit Blindness)*, dan *Automated Fleet Dispatching Bridge*.

---

## 1. Konteks Bisnis & Misi Utama untuk AI Agent

Dalam distribusi FMCG dan perdagangan grosir di Indonesia, kendala fatal pada proses pesanan konvensional adalah:

### Masalah: Kebutaan Limit Kredit (*Credit Limit Blindness*)
* Tim sales dan staf gudang didorong oleh target omset bulanan.
* Mereka terus menerima pesanan dari toko yang sebenarnya telah menunggak faktur lebih dari 30 hingga 60 hari di bagian akuntansi pusat.
* Ketika distributor melakukan audit tahunan, ditemukan tumpukan piutang tak tertagih (*Bad Debt*) hingga ratusan juta rupiah yang mengancam arus kas perusahaan.

---

### Misi Anda (AI Agent)
Anda ditugaskan untuk membangun microservice **`fledger-order`** yang menuntaskan masalah tersebut:
1. **Katalog Produk FMCG & Tiering Harga Bertingkat**:
   - Menyediakan katalog SKU dengan satuan dus/karton dan penetapan harga bertingkat sesuai tier toko (`GROSIR`, `SEMI_GROSIR`, `RETAIL`, `STAR_OUTLET`).
2. **Alokasi & Penguncian Stok Fisik Gudang (*Stock Reservation*)**:
   - Mengunci stok fisik saat pesanan dibuat (`reserved_qty`) agar terhindar dari *overselling* (menjual barang yang stok fisiknya sudah habis di gudang).
3. **Hard Credit Gate (Pre-Flight Check ke Fledger Core)**:
   - Sebelum pesanan disetujui, sistem secara otomatis mengevaluasi 2 aturan ketat ke Fledger Core (`:8081`):
     - **Aturan 1 (Plafon Kredit)**: $(\text{Piutang Berjalan} + \text{Nilai Order Baru}) \le \text{Credit Limit Toko}$.
     - **Aturan 2 (Toleransi Aging)**: Toko tidak boleh memiliki tagihan terbuka $>30$ hari di Fledger Core.
     - Jika salah satu dilanggar, status pesanan otomatis dikunci menjadi **`CREDIT_BLOCKED`** (hanya bisa dibuka lewat *managerial override* ber-PIN khusus).
4. **Automated Dispatching Bridge ke Fledger Fleet**:
   - Begitu pesanan disetujui (`APPROVED`), sistem secara transaksional (via Outbox Pattern) menghitung total tonase muatan dalam kilogram dan memicu pembuatan Surat Jalan (DO) resmi di **Fledger Fleet** (`:8082`):
     `POST /v1/fleet/delivery-orders`

---

## 2. Arsitektur Alur Transaksi Antar-Layanan

```mermaid
sequenceDiagram
    autonumber
    actor Toko as 🏪 Pemilik Toko / Salesman
    participant OrderAPI as ⚙️ Fledger Order (:8085)
    participant OrderDB as 🗄️ PostgreSQL Order
    participant CoreAPI as 🟢 Fledger Core (:8081)
    participant FleetAPI as 🚚 Fledger Fleet (:8082)
    actor Finance as 👨‍💼 Finance Manager

    Note over Toko,OrderAPI: 1. Pembuatan Pesanan B2B
    Toko->>OrderAPI: POST /v1/order/orders (Items: 10 Dus Bimoli, 20 Dus Indomie)
    OrderAPI->>OrderDB: Kunci stok fisik (reserved_qty +30), Simpan order = PENDING_CREDIT_CHECK
    OrderAPI-->>Toko: 201 Created (ORD-00812, Total Rp 4.000.000, Berat 320 Kg)

    Note over OrderAPI,CoreAPI: 2. Evaluasi Hard Credit Gate ke Fledger Core
    OrderAPI->>CoreAPI: GET /v1/invoices & GET /v1/aging (Toko A)
    CoreAPI-->>OrderAPI: AR Berjalan Rp 8.5jt (Limit Rp 20jt), Overdue >30d = FALSE
    
    alt Lolos Evaluasi Kredit (Happy Path)
        OrderAPI->>OrderDB: Update order status = APPROVED, credit_gate = PASSED, Enqueue Outbox
    else Gagal Evaluasi Kredit (Toko Menunggak)
        OrderAPI->>OrderDB: Update order status = CREDIT_BLOCKED, credit_gate = OVERDUE_BLOCKED
        Finance->>OrderAPI: POST /v1/order/orders/{id}/override-credit (Otorisasi Darurat PIN)
        OrderAPI->>OrderDB: Update order status = APPROVED, credit_gate = OVERRIDDEN, Enqueue Outbox
    end

    rect rgb(235, 250, 235)
        Note over OrderAPI,FleetAPI: 3. Dispatching Bridge ke Fledger Fleet
        OrderAPI->>FleetAPI: POST /v1/fleet/delivery-orders (DO-00812, 320 Kg, Alamat Toko)
        FleetAPI-->>OrderAPI: 201 Created (Surat Jalan DO Terbit di Fleet Hub)
        OrderAPI->>OrderDB: Simpan fledger_fleet_do_id, Update order = DISPATCHED_TO_FLEET, Tandai Outbox SENT
    end
```

---

## 3. Struktur Direktori Proyek Bersih (Clean Architecture)

```text
fledger-order/
├── docs/                             <-- Dokumentasi teknis & spesifikasi
│   ├── AGENT-EXECUTION-BRIEF.md      <-- Panduan eksekusi ini
│   ├── ROADMAP-ORDER.md              <-- Rincian sprint 1-5
│   ├── DATABASE-SCHEMA.sql           <-- Skema DDL PostgreSQL 16
│   ├── API-SPECIFICATION.md          <-- Kontrak REST API
│   └── HARD-CREDIT-GATE-AND-FLEET-BRIDGE-GUIDE.md
├── src/                              <-- Source code backend Go
│   ├── cmd/
│   │   ├── api/                      <-- main.go (HTTP Server Chi, CORS, routes)
│   │   └── migrator/                 <-- Runner migrasi PostgreSQL otomatis
│   ├── internal/
│   │   ├── auth/jwt/                 <-- Verifier & Signer JWT
│   │   ├── config/                   <-- Config loader (.env)
│   │   ├── domain/                   <-- Entitas: product, pricing, stock, order
│   │   ├── handler/                  <-- HTTP Handlers
│   │   ├── integration/coreclient/   <-- HTTP Client ke Fledger Core (Aging & Invoices)
│   │   ├── integration/fleetclient/  <-- HTTP Client ke Fledger Fleet (Create DO)
│   │   ├── platform/httpx/           <-- Helper JSON & error responses
│   │   ├── platform/log/             <-- Structured JSON logging
│   │   ├── repository/postgres/      <-- SQL queries via pgx/v5
│   │   └── usecase/                  <-- Business logic & Outbox background worker
│   ├── migrations/                   <-- File migrasi SQL (000001_init_order.sql)
│   ├── go.mod                        <-- Go module
│   └── go.sum
├── web/                              <-- B2B Ordering Web Portal UI
│   ├── index.html                    <-- Portal Toko & Admin
│   ├── style.css                     <-- Styling modern responsif
│   ├── order-client.js               <-- Client SDK JavaScript
│   └── app.js                        <-- Logika Keranjang, Order, & Credit Gate
├── .env.example                      <-- Template variabel lingkungan
└── README.md                         <-- Onboarding guide
```

---

## 4. Batasan Teknis & Standar Integritas

1. **Integritas Mata Uang**: DILARANG menggunakan tipe data float. Semua nominal menggunakan `int64` / `BIGINT`.
2. **Kalkulasi Berat Akumulatif**: Setiap item SKU wajib mengalikan kuantitas dengan `weight_grams` untuk menghasilkan `total_weight_kg` yang dibutuhkan truk `fledger-fleet` untuk meter kapasitas muatan.
3. **Outbox Pattern Transaksional**: Pemanggilan penerbitan Surat Jalan ke `fledger-fleet` wajib menggunakan antrean outbox lokal agar tahan gangguan jaringan / restart server.
4. **Single Binary Deployment**: Web portal di `web/` wajib disajikan langsung oleh binary Go menggunakan `//go:embed all:files`.

---

## 5. Standar Variabel Lingkungan (`.env`)

```ini
APP_ENV=development
PORT=8085

DATABASE_URL=postgres://postgres:postgres@localhost:5432/fledger_order?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Fledger Core Integration
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
FLEDGER_CORE_API_KEY=dev-order-key-fledger-2026

# Fledger Fleet Integration
FLEDGER_FLEET_URL=http://localhost:8082

# Outbox Worker Settings
OUTBOX_POLL_INTERVAL=3s
OUTBOX_REQUEST_TIMEOUT=5s
OUTBOX_MAX_ATTEMPTS=10

# Security
JWT_SECRET=super-secret-fledger-order-jwt-key-minimum-32-chars!
TOKEN_TTL=24h
MANAGER_OVERRIDE_PIN=992144
```

---

## 6. Checklist Verifikasi Akhir AI Agent

Sebelum menyelesaikan tugas, AI Agent wajib memverifikasi:
- [ ] `go test ./...` lulus 100% tanpa kegagalan di `fledger-order/src`.
- [ ] `go build ./cmd/api` dan `go build ./cmd/migrator` sukses tanpa warning.
- [ ] Endpoint `/healthz` merespons `200 alive` dan `/readyz` merespons `200 ready` dengan database ping.
- [ ] Skenario E2E:
  1. Buat pesanan baru (`POST /v1/order/orders`) -> Stok fisik ter-reserve.
  2. Evaluasi Hard Credit Gate (`POST /v1/order/orders/:id/evaluate-credit`) -> Berhasil lolos / blokir sesuai data Core.
  3. Outbox worker secara otomatis menerbitkan Surat Jalan (DO) di `fledger-fleet`.
