# FLEDGER PAY — AI Agent Execution Brief & Technical Blueprint

> **Parent Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for Distribution & Supply Chain*)  
> **Service Name**: **Fledger Pay** (`fledger-pay`)  
> **Default Port**: `:8083`  
> **Target Audience**: Autonomous AI Coding Agent / Lead Software Engineer  
> **Goal**: Membangun dan mengoperasikan microservice **Fledger Pay** sebagai engine *Payment Gateway Ingestion*, *Multi-Bank Virtual Account & QRIS Generator*, serta *Automated Double-Entry Settlement* ke **Fledger Core**.

---

## 1. Konteks Bisnis & Misi Utama untuk AI Agent

Dalam distribusi FMCG dan perdagangan B2B di Indonesia, pembayaran tunai konvensional di lapangan menimbulkan 3 risiko fatal bagi distributor:
1. **Cash Kitting / Gali Lubang Tutup Lubang**: Salesman atau supir menerima uang tunai dari toko tetapi menggunakannya untuk kepentingan pribadi berhari-hari sebelum disetor ke kasir kantor.
2. **Risiko Keamanan Fisik & Uang Palsu**: Membawa uang fisik puluhan juta di jalanan rentan terhadap pembegalan dan penerimaan uang palsu dari retailer.
3. **Rekonsiliasi Manual Menumpuk di HQ**: Staf keuangan menghabiskan ratusan jam mencocokkan mutasi rekening koran bank secara manual dengan ribuan faktur terbuka.

### Misi Anda (AI Agent)
Anda ditugaskan untuk membangun microservice **`fledger-pay`** yang menyelesaikan seluruh masalah di atas secara otomatis:
- **Multi-Bank VA & Dynamic QRIS Generator**: Menerbitkan nomor rekening Virtual Account unik perbankan nasional (BCA, Mandiri, BRI, BNI) dan kode QRIS dinamis langsung ketika sebuah Surat Jalan / Faktur diterbitkan.
- **Webhook Callback Ingestion**: Menerima notifikasi pelunasan instan dari Payment Gateway (Midtrans, Xendit, atau Simulator Bank).
- **Anti-Fraud & Idempotency Engine**: Memvalidasi signature kriptografis HMAC SHA-256 dan mencegah double-crediting saat terjadi *duplicate webhook deliveries*.
- **Automated Settlement Bridge ke Fledger Core**: Begitu status bayar `SETTLED`, sistem secara transaksional (via Outbox Pattern) memicu pencatatan jurnal *double-entry* di **Fledger Core** (`:8081`):
  $$\text{Debit: Kas Bank BCA} \quad | \quad \text{Kredit: Piutang Dagang (AR)}$$
  dan mengubah status Faktur di Core menjadi `PAID`.

---

## 2. Arsitektur Alur Transaksi Antar-Layanan

```mermaid
sequenceDiagram
    autonumber
    actor Toko as 🏪 Pemilik Toko
    participant PayUI as 💻 Web Portal / Simulator (:8083)
    participant PayAPI as ⚙️ Fledger Pay API (:8083)
    participant PayDB as 🗄️ PostgreSQL Pay
    participant Gateway as 🏦 Payment Gateway / Bank
    participant CoreAPI as 🟢 Fledger Core (:8081)

    Note over Toko,CoreAPI: Tahap 1: Penerbitan Tagihan Digital
    CoreAPI->>PayAPI: POST /v1/pay/requests (Invoice ID, Amount Rp 4.000.000)
    PayAPI->>PayAPI: Generate BCA VA (89012...) & Mandiri VA & QRIS EMVCo
    PayAPI->>PayDB: Simpan payment_request & virtual_accounts
    PayAPI-->>CoreAPI: 201 Created (Nomor VA & String QRIS)

    Note over Toko,Gateway: Tahap 2: Pembayaran oleh Pemilik Toko
    Toko->>Gateway: Bayar via BCA Mobile / Scan QRIS (Rp 4.000.000)
    Gateway-->>Toko: Bukti Transfer Berhasil

    Note over Gateway,CoreAPI: Tahap 3: Webhook & Auto-Settlement
    Gateway->>PayAPI: POST /v1/pay/webhooks/midtrans (Payload + Signature)
    PayAPI->>PayAPI: Verifikasi HMAC Signature & Idempotency Key
    PayAPI->>PayDB: BEGIN TX: Insert pay_transactions, Update status = SETTLED, Enqueue Outbox: COMMIT TX
    PayAPI-->>Gateway: 200 OK (Acknowledge)

    rect rgb(235, 250, 235)
        Note over PayAPI,CoreAPI: Tahap 4: Background Outbox Worker ke Fledger Core
        PayAPI->>CoreAPI: POST /v1/transfers (Debit Bank BCA, Kredit AR Toko)
        CoreAPI-->>PayAPI: 201 Created (Journal Balanced)
        PayAPI->>CoreAPI: POST /v1/invoices/{id}/pay (Status = PAID)
        CoreAPI-->>PayAPI: 200 OK
        PayAPI->>PayDB: Tandai Outbox = SENT
    end
```

---

## 3. State Machines & Status Siklus Hidup

### 3.1. Payment Request State Machine
```
[PENDING] ────► [SETTLED] (Pembayaran lunas diterima)
    │
    ├───► [PARTIALLY_PAID] (Pembayaran bertahap)
    ├───► [EXPIRED] (Melewati batas waktu bayar tanpa transaksi)
    └───► [CANCELLED] (Faktur dibatalkan atau retur total)
```

