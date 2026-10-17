# Panduan Deployment Fledger OS ke Hugging Face Spaces (100% Gratis & Bebas Kartu Kredit)

Jika Anda terhalang verifikasi kartu kredit di Render dan Oracle Cloud, **Hugging Face Spaces** adalah penyelamat terbaik di dunia developer:
* **0% Form Kartu Kredit:** Pendaftaran hanya membutuhkan email biasa atau akun GitHub.
* **Spesifikasi 16 GB RAM + 2 vCPU Gratis:** 32x lipat lebih besar dari Render.
* **Aktif 24/7 di Cloud:** Server mandiri, laptop bebas dimatikan.
* **Domain Publik Resmi HTTPS:** Misal `https://<username>-fledger-os.hf.space` tanpa layar proteksi / tanpa password.

---

## 🚀 Langkah 1: Buat Akun & Space Baru di Hugging Face

1. Buka [https://huggingface.co/join](https://huggingface.co/join) lalu daftar dengan email Anda (atau klik *Sign Up with GitHub*).
2. Setelah login, buka: [https://huggingface.co/new-space](https://huggingface.co/new-space)
3. Isi konfigurasi Space:
   * **Space name:** `fledger-os`
   * **License:** `mit`
   * **Select the Space SDK:** Pilih **Docker**
   * **Choose a Docker template:** Pilih **Blank**
   * **Space Hardware:** Biarkan default (**CPU basic • 2 vCPU • 16 GB RAM • Free**)
   * **Privacy:** Pilih **Public**
4. Klik tombol **Create Space**.

---

## 📦 Langkah 2: Push Project Fledger ke Hugging Face

Setelah Space terbuat, Hugging Face akan menampilkan URL Git Space Anda (contoh: `https://huggingface.co/spaces/<username>/fledger-os`).

Buka PowerShell di folder project Anda (`c:\Dev\fledger`), lalu jalankan 3 perintah ini:

```bash
# 1. Commit seluruh update All-in-One Dockerfile terbaru
git add .
git commit -m "feat: deploy Fledger OS to Hugging Face Spaces"

# 2. Tambahkan remote Hugging Face (Ganti <username> dengan username akun Hugging Face Anda)
git remote add hf https://huggingface.co/spaces/<username>/fledger-os

# 3. Push kode Anda ke Hugging Face
git push hf main
```

*(Jika Hugging Face meminta password saat push, gunakan **Access Token** yang dapat dibuat di menu: `Settings ➔ Access Tokens ➔ Create New Token (Role: Write)`).*

---

## ☕ Langkah 3: Tunggu Build Selesai (~2–3 Menit)

1. Buka halaman Space Anda di browser. Anda akan melihat status: `Building Container...`
2. Begitu selesai, status akan berubah menjadi **Running** (berwarna hijau).
3. Aplikasi Fledger OS langsung tampil utuh di layar!

---

## 🌐 Tautan Live untuk Demonstrasi ke Klien

Anda dapat membagikan URL langsung (fullscreen tanpa frame Hugging Face) dengan format:

> **URL Direct:** `https://<username>-fledger-os.hf.space/`

| Modul Operasional | Tautan Direct Live |
| :--- | :--- |
| **Landing Page Perusahaan** | `https://<username>-fledger-os.hf.space/` |
| **Live Ops Hub** | `https://<username>-fledger-os.hf.space/hub` |
| **Closed-Loop Simulator** | `https://<username>-fledger-os.hf.space/simulator` |
| **Fledger Core Ledger** | `https://<username>-fledger-os.hf.space/core/web/` |
| **Fledger Fleet (Logistik)** | `https://<username>-fledger-os.hf.space/fleet/` |
| **Fledger Pay (Kasir QRIS)** | `https://<username>-fledger-os.hf.space/pay/` |
| **Fledger Force (Sales SFA)** | `https://<username>-fledger-os.hf.space/force/` |
| **Fledger Order (OMS Grosir)** | `https://<username>-fledger-os.hf.space/order/` |
| **Fledger Dunning (WA AR)** | `https://<username>-fledger-os.hf.space/dunning/` |

Semua modul, database, transaksi QRIS, dan simulasi supply chain sudah terhubung otomatis di dalam cloud 24/7!
