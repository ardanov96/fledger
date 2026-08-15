# Sequence Flows

Critical user journeys shown as Mermaid sequence diagrams. See [C4 Diagrams](c4-diagrams.md) for static architecture and [Architecture Overview](overview.md) for the narrative.

---

## 1. Login (no MFA)

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant API as POST /v1/auth/login
    participant Repo as AuthRepository
    participant JWT as JWT Signer
    participant DB as Postgres

    Client->>API: POST {username, password}
    API->>Repo: GetCredentialsByUsername(username)
    Repo->>DB: SELECT * FROM user_credentials WHERE username=?
    DB-->>Repo: row (bcrypt hash, mfa_enabled=false)
    Repo-->>API: UserCredentials

    API->>API: bcrypt.CompareHashAndPassword(hash, password)
    Note over API: ✅ password valid<br/>mfa_enabled=false → no MFA

    API->>Repo: CreateRefreshToken(user_id, family_id, raw_token)
    Repo->>DB: INSERT INTO refresh_tokens
    Note over DB: stores SHA-256(raw_token),<br/>not the raw token

    API->>JWT: SignAccessToken(user_id, tenant_id, role)
    JWT-->>API: access_token (JWT)
    API-->>Client: 200 OK {access_token, refresh_token, expires_in}
```

**Key invariants:**
- Raw password never logged
- Bcrypt cost 10 (demo) / 12 (prod), ~25ms / ~250ms per hash
- Refresh token stored as SHA-256 hash only (not reversible)
- Access token stateless JWT (no DB lookup needed)

---

## 2. Login with MFA

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant API
    participant Repo
    participant TOTP as TOTP Verifier
    participant JWT
    participant DB

    Client->>API: POST {username, password}
    API->>Repo: GetCredentials
    Repo->>DB: SELECT ...
    DB-->>Repo: row (mfa_enabled=true)
    API->>API: bcrypt verify
    API->>Repo: CreateMFAChallenge(user_id, raw_token)
    Repo->>DB: INSERT INTO mfa_challenges (TTL=5min)
    API-->>Client: 200 {type: "mfa_challenge", challenge_token}

    Note over Client: User opens authenticator,<br/>reads 6-digit code

    Client->>API: POST /v1/auth/mfa/verify {challenge_token, code}
    API->>Repo: GetMFAChallenge(challenge_token_hash)
    Repo->>DB: SELECT WHERE expires_at > NOW() AND attempts < 3
    DB-->>Repo: challenge row (with mfa_secret)
    API->>TOTP: Verify(code, secret, ±30s skew)
    TOTP-->>API: ✅ valid

    API->>Repo: MarkChallengeVerified(challenge_id)
    API->>Repo: CreateRefreshToken(user_id, ...)
    API->>JWT: SignAccessToken
    API-->>Client: 200 {access_token, refresh_token}
```

**Key invariants:**
- 3 wrong attempts → 5 min lockout (DB enforced via trigger)
- Challenge token valid 5 min
- TOTP ±30s drift window (RFC 6238)

---

## 3. Refresh Token Rotation (with reuse detection)

```mermaid
sequenceDiagram
    autonumber
    actor Client
    actor Attacker
    participant API as POST /v1/auth/refresh
    participant DB

    Note over Client: Has refresh_token_r1 (family_id=F, gen=1)

    Client->>API: POST {refresh_token: r1}
    API->>DB: SELECT refresh_tokens WHERE hash=hash(r1)
    DB-->>API: row (status='active')
    API->>DB: UPDATE status='rotated', rotated_to=r2_id
    API->>DB: INSERT (r2, status='active', family=F)
    DB-->>API: new pair
    API-->>Client: 200 {access_token, refresh_token: r2}

    Note over Client: Stores r2, discards r1 ✓

    Note over Attacker: Stole r1 BEFORE rotation<br/>(status='active' at time of steal)

    Attacker->>API: POST {refresh_token: r1}
    API->>DB: SELECT WHERE hash=hash(r1)
    DB-->>API: row (status='rotated'! ⚠️)
    Note over API: REUSE DETECTED!
    API->>DB: UPDATE family=F SET status='revoked' WHERE status IN ('active','rotated')
    DB-->>API: all tokens in family revoked
    API-->>Attacker: 401 TOKEN_REUSE_DETECTED

    Note over Client: Next refresh call → 401<br/>must re-login
```

