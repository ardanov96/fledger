// Package jwt provides minimal HS256 sign/verify helpers for the FMCG Wallet.
//
// We use the standard `golang-jwt/jwt/v5` library. The secret is read from
// config (or environment) and rotated via the SecretProvider interface.
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

// SecretProvider supplies the current HMAC secret. Production code should
// inject a provider that reads from Vault/Infisical; tests inject a static
// byte slice.
type SecretProvider interface {
	Secret() []byte
}

// StaticSecret is a trivial SecretProvider for tests / dev mode.
type StaticSecret struct{ Value []byte }

// Secret implements SecretProvider.
func (s StaticSecret) Secret() []byte { return s.Value }

// Signer mints tokens with a given TTL.
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
	return tok.SignedString(s.provider.Secret())
}

// Verifier validates tokens against a SecretProvider.
type Verifier struct {
	provider SecretProvider
	now      func() time.Time
}

// NewVerifier constructs a Verifier with the given SecretProvider.
func NewVerifier(provider SecretProvider) *Verifier {
	return &Verifier{provider: provider, now: time.Now}
}

// Verify parses + validates a token string and returns the embedded Claims.
// Value receiver — works for both Verifier and *Verifier interface satisfaction.
func (v Verifier) Verify(token string) (*Claims, error) {
	c := &Claims{}
	parsed, err := jwtv5.ParseWithClaims(token, c, func(t *jwtv5.Token) (any, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected alg %v", ErrTokenInvalid, t.Header["alg"])
		}
		return v.provider.Secret(), nil
	})
	if err != nil {
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
