// Package webhook verifies HMAC signatures on incoming payment-gateway
// callbacks (Sprint 3 / §1.2 of the brief).
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrSignatureInvalid is returned when a signature does not match.
var ErrSignatureInvalid = errors.New("webhook: signature invalid")

// ComputeMidtransSignature returns the SHA-512 hex digest used by Midtrans
// (and many Indonesian banks that follow the same scheme):
//
//	hex(SHA512(order_id + status_code + gross_amount + server_key))
func ComputeMidtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	h := sha512.New()
	h.Write([]byte(orderID))
	h.Write([]byte(statusCode))
	h.Write([]byte(grossAmount))
	h.Write([]byte(serverKey))
	return hex.EncodeToString(h.Sum(nil))
}

// ComputeGenericSHA256 returns hex(SHA256(secret + payload)) for gateways
// that sign the raw body with a shared HMAC key.
func ComputeGenericSHA256(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyMidtransSignature returns nil iff the provided signature matches the
// expected one (case-insensitive on hex).
func VerifyMidtransSignature(orderID, statusCode, grossAmount, serverKey, provided string) error {
	want := ComputeMidtransSignature(orderID, statusCode, grossAmount, serverKey)
	if !secureEqual(want, provided) {
		return fmt.Errorf("%w: midtrans signature mismatch", ErrSignatureInvalid)
	}
	return nil
}

// VerifyGenericSHA256 returns nil iff HMAC-SHA256(secret, payload) == provided.
func VerifyGenericSHA256(secret string, payload []byte, provided string) error {
	want := ComputeGenericSHA256(secret, payload)
	if !secureEqual(want, provided) {
		return fmt.Errorf("%w: generic sha256 mismatch", ErrSignatureInvalid)
	}
	return nil
}

// VerifyXenditToken matches the Xendit `x-callback-token` against the configured
// webhook secret (the simple token variant they use for v1 callbacks).
func VerifyXenditToken(secret, provided string) error {
	if !secureEqual(secret, provided) {
		return fmt.Errorf("%w: xendit token mismatch", ErrSignatureInvalid)
	}
	return nil
}

// secureEqual does a constant-time string comparison.
func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return hmac.Equal([]byte(strings.ToLower(a)), []byte(strings.ToLower(b)))
}