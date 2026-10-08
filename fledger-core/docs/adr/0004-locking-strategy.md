# ADR-0004: SELECT FOR UPDATE with Deterministic Lock Ordering

**Status:** Accepted
**Date:** 2026-08-10
**Deciders:** Project lead
**Tags:** concurrency, database, locking

## Context

The TransferService performs a double-entry transfer between two accounts. In
production, the same account can be touched by many concurrent transfers
(e.g. multiple sales reps all transferring to/from the same HQ treasury
account). We need to ensure:

1. **Correctness:** Two concurrent transfers from the same source account
   must each see the correct post-transfer balance. Without proper locking,
   the classic "lost update" problem can occur.
2. **No deadlocks:** With multiple accounts involved, naive locking can
   deadlock when two transactions lock the same pair of accounts in
   different orders.
3. **Acceptable performance:** Locking should not serialize the entire
   system; only the rows that are actually being modified.

## Options Considered

| Option | Description | Pros | Cons |
|---|---|---|---|
| **Optimistic locking** (version column, retry on conflict) | Each row has a `version` int. UPDATE WHERE id = ? AND version = ?. Retry on conflict. | Lock-free, great for low-contention. | Bad for high-contention (HQ account): lots of retries. |
| **Application-level lock** (Redis SETNX) | Acquire global key per account before DB transaction. | Works across DBs. | Extra moving part (Redis); extra round-trip; consistency issues if Redis dies. |
| **Pessimistic row lock** (`SELECT ... FOR UPDATE`) | Lock rows inside the transaction. Release on commit. | Simple, correct, well-understood. | Requires careful ordering to avoid deadlocks. |
| **Serializable isolation** | Let Postgres handle conflicts via SSI. | Automatic. | Many false-positive serialization failures, hard to reason about. |

## Decision

**Use `SELECT ... FOR UPDATE` with deterministic lock ordering (sort by ID).**

### Algorithm

Inside a single DB transaction:

1. **Sort the two account IDs** lexicographically. Call them `firstID`
   and `secondID`. This gives a total order on the pair.
2. **Lock `firstID` first** with `SELECT ... FOR UPDATE`.
3. **Lock `secondID` second** with `SELECT ... FOR UPDATE`.
4. Re-validate state under lock (currency, status, balance).
5. Insert transaction + entries + update balances.
6. Commit.

Because both concurrent transactions touching the same pair of accounts
will compute the same lock order, they cannot deadlock against each other.
The first one to acquire `firstID` blocks the second one until commit,
serializing access to that account in a deterministic way.

### Why this is correct

- **No deadlocks:** For any two transactions T1 and T2 that touch the
  same pair of accounts, both will lock the accounts in the same order.
  If T1 holds the lock and T2 is waiting, T1 will eventually release on
  commit. No circular wait → no deadlock.
- **No lost updates:** `SELECT FOR UPDATE` is a row-level lock, not a table
  lock. Other transactions touching different accounts proceed in
  parallel. Only the specific rows involved are serialized.
- **Cheap:** Postgres already implements MVCC and row-level locking
  efficiently. No additional infrastructure (no Redis, no version
  column overhead per write).

### Why not optimistic locking?

For the HQ treasury account, which is the target of many concurrent
transfers, optimistic locking would mean most transfers fail on the first
attempt and need to retry — wasting CPU and increasing latency.
Pessimistic locking with row-level locks is cheaper for this access pattern.

### Why not Serializable?

Postgres SSI (Serializable Snapshot Isolation) detects conflicts at commit
time and forces one transaction to retry. With a busy HQ account, this
causes the same retry storm problem as optimistic locking. Explicit row
locks with deterministic ordering are easier to reason about and debug.

## Implementation

The lock pattern lives in the `TransferService.Transfer` method, inside
`internal/usecase/transfer_service.go`:

```go
err = s.db.executeTx(ctx, func(tx ledger.Tx) error {
    // 4a. Lock both accounts, in a deterministic order to avoid deadlocks.
    firstID, secondID := input.FromAccountID, input.ToAccountID
    if firstID > secondID {
        firstID, secondID = secondID, firstID
    }

    first, err := s.accounts.LockForUpdate(ctx, tx, firstID)
    if err != nil {
        return fmt.Errorf("lock first account: %w", err)
    }
    second, err := s.accounts.LockForUpdate(ctx, tx, secondID)
    if err != nil {
        return fmt.Errorf("lock second account: %w", err)
    }
    // ... rest of transaction
})
```

The `LockAccountForUpdate` query is in
`internal/repository/postgres/queries/accounts.sql`:

```sql
-- name: LockAccountForUpdate :one
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE id = $1
FOR UPDATE;
```

## Consequences

### Positive

- **Provable correctness:** the lock-ordering proof is straightforward and
  easy to defend in an interview.
- **Cheap on hot accounts:** only the involved rows are locked.
- **No additional infrastructure:** uses built-in Postgres locking.

### Negative

- **Application responsibility:** the use case must remember to always
  use the ordered pattern. A code reviewer or static analysis is needed
  to catch mistakes.
- **Deadlock risk if violated:** if a future developer adds a new
  use case that locks accounts in a different order, deadlocks may appear
  in production. Mitigated by:
  - Centralized locking helper (`internal/infra/locking.go` or method
    on the AccountRepository)
  - Code review checklist
  - Load test that exercises concurrent transfers to different account
    pairs

### Follow-ups

- [x] Add a centralized `LockPairForUpdate(ctx, tx, idA, idB)` helper
      that always does the ordering *(implemented in `AccountRepository`;
      used by `TransferService.Transfer`)*
- [x] Add a load test (`tests/load/concurrent_transfer.go`) that verifies
      no deadlocks under N concurrent transfers to M accounts
      *(implemented as `TestConcurrent_NoDeadlocks_100x50` —
      100 goroutine × 50 iterasi ke 10 akun, PASS dengan `-race` di CI Linux;
      plus 4 test concurrent lain di `transfer_concurrent_test.go`)*
- [ ] Add a Postgres advisory lock at the top of each transaction as a
      belt-and-suspenders measure (optional, if real deadlocks are observed)
      *(belum; tidak perlu sampai ada indikasi deadlock di production)*
<task_progress>- [x] Verifikasi bukti repo (migrations, internal code, ADR, tests, CI)
- [x] Update SPRINTS.md (tambah Sprint 7, sinkronkan test count & roadmap status)
- [x] Update roadmap-production-grade.md (Fase 0,1,2,3,4 partial,5 partial,6 partial,9 partial — semua ditick sesuai bukti)
- [x] Update ADR-0004 (tick 2 follow-up yang sudah selesai)
- [ ] Update roadmap-fmcg-wallet-portfolio.md (Sprint 1-6 → done)
- [ ] Update tech-stack-fmcg-wallet-portfolio.md (stack additions)

## References

- [PostgreSQL docs: Explicit Locking](https://www.postgresql.org/docs/current/explicit-locking.html)
- [PostgreSQL docs: Transaction Isolation](https://www.postgresql.org/docs/current/transaction-iso.html)
- [Use The Index, Luke: Locking and Concurrency](https://use-the-index-luke.com/sql/transaction-iso)
- [Designing Data-Intensive Applications, Ch. 7 (Transactions)](https://dataintensive.net/)
