# Panduan Deployment Fledger OS ke Render.com (100% Gratis & Tanpa Kartu Kredit)

Panduan ini memungkinkan Anda menjalankan ekosistem **FLEDGER OS** secara live 24/7 di Cloud **tanpa memasukkan kartu kredit/debit apa pun**.

---

## 🌟 Mengapa Render.com?

* **0 Biaya & 0 Syarat Kartu:** Cukup login menggunakan akun **GitHub** Anda.
* **All-in-One Self-Contained:** Ke-6 microservice Go, Gateway, PostgreSQL 16, Redis 7, dan NATS JetStream berjalan otomatis di dalam satu container ringkas (~160 MB RAM).
* **Otomatis HTTPS & Domain Publik:** Mendapatkan URL live resmi (misal: `https://fledger-os.onrender.com`) yang bisa dibuka dari perangkat apa pun di seluruh dunia.
* **Auto-Seed Demo Data:** Data sampel akun akuntansi, armada truk, toko kanvaser, dan katalog FMCG langsung siap pakai untuk pitching.

---

## 🚀 Langkah 1: Push Kode ke GitHub

Pastikan seluruh file konfigurasi baru (`Dockerfile`, `render.yaml`, dan script container) sudah ter-commit dan di-push ke repository GitHub Anda:

Buka terminal di folder project Anda (`c:\Dev\fledger`):

```bash
git add .
git commit -m "feat: add All-in-One cloud container and Render blueprint"
git push origin main
```
*(Sesuaikan nama branch jika menggunakan `master`).*

---

## 🌐 Langkah 2: Buka Render.com & Login dengan GitHub

1. Buka [https://render.com/](https://render.com/)
2. Klik tombol **Get Started** atau **Sign In**.
3. Pilih opsi **GitHub** (Anda tidak akan pernah dimintai data kartu kredit).
4. Berikan izin akses ke repository `fledger` Anda.

---

## 📦 Langkah 3: Buat Web Service Baru

1. Di Dashboard Render, klik tombol **New +** (kanan atas) ➔ Pilih **Web Service**.
2. Pilih repository `fledger` yang muncul di daftar, lalu klik **Connect**.
3. Isi formulir konfigurasi sederhana:
   * **Name:** `fledger-os` *(atau nama brand Anda)*
   * **Region:** **Singapore** *(paling dekat dengan Indonesia)*
   * **Language / Environment:** **Docker** *(otomatis terdeteksi dari Dockerfile)*
   * **Instance Type:** Pilih **Free ($0/month)**
4. Gulir ke bagian paling bawah, lalu klik tombol **Deploy Web Service**.

---

## ☕ Langkah 4: Tunggu Proses Build Selesai (~3–5 Menit)

Render akan mulai mengompilasi binary Go dan menyiapkan database. Anda bisa melihat log prosesnya secara real-time.

Ketika muncul pesan:
```text
>>> FLEDGER OS Gateway is ONLINE at port 10000
==> Your service is live 🎉
```

Klik URL yang diberikan oleh Render di bagian atas (contoh: `https://fledger-os.onrender.com`).

---

## 🧭 Direktori Tautan Live untuk Demonstrasi Klien

Ganti `fledger-os.onrender.com` dengan URL yang Anda dapatkan dari Render:

| Modul Operasional | Tautan Live | Keterangan Demo |
| :--- | :--- | :--- |
| **Landing Page Utama** | `https://fledger-os.onrender.com/` | Halaman Company Profile, kalkulator ROI & value proposition FMCG. |
| **Live Ecosystem Hub** | `https://fledger-os.onrender.com/hub` | Dashboard status 6 microservices & monitor gateway. |
| **Closed-Loop Simulator** | `https://fledger-os.onrender.com/simulator` | Simulasi siklus bisnis tertutup (Order ➔ DO ➔ Bayar ➔ Jurnal). |
| **Fledger Core Ledger** | `https://fledger-os.onrender.com/core/web/` | Buku besar akuntansi, COA, dan audit trail PSAK. |
| **Fledger Fleet (Logistik)** | `https://fledger-os.onrender.com/fleet/` | Surat Jalan (DO), armada truk, dan tanda tangan digital supir. |
| **Fledger Pay (Gateway)** | `https://fledger-os.onrender.com/pay/` | Kasir pembayaran QRIS dinamis dan Virtual Account. |
| **Fledger Force (SFA)** | `https://fledger-os.onrender.com/force/` | Aplikasi mobile kanvaser sales & anti-cash kitting. |
| **Fledger Order (OMS)** | `https://fledger-os.onrender.com/order/` | Portal grosir B2B, batas kredit piutang, dan stok gudang. |
| **Fledger Dunning (WA)** | `https://fledger-os.onrender.com/dunning/` | Penagihan piutang otomatis WhatsApp cadence. |

---

## ⚡ Tips Pro: Menjaga Server Selalu Aktif 24/7 (Anti-Sleep)

Pada paket Free Render, jika website tidak dibuka sama sekali selama 15 menit, server akan masuk mode *sleep* (dan butuh waktu ~40 detik untuk bangun saat diklik pertama kali).

**Agar website selalu ON instan di depan klien tanpa pernah tertidur:**
1. Buka [cron-job.org](https://cron-job.org/) (Layanan ping gratis, tanpa kartu kredit).
2. Buat job baru:
   * **URL:** `https://fledger-os.onrender.com/healthz`
   * **Schedule:** Setiap 10 menit (*Every 10 minutes*).
3. Selesai! Cron-job akan otomatis menyapa server Anda setiap 10 menit sehingga Fledger OS **tetap aktif 24 jam nonstop** siap kapan pun klien membuka link Anda!
