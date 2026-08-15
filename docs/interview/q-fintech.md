# Fintech Q&A — FMCG Wallet

5 anticipated pertanyaan interview tentang domain fintech dan jawaban berbasis codebase kami.

---

## Q1: Why double-entry accounting vs single-entry?

**Answer:**

Double-entry adalah gold standard untuk financial systems karena **self-balancing property**: total debit harus selalu sama dengan total credit. Ini memberikan beberapa advantage yang single-entry tidak punya:

1. **Built-in error detection** — kalau debit ≠ credit, ada yang salah. Single-entry bisa silently corrupt tanpa terdeteksi.
2. **Audit trail natural** — setiap transaksi tahu persis "dari mana" (debit) dan "ke mana" (credit). Single-entry perlu field tambahan untuk tracking.
3. **Per-account balance derivation** — bisa derive saldo dari sum(debit) - sum(credit), tidak perlu maintain balance terpisah (yang bisa drift).
4. **Compliance** — SOX, IFRS, GAAP semuanya require double-entry untuk financial reporting.

**Di codebase kami:**
- `internal/domain/ledger/service.go` `Transfer()` orchestrating double-entry dalam 1 atomic tx.
- Setiap transfer create **2 entries** (1 debit + 1 credit) di `ledger_entries` table.
- DB trigger `prevent_entry_modification()` ensures entries **immutable** post-creation (no UPDATE/DELETE).
- Reconciler checks `SUM(debit) == SUM(credit)` per period (trial balance).

**Trade-off acknowledged:**
- Double-entry lebih banyak writes (2 rows per transaction vs 1).
- Tapi di era SSDs dan Postgres partitioning, ini negligible.

**For FMCG Wallet specifically:**
- 270k+ distributors di Indonesia, average 10-50 transfers/distributor/day = ~2.7M-13.5M transfers/day nationally.
- Double-entry cost: ~2x write amplification = ~5-27M entries/day. Postgres handles this trivially (millions of rows per day is normal).

---

## Q2: How does hash chain prevent tampering?

**Answer:**

Hash chain adalah cryptographic log integrity technique yang dipakai di Bitcoin, Certificate Transparency, dan banyak audit log systems. Prinsipnya:

```
entry_n.entry_hash = SHA256(entry_n.payload || entry_n.prev_hash)
```

Jadi **setiap entry mengandung hash entry sebelumnya**. Modify 1 entry → entry_hash changes → next entry's prev_hash doesn't match → entire chain breaks.

**Di codebase kami:**
- `internal/domain/ledger/hasher.go` implements SHA-256 with canonical pipe-separated encoding (avoid delimiter collision in payload).
- `internal/usecase/hashchain_verifier.go` `Verifier.Verify()` walks entries per account, recomputes hashes, compares `prev_hash + entry_hash`.
- `migrations/000007_add_hash_chain.up.sql` adds `prev_hash` + `entry_hash` columns.
- Reconciler `RunReconciliation(ctx, period_id, run_hash_check=true)` calls verifier and reports `hash_chain_errors`.

**Defense layers:**
1. **DB trigger** `prevent_entry_modification()` — already blocks UPDATE/DELETE at schema level.
2. **Hash chain** — if attacker bypasses trigger (e.g., super_admin direct DB access), the chain breaks.
3. **Reconciler** — detects break within 1 hour (background ticker) atau instant (manual API call).

**Trade-off:**
- Storage: 64 bytes extra per entry (SHA-256 hex). For 100M entries = 6.4GB extra. Trivial.
- Compute: O(N) hash per verification. For period with 1M entries, ~1 second on modern CPU. Acceptable.

**What's NOT protected by hash chain:**
- Entry deletion (whole chain shifts). DB trigger blocks this.
- Replay attacks (re-submit old transaction). Handled by Idempotency-Key.

---

## Q3: How do you handle FX rate volatility?

**Answer:**

Two-layer strategy: **rate snapshot per transaction** (audit-grade) + **TTL-based rate validity** (operational).

### Layer 1: Rate snapshot per transaction

When cross-currency transfer happens:
1. System looks up **latest active FX rate** (where `now() BETWEEN effective_at AND expires_at`).
2. Computes converted amount using `money.Convert(amount, fromDP, toDP, rate)` with half-up rounding.
3. Writes **2 asymmetric entries** (debit in source currency, credit in destination currency).
4. Snapshots `fx_rate_id` + `fx_rate_locked_at` on the transaction header.

**Why snapshot, not dynamic at read-time?**
- **Audit requirement:** historical reports must show the exact rate AT the time of transaction.
- **Regulatory compliance:** bank reconciliation requires reproducible FX calculations.
- **Tamper detection:** if rate changes after transaction, the snapshot stays put.

