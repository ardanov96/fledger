# Panduan Integrasi Ekspedisi Dashboard & Fledger Fleet

Dokumen ini menjelaskan arsitektur integrasi antara sistem frontend ekspedisi (misalnya `ekspedisi-dashboard`) dengan microservice **Fledger Fleet** (`:8082`) dan akuntansi pusat **Fledger Core** (`:8081`).

---

## 1. Arsitektur & Peran Layanan

```
+-------------------------------------------------------------+
|                     FRONTEND LAYER                          |
|  - Ekspedisi Dashboard (Next.js/React di :3000/:5173)       |
|  - Driver Mobile Web & Dispatcher Portal (fledger-fleet/web)|
+-------------------------------------------------------------+
                               |
                               | (HTTP REST + CORS)
                               v
+-------------------------------------------------------------+
|                  FLEDGER FLEET MICROSERVICE                 |
|               (Go Chi API Listening on :8082)               |
|  - Manajemen Armada, Supir, Rute & Trip Pengiriman          |
|  - Digital Proof of Delivery (Canvas Signature & Foto Fisik)|
|  - Transactional Outbox Worker                              |
+-------------------------------------------------------------+
                               |
                               | (HTTP /v1/invoices - Idempotent Retry)
                               v
+-------------------------------------------------------------+
|                     FLEDGER CORE ENGINE                     |
|                 (Akuntansi & Buku Besar Pusat)               |
|  - Penerbitan Invoice Bersih Tanpa Sengketa                 |
|  - Jurnal Piutang Usaha & Pendapatan Logistik               |
+-------------------------------------------------------------+
```

---

## 2. Pilihan Integrasi Frontend

### Opsi 1: Menggunakan Web Portal Bawaan Fledger Fleet
Fledger Fleet telah menyediakan antarmuka web modern mandiri di direktori `fledger-fleet/web/`.
- **Akses Langsung**: Buka browser ke `http://localhost:8082/`
- **Fitur Tersedia**:
  1. **Mode Supir (Digital POD)**:
     - Deteksi Surat Jalan (DO) hari ini per supir.
     - Checklist SKU barang: penerimaan fisik vs retur di lapangan.
     - Tanda tangan digital sentuh/mouse (HTML5 Canvas).
     - Validasi foto fisik jika ada retur barang.
     - Auto-kalkulasi nominal bersih (*clean delivery nominal*).
  2. **Dispatcher Hub**:
     - Pembuatan trip & penugasan DO.
     - Meter kapasitas beban (berat DO vs kapasitas truk).
     - Tombol pemberangkatan (*dispatch*).
     - Monitor status Outbox Worker sync ke Core.
  3. **Master Data**:
     - Pendaftaran truk dan supir.
     - Generator DO demo instan (*one-click demo DO*).

---

### Opsi 2: Integrasi ke `ekspedisi-dashboard` (React / Next.js / Vue / Vanilla JS)

CORS telah diaktifkan secara global di Fledger Fleet (`*`), sehingga aplikasi frontend yang berjalan di port lain (seperti `localhost:3000` atau `localhost:5173`) dapat memanggil Fledger Fleet langsung tanpa reverse proxy.

#### Langkah A: Salin / Import `fleet-client.js`
Salin file `fledger-fleet/web/fleet-client.js` ke direktori proyek frontend Anda (misal `src/services/fleet-client.js` atau `lib/fleet-client.js`).

```javascript
import FledgerFleetClient from './fleet-client';

const fleet = new FledgerFleetClient('http://localhost:8082');

// 1. Dev login (untuk environment development)
await fleet.devLogin();

// 2. Ambil Surat Jalan hari ini
const trips = await fleet.listTodayTrips();

// 3. Ambil daftar DO yang perlu dikirim
const dos = await fleet.listDeliveryOrders('IN_TRANSIT');
```

#### Langkah B: Submit Proof of Delivery (POD)
Ketika supir menyelesaikan pengantaran di toko pelanggan:

```javascript
const podPayload = {
  recipient_name: "Ibu Siti Fatimah",
  recipient_phone: "081298765432",
  driver_notes: "Diterima dalam kondisi baik",
  signature_data_url: canvas.toDataURL("image/png"), // Base64 Canvas
  photo_evidence_url: "", // Wajib diisi jika ada rejected_qty > 0
  items_result: [
    {
      sku_id: "SKU-OIL-BIMOLI-2L",
      delivered_qty: 10,
      rejected_qty: 0,
      reject_reason: ""
    },
    {
      sku_id: "SKU-MIE-INDOMIE-GRG",
      delivered_qty: 4,
      rejected_qty: 1,
      reject_reason: "KEMASAN_RUSAK" // Foto wajib ada jika ini terjadi
    }
  ]
};

try {
  const result = await fleet.submitPOD(selectedDOId, podPayload);
  console.log("Faktur Fledger Core Terbit:", result.fledger_invoice_id);
} catch (err) {
  console.error("Gagal submit POD:", err.message);
}
```

---

## 3. Spesifikasi REST API Endpoint Ringkas

| Method | Endpoint | Deskripsi | Auth Header |
|---|---|---|---|
| `GET` | `/healthz` | Health check service | Tidak perlu |
| `POST` | `/v1/dev/login?tenant_id=...` | Mint JWT token dev lokal | Tidak perlu |
| `GET` | `/v1/fleet/vehicles` | Daftar armada kendaraan | `Bearer <token>` |
| `POST` | `/v1/fleet/vehicles` | Registrasi truk baru | `Bearer <token>` |
| `GET` | `/v1/fleet/drivers` | Daftar supir | `Bearer <token>` |
| `POST` | `/v1/fleet/drivers` | Registrasi supir baru | `Bearer <token>` |
| `GET` | `/v1/fleet/trips` | Daftar trip pengiriman | `Bearer <token>` |
| `GET` | `/v1/fleet/trips/today` | Trip berjalan hari ini | `Bearer <token>` |
| `POST` | `/v1/fleet/trips` | Buat trip perjalanan baru | `Bearer <token>` |
| `POST` | `/v1/fleet/trips/:id/dispatch` | Berangkatkan armada | `Bearer <token>` |
| `GET` | `/v1/fleet/delivery-orders` | Daftar Surat Jalan | `Bearer <token>` |
| `POST` | `/v1/fleet/delivery-orders` | Terbitkan Surat Jalan | `Bearer <token>` |
| `POST` | `/v1/fleet/delivery-orders/:id/pod` | Submit bukti POD & sync Core | `Bearer <token>` |
| `GET` | `/v1/fleet/outbox/counts` | Status antrean sync Core | `Bearer <token>` |

---

## 4. Cara Menjalankan Layanan

1. **Jalankan Database & Migrasi**:
   ```bash
   cd fledger-fleet/migrations
   # Pastikan skema migrasi 000001_init_fleet.up.sql telah diaplikasikan
   ```

2. **Jalankan Fledger Fleet API**:
   ```bash
   cd fledger-fleet/src
   go run ./cmd/api
   ```
   Service akan aktif di `http://localhost:8082`.

3. **Buka Web Portal**:
   Buka peramban ke: `http://localhost:8082/`
