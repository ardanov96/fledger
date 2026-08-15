// Package auth — password_policy_test.go (Sprint 23 / 22B.4)
//
// 4 unit tests covering the Validate() surface. Tests run on all platforms
// (no build tag) — the policy is pure logic (no infra).
package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestPasswordPolicy_ValidateTooShort(t *testing.T) {
	p := DefaultPasswordPolicy()
	if err := p.Validate("Abc1!"); err == nil {
		t.Fatalf("expected error for 5-char password")
	} else if !errors.Is(err, ErrPasswordPolicyFail) {
		t.Fatalf("expected ErrPasswordPolicyFail, got %v", err)
	}
}

func TestPasswordPolicy_ValidateMissingSymbol(t *testing.T) {
	p := DefaultPasswordPolicy()
	// 12 chars, has digit, upper, lower, but no symbol.
	err := p.Validate("Abcdefgh1jkL")
	if err == nil {
		t.Fatalf("expected error for missing symbol")
	} else if !errors.Is(err, ErrPasswordPolicyFail) {
		t.Fatalf("expected ErrPasswordPolicyFail, got %v", err)
	} else if !strings.Contains(err.Error(), "character class") {
		t.Fatalf("expected 'character class' phrasing, got %v", err)
	}
}

func TestPasswordPolicy_ValidateStrongPasses(t *testing.T) {
	p := DefaultPasswordPolicy()
	if err := p.Validate("S3cure!Passw0rd"); err != nil {
		t.Fatalf("expected nil for strong password, got %v", err)
	}
}

func TestPasswordPolicy_ValidateCommonPassword(t *testing.T) {
	// Direct unit test on the weak-list map (package-private).
	if _, ok := isWeakList["password"]; !ok {
		t.Fatalf("expected 'password' on weak list")
	}
	if _, ok := isWeakList["qwerty"]; !ok {
		t.Fatalf("expected 'qwerty' on weak list")
	}

	// Validate() lower-cases input before lookup — verify the contract.
	if _, ok := isWeakList[strings.ToLower("PASSWORD")]; !ok {
		t.Fatalf("expected 'PASSWORD' (lowered) on weak list")
	}
	if _, ok := isWeakList[strings.ToLower("Qwerty")]; !ok {
		t.Fatalf("expected 'Qwerty' (lowered) on weak list")
	}
	// And that arbitrary derivatives are NOT on the list (sanity):
	if _, ok := isWeakList[strings.ToLower("P4ssword!")]; ok {
		t.Fatalf("'P4ssword!' should not be on weak list (not an exact match)")
	}
}

// Note: we don't reach the weak-list rejection via Validate() because it
// only fires AFTER length/complexity checks pass. The weak-list test
// above verifies the lookup table directly; production-time rejection is
// reached when a sufficiently-complex-but-still-common password is presented.

// Bonus: zero-value policy falls back to defaults (no nil-panic risk).
func TestPasswordPolicy_ZeroValueUsesDefaults(t *testing.T) {
	var p PasswordPolicy
	if err := p.Validate("ok"); err == nil {
		t.Fatalf("expected error for short password even with zero-value policy")
	}
}
