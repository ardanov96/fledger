// Package auth provides low-level helpers for Sprint 13 auth hardening.
//
//   - BcryptPasswordHasher: bcrypt-based password hashing/verification.
//   - TOTPGenerator: RFC 6238 TOTP (time-based one-time password) for MFA.
//
// These are isolated in `internal/platform/auth` so the use case layer
// stays free of crypto details (it depends only on these interfaces).
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// =============================================================================
// PasswordHasher — bcrypt
// =============================================================================

// PasswordHasher hashes and verifies passwords.
type PasswordHasher interface {
	Hash(plaintext string) (string, error)
	Verify(plaintext, hash string) bool
}

// BcryptPasswordHasher is a bcrypt-backed PasswordHasher.
// Default cost = 12 (good balance for production: ~250ms per hash).
type BcryptPasswordHasher struct {
	Cost int
}

// NewBcryptPasswordHasher constructs with default cost 12.
func NewBcryptPasswordHasher() *BcryptPasswordHasher {
	return &BcryptPasswordHasher{Cost: 12}
}

// Hash returns a bcrypt hash of the plaintext.
func (h *BcryptPasswordHasher) Hash(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("password: empty plaintext")
	}
	cost := h.Cost
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt hash: %w", err)
	}
	return string(b), nil
}

// Verify compares plaintext against a bcrypt hash. Returns false on any error.
func (h *BcryptPasswordHasher) Verify(plaintext, hash string) bool {
	if plaintext == "" || hash == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
	return err == nil
}

// =============================================================================
// TokenGenerator — opaque refresh token generation
// =============================================================================

// TokenGenerator produces opaque (non-JWT) tokens for refresh tokens and MFA
// challenges. Uses crypto/rand for 32 bytes = 256 bits of entropy.
type TokenGenerator interface {
	Generate() (raw string, hash string, err error)
}

// DefaultTokenGenerator uses SHA-256 to derive the hash from the raw token.
type DefaultTokenGenerator struct{}

// NewDefaultTokenGenerator returns a DefaultTokenGenerator.
func NewDefaultTokenGenerator() *DefaultTokenGenerator {
	return &DefaultTokenGenerator{}
}

// Generate returns the raw token (URL-safe base64, 43 chars) and its hex SHA-256 hash.
func (g *DefaultTokenGenerator) Generate() (raw string, hashStr string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("token gen: rand: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	// Hash is SHA-256(raw) as hex.
	h := sha256Sum([]byte(raw))
	return raw, h, nil
}

// =============================================================================
// TOTP — RFC 6238
// =============================================================================

// TOTPGenerator verifies time-based one-time passwords (TOTP).
// Implementation follows RFC 6238 with HMAC-SHA1 (default), 6 digits, 30s period.
type TOTPGenerator struct {
	// Period (seconds). Default 30.
	Period int
	// Digits. Default 6.
	Digits int
	// Skew (allowed clock-drift windows). Default 1 (±30s).
	Skew int
	// now is injectable for tests.
	now func() time.Time
}

// NewTOTPGenerator constructs with defaults (period=30, digits=6, skew=1).
func NewTOTPGenerator() *TOTPGenerator {
	return &TOTPGenerator{
		Period: 30,
		Digits: 6,
		Skew:   1,
		now:    time.Now,
	}
}

// GenerateSecret returns a base32-encoded random secret (20 bytes = 160 bits).
func (g *TOTPGenerator) GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("totp secret: rand: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// otpURL builds an otpauth:// URL for QR code provisioning (Google Authenticator compatible).
// Format: otpauth://totp/{label}?secret={base32secret}&issuer={issuer}&algorithm=SHA1&digits=6&period=30
func (g *TOTPGenerator) OTPURL(secret, label, issuer string) string {
	q := url.Values{}
	q.Set("secret", secret)
	if issuer != "" {
		q.Set("issuer", issuer)
	}
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", g.digits()))
	q.Set("period", fmt.Sprintf("%d", g.period()))
	return "otpauth://totp/" + url.PathEscape(label) + "?" + q.Encode()
}

// Verify checks the provided code against the secret at the current time,
// allowing ±Skew windows.
func (g *TOTPGenerator) Verify(secret, code string) bool {
	if len(code) != g.digits() {
		return false
	}
	now := g.now()
	for i := -g.Skew; i <= g.Skew; i++ {
		t := now.Add(time.Duration(i) * time.Duration(g.period()) * time.Second)
		expected := g.generateCode(secret, t.Unix()/int64(g.period()))
		if hmac.Equal([]byte(code), []byte(expected)) {
			return true
		}
	}
	return false
}

func (g *TOTPGenerator) digits() int {
	if g.Digits <= 0 {
		return 6
	}
	return g.Digits
}

func (g *TOTPGenerator) period() int {
	if g.Period <= 0 {
		return 30
	}
	return g.Period
}

// generateCode produces the TOTP code for a given secret and time counter.
func (g *TOTPGenerator) generateCode(secret string, counter int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	// Encode counter as 8-byte big-endian.
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))
	// HMAC-SHA1.
	h := hmac.New(sha1.New, key)
	h.Write(buf)
	sum := h.Sum(nil)
	// Dynamic truncation per RFC 4226.
	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	// Modulo to N digits.
	mod := uint32(1)
	for i := 0; i < g.digits(); i++ {
		mod *= 10
	}
	code := truncated % mod
	return fmt.Sprintf("%0*d", g.digits(), code)
}
