# FLEDGER PAY — Sprint Roadmap & Definition of Done

> **Parent Ecosystem**: **FLEDGER OS**  
> **Service**: `fledger-pay`  
> **Execution Mode**: Autonomous AI Agent / Lead Engineer Sprint

---

## 📅 Rincian 5 Sprint Pengembangan

```mermaid
gantt
    title Roadmap Pengembangan Fledger Pay
    dateFormat  YYYY-MM-DD
    section Sprint 1
    DB Schema & Migration Setup          :active, s1_1, 2026-10-10, 2026-10-13
    Config Loader & Health Checks        :s1_2, 2026-10-12, 2026-10-14
    section Sprint 2
    Payment Request Engine               :s2_1, 2026-10-15, 2026-10-18
    Multi-Bank VA & QRIS Generator       :s2_2, 2026-10-18, 2026-10-21
    section Sprint 3
    Webhook Callback Ingestion Engine    :s3_1, 2026-10-22, 2026-10-25
    HMAC Signature & Idempotency Guard   :s3_2, 2026-10-25, 2026-10-28
    section Sprint 4
    Fledger Core Settlement Client       :s4_1, 2026-10-29, 2026-11-02
    Transactional Outbox Drain Worker    :s4_2, 2026-11-02, 2026-11-05
    section Sprint 5
    Interactive Simulator Web Portal     :s5_1, 2026-11-06, 2026-11-08
    E2E Smoke Tests & Documentation      :s5_2, 2026-11-08, 2026-11-10
```

---

### Sprint 1: Pondasi Basis Data, Konfigurasi & Health Probes
- **Target**: Basis data PostgreSQL berjalan dengan seluruh tabel pembayaran dan migrasi otomatis.
- **Deliverables**:
  - Script migrasi `migrations/` mengimplementasikan skrip [`DATABASE-SCHEMA.sql`](DATABASE-SCHEMA.sql).
  - Binary `cmd/migrator` dan `cmd/api` dengan graceful shutdown.
  - Config loader membaca `.env` (`PORT=8083`, `DATABASE_URL`, `FLEDGER_CORE_URL`, `WEBHOOK_SECRET`).
  - Liveness probe `/healthz` dan Deep readiness probe `/readyz` (PostgreSQL ping).
  - Dev token auth helper `/v1/dev/login`.

---

### Sprint 2: Engine Permintaan Pembayaran, Multi-Bank VA & QRIS Dinamis
- **Target**: Service dapat menerbitkan nomor Virtual Account unik (BCA, Mandiri, BRI) dan QRIS dinamis per invoice.
- **Deliverables**:
  - Modul generator nomor VA dengan prefix bank dan nomor HP/faktur unik.
  - Modul generator QRIS payload string standar EMVCo B2B.
  - Endpoint `POST /v1/pay/requests` (menerbitkan VA dan QRIS serentak).
  - Endpoint `GET /v1/pay/requests` dan `GET /v1/pay/requests/:id`.
  - Endpoint `POST /v1/pay/requests/:id/cancel`.

---

### Sprint 3: Webhook Ingestion, Validasi Signature & Proteksi Anti-Fraud
- **Target**: Menerima notifikasi pelunasan dari payment gateway atau simulator bank dengan proteksi anti-tamper.
- **Deliverables**:
  - Endpoint `POST /v1/pay/webhooks/:gateway` (Midtrans, Xendit, Direct Bank).
  - Validasi kriptografis HMAC SHA-256 / SHA-512 signature key.
  - **Idempotency Guard**: Menolak pemrosesan ganda jika bank mengirimkan webhook ganda (*duplicate callback rejection*).
  - Penyimpanan detail transaksi ke `pay_transactions`.
  - Transisi status `pay_payment_requests` dari `PENDING` ➔ `SETTLED`.

---

### Sprint 4: Integrasi Settlement ke Fledger Core & Outbox Worker
- **Target**: Pelunasan secara otomatis memicu pembukuan akuntansi double-entry di Fledger Core.
- **Deliverables**:
  - Transaksional Outbox: Insert event `fledger.pay.settled` ke `pay_settlement_outbox` dalam database transaction yang sama dengan transaksi pembayaran.
  - Background worker `DrainLoop` dengan interval polling berkala dan exponential backoff retry.
  - HTTP Client adapter yang memanggil Fledger Core:
    - Melakukan transfer pembukuan: Debit Kas Bank, Kredit Piutang Usaha (AR).
    - Menandai status invoice di Fledger Core sebagai `PAID`.
  - Endpoint `GET /v1/pay/outbox/counts`.

---

### Sprint 5: Web Simulator Portal & Verifikasi Pengujian E2E
- **Target**: Antarmuka web interaktif untuk demonstrasi simulasi pembayaran sandbox dan uji E2E bebas bug.
- **Deliverables**:
  - Direktori `fledger-pay/web/` berisi Portal Web Mandiri:
    - Menampilkan faktur terbuka & pilihan bank (BCA, Mandiri, QRIS).
    - Tombol "Bayar Sekarang (Sandbox Simulator)" untuk uji coba seketika.
    - Indikator real-time pelunasan dan sinkronisasi ke Core.
  - Static file server di `main.go` menyajikan web portal dari root `:8083`.
  - Middleware CORS terbuka (`*`).
  - Unit tests & E2E smoke tests script dengan 100% test pass.

---

## ✅ Definition of Done (DoD)

Sebuah sprint dinyatakan **SELESAI (DONE)** jika:
1. Kode Go berhasil dikompilasi (`go build ./cmd/api`) tanpa warning dan lolos linter.
2. Seluruh unit tests lulus 100% (`go test -race ./...`).
3. **Integritas Uang**: Tidak ada tipe data float untuk uang. Semua nominal menggunakan `int64` / `BIGINT`.
4. **Idempotency**: Pengiriman ulang webhook dengan reference yang sama tidak memicu double settlement.
5. **Zero Secret Hardcoding**: Semua kunci HMAC, JWT secret, dan kredensial dimuat melalui environment variables.
