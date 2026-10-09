# FLEDGER ORDER — B2B Order Management System & Hard Credit Gate

> **Service**: `fledger-order`  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)  
> **Port**: `:8085`  
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router, Web Ordering Portal

---

## 🎯 Tanggung Jawab Layanan
1. **Katalog Produk FMCG & Multi-Tier Pricing**: Mengelola SKU dan harga bertingkat (`GROSIR`, `SEMI_GROSIR`, `RETAIL`, `STAR_OUTLET`).
2. **Alokasi Stok Gudang (*Stock Reservation*)**: Mengunci stok fisik saat pesanan dibuat agar mencegah *overselling*.
3. **Hard Credit Gate (Anti-Credit Blindness)**:
   - Evaluasi otomatis sebelum pesanan disetujui:
     - $(\text{Piutang Berjalan} + \text{Order Baru}) \le \text{Credit Limit Toko}$.
     - Menolak toko yang menunggak faktur $>30$ hari di Fledger Core (`CREDIT_BLOCKED`).
4. **Dispatching Bridge ke Fledger Fleet**:
   - Otomatis menerbitkan Surat Jalan (DO) di `fledger-fleet` dengan akumulasi tonase berat muatan dalam kilogram.

---

## 📚 Dokumen Spesifikasi AI Agent
Untuk AI Coding Agent atau Lead Developer yang akan mengeksekusi proyek ini, silakan baca dokumentasi di folder `docs/`:
- 📄 **[AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md)**: Blueprint teknis, arsitektur, dan checklist eksekusi AI Agent.
- 📄 **[ROADMAP-ORDER.md](docs/ROADMAP-ORDER.md)**: Rincian 5 Sprint pengembangan dan *Definition of Done (DoD)*.
- 📄 **[DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql)**: Skema DDL PostgreSQL 16 lengkap (tabel, foreign key, index).
- 📄 **[API-SPECIFICATION.md](docs/API-SPECIFICATION.md)**: Kontrak REST API dan format payload.
- 📄 **[HARD-CREDIT-GATE-AND-FLEET-BRIDGE-GUIDE.md](docs/HARD-CREDIT-GATE-AND-FLEET-BRIDGE-GUIDE.md)**: Panduan logika mitigasi kredit macet dan jembatan ke Fleet.

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
Buka browser ke `http://localhost:8085/` untuk mengakses B2B Order Portal.
