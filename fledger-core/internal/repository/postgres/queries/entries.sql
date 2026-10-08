-- =============================================================================
-- Ledger entry queries
-- Entries are IMMUTABLE — only INSERT, never UPDATE/DELETE.
-- =============================================================================

-- name: CreateEntry :one
-- Inserts a new ledger entry. MUST be inside a transaction.
-- The currency must match the account's currency (enforced by trigger).
INSERT INTO ledger_entries (
    id, transaction_id, account_id, amount, type, ref_type, ref_id,
    period_id, description, currency, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, transaction_id, account_id, amount, type, ref_type, ref_id,
          period_id, description, currency, metadata, created_at;

-- name: GetEntriesByTransaction :many
-- Returns all entries for a given transaction.
SELECT id, transaction_id, account_id, amount, type, ref_type, ref_id,
       period_id, description, currency, metadata, created_at
FROM ledger_entries
WHERE transaction_id = $1
ORDER BY created_at, id;

-- name: ListEntriesByAccount :many
-- Cursor-paginated list of entries for an account (statement view).
SELECT id, transaction_id, account_id, amount, type, ref_type, ref_id,
       period_id, description, currency, metadata, created_at
FROM ledger_entries
WHERE account_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC, id DESC
LIMIT $3;

-- name: SumEntriesForAccount :one
-- Authoritative balance from entries. Positive = net debit.
SELECT COALESCE(SUM(
    CASE WHEN type = 'debit' THEN amount ELSE -amount END
), 0)::BIGINT AS balance
FROM ledger_entries
WHERE account_id = $1;

-- name: TrialBalanceGlobal :one
-- Defense-in-depth: SUM(debit) - SUM(credit) across the ENTIRE ledger.
-- Must be zero in a healthy system. Used by reconciler (Fase 1B).
SELECT
    COALESCE(SUM(CASE WHEN type = 'debit' THEN amount ELSE 0 END), 0)::BIGINT AS total_debit,
    COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0)::BIGINT AS total_credit,
    (COALESCE(SUM(CASE WHEN type = 'debit' THEN amount ELSE 0 END), 0) -
     COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END), 0))::BIGINT AS imbalance
FROM ledger_entries;
