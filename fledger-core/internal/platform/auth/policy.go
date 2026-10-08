// Package auth — password_policy.go (Sprint 23 / 22B.4)
//
// PasswordPolicy is a baseline enforcement of password hygiene. This is a
// defense-in-depth layer on top of bcrypt + account lockout (Sprint 13):
// even if a weak password slips through (e.g. legacy users seeded before
// this policy lands), bcrypt + 5-attempt lockout protects the auth flow.
//
// The policy is pluggable via the AuthConfig so production deployments can
// tighten (or relax) the defaults without a code deploy. Defaults target
// NIST SP 800-63B minimums:
//
//   - MinLength 12 (NIST recommendation for memorized secrets)
//   - RequireDigit + RequireUpper to break dictionary attacks
//   - RequireSymbol minimal — symbols are ASCII printable chars that are
//     not alphanumerics; allows users to keep common patterns like "Foo123!bar"
//   - MaxLength 128 (bcrypt hard limit is 72 bytes — but user input can be
//     pre-hashed; we cap at 128 to avoid abuse)
package auth

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// PasswordPolicy is the policy struct passed to AuthService.
type PasswordPolicy struct {
	MinLength     int  // default 12
	MaxLength     int  // default 128
	RequireDigit  bool // default true
	RequireUpper  bool // default true
	RequireLower  bool // default true
	RequireSymbol bool // default true
}

// DefaultPasswordPolicy returns the security baseline.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:     12,
		MaxLength:     128,
		RequireDigit:  true,
		RequireUpper:  true,
		RequireLower:  true,
		RequireSymbol: true,
	}
}

// ErrPasswordPolicyFail is returned by Validate when a password is below
// the policy baseline. Handlers translate to HTTP 422.
var ErrPasswordPolicyFail = errors.New("auth: password policy violation")

// Validate returns nil if the password meets the policy, otherwise it
// returns ErrPasswordPolicyFail wrapped with a descriptive message. We do
// NOT echo back details about WHICH check failed (production hardening —
// don't leak policy details to potential attackers).
func (p PasswordPolicy) Validate(password string) error {
	if p.MinLength == 0 {
		p = DefaultPasswordPolicy()
	}
	if len(password) < p.MinLength {
		return policyErr("password too short")
	}
	if len(password) > p.MaxLength {
		return policyErr("password too long")
	}
	var hasDigit, hasUpper, hasLower, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	if p.RequireDigit && !hasDigit {
		return policyErr("missing required character class")
	}
	if p.RequireUpper && !hasUpper {
		return policyErr("missing required character class")
	}
	if p.RequireLower && !hasLower {
		return policyErr("missing required character class")
	}
	if p.RequireSymbol && !hasSymbol {
		return policyErr("missing required character class")
	}
	// Reject common trivial passwords (case-insensitive). Bounded list to
	// avoid uncapped memory growth — only well-known examples.
	if _, ok := isWeakList[strings.ToLower(password)]; ok {
		return policyErr("password too common")
	}
	return nil
}

func policyErr(detail string) error {
	return fmt.Errorf("%w: %s", ErrPasswordPolicyFail, detail)
}

// isWeakList is a small built-in denylist. Production deployments should
// extend this (e.g. via HIBP k-anonymity API — see ADR-0008 follow-up #2).
var isWeakList = map[string]struct{}{
	"password":      {},
	"password123":   {},
	"qwerty":        {},
	"qwerty123":     {},
	"123456789":     {},
	"123456789012":  {},
	"iloveyou":      {},
	"admin":         {},
	"admin123":      {},
	"welcome":       {},
	"welcome1":      {},
	"letmein":       {},
	"changeme":      {},
	"fmcgwallet":    {},
	"fmcg-wallet":   {},
	"salesrep123":   {},
}
