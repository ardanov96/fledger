# FLEDGER OS — Live Production & Pitching Demo Directory

> **The Ledger-First Operating System for FMCG Distribution, Fleet Logistics & Supply Chain**  
> Panduan resmi akses live production, public edge tunnel, dan direktori modul operasional untuk pitching/presentasi ke calon klien, investor, dan mitra bisnis.

---

## 🌐 Public Live Base URLs

Untuk mendemokan ekosistem **FLEDGER OS** secara live ke pihak eksternal (calon klien, direksi distributor FMCG, perbankan/fintech mitra) tanpa perlu instalasi lokal, sistem telah dilengkapi dengan gateway reverse proxy dan multi-provider live tunnel:

### 1. ⚡ Cloudflare Global Edge (Rekomendasi Utama — 1-Klik Buka Langsung)
* **Base URL**: **`https://certified-nissan-refine-remarks.trycloudflare.com`**
* **Keunggulan**: Menggunakan jaringan Cloudflare Anycast CDN, **100% tanpa layar verifikasi / tanpa input password IP**. Sangat ramah pengguna jika tautan dibagikan langsung via pesan WhatsApp atau email ke calon klien.

### 2. 🏷️ FMCG Industry-Branded URL (Localtunnel)
* **Base URL**: **`https://fledger-fmcg.loca.lt`**
* **Keunggulan**: Nama subdomain secara eksplisit mencerminkan industri distribusi FMCG (*Fast-Moving Consumer Goods*).
* **Catatan Akses Pertama**: Jika muncul layar proteksi anti-phishing Localtunnel di browser pengunjung, masukkan **Tunnel Password / IP Host**: **`180.248.114.245`** (atau klik tombol *Click to Submit*). Cukup dimasukkan 1x per browser.

### 3. 💻 On-Premise / Localhost (Presentasi Langsung via Laptop)
* **Base URL**: **`http://localhost:8000`**
* Gateway reverse proxy yang mengorkestrasi ke-6 microservices di port `:8081` s/d `:8086`.

---

## 🧭 Direktori Tautan Live per Modul & Persona Klien

Setiap modul di bawah ini dapat diakses secara langsung dan independen melalui browser:

