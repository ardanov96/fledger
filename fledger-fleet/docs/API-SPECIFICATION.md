# FLEDGER FLEET — REST API Specification & Integration Contracts

> **Base URL**: `http://localhost:8082`  
> **Auth**: Bearer Token (JWT dengan claim `tenant_id`, `role`, `user_id`)  
> **Target Consumer**: Driver Mobile Web, Warehouse Dispatch Portal, Fledger Core Settlement Engine

---

## 1. Endpoints Master Data Armada & Supir

### 1.1. List Kendaraan Armada
* **Method**: `GET /v1/fleet/vehicles`
* **Query Params**: `status` (opsional: `AVAILABLE`, `ON_TRIP`, `MAINTENANCE`)
* **Response `200 OK`**:
```json
[
  {
    "id": "v1000000-0000-0000-0000-000000000001",
    "plate_number": "B 9123 FMC",
    "vehicle_type": "CDE_BOX",
    "brand_model": "Isuzu Traga Box",
    "capacity_kg": 1500.00,
    "status": "AVAILABLE"
  }
]
```

### 1.2. Registrasi Kendaraan
* **Method**: `POST /v1/fleet/vehicles`
* **Request Body**:
```json
{
  "plate_number": "B 9124 FMC",
  "vehicle_type": "CDD_BOX",
  "brand_model": "Mitsubishi Colt Diesel",
  "capacity_kg": 4000.00
}
```

### 1.3. List Supir
* **Method**: `GET /v1/fleet/drivers`
* **Query Params**: `status` (opsional: `ACTIVE`, `ON_DUTY`)
* **Response `200 OK`**:
```json
[
  {
    "id": "d1000000-0000-0000-0000-000000000001",
    "full_name": "Budi Santoso",
    "phone_number": "081298765432",
    "license_number": "SIM-B1-881230",
    "status": "ACTIVE"
  }
]
```

---

## 2. Endpoints Trip & Dispatching

### 2.1. Buat Trip Perjalanan Baru
* **Method**: `POST /v1/fleet/trips`
* **Request Body**:
```json
{
  "trip_number": "TRIP-202610-001",
  "vehicle_id": "v1000000-0000-0000-0000-000000000001",
  "driver_id": "d1000000-0000-0000-0000-000000000001",
  "do_ids": [
    "do-00000000-0000-0000-0000-000000000001",
    "do-00000000-0000-0000-0000-000000000002"
  ],
  "notes": "Rute Pengantaran Area Jakarta Barat - 5 Outlet"
}
```
* **Response `201 Created`**:
```json
{
  "id": "t1000000-0000-0000-0000-000000000001",
  "trip_number": "TRIP-202610-001",
  "status": "DRAFT",
  "total_stops": 2,
  "created_at": "2026-10-08T08:00:00Z"
}
```

### 2.2. Berangkatkan Trip (Dispatch)
* **Method**: `POST /v1/fleet/trips/:id/dispatch`
* **Deskripsi**: Mengubah status Trip menjadi `IN_TRANSIT` dan seluruh DO yang ada di dalamnya menjadi `OUT_FOR_DELIVERY`.
* **Response `200 OK`**:
```json
{
  "id": "t1000000-0000-0000-0000-000000000001",
  "status": "IN_TRANSIT",
  "departure_time": "2026-10-08T09:00:00Z"
}
```

---

## 3. Endpoints Delivery Order (Surat Jalan) & Proof of Delivery (POD)

### 3.1. Buat Delivery Order (DO) Baru
* **Method**: `POST /v1/fleet/delivery-orders`
* **Request Body**:
```json
{
  "do_number": "DO-202610-0089",
  "customer_id": "c1000000-0000-0000-0000-000000000001",
  "customer_name": "Toko Berkah Mandiri",
  "destination_address": "Jl. Raya Daan Mogot No. 45, Jakarta Barat",
  "destination_lat": -6.152345,
  "destination_lng": 106.758912,
  "items": [
    {
      "product_sku": "SKU-OIL-01",
      "product_name": "Minyak Goreng 2L Pouch",
      "qty_ordered": 10,
      "unit_price_cents": 500000
    }
  ]
}
```
* **Response `201 Created`**:
```json
{
  "id": "do-00000000-0000-0000-0000-000000000001",
  "do_number": "DO-202610-0089",
  "nominal_ordered_cents": 5000000,
  "status": "PENDING"
}
```

### 3.2. Submit Digital POD & Selesaikan Pengantaran (Core Trigger)
* **Method**: `POST /v1/fleet/delivery-orders/:id/pod`
* **Headers**: `Idempotency-Key: <UUID>`
* **Request Body**:
```json
{
  "recipient_name": "Ibu Siti Fatimah",
  "recipient_phone": "081311223344",
  "signature_data_url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
  "photo_evidence_urls": [
    "https://storage.fledger.io/evidence/pod-do-0089-damage.jpg"
  ],
  "delivered_lat": -6.152350,
  "delivered_lng": 106.758920,
  "driver_notes": "2 karton minyak bocor akibat guncangan jalan",
  "items_result": [
    {
      "product_sku": "SKU-OIL-01",
      "qty_delivered": 8,
      "qty_rejected": 2,
      "rejection_reason": "DAMAGED_LEAK"
    }
  ]
}
```
* **Alur Eksekusi Internal**:
  1. Validasi: Hitung `nominal_delivered_cents` = $8 \times 500.000 = 4.000.000$.
  2. Status DO berubah menjadi: `DELIVERED_PARTIAL`.
  3. Panggil Fledger Core API:
     ```http
     POST http://localhost:8081/v1/invoices
     Authorization: Bearer <FLEDGER_CORE_JWT>
     X-Tenant-ID: 00000000-0000-0000-0000-000000000001
     Idempotency-Key: do-00000000-0000-0000-0000-000000000001

     {
       "customer_id": "c1000000-0000-0000-0000-000000000001",
       "invoice_number": "INV-DO-202610-0089",
       "amount": 4000000,
       "currency": "IDR",
       "due_date": "2026-10-22T23:59:59Z",
       "notes": "Generated from DO-202610-0089 (8 diterima, 2 rusak/bocor)"
     }
     ```
  4. Simpan `fledger_invoice_id` dari respon Core ke tabel `fleet_delivery_orders`.
* **Response `200 OK`**:
```json
{
  "do_id": "do-00000000-0000-0000-0000-000000000001",
  "status": "DELIVERED_PARTIAL",
  "nominal_delivered_cents": 4000000,
  "fledger_invoice_id": "f1000000-0000-0000-0000-000000000099",
  "message": "POD successfully submitted and clean invoice synced to Fledger Core"
}
```

---

## 4. Error Handling & Retry Matrix

| Kasus | HTTP Status | Respons / Tindakan |
|---|---|---|
| DO belum berstatus `OUT_FOR_DELIVERY` | `400 Bad Request` | `{"error": "DO must be OUT_FOR_DELIVERY before submitting POD"}` |
| Barang ditolak tanpa foto bukti | `422 Unprocessable Entity` | `{"error": "Photo evidence is required when rejected items > 0"}` |
| Fledger Core offline / timeout | `200 OK (Queued)` | POD tetap disimpan lokal, event masuk ke outbox table untuk retry otomatis ke Core setiap 30 detik |
