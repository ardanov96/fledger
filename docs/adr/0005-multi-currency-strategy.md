# ADR-0005: Multi-Currency Strategy (Rate Snapshot per Transaction)

**Status:** Accepted (Sprint 12 — 2026-08-14)
**Deciders:** Architecture team
**Supersedes:** Partial — extends the Fase 1D stub from `roadmap-production-grade.md` v1.2.

---

## Context

The system stores account balances per `currency` (ISO 4217 code) and the
double-entry ledger guarantees `SUM(debit) == SUM(credit)` per currency scope.
As of Sprint 11 we have:
- `currencies` column implicitly in entries (defaulted to `IDR`).
- DB trigger `currency consistency check` enforces entry.currency matches
  account.currency (in `migrations/000004`).
- Single currency per tenant (IDR-only deployments).

The roadmap asks for **multi-currency support** so a single tenant can
hold accounts in multiple currencies and perform cross-currency transfers.
This unlocks:
- **Importer/Exporter scenarios** — pay USD-denominated supplier from IDR-denominated HQ.
- **Tourism / cross-border F&B** — outlets near borders handle multi-currency cash flow.
- **Future-proofing** — when a partner integrates a multi-currency payment gateway, we're ready.

The key design question: **when do we lock in the FX rate?**

## Options Considered

### Option 1: Single currency per tenant (status quo extension)
- **Pros:** Simple, no rate lookup at transfer time, no historical ambiguity.
- **Cons:** Not multi-currency at all. Can't support USD-denominated supplier
  payments if HQ is IDR-only. Fails the brief.

### Option 2: Multi-currency per account, **rate snapshot per transaction** ✅ (chosen)
- **Pros:**
  - Immutable history: each transaction records the rate that was used.
  - Reports can replay transactions at historical rates.
  - Reconciler can detect retroactive rate manipulation (rate is stored, not computed).
  - Matches how banks record FX trades: trade-time rate locked, not read-time rate.
- **Cons:**
  - One extra column (`fx_rate_id`) + one extra column (`fx_rate_locked_at`)
    on `transactions`. ~16 bytes/row overhead. Negligible.
  - Rate lookup at transfer time adds latency. Mitigated by indexed
    `(from, to, effective_at DESC)` lookup.

### Option 3: Multi-currency per account, dynamic rate at read-time
- **Pros:** No rate storage needed; latest rate always used.
- **Cons:**
  - **Historical ambiguity**: a 2024 transfer "worth" different IDR amount
    depending on when you query. Cannot reconcile historical reports.
  - **Cannot detect retroactive rate manipulation** — operator changes a
    rate today and yesterday's transfers appear different.
  - Fails the audit-grade guarantee that the ledger is intended for.

### Option 4: Composite currency (single balance per tenant, mixed currencies)
- Rejected: violates per-account currency invariant established in Sprint 2.

## Decision

**Option 2 — rate snapshot per transaction.**

Concretely:
- `transactions.fx_rate_id UUID REFERENCES fx_rates(id)` — nullable, set when
  `from_currency != to_currency`. NULL = same-currency transfer.
- `transactions.fx_rate_locked_at TIMESTAMPTZ` — nullable, the timestamp at
  which the rate was looked up. Used for tolerance-window checks (e.g. client
  supplies expected rate + lock time; server validates within 5 min).
- DB trigger `enforce_fx_rate_snapshot` (in `migrations/000012`) blocks
  cross-currency transactions without a valid `fx_rate_id`.
- Application logic (in `TransferService`) looks up the latest active rate for
  `(tenant, from, to)` at transfer time, converts the amount, and persists
  both `fx_rate_id` and `fx_rate_locked_at` in the same transaction.

### Asymmetric entries (chosen sub-decision)

Cross-currency transfers create **two entries with different amounts**:
- Debit entry: amount in source currency, in the source account.
- Credit entry: `convertedAmount = source × rate` in target currency, in the
  destination account.

This is **NOT** a violation of the double-entry invariant. The invariant
`SUM(debit) == SUM(credit)` holds **within a single currency scope**. The
conversion happens explicitly via the `fx_rate` reference, so the cross-pair
relationship is recorded in `transactions.fx_rate_id` (not in the entries).

