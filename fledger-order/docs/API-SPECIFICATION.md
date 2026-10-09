# FLEDGER ORDER — REST API Specification & Integration Contracts

> **Microservice**: `fledger-order`  
> **Base Port**: `:8085`  
> **Auth**: Bearer JWT (`Authorization: Bearer <token>`) & `X-Tenant-ID: <UUID>`

---

## 1. Ringkasan Endpoint

| Method | Endpoint | Fungsi | Hak Akses |
|---|---|---|---|
| `GET` | `/healthz` | Liveness Probe | Publik |
| `GET` | `/readyz` | Readiness Probe (Ping DB) | Publik |
| `POST` | `/v1/dev/login?tenant_id=...` | Mint Dev JWT Token | Dev Mode |
| `GET` | `/v1/order/products` | Katalog SKU FMCG & Harga | Autentikasi |
| `POST` | `/v1/order/products` | Tambah SKU Produk Baru | Admin / Master |
| `GET` | `/v1/order/products/:id` | Detail SKU & Tiering Harga | Autentikasi |
| `POST` | `/v1/order/products/:id/pricing` | Atur Tiering Harga Grosir | Admin / Master |
| `GET` | `/v1/order/inventory` | Cek Stok Fisik & Stok Tersedia | Autentikasi |
| `POST` | `/v1/order/inventory/adjust` | Penyesuaian / Masuk Stok Gudang | Admin Gudang |
| `POST` | `/v1/order/orders` | Buat Pesanan Baru (Draft) | Toko / Salesman |
| `GET` | `/v1/order/orders` | Daftar Pesanan B2B | Autentikasi |
| `GET` | `/v1/order/orders/:id` | Detail Pesanan & Rincian Item | Autentikasi |
| `POST` | `/v1/order/orders/:id/evaluate-credit` | Evaluasi Hard Credit Gate ke Core | Sistem / Sales |
| `POST` | `/v1/order/orders/:id/override-credit` | Override Blokir oleh Manager | Finance Manager |
| `POST` | `/v1/order/orders/:id/dispatch-fleet` | Terbitkan DO ke Fledger Fleet | Dispatcher |
| `POST` | `/v1/order/orders/:id/cancel` | Batalkan Pesanan & Lepas Stok | Autentikasi |
| `GET` | `/v1/order/outbox/counts` | Status Antrean Outbox ke Fleet | Admin |

---

## 2. Rincian Endpoint Kunci

### 2.1. Buat Pesanan Baru (Create Purchase Order)
Membuat pesanan baru, menghitung harga otomatis berdasarkan tier toko, dan mengunci stok gudang (*reserve stock*):

```http
POST /v1/order/orders HTTP/1.1
Host: localhost:8085
Authorization: Bearer <JWT_TOKEN>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json
```

**Request**:
```json
{
  "customer_id": "c1000000-0000-0000-0000-000000000001",
  "customer_name": "Toko Sumber Rezeki",
  "customer_tier": "GROSIR",
  "customer_phone": "081298765432",
  "destination_address": "Jl. Raya Daan Mogot No. 45, Jakarta Barat",
  "items": [
    {
      "product_id": "p1111111-2222-3333-4444-555566667777",
      "quantity": 10
    },
    {
      "product_id": "p2222222-2222-3333-4444-555566667777",
      "quantity": 20
    }
  ],
  "notes": "Pengantaran pagi sebelum jam 11:00"
}
```

**Response `201 Created`**:
```json
{
  "id": "o8881111-2222-3333-4444-555566667777",
  "order_number": "ORD-202610-00812",
  "customer_name": "Toko Sumber Rezeki",
  "total_weight_kg": 320,
  "subtotal": 4200000,
  "discount": 200000,
  "total_amount": 4000000,
  "status": "PENDING_CREDIT_CHECK",
  "items": [
    {
      "sku": "SKU-OIL-BIMOLI-2L",
      "name": "Minyak Goreng Bimoli 2L (Dus)",
      "quantity": 10,
      "unit_price": 120000,
      "line_total": 1200000
    },
    {
      "sku": "SKU-MIE-INDOMIE-GRG",
      "name": "Indomie Goreng Original (Dus)",
      "quantity": 20,
      "unit_price": 140000,
      "line_total": 2800000
    }
  ]
}
```

---

### 2.2. Evaluasi Hard Credit Gate (Pre-Flight Check ke Fledger Core)
Memeriksa status piutang toko ke Fledger Core (`:8081`). Jika melanggar batas kredit atau memiliki faktur macet $>30$ hari, pesanan otomatis dikunci `CREDIT_BLOCKED`.

