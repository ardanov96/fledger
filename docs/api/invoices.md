# Invoices API

Customer receivables: create invoices, record payments (FIFO or manual allocation), view aging buckets, set credit limits. Built on top of the [double-entry ledger](transfers.md).

---

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/v1/invoices` | Create invoice (atomic credit-limit check) |
| `GET` | `/v1/invoices/{id}` | Get invoice by ID |
| `GET` | `/v1/invoices` | List invoices (filter by customer_id, status, aging_bucket) |
| `POST` | `/v1/customers/{id}/payments` | Record payment (FIFO or manual allocation) |
| `GET` | `/v1/customers/{id}/aging` | Aging summary (6 buckets) |
| `POST` | `/v1/customers/{id}/credit-limit` | Set/update customer credit limit |

All require `Authorization: Bearer <access_token>`.

---

## `POST /v1/invoices`

Create a customer invoice. Atomically checks credit limit before insert.

### Request body
```json
{
  "customer_id": "55555555-5555-5555-5555-555555555555",
  "code": "INV-2026-001",
  "amount_minor": 2500000000,
  "currency": "IDR",
  "due_date": "2026-09-15T00:00:00Z",
  "description": "Q3 distribution services"
}
```

### Response 201 Created
```json
{
  "data": {
    "id": "inv_abc123",
    "customer_id": "55555555-...",
    "code": "INV-2026-001",
    "amount_minor": 2500000000,
    "paid_amount_minor": 0,
    "currency": "IDR",
    "due_date": "2026-09-15T00:00:00Z",
    "status": "open",
    "issued_at": "2026-08-15T10:00:00Z",
    "period_id": "period_xyz"
  }
}
```

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 422 | `CREDIT_LIMIT_EXCEEDED` | `used + amount > limit` |
| 409 | `INVALID_STATE` | Customer or period not active |

---

## `POST /v1/customers/{id}/payments`

Record a payment, allocate to invoices. **Default mode = FIFO** (oldest due_date first). **Manual mode** = caller specifies which invoices + amounts.

### Request body (FIFO mode — default)
```json
{
  "amount_minor": 1000000000,
  "method": "transfer",
  "mode": "fifo",
  "description": "Partial payment August"
}
```

### Request body (manual mode)
```json
{
  "amount_minor": 1000000000,
  "method": "transfer",
  "mode": "manual",
  "allocations": [
    {"invoice_id": "inv_abc123", "amount_minor": 700000000},
    {"invoice_id": "inv_def456", "amount_minor": 300000000}
  ]
}
```

### Response 201 Created
```json
{
  "data": {
    "payment_id": "pay_xyz789",
    "method": "transfer",
    "customer_id": "55555555-...",
    "total_minor": 1000000000,
    "allocations": [
      {"invoice_id": "inv_abc123", "amount_minor": 700000000},
      {"invoice_id": "inv_def456", "amount_minor": 300000000}
    ]
  }
}
```

### Idempotency
Requires `Idempotency-Key` header. Same key + same body → returns original payment (200).

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 400 | `IDEMPOTENCY_KEY_MISSING` | Header required |
| 409 | `IDEMPOTENCY_CONFLICT` | Same key, different body |
| 422 | `PAYMENT_ALLOCATION_MISMATCH` | Manual mode: SUM(allocations) != amount |
| 422 | `INVOICE_OVERPAID` | Allocation exceeds invoice outstanding |

---

## `GET /v1/customers/{id}/aging`

Returns aging summary for outstanding invoices, bucketed by days overdue.

### Response 200 OK
```json
{
  "data": [
    {"bucket": "current", "count": 5, "outstanding_minor": 1200000000},
    {"bucket": "d_1_7",    "count": 2, "outstanding_minor":  300000000},
    {"bucket": "d_8_30",   "count": 1, "outstanding_minor":  150000000},
    {"bucket": "d_31_60",  "count": 0, "outstanding_minor":         0},
    {"bucket": "d_61_90",  "count": 1, "outstanding_minor":  400000000},
    {"bucket": "d_90_plus","count": 0, "outstanding_minor":         0}
  ]
}
```

| Bucket | Days overdue |
|---|---|
| `current` | 0 (not yet due) |
| `d_1_7` | 1-7 |
| `d_8_30` | 8-30 |
| `d_31_60` | 31-60 |
| `d_61_90` | 61-90 |
| `d_90_plus` | 90+ |

---

## `POST /v1/customers/{id}/credit-limit`

Upsert customer credit cap.

### Request body
```json
{
  "limit_minor": 10000000000,
  "effective_from": "2026-08-15T00:00:00Z"
}
```

### Response 200 OK
```json
{
  "data": {
    "customer_id": "55555555-...",
    "limit_minor": 10000000000,
    "used_minor": 2500000000,
    "available_minor": 7500000000
  }
}
```

---

## Aging bucket monotonicity (property-tested)

The aging bucket assignment respects monotonicity: `bucket(N+1 days) >= bucket(N days)`. Property test: `TestProperty_AgingBucketMonotonic` (Sprint 16).

---

## Implementation

- `internal/usecase/invoice_service.go` — `InvoiceService` orchestration
- `internal/repository/postgres/tx_adapter_invoice.go` — tx adapter with RLS
- `internal/domain/invoice/invoice.go` — entities + repository interface
- `migrations/000006_create_invoices.up.sql` — schema + view `v_invoice_aging`
- [Sprint 8: Invoice & Credit + Hash Chain + JWT/RBAC](../SPRINTS.md#sprint-8--invoice--credit--hash-chain--jwtrbac--2026-08-11)