Alternative sub-option (rejected): convert everything to a common currency
(e.g. USD) at transfer time, store symmetric entries in USD, keep original
amounts in metadata. Rejected because: (a) loses native currency information
needed for statement/aging reports; (b) introduces conversion at every read.

## Implementation Reference

- `migrations/000012_multi_currency.up.sql` — schema + trigger + seed.
- `internal/domain/currency/currency.go` — domain types + Repository interface.
- `internal/repository/postgres/currency_repo.go` — Postgres impl.
- `internal/repository/postgres/tx_adapter_currency.go` — pgx.Tx adapter.
- `internal/usecase/currency_service.go` — business logic (Convert,
  CreateCurrency, CreateFxRate, LookupRateForTransfer).
- `internal/usecase/currency_service_test.go` — 8 unit tests including
  same-currency identity, USD→IDR math, JPY decimal-place shift, expired
  rate handling, validation errors.
- `internal/platform/money/money.go` — added `money.Convert(amount, fromDP,
  toDP, rate)` helper.
- `internal/domain/ledger/transaction.go` — added `FxRateID`, `FxRateLockedAt`
  to Transaction entity.
- `internal/domain/ledger/service.go` — `TransferInput.ExpectedFxRateID`,
  `TransferInput.ExpectedRateLockAt` for client pinning.
- `internal/usecase/transfer_service.go` — cross-currency path with FX rate
  lookup + asymmetric entries + rate snapshot in transaction header.
- `internal/handler/currency.go` — 9 REST endpoints.
- `internal/handler/dto.go` — extended `TransferRequest` + `TransferResponse`
  with cross-currency fields.
- `internal/auth/rbac/rbac.go` + `policies/rbac_policy.csv` — `ObjectCurrency`,
  `ObjectFxRate` policies.
- `cmd/api/currency_adapters.go` — `currencyAPIAdapter` (handler → use case)
  + `fxRateLookupAdapter` (transfer service → currency service).

## Trade-offs

| Decision | Pro | Con |
|---|---|---|
| Rate snapshot per transaction | Immutable history, audit-grade | 16 bytes/row overhead |
| Asymmetric entries (currency per account) | Preserves native currency in reports | Cross-pair reconciliation requires `transactions.fx_rate_id` join |
| DB trigger for rate enforcement | Defense-in-depth, no app-layer bypass | Trigger adds 1 round-trip |
| `NUMERIC(20,10)` for rate | High precision (10 fractional digits) | Slightly more storage than `FLOAT8` |
| TTL via `expires_at` (admin-managed) | Explicit, auditable | Manual refresh burden |
| Source enum (`manual`/`api`/`bank`/`seed`) | Traceability | Adds 1 column |

## Operational Notes

- **Rate sourcing**: Sprint 12 ships **manual + seed** modes only.
  Operator sets rates via `POST /v1/fx-rates`. Sprint 13+ can add an `api`
  source that pulls from a configurable provider (e.g. `exchangerate-api.com`,
  Bank Indonesia reference rate). The enum is already in place.
- **Seed default**: `IDR` is seeded by default in `000012`. USD/JPY/EUR/SGD
  need to be added by admin before cross-currency is usable. The seed
  inserts only IDR to keep schema migration idempotent.
- **Reconciler integration**: Sprint 10 reconciler does not yet consider FX
  rates when computing trial balance per period. Future work: extend
  reconciler to compute per-currency trial balance (so USD balance and IDR
  balance are tracked separately, both summing to zero within their scope).

## References

- `migrations/000004_create_entries.up.sql` — original currency consistency
  trigger (entry.currency must match account.currency).
- `migrations/000007_add_hash_chain.up.sql` — hash chain (Fase 1C) — works
  unchanged across currencies (entry_hash is per-account, not per-currency).
- `docs/SPRINTS.md` — Sprint 12 section (post-completion).
- `internal/usecase/hashchain_verifier.go` — verifier is currency-agnostic.
