// Package jwt provides HS256 sign/verify helpers for the Fledger Fleet service.
//
// Mirrors the API surface of fledger-core/internal/auth/jwt so that the same
// secret can be used to mint tokens accepted by both services (handy for E2E
// tests that share a tenant).
package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

const (
	Issuer   = "fledger-fleet"
	Audience = "fledger-fleet-api"
)

var (
	ErrTokenExpired  = errors.New("jwt: token expired")
	ErrTokenInvalid  = errors.New("jwt: token invalid")
	ErrTokenMalformed = errors.New("jwt: token malformed")
)

// Claims is the custom claim set embedded in every fleet token.
type Claims struct {
	UserID  string   `json:"sub"`
	Tenant  string   `json:"tenant"`
	Role    string   `json:"role"`
	Scopes  []string `json:"scopes,omitempty"`
	jwtv5.RegisteredClaims
}

// SecretProvider supplies the HMAC secret(s). Single-key in dev; multi-key
// is supported for zero-downtime rotation in production.
type SecretProvider interface {
	Secrets() [][]byte
}

// StaticSecret is a trivial provider for tests and dev.
type StaticSecret struct{ Value []byte }

func (s StaticSecret) Secrets() [][]byte { return [][]byte{s.Value} }

// Signer mints tokens with the primary secret.
type Signer struct {
	provider SecretProvider
	now      func() time.Time
}

func NewSigner(p SecretProvider) *Signer {
	return &Signer{provider: p, now: time.Now}
}

// Sign mints a JWT for the given principal with the configured TTL.
func (s *Signer) Sign(c Claims, ttl time.Duration) (string, error) {
	keys := s.provider.Secrets()
	if len(keys) == 0 {
		return "", fmt.Errorf("jwt: no secrets configured")
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
	return tok.SignedString(keys[0])
}

// Verifier validates tokens against any of the configured secrets.
type Verifier struct {
	provider SecretProvider
	now      func() time.Time
}

func NewVerifier(p SecretProvider) *Verifier {
	return &Verifier{provider: p, now: time.Now}
}

// Verify parses + validates a token, trying each secret in order. First match wins.
func (v Verifier) Verify(token string) (*Claims, error) {
	keys := v.provider.Secrets()
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no secrets configured", ErrTokenInvalid)
	}
	c := &Claims{}
	parsedTok, err := jwtv5.ParseWithClaims(token, c, func(t *jwtv5.Token) (any, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected alg %v", ErrTokenInvalid, t.Header["alg"])
		}
		return keys[0], nil
	})
	if err != nil {
		for i := 1; i < len(keys); i++ {
			c2 := &Claims{}
			p2, err2 := jwtv5.ParseWithClaims(token, c2, func(t *jwtv5.Token) (any, error) {
				if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("%w: unexpected alg %v", ErrTokenInvalid, t.Header["alg"])
				}
				return keys[i], nil
			})
			if err2 == nil && p2.Valid {
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
	if !parsedTok.Valid {
		return nil, fmt.Errorf("%w: parsed.Valid == false", ErrTokenInvalid)
	}
	return c, nil
}