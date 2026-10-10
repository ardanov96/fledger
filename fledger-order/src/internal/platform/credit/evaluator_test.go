package credit

import (
	"testing"
	"time"
)

// TestIsOrderExpired covers the small helper.
func TestIsOrderExpired(t *testing.T) {
	if IsOrderExpired(time.Now().Add(-6 * time.Minute)) != true {
		t.Fatal("expected expired")
	}
	if IsOrderExpired(time.Now().Add(-1 * time.Minute)) != false {
		t.Fatal("expected NOT expired")
	}
}
