# FLEDGER FORCE — AI Agent Execution Brief & Technical Blueprint

> **Parent Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for Distribution & Supply Chain*)  
> **Service Name**: **Fledger Force** (`fledger-force`)  
> **Default Port**: `:8084`  
> **Target Audience**: Autonomous AI Coding Agent / Lead Software Engineer  
> **Goal**: Membangun dan mengoperasikan microservice **Fledger Force** sebagai engine *Sales Force Automation (SFA)*, *Geofenced Beat Plan*, *Mobile Cash Collection*, dan *Anti-Cash Kitting Daily Settlement Lock* terintegrasi ke **Fledger Core**.

---

## 1. Konteks Bisnis & Misi Utama untuk AI Agent

Dalam distribusi FMCG dan perdagangan B2B di Indonesia, salesman keliling (*canvasser* / motoris) adalah garda terdepan yang paling rentan terhadap kecurangan keuangan (*fraud* lapangan):

### Masalah 1: Praktik *Cash Kitting* (Gali Lubang Tutup Lubang)
* Salesman menagih uang tunai Rp 10–30 juta dari toko A pada hari Senin.
* Uang tersebut tidak langsung disetorkan ke kasir kantor hari itu, melainkan dipakai untuk keperluan pribadi.
* Hari Kamis, salesman menagih toko B dan memakai uang toko B untuk menutup tagihan toko A.
* Jika perputaran uang macet, salesman melarikan diri dan distributor menanggung kerugian puluhan hingga ratusan juta rupiah.

### Masalah 2: Kunjungan Fiktif (*Ghost Visits*)
* Salesman mengaku telah mengunjungi 20 toko, namun kenyataannya hanya nongkrong di warung kopi. Toko tidak pernah dikunjungi, dan pesanan atau uang tagihan terabaikan.

---

### Misi Anda (AI Agent)
Anda ditugaskan untuk membangun microservice **`fledger-force`** yang menuntaskan kedua masalah di atas secara sistemik:
1. **Rute Kunjungan Terverifikasi GPS (*Geofenced Beat Plan*)**:
   - Memastikan salesman hanya bisa check-in jika posisi GPS HP berada dalam radius toleransi (default: 100 meter) dari koordinat resmi toko menggunakan rumus Haversine.
2. **Mobile Cash Collection & Tanda Terima Digital**:
   - Begitu toko membayar tunai ke salesman, sistem langsung menerbitkan kwitansi digital unik (`RCP-YYYYMM-XXXXX`) dan mengirimkan payload notifikasi WhatsApp ke pemilik toko. Salesman tidak bisa lagi berdalih "toko belum bayar".
3. **Pemindahan Beban Uang ke Salesman Wallet di Fledger Core**:
   - Secara transaksional (via Outbox Pattern), mutasi dicatat di Fledger Core:
     $$\text{Debit: Account:Salesman-Wallet} \quad | \quad \text{Kredit: Account:Customer-AR}$$
     Piutang toko lunas, dan beban uang fisik **100% berpindah menjadi hutang pribadi salesman** kepada perusahaan.
4. **Daily Settlement Lock (Kunci Setoran Sore Hari)**:
   - Di sore hari (EOD), kasir gudang memindai QR saldo salesman dan menghitung uang fisik.
   - Jika uang fisik cocok, kasir mengonfirmasi setoran, memicu mutasi di Fledger Core:
     $$\text{Debit: Account:HQ-Cash} \quad | \quad \text{Kredit: Account:Salesman-Wallet}$$
   - Saldo kas salesman kembali ke 0. Jika salesman belum setor, statusnya terkunci (`SETTLEMENT_LOCKED`) dan tidak dapat mengambil pesanan baru esok hari.

---

## 2. Arsitektur Alur Transaksi Antar-Layanan