**Critical security property:** Theft of a refresh token between legitimate rotations causes **whole-family revocation**. This protects against token theft even when attacker is faster than legitimate rotation.

---

## 4. Double-Entry Transfer (same currency)

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant API as POST /v1/transfers
    participant Svc as TransferService
    participant DB

    Client->>API: POST + Idempotency-Key
    API->>API: validate request body
    API->>Svc: Transfer(ctx, input)

    Note over Svc: 1. Idempotency check
    Svc->>DB: SELECT transaction WHERE idem_key=? AND tenant_id=?
    DB-->>Svc: not found → proceed

    Note over Svc: 2. LockPairForUpdate(src_id, dst_id)<br/>UUID-sorted to avoid deadlock
    Svc->>DB: SELECT * FROM accounts WHERE id IN (...) ORDER BY id FOR UPDATE
    DB-->>Svc: src + dst accounts (locked)

    Note over Svc: 3. Validate state + balance
    Svc->>Svc: src.status=active, dst.status=active<br/>src.balance >= amount

    Note over Svc: 4. Find or create period
    Svc->>DB: SELECT period WHERE status='open' AND tenant_id=?
    DB-->>Svc: open period

    Note over Svc: 5. Insert transaction + 2 entries (in 1 tx)
    Svc->>DB: BEGIN
    Svc->>DB: INSERT INTO transactions (idem_key, status=pending, ...)
    Svc->>DB: INSERT INTO ledger_entries (debit src, credit dst, 2 rows)
    Note over DB: DB trigger checks prev_hash,<br/>appends to SHA-256 chain
    Svc->>DB: UPDATE accounts SET cached_balance=...
    Svc->>DB: UPDATE transactions SET status='posted'
    Svc->>DB: COMMIT

    Svc-->>API: Transaction{ID, status='posted', entries=[...]}
    API-->>Client: 201 Created
```

**Concurrency guarantees:**
- `LockPairForUpdate` uses UUID-sorted order to prevent A↔B / B↔A deadlock
- Test: `TestConcurrent_NoDeadlocks_100x50` (100 goroutines × 50 iterations × 10 accounts)
- Idempotency-Key scoped per (tenant, endpoint), 24h TTL

---

## 5. Cross-Currency Transfer with FX Snapshot

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant API as POST /v1/transfers
    participant Svc as TransferService
    participant FX as CurrencyService
    participant Money as money.Convert
    participant DB

    Client->>API: POST {from=USD_acct, to=IDR_acct, amount=10000, currency=USD}
    Note over API: currencies differ → FX path

    API->>Svc: Transfer(ctx, input)
    Svc->>DB: SELECT accounts
    DB-->>Svc: src=USD, dst=IDR

    Note over Svc: Cross-currency detected<br/>lookup latest active rate
    Svc->>FX: GetLatestFxRate(tenant, USD→IDR)
    FX->>DB: SELECT * FROM fx_rates WHERE now() BETWEEN effective_at AND expires_at
    DB-->>FX: rate=15750, rate_id=r_xyz
    FX-->>Svc: rate

    Svc->>Money: Convert(10000, fromDP=2, toDP=2, rate=15750)
    Money-->>Svc: 157500000 (IDR 1,575,000.00, half-up)

    Note over Svc: Asymmetric entries
    Svc->>DB: BEGIN
    Svc->>DB: INSERT transaction (fx_rate_id=r_xyz, fx_rate_locked_at=now)
    Svc->>DB: INSERT entry (debit src 10000 USD, prev_hash computed)
    Svc->>DB: INSERT entry (credit dst 157500000 IDR, prev_hash computed)
    Svc->>DB: UPDATE balances (USD -100, IDR +1575000)
    Svc->>DB: COMMIT

    Note over DB: DB trigger enforce_fx_rate_snapshot<br/>verifies fx_rate_id + fx_rate_locked_at<br/>required for cross-currency

    Svc-->>API: Transaction{FxRateID=r_xyz, FxRateLockedAt=now}
    API-->>Client: 201 Created
```