**Example:** Transaction from August 15 used rate 15,750. Audit query on October 1 must show 15,750, NOT current rate 15,800.

### Layer 2: TTL-based rate validity

FX rates have `effective_at` + `expires_at` columns. Admin updates rates via `POST /v1/fx-rates` periodically.

- Default TTL: 30 days (configurable).
- New transfer uses rate valid at `now()` (between effective and expires).
- Expired rate → `ErrFxRateNotFound`.

### Trade-offs acknowledged

- **Rate staleness:** admin must update rates regularly. Could be automated via FX provider API (Sprint 13+).
- **Cross-currency double-entry invariant:** sum(debit) ≠ sum(credit) in absolute terms (different currencies). Invariant holds **per-currency**: sum(USD debit) == sum(USD credit); sum(IDR credit) == sum(IDR debit) at the converted rate.

---

## Q4: Reconciliation — manual vs automated?

**Answer:**

**Both**, with different SLAs:

### Automated (background worker)
- Ticker-based: runs every **1 hour** (configurable).
- Iterates all `open` + `closing` periods across all tenants.
- Default config: `RECONCILER_HASH_CHECK=false` (cheap, no hash chain walk).
- Output: `reconciler_runs` table with status (`balanced` / `imbalanced` / `tampered`).

```go
// internal/worker/reconciler_worker.go
ticker := time.NewTicker(1 * time.Hour)
for range ticker.C {
    reconcilerService.RunAllForAllTenants(ctx, runHashCheck=false)
}
```

### Manual (on-demand API)
- `POST /v1/reconciler/run` for specific `(tenant_id, period_id)`.
- `run_hash_check=true` flag for full hash chain walk.
- Used by:
  - Operator before closing a period (verify before snapshot generation).
  - After detecting suspicious activity (forensic).
  - After bulk data fix.

### Trigger sequence for production

```
event: PeriodCloseRequested (Sprint 9)
  → human operator runs manual reconciliation with hash_check=true
    → if balanced: approve close → snapshots generated
  → if imbalanced/tampered: reject close, investigate

background: hourly auto-reconciliation (no hash_check)
  → if imbalanced detected: alert operator (Slack integration planned)
  → if tampered detected: page on-call (Sev 1 incident)
```

### Trade-offs

- **Background worker:** great for early warning, but only checks `open`/`closing` periods (not closed historical). That's deliberate — closed periods should already be reconciled before close.
- **Manual run:** needed for investigation, audit prep, period-close gate.

---

## Q5: Idempotency-Key TTL — how long is safe?

**Answer:**

Current implementation: **24 hours** (in `internal/domain/auth/auth.go` `CreateRefreshToken` cleanup logic).

### Why 24 hours?

- **Long enough:** most client retries happen within seconds-to-minutes (network blip, browser refresh, mobile app backgrounded).
- **Short enough:** bounded storage — table doesn't grow unbounded.
- **Standard practice:** Stripe uses 24h, AWS uses 24h, GitHub uses 1 hour.

### What if key is reused after TTL?

- After 24h, the row is cleaned up (separate cron, planned).
- Client retry with same key after cleanup → behaves like fresh request → could create duplicate!

**Solution:** Client MUST combine Idempotency-Key with sufficient entropy (UUID v4 or similar). We don't trust the TTL alone — it's defense-in-depth.

### Better approach (future)

Client-side **deterministic key derivation** (e.g., `hash(operation_type + request_body)`) ensures same operation = same key, but different operation = different key. Then TTL can be longer (7 days) safely.

### Trade-offs

| Approach | Storage | Safety | Client complexity |
|---|---|---|---|
| 24h TTL + UUID key | Bounded | Good | Low (just UUID) |
| 7d TTL + UUID key | Larger | Good | Low |
| Deterministic key + long TTL | Bounded | Excellent | High (client must implement hash) |

We picked 24h + UUID for simplicity. Production fintech at scale may pick deterministic + long TTL.

---

## Reference

- [Transfers API](../api/transfers.md) — Idempotency-Key contract
- [Reconciler API](../api/reconciler.md) — Manual + automated reconciliation
- [Currencies API](../api/currencies.md) — FX rate snapshot semantics
- [Architecture: Sequences — Cross-currency transfer](../architecture/sequences.md#5-cross-currency-transfer-with-fx-snapshot)
- [Sprint 2: Schema + Repositories + Transfer Use Case](../SPRINTS.md#sprint-2--schema--repositories--transfer-use-case--2026-08-10)
- [Sprint 12: Multi-Currency (Fase 1D)](../SPRINTS.md#sprint-12--multi-currency-fase-1d--2026-08-14)
- [ADR-0003: Double-entry ledger](../adr/0003-double-entry-ledger.md)
- [ADR-0005: Multi-currency strategy](../adr/0005-multi-currency-strategy.md)
