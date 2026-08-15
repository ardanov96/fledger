# Distributed Systems Q&A — FMCG Wallet

5 anticipated pertanyaan interview tentang distributed systems dan jawaban berbasis codebase kami.

---

## Q1: Why UUID-sorted lock ordering?

**Answer:**

**Problem:** classic deadlock scenario — 2 transactions lock accounts in opposite order:
```
TX1: lock account A → wait for account B
TX2: lock account B → wait for account A
   ↑ DEADLOCK
```

**Solution:** enforce **global total order** on lock acquisition. Sort account IDs (UUIDs) lexicographically, then always lock in sorted order. Two transactions touching the same pair MUST lock in the same order, so one waits for the other (serial), but no circular wait.

**Implementation in our codebase:**
```go
// internal/usecase/transfer_service.go
func (s *TransferService) Transfer(ctx, input) (Transaction, error) {
    // Sort account IDs to ensure deterministic lock order
    firstID, secondID := input.FromAccountID, input.ToAccountID
    if firstID > secondID {
        firstID, secondID = secondID, firstID
    }
    
    // Lock in sorted order
    first := s.accounts.LockForUpdate(ctx, tx, firstID)
    second := s.accounts.LockForUpdate(ctx, tx, secondID)
    
    // ... rest of transfer logic
}
```

**Why UUIDs instead of integers?**
- UUIDs are globally unique (no coordination needed across distributed systems).
- Integers need a central ID allocator — single point of failure.
- Lexicographic ordering on UUIDs is well-defined and stable.

**Tested by** `TestConcurrent_NoDeadlocks_100x50`:
- 100 goroutines × 50 iterations × 10 accounts = 5,000 concurrent transfers
- Assertion: zero deadlocks
- Result: PASS in <3 seconds

**Trade-offs acknowledged:**
- One TX always waits for the other (serial). Throughput limit on a single pair.
- For high-throughput scenarios, use **queue per account** (decouple read/write via event sourcing) — Sprint 18+ deferred work.
- For our scale (10-50 transfers/distributor/day), serial execution is fine.

---

## Q2: RLS vs application-layer tenant filter — why both?

**Answer:**

**Defense-in-depth principle:** never rely on a single layer for security. If one layer fails (bug, misconfig, malicious admin), the other catches it.

### Application-layer filter

```go
// In handler
func (h *Handlers) ListAccounts(r *http.Request) {
    tenantID := middleware.TenantFromContext(r.Context()).TenantID
    return h.repo.List(r.Context(), ledger.AccountFilter{TenantID: tenantID})
}
```

**Pros:** explicit, easy to audit in code, fast (no DB-level overhead).
**Cons:** ONE bug in ONE handler = full tenant breach. Easy to forget on new endpoint.

### DB-level RLS

```sql
-- migration 000014
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON accounts
  USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
```

**Pros:** cannot be bypassed — even raw SQL queries from super_admin are filtered (unless `app_admin` role).
**Cons:** performance overhead (every query now filters by GUC), harder to debug (filter happens implicitly).

### Why both?

| Failure mode | Application-layer filter | DB RLS |
|---|---|---|
| New endpoint forgets to filter | ❌ breach | ✅ blocked |
| Attacker has DB credentials, queries directly | ❌ breach | ✅ blocked |
| GUC var not set (config bug) | ✅ filtered (if app sets) | ⚠️ NULL cast fail (fails loud) |
| Migration applied, app deployed later (out-of-order) | ✅ filtered | ❌ app.current_tenant_id = NULL → deny |

### Wiring

`tenantctx.SetTenantContext(ctx, tx, *Info)` is called at the start of every `RunInTx*Domain` closure (7 adapters: ledger/invoice/period/reconciler/collection/currency/auth). The Info is derived from JWT `tenant_id` claim by `TenantContextMiddleware`.

### Trade-offs

- **Performance:** RLS adds ~5% overhead (per query must evaluate policy). Acceptable for our scale.
- **Operational complexity:** 2 layers to keep in sync. Mitigated by centralized `tx_adapter` pattern.
- **Open follow-up (Sprint 18+):** `app_admin` Postgres role for genuine system administration (bypass RLS). Currently no admin tooling.

---

## Q3: How do you handle clock skew for distributed systems?

**Answer:**

**Short answer:** we're not really distributed YET — single Fly.io app, single Postgres. So clock skew is bounded by NTP sync within a datacenter (~1ms).

**Long answer (when we go multi-region):**

### 1. TOTP already handles clock skew
- TOTP RFC 6238 allows ±1 step (default 30s window). User's phone clock can be 30s off from server's clock and still validate.
- Implementation: `TOTPGenerator` `Verify(code, secret, time.Now(), skew=1)`.

### 2. JWT exp can drift
- `exp` claim is validated with 60s grace period (standard JWT practice).
- Client refresh tokens can have clock issues → handled by reuse-detection (Sprint 13).

### 3. Reconciler timestamps can drift
- Reconciler `started_at` / `finished_at` use Postgres `NOW()` (server clock).
- For multi-region: **logical clock** (Lamport timestamp or vector clock) needed. Deferred to multi-region sprint.

### 4. fx_rate_locked_at
- Set from `time.Now()` in Go (host clock).
- For multi-region: use Postgres `statement_timestamp()` instead — same clock as audit log + ledger entries.

### NTP enforcement (production)
- Fly.io VMs sync to NTP automatically (host-level).
- Docker image includes `tzdata` for timezone correctness.
- All times stored as `TIMESTAMP WITH TIME ZONE` (UTC) — never local time.

### Trade-off

- Current implementation assumes **single time source** (Postgres NOW() + Go time.Now() both come from host clock).
- For multi-region: need to either (a) centralize time via Google NTP Public, (b) use HLC, or (c) accept eventual consistency on timestamps.