**Audit-grade property:** `transactions.fx_rate_id + fx_rate_locked_at` are written once and immutable. Historical reports can replay the exact rate used, regardless of later rate changes.

---

## 6. Period Close (Two-Step Approval)

```mermaid
sequenceDiagram
    autonumber
    actor Operator as hq_finance
    actor Approver as hq_admin
    participant API
    participant Svc as PeriodService
    participant DB

    Operator->>API: POST /v1/periods/{id}/close-requests
    API->>Svc: RequestClose(ctx, period_id)
    Svc->>DB: SELECT period FOR UPDATE
    DB-->>Svc: period (status=open)
    Svc->>DB: INSERT close_request (status=pending)
    Svc->>DB: UPDATE period SET status='closing'
    Svc-->>API: CloseRequest{id, status=pending}
    API-->>Operator: 201 Created

    Note over Operator: Wait for second approver

    Approver->>API: POST /v1/close-requests/{id}/approve
    API->>Svc: ApproveClose(ctx, request_id)
    Svc->>DB: SELECT request FOR UPDATE
    DB-->>Svc: request (status=pending)

    Note over Svc: Compute trial balance
    Svc->>DB: SELECT SUM(debit), SUM(credit) FROM ledger_entries WHERE period_id=?
    DB-->>Svc: debit=5000000000, credit=5000000000

    alt balanced (debit == credit)
        Note over Svc: Generate per-account snapshots
        Svc->>DB: SELECT accounts WHERE tenant_id=?
        DB-->>Svc: 3 accounts
        loop per account
            Svc->>DB: INSERT period_snapshots (account_id, balance, entry_count)
        end
        Svc->>DB: UPDATE period SET status='closed', closed_at=now
        Svc->>DB: UPDATE close_request SET status='approved'
        Svc-->>API: CloseRequest{status=approved, snapshots_created=3}
        API-->>Approver: 200 OK
    else imbalanced
        Svc-->>API: ErrDOUBLE_ENTRY_VIOLATION
        API-->>Approver: 422 Unprocessable
    end
```

**Separation of duties:** The requester (hq_finance) and approver (hq_admin) MUST be different users. RBAC enforces this.

---

## 7. Reconciler Run (with hash chain check)

```mermaid
sequenceDiagram
    autonumber
    actor Operator
    participant API
    participant Svc as ReconcilerService
    participant Probe as LedgerProbe
    participant Hash as HashChainVerifier
    participant DB

    Operator->>API: POST /v1/reconciler/run {tenant_id, period_id, run_hash_check=true}
    API->>Svc: RunReconciliation(ctx, input)
    Svc->>DB: INSERT reconciler_runs (status=running, triggered_by=api)
    DB-->>Svc: run_id

    Note over Svc: Phase 1: Trial balance
    Svc->>Probe: TrialBalance(period_id)
    Probe->>DB: SELECT SUM(CASE type='debit'...) FROM ledger_entries WHERE period_id=?
    DB-->>Probe: total_debit, total_credit
    Probe-->>Svc: (5_000_000_000, 5_000_000_000)

    Note over Svc: imbalance=0 → balanced so far

    Note over Svc: Phase 2: Per-account breakdown
    Svc->>Probe: AccountBalanceAtPeriod(period_id, account_id)
    Probe->>DB: SELECT account_id, SUM(...) GROUP BY account_id
    DB-->>Probe: per-account balances
    Probe-->>Svc: 3 account results
    Svc->>DB: INSERT reconciler_account_results (3 rows)

    Note over Svc: Phase 3: Hash chain (if requested)
    Svc->>Hash: VerifyEntries(entries)
    Hash->>Hash: Walk entries per account, recompute SHA-256
    Note over Hash: Compare prev_hash + entry_hash<br/>Any mismatch → tamper detected
    Hash-->>Svc: 0 errors (chain OK)

    Svc->>DB: UPDATE run SET status='balanced', hash_chain_ok=true
    Svc-->>API: ReconcilerRun{status='balanced', hash_chain_errors=0}
    API-->>Operator: 202 Accepted

    Note over Operator: GET /v1/reconciler/runs/{id} → status detail
```