```http
POST /v1/order/orders/o8881111-2222-3333-4444-555566667777/evaluate-credit HTTP/1.1
Host: localhost:8085
Authorization: Bearer <JWT_TOKEN>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
```

**Response 1: Kredit Lolos (`PASSED`)**:
```json
{
  "order_id": "o8881111-2222-3333-4444-555566667777",
  "order_status": "APPROVED",
  "credit_gate_status": "PASSED",
  "evaluation": {
    "credit_limit": 20000000,
    "current_ar_outstanding": 8500000,
    "new_order_amount": 4000000,
    "projected_total_ar": 12500000,
    "has_overdue_30d": false,
    "oldest_overdue_days": 12
  }
}
```

**Response 2: Kredit Diblokir (`CREDIT_BLOCKED`)**:
```json
{
  "order_id": "o8881111-2222-3333-4444-555566667777",
  "order_status": "CREDIT_BLOCKED",
  "credit_gate_status": "OVERDUE_BLOCKED",
  "evaluation": {
    "credit_limit": 20000000,
    "current_ar_outstanding": 18000000,
    "new_order_amount": 4000000,
    "projected_total_ar": 22000000,
    "has_overdue_30d": true,
    "oldest_overdue_days": 47,
    "overdue_invoice_id": "INV-202608-DO-0012",
    "block_reason": "Toko menunggak faktur INV-202608-DO-0012 selama 47 hari dan total piutang melebihi limit Rp 20.000.000."
  }
}
```

---

### 2.3. Emergency Override Kredit oleh Finance Manager
Membuka pesanan yang diblokir menggunakan otorisasi manajerial khusus dengan catatan resmi:

```http
POST /v1/order/orders/o8881111-2222-3333-4444-555566667777/override-credit HTTP/1.1
Host: localhost:8085
Authorization: Bearer <MANAGER_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json

{
  "override_pin": "992144",
  "reason": "Owner toko sudah berjanji transfer pelunasan hari ini pukul 14:00. Otorisasi darurat disetujui."
}
```

**Response `200 OK`**:
```json
{
  "order_id": "o8881111-2222-3333-4444-555566667777",
  "order_status": "APPROVED",
  "credit_gate_status": "OVERRIDDEN",
  "override_by": "finance-director-01",
  "override_at": "2026-10-09T10:30:00Z"
}
```

---

### 2.4. Terbitkan Surat Jalan ke Fledger Fleet (Dispatch to Fleet)
Memicu pembuatan Surat Jalan (DO) resmi di `fledger-fleet` (`:8082`):

```http
POST /v1/order/orders/o8881111-2222-3333-4444-555566667777/dispatch-fleet HTTP/1.1
Host: localhost:8085
Authorization: Bearer <JWT_TOKEN>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
```

**Response `200 OK`**:
```json
{
  "order_id": "o8881111-2222-3333-4444-555566667777",
  "order_status": "DISPATCHED_TO_FLEET",
  "fledger_fleet_do_id": "d5551111-2222-3333-4444-555566667777",
  "fleet_do_number": "DO-202610-00812",
  "dispatched_at": "2026-10-09T10:35:00Z"
}
```

---

## 3. Kontrak Integrasi Antar-Layanan

### A. Panggilan ke Fledger Core (`:8081`):
Membaca posisi piutang dan keterlambatan pembayaran toko:
```http
GET /v1/invoices?customer_id=c1000000-0000-0000-0000-000000000001&status=ISSUED HTTP/1.1
Host: localhost:8081
```
```http
GET /v1/aging HTTP/1.1
Host: localhost:8081
```

### B. Panggilan ke Fledger Fleet (`:8082`):
Menerbitkan Surat Jalan (DO) saat order disetujui:
```http
POST /v1/fleet/delivery-orders HTTP/1.1
Host: localhost:8082
Authorization: Bearer <SERVICE_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json

{
  "do_number": "DO-202610-00812",
  "customer_name": "Toko Sumber Rezeki",
  "destination_address": "Jl. Raya Daan Mogot No. 45, Jakarta Barat",
  "total_nominal": 4000000,
  "total_weight_kg": 320,
  "items": [
    {
      "sku_id": "SKU-OIL-BIMOLI-2L",
      "name": "Minyak Goreng Bimoli 2L (Dus)",
      "quantity": 10,
      "unit_price": 120000
    },
    {
      "sku_id": "SKU-MIE-INDOMIE-GRG",
      "name": "Indomie Goreng Original (Dus)",
      "quantity": 20,
      "unit_price": 140000
    }
  ]
}
```
