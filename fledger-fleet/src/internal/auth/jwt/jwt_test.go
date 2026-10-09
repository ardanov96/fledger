package jwt

import (
	"testing"
	"time"
)

// TestSignerVerifyRoundtrip mints and verifies a token.
func TestSignerVerifyRoundtrip(t *testing.T) {
	secret := []byte("super-secret-key-32-characters-minimum")
	s := NewSigner(StaticSecret{Value: secret})
	v := NewVerifier(StaticSecret{Value: secret})
	tok, err := s.Sign(Claims{UserID: "u1", Tenant: "t1", Role: "admin"}, time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := v.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != "u1" || claims.Tenant != "t1" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

// TestVerifierRejectsBadSignature verifies tokens signed with a different
// secret are rejected.
func TestVerifierRejectsBadSignature(t *testing.T) {
	s := NewSigner(StaticSecret{Value: []byte("secret-a-secret-b-secret-c-secret-d")})
	v := NewVerifier(StaticSecret{Value: []byte("secret-z-secret-z-secret-z-secret-z")})
	tok, err := s.Sign(Claims{UserID: "u1", Tenant: "t1"}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := v.Verify(tok); err == nil {
		t.Fatalf("expected signature mismatch error")
	}
}

// TestVerifierRejectsExpired verifies the expiry path.
func TestVerifierRejectsExpired(t *testing.T) {
	secret := []byte("super-secret-key-32-characters-minimum")
	s := &Signer{provider: StaticSecret{Value: secret}, now: func() time.Time {
		return time.Now().Add(-2 * time.Hour)
	}}
	v := NewVerifier(StaticSecret{Value: secret})
	tok, err := s.Sign(Claims{UserID: "u1", Tenant: "t1"}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := v.Verify(tok); err == nil {
		t.Fatalf("expected expiry error")
	}
}

// TestMultiKeyRotation verifies the multi-key rotation support.
func TestMultiKeyRotation(t *testing.T) {
	old := []byte("old-secret-old-secret-old-secret-old")
	new := []byte("new-secret-new-secret-new-secret-new")
	signer := NewSigner(multiKey{keys: [][]byte{new}})
	verifier := NewVerifier(multiKey{keys: [][]byte{new, old}})
	tok, err := signer.Sign(Claims{UserID: "u1", Tenant: "t1"}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := verifier.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != "u1" {
		t.Fatalf("unexpected user id")
	}
	// Old-token check: mint with old key, verifier (rotated to new+old) accepts.
	oldSigner := NewSigner(multiKey{keys: [][]byte{old}})
	oldTok, err := oldSigner.Sign(Claims{UserID: "u2", Tenant: "t1"}, time.Minute)
	if err != nil {
		t.Fatalf("sign old: %v", err)
	}
	if _, err := verifier.Verify(oldTok); err != nil {
		t.Fatalf("verify old token: %v", err)
	}
}

type multiKey struct{ keys [][]byte }

func (m multiKey) Secrets() [][]byte {
	out := make([][]byte, 0, len(m.keys))
	for _, k := range m.keys {
		if len(k) > 0 {
			out = append(out, k)
		}
	}
	return out
}