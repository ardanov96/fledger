-- =============================================================================
-- Transaction queries
-- =============================================================================

-- name: CreateTransaction :one
-- Inserts a new transaction (status: pending). MUST be inside a transaction.
INSERT INTO transactions (
    id, idempotency_key, status, description, ref_type, ref_id,
    initiator_id, tenant_id, period_id, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, idempotency_key, status, description, ref_type, ref_id,
          initiator_id, tenant_id, period_id, metadata, posted_at,
          created_at, updated_at;

-- name: GetTransactionByID :one
SELECT id, idempotency_key, status, description, ref_type, ref_id,
       initiator_id, tenant_id, period_id, metadata, posted_at,
       created_at, updated_at
FROM transactions
WHERE id = $1;

-- name: GetTransactionByIdempotencyKey :one
-- Returns the transaction matching (tenant, key) or NULL.
SELECT id, idempotency_key, status, description, ref_type, ref_id,
       initiator_id, tenant_id, period_id, metadata, posted_at,
       created_at, updated_at
FROM transactions
WHERE tenant_id = $1 AND idempotency_key = $2;

-- name: MarkTransactionPosted :exec
-- Transitions status from pending to posted and sets posted_at.
UPDATE transactions
SET status = 'posted',
    posted_at = now()
WHERE id = $1 AND status = 'pending';

-- name: MarkTransactionFailed :exec
UPDATE transactions
SET status = 'failed'
WHERE id = $1 AND status = 'pending';

-- name: MarkTransactionReversed :exec
UPDATE transactions
SET status = 'reversed'
WHERE id = $1 AND status = 'posted';

-- name: ListTransactionsByPeriod :many
SELECT id, idempotency_key, status, description, ref_type, ref_id,
       initiator_id, tenant_id, period_id, metadata, posted_at,
       created_at, updated_at
FROM transactions
WHERE tenant_id = $1
  AND ($2::uuid IS NULL OR period_id = $2)
  AND ($3::text IS NULL OR status = $3)
  AND ($4::timestamptz IS NULL OR created_at < $4)
ORDER BY created_at DESC, id DESC
LIMIT $5;