```mermaid
sequenceDiagram
    autonumber
    actor Sales as 🛵 Salesman (Canvasser)
    actor Toko as 🏪 Pemilik Toko
    participant ForceAPI as ⚙️ Fledger Force (:8084)
    participant ForceDB as 🗄️ PostgreSQL Force
    participant CoreAPI as 🟢 Fledger Core (:8081)
    actor Kasir as 👩‍💼 Kasir Gudang HQ

    Note over Sales,Toko: 1. Verifikasi GPS & Terima Kas di Lapangan
    Sales->>ForceAPI: POST /v1/force/visits/check-in (GPS Lat/Long)
    ForceAPI->>ForceAPI: Hitung Jarak Haversine (<= 100m -> Geofence OK)
    ForceAPI-->>Sales: 201 Created (Check-in terverifikasi)
    
    Toko->>Sales: Bayar uang kas Rp 2.500.000 untuk Faktur INV-001
    Sales->>ForceAPI: POST /v1/force/collections (Invoice ID, Rp 2.500.000)
    ForceAPI->>ForceDB: Insert cash_collections, update salesman cash_held (+Rp 2.5jt), Enqueue Outbox
    ForceAPI-->>Sales: 201 Created (No Kwitansi: RCP-09812 & WA Payload ke Toko)

    rect rgb(235, 250, 235)
        Note over ForceAPI,CoreAPI: Sinkronisasi Outbox ke Core Ledger
        ForceAPI->>CoreAPI: POST /v1/transfers (Debit Salesman-Wallet, Kredit Customer-AR)
        CoreAPI-->>ForceAPI: 201 Created (Tanggung jawab uang resmi di pundak Salesman)
    end

    Note over Sales,Kasir: 2. Setor Uang Fisik Sore Hari di Kantor Gudang
    Sales->>Kasir: Tiba di gudang, serahkan uang fisik Rp 2.500.000
    Kasir->>ForceAPI: GET /v1/force/settlements/reps/{id}/inquiry
    ForceAPI-->>Kasir: Wajib setor: Rp 2.500.000 (1 kwitansi)
    Kasir->>ForceAPI: POST /v1/force/settlements (Uang fisik: Rp 2.500.000)
    ForceAPI->>ForceDB: Reset salesman cash_held = 0, status = ACTIVE
    
    rect rgb(235, 250, 235)
        Note over ForceAPI,CoreAPI: Sinkronisasi EOD ke Core Ledger
        ForceAPI->>CoreAPI: POST /v1/transfers (Debit HQ-Cash, Kredit Salesman-Wallet)
        CoreAPI-->>ForceAPI: 201 Created (Saldo hutang Salesman kembali Rp 0)
    end
    ForceAPI-->>Kasir: 200 OK (Setoran EOD Selesai, Salesman siap bekerja besok)
```

---

## 3. Formula Matematika Geofencing (Haversine Formula)

AI Agent wajib mengimplementasikan formula Haversine untuk menghitung jarak antara koordinat HP salesman $(\phi_1, \lambda_1)$ dan koordinat resmi toko $(\phi_2, \lambda_2)$:

$$a = \sin^2\left(\frac{\Delta \phi}{2}\right) + \cos(\phi_1) \cdot \cos(\phi_2) \cdot \sin^2\left(\frac{\Delta \lambda}{2}\right)$$
$$c = 2 \cdot \text{atan2}\left(\sqrt{a}, \sqrt{1-a}\right)$$
$$d = R \cdot c \quad (\text{di mana } R = 6.371.000 \text{ meter})$$

- Jika $d \le \text{geofence\_radius\_meters}$ (default: 100m): `geofence_verified = true`.
- Jika $d > \text{geofence\_radius\_meters}$: `geofence_verified = false`, tetap boleh check-in tetapi ditandai *out-of-radius* dan dicatat di `force_audit_logs`.

---

## 4. Struktur Direktori Proyek Bersih (Clean Architecture)

