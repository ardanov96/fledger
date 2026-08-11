# ADR-0003: Double-Entry Ledger with Immutable Entries

**Status:** Accepted
**Date:** 2026-08-10
**Deciders:** Project lead
**Tags:** accounting, ledger, immutability

## Context

The system handles money movement between accounts. We must be able to:

1. **Prove correctness** to an external auditor (Fase 1).
2. **Reconstruct** the balance of any account at any point in time.
3. **Detect tampering** (Fase 1C — hash chain).
4. **Reverse transactions** without losing history.
5. **Pass an interview** about financial system design.

The naive approach — a single `balance` column updated on each transaction — fails all of the above.

## Decision

**Adopt double-entry bookkeeping with immutable ledger entries.**

### Core Invariants

1. **Every transaction produces at least 2 entries** (one debit, one credit).
2. **For every transaction: SUM(debit) == SUM(credit)** (the double-entry invariant).
3. **Entries are never updated or deleted.** Corrections are reversal entries.
4. **Account balance is computed as the sum of its signed entries** (authoritative).
5. **`cached_balance` on `accounts` is a denormalization** for performance.

## Rationale

### Why Double-Entry?

This is the universal accounting model. Every accounting system, every bank, every regulated financial system uses it. Reasons:

- **Provability**: The sum of all debits equals the sum of all credits, globally. An auditor can verify with a single SQL query.
- **Reconstruction**: Any account's history is the sequence of entries that touched it. No "before/after" states scattered in logs.
- **Reversal**: A refund is just an opposite-sign entry referencing the original. No `UPDATE` required.
- **Domain alignment**: Payment, invoice settlement, write-off — all of these have natural debit/credit structures. Single-entry would require us to invent ad-hoc conventions.

### Why Immutable Entries?

| Editable entries | Immutable entries |
|---|---|
| History is unreliable | History is a fact |
| Audit trail can be altered | Audit trail is provable |
| Corrections lose context | Corrections preserve context |
| Compliance nightmare | Compliance-friendly |

The cost is a slightly more complex correction flow (insert reversal instead of update), but that cost is borne once and the benefit is permanent.

### Why a `cached_balance` Column?

Computing balance as `SUM(entries.signed_amount)` is O(n) over the entry history. For an account with 10 years of monthly transactions, that's tens of thousands of rows. For a dashboard that lists 100 accounts, that's 1M+ rows per page load.

The cache is maintained in the same transaction that writes the entries, so it's always consistent at commit time. The reconciler (Fase 1) verifies `cached == authoritative` periodically.

## Schema Sketch

```sql
CREATE TABLE accounts (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code          TEXT NOT NULL UNIQUE,          -- "HQ-001", "OUTLET-JKT-12"
  name          TEXT NOT NULL,
  type          TEXT NOT NULL,                 -- 'hq' | 'outlet' | 'sales_rep' | ...
  status        TEXT NOT NULL DEFAULT 'active',
  currency      TEXT NOT NULL DEFAULT 'IDR',
  cached_balance BIGINT NOT NULL DEFAULT 0,    -- minor units (sen)
  owner_id      UUID,
  tenant_id     UUID NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE transactions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  idempotency_key TEXT NOT NULL,
  status          TEXT NOT NULL,               -- 'pending' | 'posted' | 'failed' | 'reversed'
  description     TEXT,
  ref_type        TEXT,
  ref_id          UUID,
  initiator_id    UUID,
  tenant_id       UUID NOT NULL,
  period_id       UUID NOT NULL,
  posted_at       TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, idempotency_key)
);

CREATE TABLE entries (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_id  UUID NOT NULL REFERENCES transactions(id),
  account_id      UUID NOT NULL REFERENCES accounts(id),
  amount          BIGINT NOT NULL CHECK (amount > 0),  -- always positive
  type            TEXT NOT NULL CHECK (type IN ('debit', 'credit')),
  ref_type        TEXT,
  ref_id          UUID,
  period_id       UUID NOT NULL,
  description     TEXT,
  currency        TEXT NOT NULL,
  metadata        JSONB NOT NULL DEFAULT '{}',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Entries are immutable: trigger prevents UPDATE/DELETE
CREATE FUNCTION prevent_entry_modification() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'ledger entries are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER no_entry_update BEFORE UPDATE ON entries
  FOR EACH ROW EXECUTE FUNCTION prevent_entry_modification();
CREATE TRIGGER no_entry_delete BEFORE DELETE ON entries
  FOR EACH ROW EXECUTE FUNCTION prevent_entry_modification();
```

## Consequences

### Positive
- **Provable correctness** — trial balance reconciler (Fase 1) catches any violation.
- **Audit-friendly** — history is immutable, every change is recorded.
- **Interview-defensible** — this is the textbook financial architecture.
- **Tamper-evident** — Fase 1C hash chain builds on this immutability.

### Negative
- **Schema is more complex** than a single `balance` column.
- **Reversals require more rows** — a $100 reversal is a new entry, not an update.
- **Cache must be kept in sync** — bug if not updated atomically.
- **Reporting requires understanding accounting concepts** — debit/credit.

## Implementation Notes

- The `internal/platform/money` package handles the arithmetic; it never uses `float64`.
- All updates to `accounts.cached_balance` happen within the same DB transaction as the entry inserts. Never update cached balance outside a transaction.
- The `entries.amount` column is always positive; the `type` column conveys direction. This is the standard accounting convention.
- Reversal entries reference the original via `metadata.original_entry_id` (JSONB) — keeping a structured link without breaking the schema.

## Follow-ups

- ADR-0004: Why SELECT FOR UPDATE for balance updates
- ADR-0010: Why int64 minor units for money
- ADR-0011: Why hash chain for tamper detection (Fase 1C)
- ADR-0012: Why period close / accounting cycle (Fase 1A)

## References

- [Double-entry bookkeeping (Wikipedia)](https://en.wikipedia.org/wiki/Double-entry_bookkeeping)
- [Designing Data-Intensive Applications — Ch. 7: Transactions](https://dataintensive.net/)
- [Accounting for Non-Accountants (YouTube)](https://www.youtube.com/results?search_query=double+entry+bookkeeping+explained)
- [Indonesian SAK EMKM](https://www.iaiglobal.or.id/) (Standar Akuntansi Keuangan Entitas Mikro, Kecil, dan Menengah)
