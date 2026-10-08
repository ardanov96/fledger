// Package ledger — hash chain support for tamper detection.
//
// Each ledger_entries row gets a SHA-256 hash over a canonical JSON encoding
// of (prev_hash + entry fields). The prev_hash is the entry_hash of the
// previous entry for the same account_id (ordered by created_at, id).
//
// First entry per account uses the zero hash (64 zeros) as prev_hash.
//
// Detection: if an attacker tampers with amount/description of any entry,
// recomputing the chain will show a mismatch starting at that entry.
package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// ZeroHash is the prev_hash value for the first entry of each account.
// 64 hex chars (32 bytes of zeros).
const ZeroHash = "0000000000000000000000000000000000000000000000000000000000000000"

// HashInput captures the fields used to compute an entry's hash. It is
// intentionally separate from Entry so callers must explicitly decide what
// is included (and so future schema additions don't silently change hashes).
type HashInput struct {
	PrevHash      string
	AccountID     string
	TransactionID string
	PeriodID      string
	Type          EntryType
	AmountMinor   int64
	Currency      string
	RefType       string
	RefID         string
	Description  string
	CreatedAt     time.Time
}

// ComputeHash returns the lowercase hex SHA-256 of the canonical encoding.
//
// Encoding is "prev|acct|txn|period|type|amount|currency|reftype|refid|desc|created_unix".
// We use '|' as separator (rare in financial data) and unix-nano for created_at
// to avoid timezone / formatting drift.
func ComputeHash(in HashInput) (string, error) {
	if in.PrevHash == "" {
		return "", fmt.Errorf("hasher: prev_hash required")
	}
	amountStr := strconv.FormatInt(in.AmountMinor, 10)
	createdUnix := strconv.FormatInt(in.CreatedAt.UTC().UnixNano(), 10)

	parts := []string{
		in.PrevHash,
		in.AccountID,
		in.TransactionID,
		in.PeriodID,
		string(in.Type),
		amountStr,
		in.Currency,
		in.RefType,
		in.RefID,
		in.Description,
		createdUnix,
	}

	// Join with '|' then hash the raw string. No JSON because field order
	// would become version-dependent.
	var buf []byte
	for i, p := range parts {
		if i > 0 {
			buf = append(buf, '|')
		}
		buf = append(buf, p...)
	}

	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:]), nil
}
