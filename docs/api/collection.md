# Collection Routes API

Sales-rep daily collection workflow: plan route → visit customer stops → record collections → close stops → settle end-of-day deposit.

---

## Lifecycle

```
PlanRoute → StartRoute → RecordVisit (per stop) → CloseStop (per stop)
                ↓
         CompleteRoute (when all stops closed)
                ↓
         SettleRoute (sales rep deposit; discrepancy != 0 needs approval)
                ↓
         ApproveSettlement (supervisor if discrepancy)
```

**Status precedence:** `tampered > imbalanced > balanced` (matches reconciler). Settlements: `pending → approved` (if discrepancy==0) or `pending → approved/rejected` (supervisor decision).

---

## Endpoints (11 total)

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/v1/routes` | Plan route (create + stops) |
| `GET` | `/v1/routes` | List (filter: tenant_id, sales_rep_id, date) |
| `GET` | `/v1/routes/{id}` | Route details |
| `POST` | `/v1/routes/{id}/start` | Mark in_progress |
| `POST` | `/v1/routes/{id}/complete` | Mark completed (all stops closed) |
| `POST` | `/v1/routes/{id}/settle` | Settle with deposit |
| `GET` | `/v1/routes/{id}/stops` | List stops |
| `POST` | `/v1/stops/{id}/visits` | Record collection event |
| `POST` | `/v1/stops/{id}/close` | Mark stop closed |
| `GET` | `/v1/stops/{id}/events` | List events for stop |
| `POST` | `/v1/settlements/{id}/decide` | Supervisor approve/reject |

---

## `POST /v1/routes` — PlanRoute

### Request body
```json
{
  "tenant_id": "11111111-...",
  "sales_rep_id": "44444444-...",
  "route_date": "2026-08-15",
  "customer_ids": ["55555555-...", "66666666-..."],
  "auto_populate": false,
  "metadata": {}
}
```

If `auto_populate=true` and `customer_ids` empty, system pulls customers with outstanding invoices (uses `invoice_lookup.OutstandingByCustomer`).

### Response 201 Created
```json
{
  "data": {
    "id": "route_abc123",
    "tenant_id": "...",
    "sales_rep_id": "...",
    "route_date": "2026-08-15",
    "status": "planned",
    "total_planned_minor": 5000000000,
    "stops": [
      {"id": "stop_001", "customer_id": "...", "sequence": 1, "status": "pending"},
      {"id": "stop_002", "customer_id": "...", "sequence": 2, "status": "pending"}
    ]
  }
}
```

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 409 | `INVALID_STATE` | Route already exists for (tenant, sales_rep, date) |

---

## `POST /v1/stops/{id}/visits` — RecordVisit

Record a collection event at a stop. Updates `stop.actual_collection_minor` via DB trigger.

### Request body
```json
{
  "amount_minor": 1500000,
  "payment_method": "cash",
  "reference": "Receipt #1234",
  "notes": "Partial collection"
}
```

`payment_method` enum: `cash` | `qris` | `transfer` | `cheque`.

### Response 201 Created
```json
{
  "data": {
    "event_id": "evt_xyz789",
    "stop_id": "stop_001",
    "amount_minor": 1500000,
    "payment_method": "cash",
    "collected_at": "2026-08-15T10:30:00Z",
    "stop": {
      "id": "stop_001",
      "status": "visited",
      "actual_collection_minor": 1500000
    }
  }
}
```

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 409 | `INVALID_STATE` | Stop already `closed` or `skipped` |
| 400 | `INVALID_INPUT` | Invalid payment_method or amount <= 0 |

---

## `POST /v1/routes/{id}/settle` — SettleRoute

End-of-day deposit. Computes `discrepancy = settled_amount - expected_total`.

### Request body
```json
{
  "settled_amount_minor": 4800000,
  "notes": "Cash deposit to BCA account #12345"
}
```

### Response 201 Created
```json
{
  "data": {
    "settlement_id": "stl_xyz789",
    "route_id": "route_abc123",
    "expected_amount_minor": 5000000,
    "settled_amount_minor": 4800000,
    "discrepancy_minor": -200000,
    "status": "pending",
    "submitted_at": "2026-08-15T18:00:00Z"
  }
}
```

`status = approved` if `discrepancy == 0` (auto-approve). `status = pending` if `discrepancy != 0` (requires supervisor decision via `/v1/settlements/{id}/decide`).

---

## `POST /v1/settlements/{id}/decide` — ApproveSettlement

Supervisor approves or rejects a pending settlement.

### Request body
```json
{
  "approve": true,
  "notes": "Discrepancy explained — 2 invoices written off"
}
```

### RBAC matrix
| Role | Plan route | Record visit | Settle | Approve |
|---|---|---|---|---|
| `hq_admin` | ✅ | ✅ | ✅ | ✅ |
| `hq_finance` | ❌ | ❌ | ❌ | ✅ |
| `sales_rep` | ✅ (own routes) | ✅ | ✅ | ❌ |
| `auditor` | ❌ (read) | ❌ (read) | ❌ (read) | ❌ (read) |

`sales_rep` role is additionally scoped by RLS policy: only sees routes they own (`sales_rep_id = user_id`).

---

## Invariants (property-tested in Sprint 16)

- `stop.actual_collection_minor == SUM(events.amount_minor)` per stop
- `route.total_collected_minor == SUM(stops.actual_collection_minor)` per route
- Settlement `discrepancy == settled - expected` (exact arithmetic)

---

## Implementation

- `internal/usecase/collection_service.go` — `CollectionService` (7 use case methods)
- `internal/repository/postgres/tx_adapter_collection.go` — tx adapter with RLS + sales_rep scope
- `migrations/000011_collection.up.sql` — schema + 2 DB triggers (auto-update totals)
- `internal/handler/collection.go` — 11 HTTP handlers
- [Sprint 11: Collection & Route Module](../SPRINTS.md#sprint-11--collection--route-module-portfolio-sprint-4--fase-8-partial--2026-08-13)
