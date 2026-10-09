# FLEDGER PAY — B2B Payment Gateway & Auto-Settlement Engine

> **Service**: `fledger-pay`  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)  
> **Port**: `:8083`  
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router, Zero npm dependencies

---

## 🎯 Tanggung Jawab Layanan
1. **Multi-Bank Virtual Account & QRIS Generator**: Menerbitkan nomor VA unik (BCA, Mandiri, BRI, BNI) dan QRIS dinamis per faktur/Surat Jalan.
2. **Webhook Callback Ingestion**: Menerima konfirmasi pembayaran seketika dari Payment Gateway atau simulator bank dengan validasi tanda tangan HMAC SHA-256.
3. **Idempotency Guard**: Menjamin tidak ada *double-crediting* meskipun gateway mengirim callback berulang kali.
4. **Automated Settlement Bridge ke Fledger Core**: Melakukan mutasi jurnal *double-entry* (Debit Kas Bank, Kredit Piutang Usaha) dan menandai faktur sebagai `PAID` secara transaksional via Outbox pattern.
5. **Interactive Payment Simulator**: Menyediakan portal web sandbox (`web/`) untuk simulasi pelunasan langsung saat demonstrasi produk.

---

## 📚 Dokumen Spesifikasi AI Agent
Untuk AI Coding Agent atau Lead Developer yang akan mengeksekusi proyek ini, silakan baca dokumentasi di folder `docs/`:
- 📄 **[AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md)**: Blueprint teknis, arsitektur, dan checklist eksekusi AI Agent.
- 📄 **[ROADMAP-PAY.md](docs/ROADMAP-PAY.md)**: Rincian 5 Sprint pengembangan dan *Definition of Done (DoD)*.
- 📄 **[DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql)**: Skema DDL PostgreSQL 16 lengkap (tabel, foreign key, index).
- 📄 **[API-SPECIFICATION.md](docs/API-SPECIFICATION.md)**: Kontrak REST API dan format payload webhook.
- 📄 **[SIMULATOR-AND-SETTLEMENT-GUIDE.md](docs/SIMULATOR-AND-SETTLEMENT-GUIDE.md)**: Panduan alur pengujian sandbox dan rekonsiliasi ke Core.

---

## 🚀 Quick Start (Setelah Implementasi Selesai)

```bash
# 1. Konfigurasi Environment
cp .env.example .env

# 2. Migrasi Database
cd src
go run ./cmd/migrator

# 3. Jalankan API Server
go run ./cmd/api
```
Buka browser ke `http://localhost:8083/` untuk mengakses Portal Simulator Pembayaran.
