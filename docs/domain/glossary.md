# Domain Glossary

> Daftar istilah bisnis & teknis yang dipakai di project ini.
> Referensi cepat saat ngoding, review code, atau interview.

---

## 📒 Accounting & Ledger

### Account
Representasi sebuah "pos" di chart of accounts. Setiap account punya tipe (HQ, outlet, sales rep, customer, revenue, receivable, payable, cash) dan saldo (cached + authoritative).

### Chart of Accounts (COA)
Daftar seluruh account yang dipakai sebuah tenant. Setiap bisnis punya COA sendiri-sendiri.

### Debit
Sisi kiri double-entry. Untuk akun bertipe asset/expense: menambah saldo. Untuk akun liability/equity/revenue: mengurangi saldo.

### Kredit
Sisi kanan double-entry. Kebalikan debit.

### Double-Entry Bookkeeping
Sistem pembukuan di mana setiap transaksi minimal menghasilkan 2 entry (1 debit + 1 kredit) dengan total yang seimbang. Invariant: `SUM(debit) == SUM(kredit)` untuk semua entry dalam satu transaction.

### Entry (Ledger Entry)
Pencatatan individual dalam ledger. Immutable (tidak bisa di-UPDATE/DELETE). Satu transaction bisa punya banyak entry (biasanya 2+).

### Transaction
Sekelompok entry yang berasal dari satu business event. Misal: "Bayar invoice INV-001 oleh outlet JKT-12" = 1 transaction dengan 2 entry (debit cash, kredit receivable).

### Immutability
Entries tidak pernah di-edit atau dihapus. Koreksi selalu lewat reversal entry (entry baru yang membalikkan efek entry lama). Ini prasyarat audit trail yang trustworthy.

### Period (Accounting Period)
Rentang waktu (biasanya 1 bulan) tempat entry dicatat. Setelah di-close, period tidak bisa ditambah entry baru. Koreksi ke period yang sudah closed = opening balance entry di period baru.

### Trial Balance
Verifikasi bahwa `SUM(semua debit) - SUM(semua kredit) = 0`. Di sistem kita, ini dilakukan via reconciler background job (Fase 1B).

### Cached Balance vs Authoritative Balance
- **Cached**: kolom `accounts.cached_balance` — cepat, mungkin stale sementara
- **Authoritative**: `SUM(entries.signed_amount) for this account` — selalu benar, tapi O(n)
- Reconciler memverifikasi keduanya cocok (harus 0 selisihnya di sistem sehat)

---

## 💰 Receivables & Collection

### Invoice
Tagihan dari seller (HQ) ke buyer (outlet/customer). Punya amount, due_date, status (open/partial/paid/overdue).

### Receivable
Piutang. Aset — uang yang seharusnya diterima tapi belum masuk. Di COA, ini akun `receivable`.

### Aging Bucket
Klasifikasi invoice berdasarkan umur piutang: current (0 hari), 1-7 hari, 8-30 hari, 31-60 hari, 61-90 hari, 90+ hari. Dipakai untuk analisis collection efficiency.

### Partial Payment
Pembayaran yang tidak menutup satu invoice penuh. Bisa di-allocate ke multiple invoice sekaligus. Tracker di tabel `invoice_payments` + `payment_allocations`.

### Collection Route
Rute harian sales rep: kumpulan customer yang dikunjungi, beserta invoice yang harus di-collect. Tiap stop = 1 customer + N invoice.

### Settlement
Proses sales rep menyetorkan total collection ke HQ. Discrepancy = setoran vs catatan tidak cocok.

### Write-off
Penghapusan piutang yang tidak bisa ditagih. Lewat approval flow (finance HQ). Hasilnya: ledger adjustment yang mengurangi receivable dan承认 bad debt expense.

### Net-off (F&B specific)
Perhitungan net cashflow outlet F&B: `revenue - stock_payable_to_HQ - opex_float`. Tiga arus kas sekaligus.

---

## 🏢 Multi-Tenancy (Fase 5)

### Tenant
Sebuah "organisasi" atau "klien" yang punya data terisolasi. Multi-tenant = satu instance app, banyak tenant.

### Tenant Isolation
Mekanisme agar data tenant A tidak bisa diakses tenant B. Kami pakai: shared DB + `tenant_id` di setiap row + Postgres Row-Level Security (RLS).

### Row-Level Security (RLS)
Fitur Postgres untuk enforce filter di level DB. Setiap query otomatis di-filter sesuai tenant context (di-set via `SET LOCAL app.tenant_id = '...'`).

---

## 🔧 Technical

### Idempotency Key
Header `Idempotency-Key` yang dikirim client untuk mencegah double-processing kalau request di-retry. Kami simpan di tabel `idempotency_keys` + di kolom `transactions.idempotency_key`.

### Outbox Pattern
Pola untuk guarantee event delivery: event ditulis ke tabel `outbox_events` dalam transaction yang sama dengan business write. Worker publish ke NATS async. Zero event loss even kalau broker down.

### Hash Chain
Setiap entry punya `prev_hash` + `entry_hash`. Verifier scan integrity periodik mendeteksi tampering. Blockchain-like audit trail.

### Materialized View
View di-DB yang materialized (disimpan secara fisik). Refresh periodik. Dipakai untuk dashboard query yang berat (Fase 4D).

### CQRS-lite
Pemisahan write model (Postgres, source of truth) dan read model (materialized view + Redis cache). Tiap write emit event yang invalidate read cache.

### Saga
Long-running transaction yang span multiple service / database. Kami pakai River (Postgres-backed) untuk durable workflow.

---

## 🇮🇩 Indonesia-specific

### PPN (Pajak Pertambahan Nilai)
VAT. Default 11% (per 2022, sebelumnya 10%), naik ke 12% (per 2025). Dihitung per invoice, dipisah jadi line tersendiri (gross, tax, net).

### PPh
Withholding tax. Untuk transaksi tertentu (jasa, dividen, dll). Di Fase 8.

### e-Faktur
Format faktur pajak elektronik dari DJP. Generate dari invoice system untuk compliance.

### Rupiah (IDR)
Mata uang. Tidak ada subunit fractional (technically). Tapi kami simpan dalam 2 decimal place (sen) untuk konsistensi multi-currency future.

### QRIS
QR Code Indonesian Standard. Payment standard dari BI. Dipakai di collection (Fase 10).

### NIK
Nomor Induk Kependudukan (KTP). PII — di-encrypt di kolom DB dengan pgcrypto.

### OJK
Otoritas Jasa Keuangan. Regulator fintech. Kalau target compliance OJK-grade (regulated), ada tambahan requirement (KYC, AML reporting, dll).

---

## 📚 Referensi Belajar

- **Double-entry accounting**: cari video "double entry bookkeeping explained" di YouTube (15 menit cukup)
- **COA untuk UMKM**: download SAK EMKM dari iaiglobal.or.id
- **PPN**: baca di ortax.com atau djponline.pajak.go.id
- **Receivables & aging**: textbook accounting (Kieso, Weygandt, Warfield)
- **Idempotency**: Stripe API docs (https://stripe.com/docs/api/idempotent_requests)
- **Outbox pattern**: microservices.io/patterns/data/transactional-outbox.html
- **Hash chain**: Bitpay/bitcoin white paper (underlying concept)
