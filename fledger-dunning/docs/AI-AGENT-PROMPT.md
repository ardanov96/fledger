# MASTER PROMPT & INSTRUCTION BRIEF UNTUK AI CODING AGENT
## Proyek: FLEDGER DUNNING (`fledger-dunning`) — Automated AR Dunning & WhatsApp Gateway

Salin (*copy*) seluruh teks instruksi di bawah ini dan berikan langsung ke AI Coding Agent Anda:

---

```markdown
Halo AI Agent! Anda ditugaskan untuk membangun microservice **Fledger Dunning (`fledger-dunning`)** dari awal hingga beroperasi penuh (production-ready) di dalam repositori monorepo Fledger OS.

### 📍 Workspace & Konteks Layanan
- **Direktori Kerja**: `c:\Dev\fledger\fledger-dunning` (Bekerja secara spesifik di dalam folder ini).
- **Service Name**: `fledger-dunning`
- **Default Port**: `:8086`
- **Tech Stack Wajib**:
  - Bahasa: Go 1.23+
  - Router: `github.com/go-chi/chi/v5`
  - Driver DB: PostgreSQL 16 menggunakan `github.com/jackc/pgx/v5` (`pgxpool`)
  - Format Uang: Murni `int64` / `BIGINT` minor units (Zero float rule: 1 IDR = 1 unit).
  - Frontend Portal: Embedded Vanilla JS/CSS (`embed.FS`) di `web/` (Zero Node.js/NPM dependencies).

---

### 📚 Dokumen Spesifikasi yang Wajib Dibaca Terlebih Dahulu
Sebelum menulis kode, Anda WAJIB membaca dan mematuhi dokumen acuan yang telah disiapkan di `docs/`:
1. `fledger-dunning/docs/AGENT-EXECUTION-BRIEF.md` — Blueprint arsitektur dan misi utama.
2. `fledger-dunning/docs/DATABASE-SCHEMA.sql` — Skema DDL PostgreSQL 16 (tabel konfigurasi, kontak toko, antrian, statement, pesan).
3. `fledger-dunning/docs/API-SPECIFICATION.md` — Spesifikasi kontrak REST API port :8086.
4. `fledger-dunning/docs/ROADMAP-DUNNING.md` — 5 Sprint deliverables dan kriteria Definition of Done (DoD).
5. `fledger-dunning/docs/WHATSAPP-CADENCE-AND-STATEMENT-GUIDE.md` — Panduan logika cadence, anti-ban jitter, dan PDF generator.
6. `fledger-dunning/.env.example` — Variabel konfigurasi environment.

---

### 🎯 5 Fitur Kunci yang Wajib Diimplementasikan

1. **Automated WhatsApp Dunning Cadence (5 Tahap Otomatis)**:
   - Endpoint: `POST /v1/dunning/queues/ingest-invoice`.
   - Menerima data faktur baru dari Fledger Order/Core dan secara otomatis menghitung serta menyimpan 5 jadwal antrian:
     - `PRE_DUE_H3`: H-3 sebelum jatuh tempo (Pengingat sopan).
     - `DUE_DATE`: Hari H jatuh tempo (Penagihan resmi).
     - `OVERDUE_H3`: H+3 lewat jatuh tempo (Pemberitahuan keterlambatan).
     - `OVERDUE_H7`: H+7 lewat jatuh tempo (Peringatan tegas).
     - `OVERDUE_H14`: H+14 lewat jatuh tempo (Pemberitahuan eskalasi pemblokiran kredit toko di Fledger Order).
   - Setiap pesan memuat deep-link pembayaran 1-klik ke Fledger Pay: `http://localhost:8083/pay/{invoice_id}`.

2. **Self-Healing Loop (Pembatalan Otomatis saat Lunas)**:
   - Endpoint: `POST /v1/dunning/webhooks/pay`.
   - Ketika toko membayar di Fledger Pay dan webhook masuk:
     - Eksekusi transaksi PostgreSQL atomik (`pgx.Tx`):
       `UPDATE dunning_queues SET status = 'CANCELLED_BY_PAYMENT' WHERE invoice_id = $1 AND status = 'QUEUED'`.
     - Otomatis kirim pesan WhatsApp konfirmasi tanda terima lunas resmi ke pemilik toko.

3. **Anti-Ban Jitter Engine & Spintax Generator**:
   - Di background dispatcher worker (`POST /v1/dunning/queues/cron-run` atau background loop):
     - Berikan jeda acak (*random sleep*) antara 3 s/d 8 detik antar pengiriman pesan untuk menghindari deteksi spam WhatsApp.
     - Variasikan salam dan pembuka pesan menggunakan pola spintax `{Halo|Selamat pagi|Yth.}`.

