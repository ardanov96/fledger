// Package jwt — tests for multi-key rotation (Sprint 33).
//
// Verifies the zero-downtime rotation scenario:
//   - Old key signs tokens; new verifier with primary=new, secondary=old
//     can verify them.
//   - New key signs tokens; verifier accepts them.
//   - Single-key mode still works (backward compat).
package jwt

import (
	"errors"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

const (
	oldKey = "old-key-32-bytes-aaaaaaaaaaaaaaaaaaaa"
	newKey = "new-key-32-bytes-bbbbbbbbbbbbbbbbbbbb"
)

func TestRotation_OldTokenVerifiesWithMultiKeyVerifier(t *testing.T) {
	// Sign with OLD key
	signer := NewSigner(StaticSecret{Value: []byte(oldKey)})
	tok, err := signer.Sign(Claims{
		UserID: "user-1",
		Tenant: "tenant-1",
		Role:   "admin",
	}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Verifier with PRIMARY=new, SECONDARY=old (rotation window)
	v := NewVerifier(MultiKeySecret{Keys: [][]byte{
		[]byte(newKey), // primary (signs NEW tokens)
		[]byte(oldKey), // secondary (verifies OLD tokens still in flight)
	}})
	c, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify old token with multikey verifier: %v", err)
	}
	if c.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", c.UserID)
	}
}

func TestRotation_NewTokenVerifiesWithMultiKeyVerifier(t *testing.T) {
	signer := NewSigner(MultiKeySecret{Keys: [][]byte{
		[]byte(newKey), // primary
	}})
	tok, err := signer.Sign(Claims{UserID: "user-2"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	v := NewVerifier(MultiKeySecret{Keys: [][]byte{
		[]byte(newKey),
		[]byte(oldKey),
	}})
	c, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify new token: %v", err)
	}
	if c.UserID != "user-2" {
		t.Errorf("UserID = %q, want user-2", c.UserID)
	}
}

func TestRotation_OldTokenFailsAfterRotationComplete(t *testing.T) {
	// Sign with OLD key
	signer := NewSigner(StaticSecret{Value: []byte(oldKey)})
	tok, _ := signer.Sign(Claims{UserID: "user-3"}, time.Hour)

	// Verifier after rotation: PRIMARY=new only, no secondary
	v := NewVerifier(MultiKeySecret{Keys: [][]byte{
		[]byte(newKey),
	}})
	_, err := v.Verify(tok)
	if err == nil {
		t.Errorf("old token should fail verification with new-only verifier")
	}
	if !errors.Is(err, ErrTokenInvalid) && !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenInvalid/ErrTokenExpired, got: %v", err)
	}
}

func TestSingleKey_BackwardCompat(t *testing.T) {
	// StaticSecret with single value still works
	signer := NewSigner(StaticSecret{Value: []byte(oldKey)})
	tok, err := signer.Sign(Claims{UserID: "user-4"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	v := NewVerifier(StaticSecret{Value: []byte(oldKey)})
	c, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify with single-key verifier: %v", err)
	}
	if c.UserID != "user-4" {
		t.Errorf("UserID = %q, want user-4", c.UserID)
	}
}

func TestEmptySecondary_TreatedAsSingleKey(t *testing.T) {
	// MultiKeySecret with empty secondary should still work
	signer := NewSigner(MultiKeySecret{Keys: [][]byte{
		[]byte(oldKey),
		nil, // empty secondary (operator unset but key remains in struct)
	}})
	tok, err := signer.Sign(Claims{UserID: "user-5"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	v := NewVerifier(MultiKeySecret{Keys: [][]byte{
		[]byte(oldKey),
		nil,
	}})
	c, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if c.UserID != "user-5" {
		t.Errorf("UserID = %q, want user-5", c.UserID)
	}
}

func TestNoSecretsConfigured_Fails(t *testing.T) {
	signer := NewSigner(MultiKeySecret{Keys: nil})
	_, err := signer.Sign(Claims{UserID: "u"}, time.Hour)
	if err == nil {
		t.Errorf("signing with no secrets should fail")
	}

	v := NewVerifier(MultiKeySecret{Keys: nil})
	_, err = v.Verify("any.token.string")
	if err == nil {
		t.Errorf("verifying with no secrets should fail")
	}
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got: %v", err)
	}
}

func TestWrongSecret_AllKeysReject(t *testing.T) {
	signer := NewSigner(StaticSecret{Value: []byte(oldKey)})
	tok, _ := signer.Sign(Claims{UserID: "u"}, time.Hour)

	// Verifier has neither old nor new key
	v := NewVerifier(MultiKeySecret{Keys: [][]byte{
		[]byte("completely-different-key-aaaaaaaaaaaaa"),
		[]byte("completely-different-key-bbbbbbbbbbbbb"),
	}})
	_, err := v.Verify(tok)
	if err == nil {
		t.Errorf("token signed with old key should fail when verifier has neither key")
	}
}

func TestExpiredToken_FailsRegardlessOfKeys(t *testing.T) {
	// Sign with very short TTL
	signer := NewSigner(StaticSecret{Value: []byte(oldKey)})
	tok, _ := signer.Sign(Claims{
		UserID: "u",
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Hour)), // already expired
		},
	}, time.Hour)

	v := NewVerifier(MultiKeySecret{Keys: [][]byte{[]byte(oldKey)}})
	_, err := v.Verify(tok)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}