# Transfers API

The **double-entry ledger** core. Each transfer writes exactly 2 entries (1 debit + 1 credit) that must sum to zero within currency scope. Cross-currency transfers use **asymmetric entries** with FX rate snapshot.

---

## `POST /v1/transfers`

Create a new transfer between two accounts. Idempotent via `Idempotency-Key` header.

### Required headers
- `Authorization: Bearer <access_token>`
- `Idempotency-Key: <unique-string>` (24h TTL per (tenant, key))
- `Content-Type: application/json`

### Request body
```json
{
  "from_account_id": "11111111-1111-1111-1111-111111111111",
  "to_account_id": "22222222-2222-2222-2222-222222222222",
  "amount_minor": 10000,
  "currency": "IDR",
  "description": "Payment to supplier",
  "expected_fx_rate_id": null,
  "expected_rate_lock_at": null
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `from_account_id` | UUID | ✅ | Source account (must be `active`) |
| `to_account_id` | UUID | ✅ | Destination account (must be `active`) |
| `amount_minor` | int64 | ✅ | Amount in **minor units** (e.g., 10000 = IDR 100.00) |
| `currency` | string | ✅ | ISO 4217 code; must match source account currency OR trigger FX lookup |
| `description` | string | ❌ | Free text, ≤256 chars |
| `expected_fx_rate_id` | UUID | ❌ | Client-side rate pinning (cross-currency) |
| `expected_rate_lock_at` | RFC3339 | ❌ | Timestamp tolerance window for pinned rate (±5 min) |

### Response 201 Created
```json
{
  "data": {
    "transaction_id": "tx_abc123...",
    "from_account_id": "11111111-...",
    "to_account_id": "22222222-...",
    "amount_minor": 10000,
    "currency": "IDR",
    "status": "posted",
    "fx_rate_id": null,
    "fx_rate_locked_at": null,
    "created_at": "2026-08-15T10:00:00Z"
  }
}
```

### Idempotent replay response (same key + same body)
Returns `200 OK` with the **same transaction** (instead of 201). Idempotency-Key TTL is 24h.

### Cross-currency transfer

When `currency != source_account.currency`, the system:
1. Looks up latest active FX rate for (tenant, from, to)
2. Converts amount using `money.Convert(fromDP, toDP, rate)` (half-up rounding)
3. Writes **asymmetric entries**: debit in source currency, credit in destination currency
4. Snapshots `fx_rate_id` + `fx_rate_locked_at` on the transaction header

```json
{
  "data": {
    "transaction_id": "tx_def456...",
    "from_account_id": "11111111-...", // USD account
    "to_account_id": "22222222-...",   // IDR account
    "amount_minor": 10000,             // USD 100.00 (source currency)
    "currency": "USD",
    "status": "posted",
    "fx_rate_id": "rate_xyz789...",
    "fx_rate_locked_at": "2026-08-15T10:00:00Z"
  }
}
```

### Failure modes

| Status | Code | Reason |
|---|---|---|
| 400 | `IDEMPOTENCY_KEY_MISSING` | Header not provided |
| 400 | `VALIDATION_FAILED` | Missing/invalid fields |
| 409 | `IDEMPOTENCY_CONFLICT` | Same key + different body |
| 409 | `INVALID_STATE` | Source/destination account not `active` |
| 422 | `INSUFFICIENT_BALANCE` | Source balance < amount |
| 422 | `CURRENCY_MISMATCH` | Bad currency for accounts |

### cURL example
```bash
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/transfers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -d '{
    "from_account_id": "11111111-1111-1111-1111-111111111111",
    "to_account_id": "22222222-2222-2222-2222-222222222222",
    "amount_minor": 10000,
    "currency": "IDR",
    "description": "Payment to supplier"
  }'
```

### Cross-currency cURL
```bash
# Convert USD 100 → IDR (rate 15750)
curl -X POST https://fmcg-wallet-demo.fly.dev/v1/transfers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: $(uuidgen)" \
  -H "Content-Type: application/json" \
  -d '{
    "from_account_id": "<usd-acct>",
    "to_account_id": "<idr-acct>",
    "amount_minor": 10000,
    "currency": "USD",
    "description": "USD payment to IDR supplier"
  }'
# → writes USD 100 debit + IDR 1,575,000 credit (asymmetric entries)
```

---

## Concurrency guarantees

| Scenario | Behavior |
|---|---|
| Same key, same body | Returns original transaction (200) |
| Same key, different body | 409 `IDEMPOTENCY_CONFLICT` |
| Concurrent transfers same source | All serialize via `SELECT FOR UPDATE` (deadlock-free via UUID-sort) |
| Same-source same-amount, different keys | All succeed (no double-spend; each has unique idempotency_key) |

### Concurrency proof
Tested by `TestConcurrent_TransfersFromSameSource` (50 concurrent → 0 lost updates) and `TestConcurrent_NoDeadlocks_100x50` (100×50 = 5000 transfers → 0 deadlocks).

---

## Implementation

- `internal/usecase/transfer_service.go` — `TransferService.Transfer()` orchestration
- `internal/repository/postgres/tx_adapter.go` — `RunInTxDomain` with auto-tenant binding
- `internal/domain/ledger/service.go` — `TransferInput` interface
- `internal/handler/handlers.go:274` — `CreateTransfer` handler
- [ADR-0004: Locking strategy](../adr/0004-locking-strategy.md)
- [Sprint 12: Multi-Currency rate snapshot](../SPRINTS.md#sprint-12--multi-currency-fase-1d--2026-08-14)

See [API Overview](overview.md) for error envelope, status codes, and idempotency semantics.
