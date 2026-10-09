# FLEDGER PAY — REST API Specification & Webhook Contracts

> **Microservice**: `fledger-pay`  
> **Base Port**: `:8083`  
> **Auth**: Bearer JWT (`Authorization: Bearer <token>`) & `X-Tenant-ID: <UUID>`

---

## 1. Ringkasan Endpoint

| Method | Endpoint | Fungsi | Autentikasi |
|---|---|---|---|
| `GET` | `/healthz` | Liveness Probe | Publik |
| `GET` | `/readyz` | Readiness Probe (Ping DB) | Publik |
| `POST` | `/v1/dev/login?tenant_id=...` | Mint Dev JWT Token | Dev Mode |
| `POST` | `/v1/pay/requests` | Terbitkan VA & QRIS per Faktur | Wajib JWT |
| `GET` | `/v1/pay/requests` | Daftar Permintaan Pembayaran | Wajib JWT |
| `GET` | `/v1/pay/requests/:id` | Detail Pembayaran (Status & VA) | Wajib JWT |
| `POST` | `/v1/pay/requests/:id/cancel` | Batalkan Permintaan Pembayaran | Wajib JWT |
| `GET` | `/v1/pay/transactions` | Riwayat Transaksi Lunas | Wajib JWT |
| `GET` | `/v1/pay/outbox/counts` | Status Antrean Outbox ke Core | Wajib JWT |
| `POST` | `/v1/pay/webhooks/:gateway` | Webhook Callback Ingestion | HMAC Signature |
| `POST` | `/v1/pay/simulator/settle` | Simulasi Pembayaran Sandbox | Dev/Sandbox |

---

## 2. Rincian Endpoint

### 2.1. Terbitkan Permintaan Pembayaran (Create Payment Request)
Menghasilkan nomor Virtual Account multi-bank dan QRIS dinamis untuk faktur tertentu.

```http
POST /v1/pay/requests HTTP/1.1
Host: localhost:8083
Authorization: Bearer <JWT_TOKEN>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Content-Type: application/json
```

**Request Body**:
```json
{
  "fledger_invoice_id": "a1000000-0000-0000-0000-000000000001",
  "customer_id": "c1000000-0000-0000-0000-000000000001",
  "customer_name": "Toko Sumber Rezeki",
  "customer_phone": "081298765432",
  "amount": 4000000,
  "currency": "IDR",
  "expiry_minutes": 1440,
  "enabled_banks": ["BCA", "MANDIRI", "BRI"],
  "enable_qris": true
}
```

**Response `201 Created`**:
```json
{
  "id": "e9871100-2222-3333-4444-555566667777",
  "request_number": "PAY-202610-00891",
  "fledger_invoice_id": "a1000000-0000-0000-0000-000000000001",
  "customer_name": "Toko Sumber Rezeki",
  "amount": 4000000,
  "status": "PENDING",
  "expires_at": "2026-10-10T14:00:00Z",
  "virtual_accounts": [
    {
      "bank_code": "BCA",
      "va_number": "89012081298765432",
      "va_name": "FLEDGER - TOKO SUMBER REZEKI",
      "expected_amount": 4000000,
      "status": "ACTIVE"
    },
    {
      "bank_code": "MANDIRI",
      "va_number": "88701081298765432",
      "va_name": "FLEDGER - TOKO SUMBER REZEKI",
      "expected_amount": 4000000,
      "status": "ACTIVE"
    }
  ],
  "qris": {
    "qr_string": "00020101021226680016ID.CO.FLEDGER.WWW0118936009180000000001520454115303360540740000005802ID5918TOKO SUMBER REZEKI6007JAKARTA6304A1B2",
    "expected_amount": 4000000,
    "status": "ACTIVE"
  }
}
```

---

### 2.2. Webhook Ingestion Callback
Menerima notifikasi seketika dari Payment Gateway / Bank saat toko membayar.

```http
POST /v1/pay/webhooks/midtrans HTTP/1.1
Host: localhost:8083
Content-Type: application/json
X-Signature-SHA256: 7d6b38c...
```

**Payload Standar Gateway**:
```json
{
  "transaction_time": "2026-10-09 14:15:00",
  "transaction_status": "settlement",
  "order_id": "PAY-202610-00891",
  "gross_amount": "4000000.00",
  "payment_type": "bank_transfer",
  "bank": "bca",
  "va_number": "89012081298765432",
  "signature_key": "7d6b38c35d9475fa4e0c..."
}
```

**Logika Pemrosesan**:
1. **Signature Verification**:
   $$\text{SHA512}(\text{order\_id} + \text{status\_code} + \text{gross\_amount} + \text{server\_key}) \stackrel{?}{=} \text{signature\_key}$$
2. **Idempotency Guard**: Memeriksa `pay_transactions.external_reference`. Jika sudah tercatat, kembalikan `200 OK` tanpa memproses ulang.
3. **Pencatatan Transaksi**: Insert ke `pay_transactions`.
4. **Update Status Permintaan**: Ubah `pay_payment_requests.status = 'SETTLED'`.
5. **Enqueue Outbox Settlement**: Masukkan payload ke `pay_settlement_outbox` untuk diproses ke `fledger-core`.
6. Kembalikan response `200 OK`.

---

### 2.3. Simulator Pembayaran Sandbox (Demo & Pengujian Instan)
Memungkinkan penguji atau salesman mengeksekusi simulasi bayar tanpa rekening bank riil.

```http
POST /v1/pay/simulator/settle HTTP/1.1
Host: localhost:8083
Content-Type: application/json
```

**Request**:
```json
{
  "request_number": "PAY-202610-00891",
  "channel": "VA_BCA",
  "amount": 4000000,
  "payer_name": "Ibu Siti Fatimah (BCA Mobile Demo)"
}
```

**Response `200 OK`**:
```json
{
  "status": "SETTLED",
  "request_number": "PAY-202610-00891",
  "transaction_id": "11223344-5566-7788-9900-aabbccddeeff",
  "fledger_invoice_id": "a1000000-0000-0000-0000-000000000001",
  "settled_at": "2026-10-09T14:15:30Z",
  "outbox_status": "QUEUED"
}
```

---

## 3. Kontrak Integrasi Outbox Worker ke Fledger Core

Ketika outbox worker memproses pengiriman ke Fledger Core:

```http
POST /v1/transfers HTTP/1.1
Host: localhost:8081
Authorization: Bearer <FLEDGER_CORE_SERVICE_JWT>
X-Tenant-ID: 00000000-0000-0000-0000-000000000001
Idempotency-Key: <PAY_TRANSACTION_ID>
Content-Type: application/json

{
  "source_account_id": "acc-customer-ar-001",
  "destination_account_id": "acc-bank-bca-001",
  "amount": 4000000,
  "currency": "IDR",
  "reference_type": "INVOICE_SETTLEMENT",
  "reference_id": "a1000000-0000-0000-0000-000000000001",
  "description": "Auto-settlement via BCA Virtual Account (PAY-202610-00891)"
}
```
Dan memanggil update invoice di Fledger Core:
```http
POST /v1/invoices/a1000000-0000-0000-0000-000000000001/pay HTTP/1.1
```
