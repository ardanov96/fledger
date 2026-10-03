// quickbcrypt.go - test-only bcrypt helper with low cost.
//
// Keeps E2E test setup fast (real bcrypt cost 10+ would add 50-100ms per hash).
// At cost 4, a single hash is ~1ms — fine for the few users we seed in tests.
//
// PRODUCTION CODE MUST USE bcrypt.DefaultCost (or higher) — see auth_repo.go.
package main

import (
	"golang.org/x/crypto/bcrypt"
)

// quickBcrypt hashes password at the given cost (callers use 4 for tests).
func quickBcrypt(password string, cost int) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}