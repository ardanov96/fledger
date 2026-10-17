# Panduan Deployment Fledger OS ke Vercel (100% Gratis & Tanpa Kartu Kredit)

Dokumen ini adalah panduan resmi untuk men-deploy portal publik dan simulator interaktif **FLEDGER OS** ke **Vercel**:
* **100% Bebas Biaya & Tanpa Kartu Kredit:** Cukup login menggunakan akun GitHub Anda.
* **Aktif 24/7 di Cloud:** Server global Vercel Edge, laptop Anda bebas dimatikan.
* **Domain Publik HTTPS Cepat:** Contoh: `https://fledger-os.vercel.app`.

---

## 🧭 Apa Saja yang Live di Vercel?

Seluruh antarmuka web dan simulator interaktif telah dibundel ke folder `public/` dan siap dibuka kapan saja:

| Halaman & Modul | URL di Vercel | Keterangan Demonstrasi |
| :--- | :--- | :--- |
| **Landing Page Perusahaan** | `https://<nama-project>.vercel.app/` | Profil SaaS FMCG, Bento Grid, kalkulator pemulihan piutang. |
| **Closed-Loop Simulator** | `https://<nama-project>.vercel.app/simulator` | **Paling Utama:** Simulasi interaktif siklus bisnis distribusi (Order ➔ DO ➔ Retur Fisik ➔ Faktur Bersih ➔ Kas Terkunci). |
| **Live Ecosystem Hub** | `https://<nama-project>.vercel.app/hub` | Dashboard pusat kendali 6 microservices & arsitektur sistem. |
| **Enterprise Demo Booking** | `https://<nama-project>.vercel.app/demo` | Portal jadwal demo untuk calon klien/investor. |
| **Fledger Core Ledger Web** | `https://<nama-project>.vercel.app/core/web/` | Antarmuka buku besar akuntansi & COA PSAK. |
| **Fledger Fleet (Logistik)** | `https://<nama-project>.vercel.app/fleet/` | Portal logistik surat jalan & armada truk. |
| **Fledger Pay (Kasir)** | `https://<nama-project>.vercel.app/pay/` | Kasir pembayaran QRIS & Virtual Account. |
| **Fledger Force (Sales SFA)** | `https://<nama-project>.vercel.app/force/` | Web PWA kanvaser salesman lapangan. |
| **Fledger Order (OMS Grosir)** | `https://<nama-project>.vercel.app/order/` | Katalog produk B2B grosir dan keranjang pesanan toko. |

---

## 🚀 Cara Deploy ke Vercel (Hanya 2 Menit):

### Langkah 1: Push Perubahan ke GitHub
Buka PowerShell di folder project Anda (`c:\Dev\fledger`), lalu jalankan:

```bash
git add .
git commit -m "feat: setup Vercel static public bundles and routing"
git push origin main
```
*(Ganti `main` jika branch Anda bernama `master`).*

---

### Langkah 2: Import Project di Vercel
1. Buka [https://vercel.com/new](https://vercel.com/new)
2. Klik tombol **Continue with GitHub** (Tanpa verifikasi kartu kredit).
3. Cari repository **`fledger`** di daftar, lalu klik tombol **Import**.
4. Di halaman *Configure Project*:
   * **Project Name:** `fledger-os` *(atau nama brand Anda)*
   * **Framework Preset:** Biarkan **Other**
   * **Root Directory:** `./`
   * *Build and Output Settings:* Tidak perlu diubah (file `vercel.json` sudah mengaturnya otomatis ke folder `public`).
5. Klik tombol biru: **Deploy**.

---

### Langkah 3: Selesai! 🎉
Dalam waktu kurang dari **1 menit**, Vercel akan selesai memproses dan menampilkan konfeti:
> **Congratulations! Your project has been deployed.**

Klik URL yang diberikan (contoh: `https://fledger-os.vercel.app`).

Tautan ini dapat Anda kirimkan ke WhatsApp calon klien, direksi, maupun investor. Halaman akan terbuka dengan sangat cepat dan dapat diakses 24 jam nonstop dari perangkat apa pun di seluruh dunia!
