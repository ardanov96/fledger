package config

import (
	"os"
	"testing"
)

// TestLoad_Success covers the happy-path env wiring used by main.go.
func TestLoad_Success(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5432/z")
	t.Setenv("FLEDGER_CORE_URL", "http://localhost:8081")
	t.Setenv("FLEDGER_TENANT_ID", "00000000-0000-0000-0000-000000000001")
	t.Setenv("JWT_SECRET", "super-secret-key-32-characters-minimum-fleet")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != 8082 {
		t.Errorf("port default = %d", c.Port)
	}
	if c.DBMaxOpenConns != 25 {
		t.Errorf("conns default = %d", c.DBMaxOpenConns)
	}
}

// TestLoad_RequiresEveryField proves the config loader refuses to boot with
// missing secrets (DoD: no hard-coded creds, no silent fallbacks).
func TestLoad_RequiresEveryField(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("FLEDGER_CORE_URL")
	os.Unsetenv("FLEDGER_TENANT_ID")
	os.Unsetenv("JWT_SECRET")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error for empty config")
	}
}

// TestLoad_RejectsShortJWTSecret guards against misconfigured secrets.
func TestLoad_RejectsShortJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5432/z")
	t.Setenv("FLEDGER_CORE_URL", "http://localhost:8081")
	t.Setenv("FLEDGER_TENANT_ID", "00000000-0000-0000-0000-000000000001")
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := Load(); err == nil {
		t.Fatalf("expected JWT_SECRET too-short error")
	}
}