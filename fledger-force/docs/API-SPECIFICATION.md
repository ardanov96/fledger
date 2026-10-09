# FLEDGER FORCE — REST API Specification & Service Contracts

> **Microservice**: `fledger-force`  
> **Base Port**: `:8084`  
> **Auth**: Bearer JWT (`Authorization: Bearer <token>`) & `X-Tenant-ID: <UUID>`

---

## 1. Ringkasan Endpoint

| Method | Endpoint | Fungsi | Hak Akses |
|---|---|---|---|
| `GET` | `/healthz` | Liveness Probe | Publik |
| `GET` | `/readyz` | Readiness Probe (Ping DB) | Publik |
| `POST` | `/v1/dev/login?tenant_id=...` | Mint Dev JWT Token | Dev Mode |
| `GET` | `/v1/force/sales-reps` | Daftar Salesman & Saldo Kas | Supervisor / Admin |
| `POST` | `/v1/force/sales-reps` | Daftarkan Salesman Baru | Supervisor / Admin |
| `GET` | `/v1/force/stores` | Daftar Toko & Koordinat GPS | Salesman / Admin |
| `POST` | `/v1/force/stores` | Daftarkan Toko & Geofence | Supervisor / Admin |
| `GET` | `/v1/force/beat-plans/today` | Rute Kunjungan Harian Sales | Salesman |
| `POST` | `/v1/force/beat-plans` | Buat Rencana Rute Kunjungan | Supervisor |
| `POST` | `/v1/force/visits/check-in` | Check-in Toko dengan Verifikasi GPS | Salesman |
| `POST` | `/v1/force/visits/:id/complete` | Selesaikan Kunjungan Toko | Salesman |
| `POST` | `/v1/force/collections` | Terima Pembayaran Kas Tunai Toko | Salesman |
| `GET` | `/v1/force/collections/today` | Daftar Kas Diterima Hari Ini | Salesman |
| `GET` | `/v1/force/settlements/reps/:id/inquiry` | Inquiry Kas Salesman oleh Kasir | Kasir HQ |
| `POST` | `/v1/force/settlements` | Setoran Kas Fisik EOD Sore Hari | Kasir HQ |
| `GET` | `/v1/force/outbox/counts` | Status Antrean Outbox ke Core | Admin |

---

## 2. Rincian Endpoint Kunci

### 2.1. Check-In Toko dengan Verifikasi GPS Geofencing
Memvalidasi bahwa salesman benar-benar berada di lokasi toko menggunakan formula Haversine:

```http
POST /v1/force/visits/check-in HTTP/1.1
Host: localhost:8084
Authorization: Bearer <SALESMAN_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json
```

**Request**:
```json
{
  "beat_plan_id": "b1111111-2222-3333-4444-555566667777",
  "store_id": "s1111111-2222-3333-4444-555566667777",
  "latitude": -6.175392,
  "longitude": 106.827153,
  "visit_type": "TAKING_ORDER_AND_COLLECTION"
}
```

**Response `201 Created`**:
```json
{
  "id": "v9991111-2222-3333-4444-555566667777",
  "store_name": "Toko Sumber Rezeki",
  "distance_meters": 24,
  "geofence_verified": true,
  "check_in_at": "2026-10-09T09:15:00Z",
  "status": "CHECKED_IN"
}
```
*(Catatan: Jika `distance_meters > geofence_radius_meters`, `geofence_verified` bernilai `false`, dicatat di audit log sebagai kunjungan di luar radius).*

---

### 2.2. Terima Kas Tunai Toko (Mobile Cash Collection)
Menerima uang fisik dari pemilik toko, menghasilkan nomor kwitansi digital unik, dan memicu perpindahan tanggung jawab ke wallet salesman.

```http
POST /v1/force/collections HTTP/1.1
Host: localhost:8084
Authorization: Bearer <SALESMAN_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json
```

**Request**:
```json
{
  "visit_id": "v9991111-2222-3333-4444-555566667777",
  "store_id": "s1111111-2222-3333-4444-555566667777",
  "fledger_invoice_id": "a1000000-0000-0000-0000-000000000001",
  "amount": 2500000,
  "payer_name": "Ibu Siti Fatimah",
  "payer_phone": "081298765432"
}
```

