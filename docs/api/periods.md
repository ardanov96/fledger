# Periods API

Two-step monthly close workflow with audit-grade snapshots. Prevents a single operator from unilaterally closing a period.

---

## Lifecycle

```
┌────────┐  request   ┌─────────┐  approve   ┌────────┐
│  open  │ ────────► │ closing │ ────────► │ closed │
└────────┘           └─────────┘            └────────┘
     │                    │                     │
     │ reject             │ reject              │ reopen (admin)
     ▼                    ▼                     ▼
   (back to open)     (back to open)         ┌────────┐
                                               │  open  │
                                               └────────┘
```

States: `open` → `closing` (after `RequestClose`) → `closed` (after `ApproveClose`) OR `open` (after `RejectClose`). `closed` → `open` via `ReopenPeriod` (admin only).

---

## Endpoints

| Method | Path | RBAC Action |
|---|---|---|
| `POST` | `/v1/periods/{id}/close-requests` | `period_close:create` |
| `GET` | `/v1/close-requests/{id}` | `period_close:read` |
| `POST` | `/v1/close-requests/{id}/approve` | `period_close:approve` |
| `POST` | `/v1/close-requests/{id}/reject` | `period_close:reject` |
| `POST` | `/v1/periods/{id}/reopen` | `period_close:reopen` (admin only) |
| `GET` | `/v1/periods/{id}/snapshots` | `period_close:read` |

---

## `POST /v1/periods/{id}/close-requests`

Request a period close. Sets period status to `closing`, inserts pending request.

### Request body (optional)
```json
{
  "notes": "End-of-month close"
}
```

### Response 201 Created
```json
{
  "data": {
    "id": "cr_abc123",
    "period_id": "period_xyz",
    "requester_id": "<user-uuid>",
    "status": "pending",
    "requested_at": "2026-08-15T10:00:00Z"
  }
}
```

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 409 | `INVALID_STATE` | Period not in `open` status |
| 409 | `INVALID_STATE` | Duplicate pending request for same period |

---

## `POST /v1/close-requests/{id}/approve`

Approve a pending close request. **Atomically**:
1. Computes trial balance (SUM(debit) == SUM(credit))
2. If balanced: generates per-account snapshots, sets period `closed`
3. If imbalanced: auto-rejects, returns 422 `DOUBLE_ENTRY_VIOLATION`

### Request body (optional)
```json
{
  "notes": "Verified by hq_finance"
}
```

### Response 200 OK (balanced)
```json
{
  "data": {
    "request_id": "cr_abc123",
    "period_id": "period_xyz",
    "status": "approved",
    "trial_balance_ok": true,
    "total_debit_minor": 5000000000,
    "total_credit_minor": 5000000000,
    "imbalance_minor": 0,
    "snapshots_created": 3,
    "decided_at": "2026-08-15T10:05:00Z",
    "decided_by": "<approver-uuid>"
  }
}
```

### Response 422 (imbalanced)
```json
{
  "error": {
    "code": "DOUBLE_ENTRY_VIOLATION",
    "message": "Trial balance check failed",
    "details": {
      "total_debit_minor": 5000000000,
      "total_credit_minor": 4999999999,
      "imbalance_minor": 1
    }
  }
}
```

---

## `POST /v1/close-requests/{id}/reject`

Reject a pending request. Period returns to `open`.

### Request body
```json
{
  "rejection_reason": "Missing invoice INV-2026-005"
}
```

`rejection_reason` is **required**.

---

## `POST /v1/periods/{id}/reopen`

Admin-only. Reopens a closed period. **Snapshots are preserved** for historical comparison.

### Request body
```json
{
  "reason": "Found post-close adjustment needed"
}
```

### Failure modes
| Status | Code | Reason |
|---|---|---|
| 403 | `FORBIDDEN` | Caller lacks `period_close:reopen` permission |
| 409 | `INVALID_STATE` | Period not in `closed` status |

---

## `GET /v1/periods/{id}/snapshots`

Returns all snapshots (per-account frozen balances) for the period, sorted by `account_id`.

### Response 200 OK
```json
{
  "data": [
    {
      "id": "snap_001",
      "period_id": "period_xyz",
      "request_id": "cr_abc123",
      "account_id": "11111111-...",
      "balance_minor": 1500000000,
      "currency": "IDR",
      "entry_count": 12,
      "snapshot_at": "2026-08-15T10:05:00Z"
    }
  ]
}
```

---

## RBAC matrix (from `internal/auth/rbac/policies/rbac_policy.csv`)

| Role | create | read | approve | reject | reopen |
|---|---|---|---|---|---|
| `hq_admin` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `hq_finance` | ✅ | ✅ | ✅ | ✅ | ❌ |
| `auditor` | ❌ | ✅ | ❌ | ❌ | ❌ |
| `outlet_manager` | ❌ | ✅ | ❌ | ❌ | ❌ |
| `sales_rep` | ❌ | ❌ | ❌ | ❌ | ❌ |

---

## Implementation

- `internal/usecase/period_service.go` — `PeriodService` (RequestClose, ApproveClose, RejectClose, Reopen)
- `migrations/000008_period_close.up.sql` — DB triggers `no_post_to_closed_period` + `no_entry_in_closed_period`
- `migrations/000009_period_approval.up.sql` — schema for `period_close_requests` + `period_snapshots`
- `internal/handler/periods.go` — 6 HTTP handlers
- [Sprint 9: Period Close with Approval](../SPRINTS.md#sprint-9--period-close-with-approval-workflow-fase-1a--2026-08-11)
