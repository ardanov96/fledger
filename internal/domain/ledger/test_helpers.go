package ledger

// Test helper errors. These are exposed so external _test packages can
// produce typed sentinel errors matching the domain's regular errors.
//
// IMPORTANT: in production code, use the canonical sentinels from
// platform/errors (ErrAccountNotFound, ErrIdempotencyConflict, ErrNotFound).
// In test code, use these helpers when you need to satisfy a domain
// interface with a typed error.
//
// We can't directly re-export apperrors from here (would create an
// import cycle: platform/errors -> usecase -> ledger). Instead, we
// create simple plain errors and let the use case treat them as the
// expected typed error. Use apperrors directly in production code.

import "errors"

// AccountNotFoundForTest returns a typed error for "account not found".
// Test-only.
func AccountNotFoundForTest() error { return errors.New("account not found") }

// IdempotencyConflictForTest returns a typed error for "duplicate
// idempotency key". Test-only.
func IdempotencyConflictForTest() error { return errors.New("idempotency conflict") }

// NotFoundForTest returns a typed error for "not found". Test-only.
func NotFoundForTest() error { return errors.New("not found") }
