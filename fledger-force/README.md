# FLEDGER FORCE — Sales Force Automation & Anti-Cash Kitting Engine

> **Service**: `fledger-force`  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)  
> **Port**: `:8084`  
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router, Geofencing Haversine, PWA Mobile UI

---

## 🎯 Tanggung Jawab Layanan
1. **Rute Kunjungan Terverifikasi GPS (*Geofenced Beat Plan*)**: Memastikan salesman berada dalam radius 100m dari toko saat check-in menggunakan rumus Haversine.
2. **Mobile Cash Collection & Tanda Terima Digital**: Menerbitkan kwitansi resmi instan dan mengirimkan bukti lunas WhatsApp ke pemilik toko.
3. **Pencegahan Fraud Uang Kas (*Anti-Cash Kitting*)**:
   - Memindahkan beban kas dari piutang toko ke saldo hutang pribadi salesman di Fledger Core:
     $$\text{Debit: Account:Salesman-Wallet} \quad | \quad \text{Kredit: Account:Customer-AR}$$
4. **Daily Settlement Lock (Tutup Kasir Sore Hari)**:
   - Kasir gudang memvalidasi setoran uang fisik, mereset saldo salesman ke Rp 0, dan mencatat kas masuk di Core:
     $$\text{Debit: Account:HQ-Cash} \quad | \quad \text{Kredit: Account:Salesman-Wallet}$$
   - Salesman terkunci otomatis jika tidak menyetorkan kas fisik di akhir hari.

---

## 📚 Dokumen Spesifikasi AI Agent
Untuk AI Coding Agent atau Lead Developer yang akan mengeksekusi proyek ini, silakan baca dokumentasi di folder `docs/`:
- 📄 **[AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md)**: Blueprint teknis, arsitektur, dan checklist eksekusi AI Agent.
- 📄 **[ROADMAP-FORCE.md](docs/ROADMAP-FORCE.md)**: Rincian 5 Sprint pengembangan dan *Definition of Done (DoD)*.
- 📄 **[DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql)**: Skema DDL PostgreSQL 16 lengkap (tabel, foreign key, index).
- 📄 **[API-SPECIFICATION.md](docs/API-SPECIFICATION.md)**: Kontrak REST API dan format payload.
- 📄 **[CASH-COLLECTION-AND-EOD-GUIDE.md](docs/CASH-COLLECTION-AND-EOD-GUIDE.md)**: Panduan operasional penerimaan kas dan meja setor kasir.

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
Buka browser ke `http://localhost:8084/` untuk mengakses PWA Salesman & Portal Kasir.
