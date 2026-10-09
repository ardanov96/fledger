# FLEDGER DUNNING — API Specification

> **Service**: `fledger-dunning`  
> **Base Port**: `:8086`  
> **Protocol**: REST over HTTP/1.1 with JSON & Multi-Part PDF  
> **Auth Standard**: Bearer JWT (`Authorization: Bearer <token>`) & Tenant Header (`X-Tenant-ID: <UUID>`)  
> **Currency Unit**: Integer minor units (`int64`, 1 IDR = 1 unit)  

---

## 1. Ringkasan Endpoint

| Method | Endpoint | Deskripsi | Otorisasi |
|---|---|---|---|
| `GET` | `/healthz` | Liveness probe status service | Public |
| `GET` | `/readyz` | Readiness probe (cek DB & WhatsApp socket) | Public |
| `GET` | `/v1/dunning/whatsapp/status` | Cek status koneksi WhatsApp (Connected/QR) | Finance, Admin |
| `POST` | `/v1/dunning/whatsapp/qr/generate` | Generate QR code untuk pairing WA baru | Finance, Admin |
| `POST` | `/v1/dunning/whatsapp/disconnect` | Putuskan koneksi WhatsApp aktif | Admin |
| `POST` | `/v1/dunning/whatsapp/test-send` | Kirim pesan WhatsApp uji coba | Finance, Admin |
| `GET` | `/v1/dunning/queues` | Daftar antrian pesan penagihan | Finance, Collector |
| `POST` | `/v1/dunning/queues/ingest-invoice` | Ingest faktur baru & jadwal 5 stage dunning | Core, Internal |
| `POST` | `/v1/dunning/queues/cancel-invoice` | Batalkan antrian dunning jika sudah lunas | Pay, Core, Internal |
| `POST` | `/v1/dunning/queues/{id}/dispatch` | Kirim paksa pesan dunning tertentu sekarang | Finance, Collector |
| `POST` | `/v1/dunning/queues/cron-run` | Trigger worker kirim pesan yang jatuh tempo | Worker, Cron |
| `GET` | `/v1/dunning/statements` | Daftar dokumen e-Statement rekening koran | Finance, Admin |
| `POST` | `/v1/dunning/statements/generate` | Render PDF rekening koran toko bulanan | Finance, Admin |
| `POST` | `/v1/dunning/statements/{id}/send` | Kirim PDF rekening koran via WhatsApp | Finance, Admin |
| `GET` | `/v1/dunning/contacts` | Daftar kontak WhatsApp resmi toko | Sales, Finance |
| `POST` | `/v1/dunning/contacts` | Daftarkan / update kontak WhatsApp toko | Sales, Finance |
| `POST` | `/v1/dunning/webhooks/pay` | Webhook pembayaran lunas dari Fledger Pay | System Secret |

---

## 2. Rincian Endpoint

### 2.1. Cek Status WhatsApp Gateway
* **`GET /v1/dunning/whatsapp/status`**
* **Headers**:
  ```http
  Authorization: Bearer <JWT>
  X-Tenant-ID: a0000000-0000-0000-0000-000000000001
  ```
* **Response `200 OK`**:
  ```json
  {
    "status": "success",
    "data": {
      "session_name": "official-distributor-wa",
      "connection_status": "CONNECTED",
      "phone_connected": "6281198765432",
      "provider": "MOCK",
      "last_heartbeat": "2026-10-09T17:35:00Z"
    }
  }
  ```

---

### 2.2. Ingest Faktur Baru & Buat Jadwal Dunning
Dipanggil saat pesanan dikirim & faktur terbit di Core/Order. Secara otomatis menyusun jadwal 5 tahap dunning.
* **`POST /v1/dunning/queues/ingest-invoice`**
* **Request Body**:
  ```json
  {
    "invoice_id": "INV-2026-003",
    "invoice_number": "INV/2026/10/003",
    "store_id": "CUST-001",
    "due_date": "2026-10-25",
    "amount_due_minor": 7500000,
    "payment_link_url": "http://localhost:8083/pay/INV-2026-003"
  }
  ```
* **Response `201 Created`**:
  ```json
  {
    "status": "success",
    "data": {
      "invoice_id": "INV-2026-003",
      "schedules_created": [
        {"stage": "PRE_DUE_H3", "scheduled_at": "2026-10-22T09:00:00Z"},
        {"stage": "DUE_DATE", "scheduled_at": "2026-10-25T09:00:00Z"},
        {"stage": "OVERDUE_H3", "scheduled_at": "2026-10-28T09:00:00Z"},
        {"stage": "OVERDUE_H7", "scheduled_at": "2026-11-01T09:00:00Z"},
        {"stage": "OVERDUE_H14", "scheduled_at": "2026-11-08T09:00:00Z"}
      ]
    }
  }
  ```

---

### 2.3. Webhook Pelunasan Faktur dari Fledger Pay
Ketika toko membayar via QRIS/VA di Fledger Pay, webhook ini membatalkan semua sisa jadwal dunning yang berstatus `QUEUED`.
* **`POST /v1/dunning/webhooks/pay`**
* **Headers**:
  ```http
  X-Webhook-Secret: dunning-super-secret-key-2026
  X-Tenant-ID: a0000000-0000-0000-0000-000000000001
  ```
