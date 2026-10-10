# FLEDGER DUNNING — Automated Accounts Receivable Dunning & WhatsApp Gateway

> **Service**: `fledger-dunning`  
> **Ecosystem**: **FLEDGER OS** (*The Ledger-First Operating System for FMCG Distribution & Logistics*)  
> **Port**: `:8086`  
> **Tech Stack**: Go 1.23+, PostgreSQL 16 (`pgx/v5`), Chi Router, WhatsApp Engine, PDF e-Statement

---

## 🎯 Tanggung Jawab Layanan
1. **Automated WhatsApp Dunning Cadence**:
   - Menjadwalkan pengiriman pesan penagihan otomatis bertingkat: H-3 (Pengingat ramah), Hari H (Jatuh tempo), H+3 (Pemberitahuan keterlambatan), H+7 (Peringatan tegas), dan H+14 (Pemberitahuan pemblokiran fasilitas kredit di `fledger-order`).
2. **1-Click WhatsApp Payment Link Integration**:
   - Setiap pesan memuat deep-link pembayaran langsung ke `fledger-pay` (QRIS & Virtual Account BCA/Mandiri/BRI).
3. **Self-Healing Loop (Pembatalan Otomatis saat Lunas)**:
   - Ketika toko membayar dan webhook Fledger Pay masuk, sisa seluruh antrian dunning berstatus `QUEUED` untuk faktur tersebut langsung dibatalkan otomatis (*atomic cancel*), dan sistem mengirimkan WhatsApp tanda terima lunas resmi ke pemilik toko.
4. **Anti-Ban Jitter Engine**:
   - Melindungi nomor WhatsApp distributor dari risiko blokir spam melalui jeda acak (*random jitter sleep* 3–8 detik) dan variasi teks pesan (*spintax*).
5. **Monthly PDF e-Statement Engine**:
   - Setiap tanggal 1 awal bulan: Otomatis me-render dokumen PDF Rekening Koran Toko 30 hari dan mengirimkannya via WhatsApp Document API.
6. **Embedded Web Management Portal**:
   - Single binary Go menyajikan web UI dashboard pemantauan antrian penagihan, QR session pairing, dan simulator chat WhatsApp tanpa ketergantungan runtime Node.js/NPM.

---

## 📚 Berkas Dokumentasi & Spesifikasi
- 🤖 **[AI-AGENT-PROMPT.md](docs/AI-AGENT-PROMPT.md)**: Master prompt siap salin (*copy-paste*) untuk menginstruksikan AI Coding Agent memulai eksekusi.
- 📄 **[AGENT-EXECUTION-BRIEF.md](docs/AGENT-EXECUTION-BRIEF.md)**: Blueprint teknis dan manual eksekusi lengkap untuk AI Coding Agent.
- 🗄️ **[DATABASE-SCHEMA.sql](docs/DATABASE-SCHEMA.sql)**: Skema DDL PostgreSQL 16 (konfigurasi, kontak toko, antrian dunning, rekening koran bulanan, log pesan).
- 🌐 **[API-SPECIFICATION.md](docs/API-SPECIFICATION.md)**: Spesifikasi REST API lengkap (Port `:8086`) dan format template pesan WhatsApp.
- 📖 **[WHATSAPP-CADENCE-AND-STATEMENT-GUIDE.md](docs/WHATSAPP-CADENCE-AND-STATEMENT-GUIDE.md)**: Panduan operasional dunning cadence, anti-ban jitter, dan engine e-statement.
- 🗺️ **[ROADMAP-DUNNING.md](docs/ROADMAP-DUNNING.md)**: Rincian 5 Sprint pengembangan dan *Definition of Done (DoD)*.