---

## Q4: What's your consistency model — strong or eventual?

**Answer:**

**Strong consistency** for financial data (transfers, invoices, reconciler state). **Eventual consistency** is acceptable for secondary features (audit log, rate snapshots).

### Strong consistency scope

| Data | Why strong | How |
|---|---|---|
| `transactions` + `ledger_entries` | Money must be exact | `BEGIN/COMMIT` in 1 tx |
| `account.cached_balance` | No drift allowed | Updated in same tx as transfer |
| `reconciler_runs.status` | Operator needs immediate truth | tx-scoped update |
| `period_close_requests` | Two-step approval workflow | Row-level locks |

All writes go through `tx.RunInTx*Domain()` which uses `BEGIN ISOLATION LEVEL READ COMMITTED` (Postgres default).

### Eventual consistency scope

| Data | Why eventual | How |
|---|---|---|
| `audit_logs` | Append-only, can lag behind by milliseconds | Inserted by `AuditMiddleware` (async via background queue planned) |
| `reconciler_runs` (background) | Hourly scan, no real-time requirement | Ticker worker |
| FX rate snapshot on tx | Rate can be slightly stale (within TTL) | Read at transfer time |

### Why hybrid?

Financial data MUST be strongly consistent — money lost = trust lost forever. Operational metadata (audit, monitoring) can be eventually consistent — eventual is fine if lag is bounded.

### CAP theorem trade-off

- **CP (consistency + partition tolerance):** chosen for financial writes. During partition, writes may fail rather than serve stale data.
- **AP (availability + partition tolerance):** chosen for reads. Cached balances may be slightly stale (refresh every N seconds).

**Example:**
- During Postgres failover, transfers may fail with 503 (consistent but unavailable).
- Read endpoint always serves latest committed state (consistent, eventually available).

---

## Q5: How would you scale to 1M req/s?

**Answer:**

**Current state:** single Fly.io app, shared-cpu-1x VM, ~100 req/s capacity.

**Path to 1M req/s = 10,000x scale.** Requires multiple phases:

### Phase 1: Vertical scale (1 → 10x)
- Upgrade VM to performance-1x or performance-2x (more CPU + memory).
- Increase Postgres connection pool size.
- Add read replicas for `/metrics` + `/healthz` + `/readyz`.
- **Target:** ~1,000 req/s.

### Phase 2: Horizontal app scale (10 → 100x)
- Multiple Fly.io machines behind load balancer.
- Postgres still single-writer, multi-reader (1 primary + 5 read replicas).
- Fly machines count = `desired_capacity / per_machine_capacity`.
- **Target:** ~10,000 req/s.

### Phase 3: Sharding (100 → 10,000x)
- **Shard by tenant_id** (most natural for multi-tenant SaaS).
- Each shard: independent Postgres instance + Fly app pool.
- Routing: `tenant_id % N_shards` → shard index.
- Trade-off: cross-tenant queries become hard (e.g., global audit log).

### Phase 4: Event-driven (specialized paths)
- Read-heavy paths (audit log, reconciler runs): use **Elasticsearch / ClickHouse** instead of Postgres.
- Write paths: still Postgres (single source of truth).
- CDC (Debezium) → Kafka → read replicas.

### Architectural constraints

- **Single Fly.io region (sin):** disaster recovery risk. Need multi-region.
- **No caching layer:** every request hits Postgres. Add Redis for hot reads (account balance, FX rate).
- **No async processing:** audit log, reconciliation all synchronous. Add queue (NATS / Kafka) for async.

### Cost projection (for 1M req/s)

| Tier | Cost/month (USD) |
|---|---|
| Phase 1: vertical | ~$50-100 |
| Phase 2: horizontal (10 machines) | ~$500-1000 |
| Phase 3: sharding (10 shards) | ~$5,000-10,000 |
| Phase 4: event-driven | ~$10,000-20,000 |

### What we have today (1,000x scale gap)

- ✅ `SELECT FOR UPDATE` + UUID-sort for no-deadlock concurrency.
- ✅ Idempotency-Key for safe client retries.
- ✅ Cursor-based pagination (no OFFSET scans).
- ✅ Hash chain for audit-grade integrity.
- ✅ Connection pooling via pgxpool.
- ❌ Read replicas (Phase 2)
- ❌ Sharding (Phase 3)
- ❌ Caching layer (Redis)
- ❌ Async queue (NATS / Kafka)
- ❌ Multi-region (deferred to Phase 3+)

### Trade-offs

- **Vertical scale hits ceiling fast** (single VM = single Postgres = bottleneck).
- **Sharding is hard** — cross-shard transactions need 2PC or eventual consistency.
- **Caching adds invalidation complexity** — stale balance risks overdraft.
- **Async adds complexity** — eventual consistency for audit is fine, but for transfer it's not.

We pick **vertical → horizontal → sharding** in that order, because each phase has higher cost but also higher ROI for our scale.

---

## Reference

- [C4 Diagrams](../architecture/c4-diagrams.md) — System context
- [Architecture: Sequences](../architecture/sequences.md) — Critical user journeys
- [ADR-0004: Locking strategy](../adr/0004-locking-strategy.md) — UUID-sorted lock order rationale
- [ADR-0006: Tenant RLS + Field-Level Authz Strategy](../adr/0006-tenant-rls-strategy.md) — RLS design
- [Sprint 15: Tenant RLS Integration](../SPRINTS.md#sprint-15--tenant-rls-integration-fase-2b--5a--2026-08-14)
- [Demo Script](demo-script.md)
- [Fintech Q&A](q-fintech.md)
- [Security Q&A](q-security.md)