| Layanan / Microservice | Tautan Live Production (Edge) | Target Persona Klien & Nilai Bisnis |
| :--- | :--- | :--- |
| **Landing Page Perusahaan** | [`/`](https://certified-nissan-refine-remarks.trycloudflare.com/) | **CEO, CFO, Direksi Utama FMCG**<br>Solusi komprehensif, showcase interaktif Bento Grid, kalkulator ROI pemulihan piutang, dan ringkasan arsitektur platform. |
| **Live Ecosystem Hub** | [`/hub`](https://certified-nissan-refine-remarks.trycloudflare.com/hub) | **CTO, VP of Engineering, IT Head**<br>Pusat kendali operasional 6 microservices secara real-time, status health/readiness probe, latensi gateway, dan tautan langsung ke setiap portal. |
| **Closed-Loop Simulator** | [`/simulator`](https://certified-nissan-refine-remarks.trycloudflare.com/simulator) | **Kepala Operasional Bisnis & Logistik**<br>Simulasi siklus bisnis tertutup: mulai dari Pesanan Toko ➔ Surat Jalan Armada ➔ Pembayaran Digital QRIS ➔ Rekonsiliasi Jurnal Akuntansi otomatis. |
| **Fledger Core Ledger** | [`/core/web/`](https://certified-nissan-refine-remarks.trycloudflare.com/core/web/) | **Manajer Keuangan, Accounting & Pajak**<br>ERP Finansial Buku Besar (*General Ledger*), Bagan Akun (COA PSAK), Faktur Piutang/Hutang, Rekonsiliasi Bank, dan Jurnal Finansial double-entry immutable. |
| **Fledger Force (SFA)** | [`/force/`](https://certified-nissan-refine-remarks.trycloudflare.com/force/) | **Sales Manager & Tim Kanvaser Lapangan**<br>Aplikasi Sales Force Automation (PWA Mobile): Check-in GPS Toko radius 100m (Haversine), pengambilan order (Order Taking), kwitansi kas, dan anti-cash kitting liability transfer. |
| **Fledger Fleet (Logistik)** | [`/fleet/`](https://certified-nissan-refine-remarks.trycloudflare.com/fleet/) | **Manajer Logistik & Koordinator Armada**<br>Manajemen armada pengiriman (*vehicles & drivers*), perencanaan rute jalan harian, Surat Jalan (DO), tanda tangan digital toko, dan foto bukti serah terima (Digital POD). |
| **Fledger Pay (Gateway)** | [`/pay/`](https://certified-nissan-refine-remarks.trycloudflare.com/pay/) | **Kasir, Treasury & Tim Penagihan**<br>Kasir pembayaran interaktif: QRIS Dinamis scannable asli, Virtual Account 6 Bank (BCA, Mandiri, BRI, BNI, Permata, CIMB), dan live settlement webhook Midtrans. |
| **Fledger Order (OMS)** | [`/order/`](https://certified-nissan-refine-remarks.trycloudflare.com/order/) | **Toko Grosir, Agen, & Fulfillment Gudang**<br>Portal pemesanan B2B grosir 24 jam dengan katalog SKU, volume pricing tiers, reservasi stok fisik gudang, dan validasi batas kredit (Hard Credit Gate). |
| **Fledger Dunning (WA)** | [`/dunning/`](https://certified-nissan-refine-remarks.trycloudflare.com/dunning/) | **Credit Control & AR Collection**<br>Antarmuka WhatsApp penagihan piutang otomatis 5 tahap (H-3 s/d H+14), koneksi WhatsApp Gateway Fonnte, simulasi self-healing cancellation saat faktur lunas, dan PDF e-Statement. |

---

## 💳 Konfigurasi Integrasi Webhook (Payment Gateway Midtrans)

Untuk menghubungkan sandbox Midtrans agar notifikasi pembayaran QRIS dan Virtual Account langsung memicu rekonsiliasi otomatis ke Fledger Pay dan Fledger Core:

* **Payment Notification URL (Webhook Callback):**
  ```text
  https://certified-nissan-refine-remarks.trycloudflare.com/v1/pay/webhooks/midtrans
  ```
* **Finish Redirect URL:**
  ```text
  https://certified-nissan-refine-remarks.trycloudflare.com/pay/
  ```
* **Unfinish / Error Redirect URL:**
  ```text
  https://certified-nissan-refine-remarks.trycloudflare.com/pay/
  ```

---

## ⏱️ Alur Demo 5 Menit yang Direkomendasikan Saat Pitching

Jika Anda memiliki waktu terbatas (5–10 menit) untuk mendemokan keunggulan **FLEDGER OS** kepada calon klien distributor FMCG:

1. **Menit 1 — Buka [Landing Page](https://certified-nissan-refine-remarks.trycloudflare.com/)**:  
   Perlihatkan problem statement industri distribusi Indonesia: kebocoran kas penagihan, sengketa retur barang supir, dan pembukuan piutang yang lambat. Tunjukkan solusi *Ledger-First*.
2. **Menit 2 — Buka [Fledger Force (SFA)](https://certified-nissan-refine-remarks.trycloudflare.com/force/)**:  
   Demokan salesman check-in toko berbasis radius GPS, membuat order, dan menagih kas dengan kwitansi instan. Jelaskan konsep *Anti-Cash Kitting* di mana beban kas berpindah ke wallet salesman secara real-time.
3. **Menit 3 — Buka [Fledger Fleet (Logistik)](https://certified-nissan-refine-remarks.trycloudflare.com/fleet/)**:  
   Tunjukkan Surat Jalan (DO), supir mengantar barang, mencatat barang diterima/retur, dan menangkap tanda tangan digital toko (POD) di layar touch HP/laptop.
4. **Menit 4 — Buka [Fledger Pay](https://certified-nissan-refine-remarks.trycloudflare.com/pay/) & [Fledger Dunning](https://certified-nissan-refine-remarks.trycloudflare.com/dunning/)**:  
   Perlihatkan QRIS dinamis yang siap di-scan toko untuk pelunasan 1-klik, dan tunjukkan bagaimana WhatsApp penagihan piutang otomatis membatalkan antrean reminder seketika faktur dinyatakan lunas (*Self-Healing*).
5. **Menit 5 — Buka [Fledger Core Ledger](https://certified-nissan-refine-remarks.trycloudflare.com/core/web/)**:  
   Tutup presentasi dengan menunjukkan buku besar akuntansi: piutang terpotong, kas bertambah, dan jurnal double-entry terbentuk otomatis dengan audit trail kriptografi SHA-256 tanpa intervensi akunting manual.

---

## 🛠️ Perintah Operasional Menjalankan Live Tunnel

Jika tunnel perlu di-restart atau dipindahkan ke mesin lain:

```powershell
# 1. Menjalankan tunnel rekomendasi Cloudflare Global Edge:
powershell -ExecutionPolicy Bypass -File .\scripts\start-live-tunnel.ps1 -Provider cloudflared

# 2. Atau menjalankan tunnel dengan subdomain FMCG:
powershell -ExecutionPolicy Bypass -File .\scripts\start-live-tunnel.ps1 -Provider localtunnel -Subdomain "fledger-fmcg"
```
