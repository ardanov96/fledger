// Package jwt provides minimal HS256 sign/verify helpers for the FMCG Wallet.
//
// We use the standard `golang-jwt/jwt/v5` library. The secret is read from
// config (or environment) and rotated via the SecretProvider interface.
//
// Sprint 33 / Fase 2E follow-up: SecretProvider can return multiple secrets
// (primary + secondary) for zero-downtime rotation. The Signer uses the
// primary; the Verifier accepts tokens signed with either.
package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// Claims is the custom claim set embedded in every FMCG Wallet token.
type Claims struct {
	UserID  string   `json:"sub"`
	Tenant  string   `json:"tenant"`
	Role    string   `json:"role"`
	Scopes  []string `json:"scopes,omitempty"`
	jwtv5.RegisteredClaims
}

// NewNumericDate wraps jwtv5.NewNumericDate for convenience in tests and callers.
func NewNumericDate(t time.Time) *jwtv5.NumericDate {
	return jwtv5.NewNumericDate(t)
}

// Issuer / Audience constants (also enforced in Validate).
const (
	Issuer   = "fmcg-wallet"
	Audience = "fmcg-wallet-api"
)

// Sentinel errors for typed error checks (use errors.Is).
var (
	ErrTokenExpired  = errors.New("jwt: token expired")
	ErrTokenInvalid   = errors.New("jwt: token invalid")
	ErrTokenMalformed = errors.New("jwt: token malformed")
)

// SecretProvider supplies the current HMAC secret(s). Production code should
// inject a provider that reads from Vault/Infisical; tests inject a static
// byte slice.
//
// Sprint 33: returns []byte so a MultiKey provider can hand back multiple
// candidates. The Verifier tries each in order; the first match wins.
//   - Sig=1 → single-key mode (current behavior, backward compatible)
//   - Sig=2 → rotation window (primary signs new tokens, secondary verifies old)
type SecretProvider interface {
	Secrets() [][]byte
}

// StaticSecret is a trivial SecretProvider for tests / dev mode.
// Returns [][]byte to match the new interface; a single-element slice is
// treated as single-key mode.
type StaticSecret struct{ Value []byte }

// Secrets implements SecretProvider.
func (s StaticSecret) Secrets() [][]byte { return [][]byte{s.Value} }

// MultiKeySecret returns N secrets in priority order (index 0 = primary).
// The Signer uses [0]; the Verifier tries [0], [1], ..., [N-1] in order.
//
// Production usage (Sprint 33 zero-downtime rotation):
//   1. Set JWT_SECRET_PRIMARY=$OLD, JWT_SECRET_SECONDARY=$NEW
//   2. Deploy → Signer uses OLD, Verifier accepts BOTH
//   3. After 24h (all old tokens expired), set PRIMARY=$NEW, SECONDARY=""
//   4. Deploy → Signer uses NEW, Verifier accepts NEW only
//   5. After another 24h, unset SECONDARY if still present (cleanup)
type MultiKeySecret struct {
	Keys [][]byte
}

// Secrets implements SecretProvider. Filters out empty entries so a partially-
// configured env (e.g., JWT_SECRET_PRIMARY set, JWT_SECRET_SECONDARY="") is
// treated as single-key mode.
func (m MultiKeySecret) Secrets() [][]byte {
	out := make([][]byte, 0, len(m.Keys))
	for _, k := range m.Keys {
		if len(k) > 0 {
			out = append(out, k)
		}
	}
	return out
}

// Signer mints tokens with a given TTL. Always signs with the PRIMARY (first)
// secret from the provider.
type Signer struct {
	provider SecretProvider
	now      func() time.Time // injectable for tests
}

// NewSigner constructs a Signer with the given SecretProvider.
func NewSigner(provider SecretProvider) *Signer {
	return &Signer{provider: provider, now: time.Now}
}

// Sign mints a new JWT for the given principal.
func (s *Signer) Sign(c Claims, ttl time.Duration) (string, error) {
	keys := s.provider.Secrets()
	if len(keys) == 0 {
		return "", fmt.Errorf("jwt: no secrets configured (empty SecretProvider)")
	}
	if c.Issuer == "" {
		c.Issuer = Issuer
	}
	if len(c.Audience) == 0 {
		c.Audience = jwtv5.ClaimStrings{Audience}
	}
	if c.ExpiresAt == nil {
		c.ExpiresAt = jwtv5.NewNumericDate(s.now().Add(ttl))
	}
	if c.IssuedAt == nil {
		c.IssuedAt = jwtv5.NewNumericDate(s.now())
	}
	if c.Subject == "" {
		c.Subject = c.UserID
	}

	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, c)
	// Always sign with the first key (primary).
	return tok.SignedString(keys[0])
}

// Verifier validates tokens against any of the configured secrets.
// Returns the Claims on first successful verification.
type Verifier struct {
	provider SecretProvider
	now      func() time.Time
}

// NewVerifier constructs a Verifier with the given SecretProvider.
func NewVerifier(provider SecretProvider) *Verifier {
	return &Verifier{provider: provider, now: time.Now}
}

// Verify parses + validates a token string and returns the embedded Claims.
// Tries keys sequentially (primary first, then secondary, etc.) — first match
// wins. Empty providers return ErrTokenInvalid.
//
// Value receiver — works for both Verifier and *Verifier interface satisfaction.
func (v Verifier) Verify(token string) (*Claims, error) {
	keys := v.provider.Secrets()
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no secrets configured", ErrTokenInvalid)
	}
	c := &Claims{}
	parsed, err := jwtv5.ParseWithClaims(token, c, func(t *jwtv5.Token) (any, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected alg %v", ErrTokenInvalid, t.Header["alg"])
		}
		// Return the FIRST key; if verification fails on signature, ParseWithClaims
		// returns an error and the fallback loop below retries with the next key.
		return keys[0], nil
	})
	if err != nil {
		// Signature failed — try remaining keys (rotation window).
		for i := 1; i < len(keys); i++ {
			c2 := &Claims{}
			parsed2, err2 := jwtv5.ParseWithClaims(token, c2, func(t *jwtv5.Token) (any, error) {
				if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("%w: unexpected alg %v", ErrTokenInvalid, t.Header["alg"])
				}
				return keys[i], nil
			})
			if err2 == nil && parsed2.Valid {
				return c2, nil
			}
		}
		switch {
		case errors.Is(err, jwtv5.ErrTokenExpired):
			return nil, fmt.Errorf("%w: %v", ErrTokenExpired, err)
		case errors.Is(err, jwtv5.ErrTokenSignatureInvalid),
			errors.Is(err, jwtv5.ErrTokenInvalidClaims),
			errors.Is(err, jwtv5.ErrTokenInvalidIssuer),
			errors.Is(err, jwtv5.ErrTokenInvalidAudience):
			return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
		default:
			return nil, fmt.Errorf("%w: %v", ErrTokenMalformed, err)
		}
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("%w: parsed.Valid == false", ErrTokenInvalid)
	}
	return c, nil
}