* **Request Body**:
  ```json
  {
    "event": "payment.settled",
    "invoice_id": "INV-2026-003",
    "paid_amount_minor": 7500000,
    "paid_at": "2026-10-24T14:15:00Z",
    "settlement_ref": "SETTLE-BCA-981240"
  }
  ```
* **Response `200 OK`**:
  ```json
  {
    "status": "success",
    "message": "Seluruh jadwal dunning untuk faktur INV-2026-003 berhasil dibatalkan otomatis",
    "cancelled_count": 5
  }
  ```

---

### 2.4. Trigger Cron Dispatch Antrian Dunning
Background worker periodik untuk mengeksekusi pesan yang `scheduled_at <= NOW()` dan `status = 'QUEUED'`.
* **`POST /v1/dunning/queues/cron-run`**
* **Response `200 OK`**:
  ```json
  {
    "status": "success",
    "processed_count": 2,
    "dispatched": [
      {
        "queue_id": "b3e0c000-0000-0000-0000-000000000001",
        "invoice_number": "INV/2026/10/001",
        "stage": "PRE_DUE_H3",
        "phone_number": "6281234567801",
        "status": "SENT",
        "jitter_seconds": 4
      }
    ]
  }
  ```

---

### 2.5. Generate & Kirim Rekening Koran Toko Bulanan (PDF e-Statement)
* **`POST /v1/dunning/statements/generate`**
* **Request Body**:
  ```json
  {
    "statement_month": "2026-09",
    "store_id": "CUST-001"
  }
  ```
* **Response `201 Created`**:
  ```json
  {
    "status": "success",
    "data": {
      "statement_id": "7f000000-0000-0000-0000-000000000001",
      "store_id": "CUST-001",
      "statement_month": "2026-09",
      "total_invoiced_minor": 15400000,
      "total_paid_minor": 15400000,
      "closing_balance_minor": 0,
      "pdf_file_path": "/storage/statements/2026-09-CUST-001.pdf",
      "dispatch_status": "GENERATED"
    }
  }
  ```

---

### 2.6. Format Pesan WhatsApp Per Stage

1. **`PRE_DUE_H3` (H-3 Jatuh Tempo)**:
   > 🔔 **PENGINGAT JATUH TEMPO FAKTUR**  
   > Yth. Pemilik **Toko Berkah Kelontong**,  
   > Tagihan Faktur **INV/2026/10/001** senilai **Rp 4.500.000** akan jatuh tempo dalam **3 hari** pada tanggal **25 Okt 2026**.  
   >  
   > Untuk kenyamanan transaksi Anda, pembayaran dapat dilakukan secara instan via QRIS / BCA Virtual Account melalui tautan resmi:  
   > 🔗 *http://localhost:8083/pay/INV-2026-001*  
   >  
   > *Abaikan pesan ini jika Anda telah melakukan pembayaran.*

2. **`DUE_DATE` (Hari H)**:
   > ⚠️ **HARI JATUH TEMPO FAKTUR**  
   > Yth. Pemilik **Toko Berkah Kelontong**,  
   > Faktur **INV/2026/10/001** senilai **Rp 4.500.000** jatuh tempo **HARI INI**.  
   > Mohon segera selesaikan pembayaran untuk menjaga kelancaran pengiriman pesanan berikutnya:  
   > 🔗 *http://localhost:8083/pay/INV-2026-001*

3. **`OVERDUE_H3` (Terlambat 3 Hari)**:
   > ⏳ **PEMBERITAHUAN KETERLAMBATAN PEMBAYARAN**  
   > Yth. Pemilik **Toko Berkah Kelontong**,  
   > Pembayaran Faktur **INV/2026/10/001** senilai **Rp 4.500.000** telah terlambat **3 hari**.  
   > Mohon segera lakukan pembayaran hari ini via tautan berikut:  
   > 🔗 *http://localhost:8083/pay/INV-2026-001*

4. **`OVERDUE_H7` (Peringatan Tegas 7 Hari)**:
   > 🚨 **PERINGATAN TUNGGAKAN PIUTANG (H+7)**  
   > Yth. Bapak/Ibu Toko Berkah Kelontong,  
   > Faktur **INV/2026/10/001** senilai **Rp 4.500.000** belum kami terima dan telah melewati batas toleransi 7 hari.  
   > Harap segera menyelesaikan pelunasan sebelum fasilitas kredit toko ditangguhkan.  
   > 🔗 *http://localhost:8083/pay/INV-2026-001*

5. **`OVERDUE_H14` (Pemberitahuan Blokir Kredit Otomatis)**:
   > 🛑 **PEMBERITAHUAN PENANGGUHAN PESANAN (CREDIT BLOCKED)**  
   > Yth. Manajemen Toko Berkah Kelontong,  
   > Karena tunggakan faktur telah melampaui 14 hari, sistem **Fledger Order** secara otomatis telah **MEMBEKUKAN (CREDIT_BLOCKED)** penerbitan Surat Jalan baru untuk toko Anda.  
   >  
   > Fasilitas pesanan akan otomatis aktif kembali segera setelah pelunasan terkonfirmasi:  
   > 🔗 *http://localhost:8083/pay/INV-2026-001*