4. **Monthly PDF e-Statement Engine**:
   - Endpoint: `POST /v1/dunning/statements/generate` & `POST /v1/dunning/statements/{id}/send`.
   - Mengambil mutasi transaksi toko dari Fledger Core dan me-render dokumen PDF Rekening Koran Toko bulanan (format bersih, elegan, tabel mutasi, saldo akhir).
   - Mock/real pengiriman file PDF via WhatsApp Document API.

5. **Embedded Web Management Portal (Dark Mode Glassmorphism)**:
   - Folder: `fledger-dunning/web/` (`index.html`, `app.js`, `style.css`).
   - Disajikan langsung oleh Go binary via `//go:embed web/*` pada rute root `/`.
   - Tampilan interaktif:
     - Status & pairing QR WhatsApp.
     - Tabel antrian dunning aktif (Pending, Sent, Cancelled by Payment).
     - Simulator layar HP WhatsApp untuk preview pesan secara langsung.
     - Tombol interaktif "Simulasikan Pembayaran Toko (Self-Healing Demo)" yang memicu webhook pelunasan dan membuktikan pembatalan antrian secara real-time.

---

### ⚠️ Aturan Rekayasa Kritis (Pelajaran dari Layanan Sebelumnya)

1. **Zero Floats**: Seluruh tipe data uang di Go wajib menggunakan `int64` (minor units IDR). Dilarang keras menggunakan `float32` atau `float64`.
2. **Atomicity**: Operasi pembatalan dunning saat pelunasan wajib berada di dalam satu transaksi database (`tx.Begin` -> `tx.Commit`).
3. **Multi-Tenant Header Fallback**: Baca header `X-Tenant-ID`. Jika kosong, berikan fallback default tenant UUID (`a0000000-0000-0000-0000-000000000001`) agar request testing dan UI tidak gagal 400/401.
4. **UI Auth Token Handling**: Pada `web/app.js`, pastikan JWT token disimpan di `localStorage` dan otomatis disisipkan ke header `Authorization: Bearer <token>` pada setiap panggilan `fetch()`.
5. **WhatsApp Provider Abstraction**:
   - Buat interface `WhatsAppProvider` dengan metode `SendTextMessage`, `SendDocument`, `GetStatus`, `GenerateQR`.
   - Sediakan `MockProvider` (default di `.env`) yang mencatat pengiriman ke log dan database pesan, sehingga seluruh sistem dapat diuji 100% tanpa scan QR HP fisik.
6. **Windows Test Compatibility**:
   - Sediakan skrip `run-tests.ps1` yang menjalankan pengujian via `go test -v -count=1 ./...` langsung. Hindari menjalankan binary `.exe` secara asynchronous dengan PowerShell `Start-Process` untuk mencegah blokir Windows AppLocker.

---

### 🏗️ Rencana Struktur Direktori Wajib
Pastikan struktur kode mengikuti pola Clean Architecture:
```text
fledger-dunning/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/             <-- parse .env
│   ├── domain/             <-- structs domain, models & interfaces
│   ├── repository/
│   │   └── postgres/       <-- raw SQL queries via pgxpool
│   ├── usecase/            <-- business logic (dunning, statement, whatsapp)
│   ├── delivery/
│   │   └── http/           <-- Chi handlers, router, middleware
│   └── platform/
│       ├── whatsapp/       <-- Provider interface, MockProvider, etc.
│       └── pdf/            <-- PDF generation engine
├── web/                    <-- HTML, CSS, JS portal (embedded)
├── docs/                   <-- Dokumentasi & skema yang sudah ada
├── go.mod
├── .env.example
├── README.md
└── run-tests.ps1
```

---

### 🚀 Target Definition of Done (DoD)
Pekerjaan Anda dianggap SELESAI jika:
1. `go build ./cmd/server` berhasil tanpa compile error.
2. Seluruh endpoint di `API-SPECIFICATION.md` terimplementasi dan merespons dengan benar.
3. Alur Ingest -> Jadwal 5 Tahap -> Webhook Pembayaran -> Pembatalan Atomik (Self-Healing) terverifikasi berjalan lancar.
4. Web UI di port `:8086` menyajikan antarmuka visual yang modern, interaktif, dan berfungsi penuh.
5. Unit tests dan integration tests lulus 100% saat dieksekusi dengan `go test -v ./...`.

Selamat bekerja! Buat kode yang bersih, terdokumentasi rapi, dan kokoh untuk produksi!
```
