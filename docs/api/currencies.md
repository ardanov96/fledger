# Currencies & FX API

ISO 4217 currency registry + FX rate management. Cross-currency transfers use **rate snapshot per transaction** for audit-grade history.

---

## Endpoints (9 total)

### Currencies

| Method | Path | RBAC Action |
|---|---|---|
| `GET` | `/v1/currencies` | `currency:read` |
| `GET` | `/v1/currencies/{code}` | `currency:read` |
| `POST` | `/v1/currencies` | `currency:create` (admin) |
| `PATCH` | `/v1/currencies/{code}` | `currency:update` (admin) |
| `POST` | `/v1/currencies/convert` | `currency:read` (pure math, no side effect) |

### FX Rates

| Method | Path | RBAC Action |
|---|---|---|
| `GET` | `/v1/fx-rates` | `fx_rate:read` |
| `GET` | `/v1/fx-rates/latest` | `fx_rate:read` |
| `GET` | `/v1/fx-rates/{id}` | `fx_rate:read` |
| `POST` | `/v1/fx-rates` | `fx_rate:create` |

---

## `GET /v1/currencies`

List active currencies (filter `?active=true` for active only).

### Response 200 OK
```json
{
  "data": [
    {
      "code": "IDR",
      "name": "Indonesian Rupiah",
      "decimal_places": 2,
      "is_active": true
    },
    {
      "code": "USD",
      "name": "US Dollar",
      "decimal_places": 2,
      "is_active": true
    },
    {
      "code": "JPY",
      "name": "Japanese Yen",
      "decimal_places": 0,
      "is_active": true
    }
  ]
}
```

---

## `POST /v1/currencies`

Register a new currency.

### Request body
```json
{
  "code": "EUR",
  "name": "Euro",
  "decimal_places": 2,
  "is_active": true
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `code` | string | ✅ | ISO 4217 3-letter code (uppercase) |
| `name` | string | ✅ | Display name |
| `decimal_places` | int | ✅ | 0 (JPY) or 2 (most currencies), 3 (BHD, KWD, OMR) |
| `is_active` | bool | ✅ default true | Soft-delete flag |

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 400 | `INVALID_CURRENCY_CODE` | Empty or non-3-letter code |
| 409 | (DB unique violation) | Code already exists |

---

## `POST /v1/fx-rates`

Register a new FX rate. Admin only.

### Request body
```json
{
  "tenant_id": "11111111-...",
  "from_currency": "USD",
  "to_currency": "IDR",
  "rate": "15750.0000000000",
  "source": "manual",
  "effective_at": "2026-08-15T00:00:00Z",
  "expires_at": "2026-09-15T00:00:00Z"
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `tenant_id` | UUID | ✅ | Tenant scope |
| `from_currency` | string | ✅ | ISO 4217 |
| `to_currency` | string | ✅ | ISO 4217; must differ from `from_currency` |
| `rate` | decimal string | ✅ | High precision (NUMERIC(20,10)), e.g. `"15750.0000000000"` |
| `source` | enum | ✅ | `manual` \| `api` \| `bank` \| `seed` |
| `effective_at` | RFC3339 | ✅ | When this rate starts being valid |
| `expires_at` | RFC3339 | ✅ | When this rate expires (TTL) |

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 400 | `INVALID_FX_RATE` | Rate <= 0 |
| 400 | (validation) | `from_currency == to_currency` |
| 400 | (validation) | `expires_at <= effective_at` |

---

## `GET /v1/fx-rates/latest?tenant_id=...&from=USD&to=IDR`

Get the currently active FX rate for a pair.

### Response 200 OK
```json
{
  "data": {
    "id": "rate_xyz789",
    "tenant_id": "...",
    "from_currency": "USD",
    "to_currency": "IDR",
    "rate": "15750.0000000000",
    "source": "manual",
    "effective_at": "2026-08-15T00:00:00Z",
    "expires_at": "2026-09-15T00:00:00Z"
  }
}
```

Failure: 404 `FX_RATE_NOT_FOUND` if no active rate exists.

---

## `POST /v1/currencies/convert` — Pure FX math

**No side effects.** Compute converted amount without creating any records. Useful for previews in UI.

### Request body
```json
{
  "tenant_id": "...",
  "from_currency": "USD",
  "to_currency": "IDR",
  "amount_minor": 10000
}
```

### Response 200 OK
```json
{
  "data": {
    "from_minor": 10000,
    "to_minor": 157500000,
    "rate": "15750.0000000000",
    "rate_id": "rate_xyz789",
    "from_decimal_places": 2,
    "to_decimal_places": 2
  }
}
```

### Decimal place handling

`money.Convert(amount, fromDP, toDP, rate)`:
- JPY (0dp) → IDR (2dp): 1000 JPY × 157.50 = 157500 IDR (with proper minor-unit shift)
- USD (2dp) → JPY (0dp): 100 USD × 157.50 = 15750 JPY
- Uses `decimal.Decimal` for precision + half-up rounding

### cURL example
```bash
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/currencies/convert \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_id": "11111111-...",
    "from_currency": "USD",
    "to_currency": "IDR",
    "amount_minor": 10000
  }'
# → 157500000 (IDR 1,575,000.00)
```

---

## Rate snapshot semantics

When `POST /v1/transfers` is called with `currency != source_account.currency`:

1. System looks up **latest active rate** (where `now() BETWEEN effective_at AND expires_at`)
2. Computes converted amount via `money.Convert`
3. Writes **2 entries**: debit in source currency, credit in converted destination currency
4. **Snapshots** `fx_rate_id` + `fx_rate_locked_at` on the transaction header

This ensures historical reports can replay the exact rate used at transaction time, not the current rate.

See [ADR-0005: Multi-currency strategy](../adr/0005-multi-currency-strategy.md) for full rationale.

---

## Implementation

- `internal/usecase/currency_service.go` — `CurrencyService` (8 methods)
- `internal/repository/postgres/currency_repo.go` — currencies + fx_rates persistence
- `internal/platform/money/money.go` — `money.Convert(amount, fromDP, toDP, rate)`
- `migrations/000012_multi_currency.up.sql` — schema + DB trigger `enforce_fx_rate_snapshot`
- [Sprint 12: Multi-Currency](../SPRINTS.md#sprint-12--multi-currency-fase-1d--2026-08-14)
