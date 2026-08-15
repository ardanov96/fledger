# Audit API

Tamper-evident append-only audit log. Records every privileged operation (freeze/close account, period approval, settlement decisions, etc.) with full context.

---

## `GET /v1/audit?limit=50&offset=0`

List audit entries for the caller's tenant, most recent first. Tenant-scoped via RLS — cannot see other tenants' audit logs.

### Required headers
- `Authorization: Bearer <access_token>` (RBAC: `audit_log:read`)

### Query parameters
| Param | Type | Default | Notes |
|---|---|---|---|
| `limit` | int | 50 | Max 500 |
| `offset` | int | 0 | Pagination offset |

### Response 200 OK
```json
{
  "data": [
    {
      "id": "audit_abc123",
      "tenant_id": "11111111-...",
      "actor_id": "user_xyz789",
      "action": "account.freeze",
      "resource_type": "account",
      "resource_id": "acc_def456",
      "method": "POST",
      "path": "/v1/accounts/acc_def456/freeze",
      "status_code": 200,
      "request_id": "01HXYZ...",
      "trace_id": "0af7651916cd43dd8448eb211c80319c",
      "metadata": {
        "reason": "Suspicious activity"
      },
      "ip_address": "203.0.113.42",
      "user_agent": "Mozilla/5.0 ...",
      "occurred_at": "2026-08-15T10:30:00Z"
    }
  ]
}
```

### Audit actions recorded

| Action | Trigger |
|---|---|
| `account.create` | `POST /v1/accounts` |
| `account.freeze` | `POST /v1/accounts/:id/freeze` |
| `account.close` | `POST /v1/accounts/:id/close` |
| `transfer.create` | `POST /v1/transfers` |
| `invoice.create` | `POST /v1/invoices` |
| `invoice.payment` | `POST /v1/customers/:id/payments` |
| `period.request_close` | `POST /v1/periods/:id/close-requests` |
| `period.approve` | `POST /v1/close-requests/:id/approve` |
| `period.reject` | `POST /v1/close-requests/:id/reject` |
| `period.reopen` | `POST /v1/periods/:id/reopen` |
| `reconciler.run` | `POST /v1/reconciler/run` |
| `currency.create` | `POST /v1/currencies` |
| `currency.update` | `PATCH /v1/currencies/:code` |
| `fx_rate.create` | `POST /v1/fx-rates` |
| `route.plan` | `POST /v1/routes` |
| `route.settle` | `POST /v1/routes/:id/settle` |
| `route.approve_settlement` | `POST /v1/settlements/:id/decide` |
| `auth.login_success` | `POST /v1/auth/login` (success path) |
| `auth.login_failure` | `POST /v1/auth/login` (wrong credentials) |
| `auth.token_refresh` | `POST /v1/auth/refresh` |
| `auth.token_reuse_detected` | `POST /v1/auth/refresh` (reuse) |
| `auth.logout` | `POST /v1/auth/logout` |

---

## Tamper evidence

`audit_logs` table has **no UPDATE or DELETE permission** granted to the API user role. Combined with Postgres immutability triggers (similar to `ledger_entries`), audit entries cannot be modified post-creation.

Database-level protection:
```sql
REVOKE UPDATE, DELETE ON audit_logs FROM fmcg;
-- Plus trigger that raises exception on any UPDATE/DELETE attempt
```

---

## Compliance use cases

| Regulation | Use |
|---|---|
| **SOX** | Immutable audit trail for financial transactions |
| **GDPR** | Right-to-erasure via `metadata.deletion_marker` (planned) |
| **PCI-DSS** | Who-when-what traceability for access to financial data |
| **Internal audit** | Compliance officer reviews quarterly |

---

## Implementation

- `internal/middleware/audit.go` — `AuditMiddleware` (smart action derivation)
- `internal/middleware/audit_test.go` — 8 tests
- `internal/domain/audit/audit.go` — `Entry` struct + `Repository` interface
- `internal/handler/audit.go` — `AuditHandlers.ListAudit` + `FreezeAccountHandler` + `CloseAccountHandler`
- `migrations/000005_create_audit_logs.up.sql` — schema with 4 indexes + 3 CHECK constraints
- `internal/repository/postgres/audit_repo.go` — Postgres-backed persistence (Sprint 8)

---

## Related Documentation

- [API Overview](overview.md) — RBAC, error envelope
- [Sprint 5: ADR-0004 + Audit Log + Middleware](../SPRINTS.md#sprint-5--adr-0004--audit-log--middleware--2026-08-10)