```text
fledger-force/
├── docs/                             <-- Dokumentasi teknis & spesifikasi
│   ├── AGENT-EXECUTION-BRIEF.md      <-- Panduan eksekusi ini
│   ├── ROADMAP-FORCE.md              <-- Rincian sprint 1-5
│   ├── DATABASE-SCHEMA.sql           <-- Skema DDL PostgreSQL 16
│   ├── API-SPECIFICATION.md          <-- Kontrak REST API
│   └── CASH-COLLECTION-AND-EOD-GUIDE.md
├── src/                              <-- Source code backend Go
│   ├── cmd/
│   │   ├── api/                      <-- main.go (HTTP Server Chi, CORS, routes)
│   │   └── migrator/                 <-- Runner migrasi PostgreSQL otomatis
│   ├── internal/
│   │   ├── auth/jwt/                 <-- Verifier & Signer JWT
│   │   ├── config/                   <-- Config loader (.env)
│   │   ├── domain/                   <-- Entitas: sales_rep, store, visit, collection
│   │   ├── handler/                  <-- HTTP Handlers
│   │   ├── integration/coreclient/   <-- HTTP Client ke Fledger Core
│   │   ├── platform/geo/             <-- Algoritma Haversine distance calculator
│   │   ├── platform/httpx/           <-- Helper JSON & error responses
│   │   ├── platform/log/             <-- Structured JSON logging
│   │   ├── repository/postgres/      <-- SQL queries via pgx/v5
│   │   └── usecase/                  <-- Business logic & Outbox background worker
│   ├── migrations/                   <-- File migrasi SQL (000001_init_force.sql)
│   ├── go.mod                        <-- Go module
│   └── go.sum
├── web/                              <-- PWA Mobile Salesman & Cashier Portal
│   ├── index.html                    <-- Portal SFA
│   ├── style.css                     <-- Styling modern mobile-friendly
│   ├── force-client.js               <-- Client SDK JavaScript
│   └── app.js                        <-- Logika PWA (Check-in, Bayar, Kasir)
├── .env.example                      <-- Template variabel lingkungan
└── README.md                         <-- Onboarding guide
```

---

## 5. Batasan Teknis & Standar Integritas

1. **Integritas Mata Uang**: DILARANG menggunakan tipe data float. Semua nominal menggunakan `int64` / `BIGINT`.
2. **Plafon Kas Maksimal**: Salesman memiliki `max_cash_limit` (default Rp 50.000.000). Jika penerimaan kas baru akan melampaui plafon, sistem wajib menolak dengan HTTP 422: *"Batas kas fisik tercapai. Harap lakukan setoran ke kasir terlebih dahulu."*
3. **Outbox Pattern Transaksional**: Mutasi ke Fledger Core dilakukan melalui antrean outbox lokal agar offline-safe dan tahan gangguan jaringan.
4. **CORS & Embedded UI**: Binary Go menyajikan file `web/` secara mandiri melalui `//go:embed all:files`.

---

## 6. Standar Variabel Lingkungan (`.env`)

```ini
APP_ENV=development
PORT=8084

DATABASE_URL=postgres://postgres:postgres@localhost:5432/fledger_force?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Fledger Core Integration
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
FLEDGER_CORE_API_KEY=dev-force-key-fledger-2026

# Outbox Worker Settings
OUTBOX_POLL_INTERVAL=3s
OUTBOX_REQUEST_TIMEOUT=5s
OUTBOX_MAX_ATTEMPTS=10

# Security
JWT_SECRET=super-secret-fledger-force-jwt-key-minimum-32-chars!
TOKEN_TTL=24h
```

---

## 7. Checklist Verifikasi Akhir AI Agent

Sebelum menyelesaikan tugas, AI Agent wajib memverifikasi:
- [ ] `go test ./...` lulus 100% tanpa kegagalan di `fledger-force/src`.
- [ ] `go build ./cmd/api` dan `go build ./cmd/migrator` sukses tanpa warning.
- [ ] Endpoint `/healthz` merespons `200 alive` dan `/readyz` merespons `200 ready` dengan database ping.
- [ ] Skenario E2E:
  1. Check-in toko dengan kalkulasi Haversine GPS.
  2. Terima uang kas toko (`POST /v1/force/collections`) -> Saldo kas salesman bertambah.
  3. Outbox worker memindahkan piutang toko ke wallet salesman di Fledger Core.
  4. Kasir menjalankan EOD settlement (`POST /v1/force/settlements`) -> Saldo kas salesman kembali ke 0.
