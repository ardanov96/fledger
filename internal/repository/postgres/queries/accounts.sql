-- =============================================================================
-- Account queries
-- sqlc generates type-safe Go from these comments.
-- =============================================================================

-- name: CreateAccount :one
-- Inserts a new account. Returns ErrAlreadyExists on code conflict.
INSERT INTO accounts (
    id, code, name, type, status, currency, cached_balance,
    owner_id, tenant_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, code, name, type, status, currency, cached_balance,
          owner_id, tenant_id, metadata, created_at, updated_at;

-- name: GetAccountByID :one
-- Fetches an account by ID. Returns NULL if not found.
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE id = $1;

-- name: GetAccountByCode :one
-- Fetches an account by tenant + code. Returns NULL if not found.
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE tenant_id = $1 AND code = $2;

-- name: ListAccountsByTenant :many
-- Lists all accounts for a tenant. Cursor-based pagination via (created_at, id).
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE tenant_id = $1
  AND ($2::text IS NULL OR type = $2)
  AND ($3::text IS NULL OR status = $3)
  AND ($4::timestamptz IS NULL OR created_at < $4)
ORDER BY created_at DESC, id DESC
LIMIT $5;

-- name: UpdateAccountStatus :one
-- Updates the status of an account.
UPDATE accounts
SET status = $2
WHERE id = $1
RETURNING id, code, name, type, status, currency, cached_balance,
          owner_id, tenant_id, metadata, created_at, updated_at;

-- name: UpdateAccountBalance :exec
-- Atomically updates cached_balance. Called inside the same transaction
-- as ledger entry inserts. Caller must have already locked the row
-- (LockAccountForUpdate) to prevent lost-update races.
UPDATE accounts
SET cached_balance = $2
WHERE id = $1;

-- name: LockAccountForUpdate :one
-- Locks an account row for update within a transaction. This is the
-- CRITICAL primitive for safe concurrent balance updates.
--
-- IMPORTANT: Caller MUST be inside a transaction. ALWAYS order locks
-- by account ID (lexicographically) across concurrent transactions
-- to prevent deadlocks.
SELECT id, code, name, type, status, currency, cached_balance,
       owner_id, tenant_id, metadata, created_at, updated_at
FROM accounts
WHERE id = $1
FOR UPDATE;

-- name: SumEntriesForAccount :one
-- Computes the authoritative balance from entries.
-- Positive = net debit, negative = net credit.
SELECT COALESCE(SUM(
    CASE WHEN type = 'debit' THEN amount ELSE -amount END
), 0)::BIGINT AS balance
FROM ledger_entries
WHERE account_id = $1;
