// sha256.go — small helper to compute SHA-256 hex strings without leaking
// the import into the main hasher.go file (keeps hasher.go focused on
// bcrypt + TOTP).
package auth

import (
	"crypto/sha256"
	"encoding/hex"
)

// sha256Sum returns the hex-encoded SHA-256 of input.
func sha256Sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