### 3.2. Outbox Settlement State Machine
```
[PENDING] ➔ [PROCESSING] ➔ [SENT]
    │             │
    └─── [FAILED] ◄─── (Retry dengan Exponential Backoff)
```

---

## 4. Struktur Direktori Proyek Bersih (Clean Architecture)

AI Agent wajib menyusun codebase `fledger-pay` mengikuti standar arsitektur bersih yang telah terbukti pada `fledger-fleet`:

```text
fledger-pay/
├── docs/                             <-- Dokumentasi teknis & spesifikasi
│   ├── AGENT-EXECUTION-BRIEF.md      <-- Panduan eksekusi ini
│   ├── ROADMAP-PAY.md                <-- Tahapan sprint 1-5
│   ├── DATABASE-SCHEMA.sql           <-- Skema DDL PostgreSQL
│   ├── API-SPECIFICATION.md          <-- Kontrak REST API
│   └── SIMULATOR-AND-SETTLEMENT-GUIDE.md
├── src/                              <-- Source code backend Go
│   ├── cmd/
│   │   ├── api/                      <-- main.go (HTTP Server Chi, CORS, routes)
│   │   └── migrator/                 <-- Runner migrasi PostgreSQL otomatis
│   ├── internal/
│   │   ├── auth/jwt/                 <-- Verifier & Signer JWT
│   │   ├── config/                   <-- Config loader (.env)
│   │   ├── domain/                   <-- Entitas inti: payment, va, qris, outbox
│   │   ├── handler/                  <-- HTTP Handlers & Webhooks
│   │   ├── integration/coreclient/   <-- HTTP Client ke Fledger Core
│   │   ├── platform/httpx/           <-- Helper JSON & error responses
│   │   ├── platform/log/             <-- Structured JSON logging
│   │   ├── repository/postgres/      <-- SQL queries via pgx/v5
│   │   └── usecase/                  <-- Business logic & Outbox background worker
│   ├── migrations/                   <-- File migrasi SQL (000001_init_pay.up.sql)
│   ├── go.mod                        <-- Go module
│   └── go.sum
├── web/                              <-- Sandbox Payment Simulator UI (HTML5/CSS/JS)
│   ├── index.html                    <-- Portal Simulator Pembayaran
│   ├── style.css                     <-- Styling modern
│   ├── pay-client.js                 <-- Client SDK JavaScript
│   └── app.js                        <-- Logika simulator interaktif
├── .env.example                      <-- Template variabel lingkungan
└── README.md                         <-- Onboarding guide
```

---

## 5. Aturan Penting & Kendala Teknis (Constraints)

1. **Integritas Mata Uang**:
   - DILARANG menggunakan tipe data `float32` atau `float64` untuk nominal uang!
   - Seluruh nominal wajib bertipe `int64` (BIGINT) dalam satuan rupiah minor/bulat.
2. **Idempotency Wajib**:
   - Webhook bank dapat dikirim ulang berkali-kali saat terjadi network timeout.
   - Periksa `pay_transactions.external_reference` atau `idempotency_key` sebelum memproses settlement. Jika sudah ada, kembalikan `200 OK` tanpa membuat transaksi duplikat.
3. **Outbox Pattern Transaksional**:
   - Jangan pernah memanggil API luar (`Fledger Core`) langsung di dalam database transaction commit.
   - Simpan mutasi pembayaran dan antrean outbox di database lokal terlebih dahulu, lalu background worker mengeksekusi panggilan ke Core.
4. **CORS Terbuka**:
   - Pasang CORS middleware pada router Chi agar portal frontend atau aplikasi lain dapat memanggil API `:8083`.
5. **Zero Dependency Web UI**:
   - Buat Web Portal Simulator di `fledger-pay/web/` menggunakan Vanilla HTML5/CSS/JS murni (tanpa npm build) yang disajikan langsung oleh `cmd/api/main.go`.

---

## 6. Standar Variabel Lingkungan (`.env`)

```ini
APP_ENV=development
PORT=8083

DATABASE_URL=postgres://postgres:postgres@localhost:5432/fledger_pay?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Fledger Core API Integration
FLEDGER_CORE_URL=http://localhost:8081
FLEDGER_TENANT_ID=00000000-0000-0000-0000-000000000001
FLEDGER_CORE_API_KEY=dev-fleet-key-fledger-2026

# Outbox Worker Settings
OUTBOX_POLL_INTERVAL=3s
OUTBOX_REQUEST_TIMEOUT=5s
OUTBOX_MAX_ATTEMPTS=10

# Security & Webhook Signatures
JWT_SECRET=super-secret-fledger-pay-jwt-key-minimum-32-chars!
WEBHOOK_SECRET=dev-webhook-secret-key-midtrans-xendit
```

---

## 7. Checklist Verifikasi Akhir AI Agent

Sebelum menyelesaikan tugas, AI Agent wajib memverifikasi:
- [ ] `go test ./...` lulus 100% tanpa kegagalan di `fledger-pay/src`.
- [ ] `go build ./cmd/api` dan `go build ./cmd/migrator` sukses tanpa warning.
- [ ] Endpoint `/healthz` merespons `200 alive` dan `/readyz` merespons `200 ready` dengan database ping.
- [ ] Skenario simulasi pembayaran:
  1. Buat payment request (`POST /v1/pay/requests`).
  2. Simulasikan bayar via endpoint `/v1/pay/simulator/settle`.
  3. Status berubah menjadi `SETTLED`.
  4. Outbox worker mengirimkan mutasi settlement ke `Fledger Core`.
