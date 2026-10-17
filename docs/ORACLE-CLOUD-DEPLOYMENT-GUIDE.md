# Panduan Deployment Fledger OS ke Oracle Cloud Always Free (24/7 Gratis Selamanya)

Dokumen ini adalah panduan lengkap dari nol untuk mendemonstrasikan **FLEDGER OS** secara live 24/7 tanpa biaya langganan bulanan (*$0 forever*), memanfaatkan kuota resmi **Oracle Cloud Always Free Tier**.

---

## 💎 Mengapa Oracle Cloud Always Free?

Oracle Cloud Infrastructure (OCI) memberikan alokasi gratis tanpa batas waktu (*Always Free Eligible*):
* **Komputasi:** ARM Ampere A1 Compute hingga **4 OCPU & 24 GB RAM** (Sangat powerful untuk 10 container Docker Fledger).
* **Storage:** **200 GB SSD Block Storage**.
* **IP Public:** 1 IP Publik Statis gratis.
* **Biaya:** **Rp 0 / Bulan selamanya**.

---

## 📋 Langkah 1: Registrasi Akun Oracle Cloud Free Tier

1. Buka [https://www.oracle.com/cloud/free/](https://www.oracle.com/cloud/free/) lalu klik **Start for free**.
2. Masukkan email, nama, dan pilih **Country: Indonesia**.
3. **Penting — Pemilihan Home Region:**
   * Pilih region yang dekat dengan Indonesia untuk latensi terbaik: **Singapore**, **Tokyo (Japan)**, atau **Sydney (Australia)**.
4. **Verifikasi Kartu:**
   * Oracle membutuhkan kartu kredit/debit berlogo Visa/Mastercard (bisa menggunakan Bank Jago, Jenius BTPN, BCA, Mandiri, dll).
   * Oracle akan melakukan *hold* verifikasi sekitar $1 (sekitar Rp 16.000) yang akan **langsung di-refund otomatis**.
   * Selama Anda memilih resource berlabel **"Always Free Eligible"**, kartu Anda tidak akan pernah didebit.

---

## 🖥️ Langkah 2: Membuat VM Instance (Ampere A1 24GB RAM)

1. Masuk ke **Oracle Cloud Console** (cloud.oracle.com).
2. Dari menu hamburger (kiri atas), pilih **Compute ➔ Instances ➔ Create Instance**.
3. Beri nama instance, misalnya: `fledger-production`.
4. Pada bagian **Image and Shape**:
   * Klik **Change Image** ➔ Pilih **Canonical Ubuntu 22.04 LTS (AArch64)**.
   * Klik **Change Shape** ➔ Pilih **Ampere (ARM Processor)** ➔ Centang `VM.Standard.A1.Flex`.
   * Atur slider:
     * **OCPU:** `2` s/d `4`
     * **Memory:** `12 GB` s/d `24 GB` (Semuanya *Always Free*).
5. Pada bagian **Networking**:
   * Pilih *Create new virtual cloud network (VCN)*.
   * Pastikan **Assign a public IPv4 address** terpilih: **Yes**.
6. Pada bagian **Add SSH Keys**:
   * Pilih **Generate SSH key pair** ➔ Klik **Save Private Key** (file `.key` ini wajib disimpan di laptop Anda untuk login via SSH).
7. Klik tombol **Create** di bagian bawah. Tunggu status instance berubah dari *Provisioning* menjadi **Running** (berwarna hijau). Catat **Public IP Address** instance Anda.

---

## 🛡️ Langkah 3: Buka Port 80 & 443 di Oracle Cloud Firewall (VCN Ingress)

> [!IMPORTANT]
> Secara default, Oracle Cloud memblokir port HTTP (80) dan HTTPS (443). Anda **wajib** membuka firewall ini agar web dapat diakses dari browser luar.

1. Di halaman detail Instance Anda, klik link subnet Anda (di bawah *Primary VNIC* ➔ *Subnet*).
2. Klik **Default Security List for...**.
3. Di tab *Ingress Rules*, klik **Add Ingress Rules**:
   * **Source Type:** CIDR
   * **Source CIDR:** `0.0.0.0/0`
   * **IP Protocol:** `TCP`
   * **Destination Port Range:** `80,443,8000`
   * **Description:** `Allow HTTP/HTTPS Web Traffic`
4. Klik **Add Ingress Rules**.

---

## 🚀 Langkah 4: Hubungkan ke VM & Pasang Docker

Buka terminal PowerShell di laptop Anda, masuk ke folder tempat Anda menyimpan file Private Key `.key` tadi:

```powershell
# Ganti dengan path key dan IP publik Oracle Anda
ssh -i .\ssh-key.key ubuntu@<IP_PUBLIK_ORACLE>
```

Setelah berhasil masuk ke terminal Ubuntu server, jalankan perintah setup otomatis berikut:

```bash
# 1. Update paket sistem
sudo apt update && sudo apt upgrade -y

# 2. Buka firewall internal Ubuntu (iptables)
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 8000 -j ACCEPT
sudo netfilter-persistent save 2>/dev/null || sudo iptables-save | sudo tee /etc/iptables/rules.v4

# 3. Install Docker & Docker Compose
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker ubuntu
newgrp docker
```

---

## 📦 Langkah 5: Clone Project & Jalankan Fledger OS

Di terminal VM server:

```bash
# 1. Clone repositori project Anda
git clone <URL_GIT_REPOSITORY_ANDA> fledger
cd fledger

# 2. Jalankan seluruh ekosistem (Database + 6 Microservices + Gateway)
docker compose up -d --build
```

Docker akan mengompilasi binary Go dan menyalakan seluruh container:
* `fledger_postgres` (Port 5432)
* `fledger_redis` (Port 6379)
* `fledger_nats` (Port 4222)
* `fledger_core` (Port 8081)
* `fledger_fleet` (Port 8082)
* `fledger_pay` (Port 8083)
* `fledger_force` (Port 8084)
* `fledger_order` (Port 8085)
* `fledger_dunning` (Port 8086)
* `fledger_gateway` (Port 80 — Reverse Proxy)

Cek status container:
```bash
docker compose ps
```

---

## 🌐 Langkah 6: Akses Live Demonstration dari Perangkat Lain

Sekarang Fledger OS sudah **aktif 24/7 di Cloud** tanpa bergantung pada laptop Anda! Anda dapat membuka link berikut dari ponsel, tablet, atau laptop klien:

| Layanan / Modul | URL Akses Langsung |
| :--- | :--- |
| **Landing Page Perusahaan** | `http://<IP_PUBLIK_ORACLE>/` |
| **Live Ops Hub** | `http://<IP_PUBLIK_ORACLE>/hub` |
| **Closed-Loop Simulator** | `http://<IP_PUBLIK_ORACLE>/simulator` |
| **Fledger Core Ledger** | `http://<IP_PUBLIK_ORACLE>/core/web/` |
| **Fledger Fleet (Logistik)** | `http://<IP_PUBLIK_ORACLE>/fleet/` |
| **Fledger Pay (Kasir & QRIS)** | `http://<IP_PUBLIK_ORACLE>/pay/` |
| **Fledger Force (Sales SFA)** | `http://<IP_PUBLIK_ORACLE>/force/` |
| **Fledger Order (OMS Grosir)** | `http://<IP_PUBLIK_ORACLE>/order/` |
| **Fledger Dunning (WA AR)** | `http://<IP_PUBLIK_ORACLE>/dunning/` |

---

## 🔒 Langkah Opsional: Pasang Domain & HTTPS Gratis (Cloudflare)

Jika Anda ingin alamat yang cantik dan profesional (misal `https://demo.perusahaananda.com`):
1. Masukkan domain Anda ke akun [Cloudflare](https://dash.cloudflare.com/) (Gratis).
2. Buat **DNS A Record**:
   * **Name:** `demo` (atau `@`)
   * **IPv4 Address:** `<IP_PUBLIK_ORACLE>`
   * **Proxy Status:** **Proxied (Awan Oranye)**
3. Cloudflare akan otomatis mengaktifkan **SSL HTTPS gratis**, kompresi CDN, dan proteksi DDoS tanpa perlu konfigurasi certbot di server!