**Status precedence** (atomic in single tx):
1. Hash chain fails → `tampered` (highest)
2. Trial balance fails → `imbalanced`
3. All checks pass → `balanced`

---

## 8. Collection Route Settlement (with discrepancy approval)

```mermaid
sequenceDiagram
    autonumber
    actor SalesRep as sales_rep
    actor Supervisor as hq_finance
    participant API
    participant Svc as CollectionService
    participant DB

    SalesRep->>API: POST /v1/routes/{id}/settle {settled_amount=4800000, notes=...}
    API->>Svc: SettleRoute(ctx, input)
    Svc->>DB: SELECT route FOR UPDATE
    DB-->>Svc: route (status=completed, total_collected=5000000)
    Svc->>Svc: discrepancy = 4800000 - 5000000 = -200000
    Note over Svc: discrepancy != 0<br/>→ status='pending' (not auto-approved)
    Svc->>DB: INSERT settlement (status=pending, discrepancy=-200000)
    Svc->>DB: UPDATE route SET status='settled'
    Svc-->>API: Settlement{status='pending'}
    API-->>SalesRep: 201 Created

    Supervisor->>API: POST /v1/settlements/{id}/decide {approve=true, notes=...}
    API->>Svc: ApproveSettlement(ctx, input)
    Svc->>DB: SELECT settlement FOR UPDATE
    DB-->>Svc: settlement (status=pending)
    Svc->>DB: UPDATE settlement SET status='approved', approved_at=now
    Svc-->>API: Settlement{status='approved'}
    API-->>Supervisor: 200 OK
```

**Invariant** (property-tested in Sprint 16):
- `discrepancy == settled_amount - expected_amount` (exact arithmetic, no float drift)
- Auto-approve only if `discrepancy == 0`

---

## 9. Multi-Tenant Isolation (Postgres RLS)

```mermaid
sequenceDiagram
    autonumber
    actor TenantA as Tenant A user
    actor TenantB as Tenant B user
    participant API as TenantContextMiddleware + TxAdapter
    participant DB as Postgres

    TenantA->>API: GET /v1/accounts (JWT: tenant_id=A)
    API->>API: extract tenant from JWT
    API->>API: build *tenantctx.Info{TenantID=A}
    API->>API: call Svc.List(ctx, info)
    Svc->>API: RunInTxDomain(ctx, fn)
    API->>API: txAdapter wraps pgx.Tx
    API->>DB: SET LOCAL app.current_tenant_id = 'A'
    Note over DB: SET LOCAL = transaction-scoped<br/>(reverts on COMMIT/ROLLBACK)
    API->>DB: SELECT * FROM accounts WHERE tenant_id = current_setting('app.current_tenant_id')::uuid
    Note over DB: RLS policy: WHERE tenant_id = A<br/>Even if app forgets WHERE,<br/>RLS silently filters
    DB-->>API: only Tenant A accounts
    API-->>TenantA: 200 OK [...]

    Note over TenantB: Same request, different JWT
    TenantB->>API: GET /v1/accounts (JWT: tenant_id=B)
    API->>DB: SET LOCAL app.current_tenant_id = 'B'
    API->>DB: SELECT * FROM accounts
    DB-->>API: only Tenant B accounts
    API-->>TenantB: 200 OK [...Tenant B...]
```

**Defense-in-depth:** RLS is enforced at DB level. Even if app code forgets to filter by `tenant_id`, the policy silently filters. Tested by `TestIntegration_RLSIsolation` (Sprint 17).

---

## Reference

- [C4 Diagrams](c4-diagrams.md) — static architecture views
- [Architecture Overview](overview.md) — narrative + module list
- [ADRs](../adr/index.md) — key decisions