**Response `201 Created`**:
```json
{
  "id": "c7771111-2222-3333-4444-555566667777",
  "receipt_number": "RCP-202610-09812",
  "amount": 2500000,
  "collected_at": "2026-10-09T09:20:00Z",
  "salesman_current_cash_held": 7500000,
  "status": "HELD_BY_SALES",
  "wa_receipt_payload": {
    "to": "081298765432",
    "message": "Terima kasih Toko Sumber Rezeki. Pembayaran kas senilai Rp 2.500.000 untuk faktur INV-202610-DO-0089 telah diterima oleh Salesman Budi (SLS-JKT-004). No Kwitansi: RCP-202610-09812."
  }
}
```

---

### 2.3. Inquiry Kas Salesman oleh Kasir HQ
Kasir gudang melihat total uang kas yang wajib disetor fisik oleh salesman sebelum menerima gepokan uang.

```http
GET /v1/force/settlements/reps/r1111111-2222-3333-4444-555566667777/inquiry HTTP/1.1
Host: localhost:8084
Authorization: Bearer <CASHIER_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
```

**Response `200 OK`**:
```json
{
  "sales_rep_id": "r1111111-2222-3333-4444-555566667777",
  "sales_rep_name": "Budi Santoso",
  "employee_code": "SLS-JKT-004",
  "total_cash_held": 7500000,
  "collections_count": 3,
  "collections": [
    {
      "receipt_number": "RCP-202610-09810",
      "store_name": "Toko Makmur",
      "amount": 3000000
    },
    {
      "receipt_number": "RCP-202610-09811",
      "store_name": "Warung Barokah",
      "amount": 2000000
    },
    {
      "receipt_number": "RCP-202610-09812",
      "store_name": "Toko Sumber Rezeki",
      "amount": 2500000
    }
  ]
}
```

---

### 2.4. Setoran Kas Fisik EOD Sore Hari (EOD Settlement)
Kasir memvalidasi uang fisik yang dihitung di meja kasir. Jika cocok, status salesman dikembalikan ke `ACTIVE` (*unlocked*).

```http
POST /v1/force/settlements HTTP/1.1
Host: localhost:8084
Authorization: Bearer <CASHIER_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json
```

**Request**:
```json
{
  "sales_rep_id": "r1111111-2222-3333-4444-555566667777",
  "physical_cash_received": 7500000,
  "cashier_notes": "Uang fisik pas, pecahan Rp 100rb dan Rp 50rb"
}
```

**Response `200 OK`**:
```json
{
  "settlement_number": "EOD-202610-0041",
  "total_system_cash": 7500000,
  "physical_cash_received": 7500000,
  "discrepancy_amount": 0,
  "status": "SETTLED",
  "sales_rep_new_cash_held": 0,
  "sales_rep_status": "ACTIVE",
  "outbox_status": "QUEUED"
}
```

---

## 3. Kontrak Integrasi Outbox Worker ke Fledger Core

### A. Saat Kas Diterima di Pasar (`force.cash_collected`):
Outbox memanggil Fledger Core:
```http
POST /v1/transfers HTTP/1.1
Host: localhost:8081
Authorization: Bearer <SERVICE_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Idempotency-Key: <COLLECTION_ID>
Content-Type: application/json

{
  "source_account_id": "ACC_CUSTOMER_AR",
  "destination_account_id": "ACC_SALES_WALLET_BUDI",
  "amount": 2500000,
  "currency": "IDR",
  "description": "Pembayaran kas lapangan Toko Sumber Rezeki (RCP-202610-09812)"
}
```

### B. Saat Kas Disetor ke Gudang Sore Hari (`force.eod_settled`):
Outbox memanggil Fledger Core:
```http
POST /v1/transfers HTTP/1.1
Host: localhost:8081
Authorization: Bearer <SERVICE_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Idempotency-Key: <SETTLEMENT_ID>
Content-Type: application/json

{
  "source_account_id": "ACC_SALES_WALLET_BUDI",
  "destination_account_id": "ACC_HQ_PHYSICAL_CASH",
  "amount": 7500000,
  "currency": "IDR",
  "description": "Setoran kas fisik EOD gudang oleh Budi Santoso (EOD-202610-0041)"
}
```
