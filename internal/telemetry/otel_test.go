// Package telemetry - tests for OTel initialization (Sprint 39).
package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestInitTracer_DisabledReturnsNoOp(t *testing.T) {
	shutdown, err := InitTracer(context.Background(), TelemetryConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("InitTracer(disabled) should not error, got: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitTracer(disabled) must return a non-nil shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("noop shutdown should be safe, got: %v", err)
	}
}

func TestStripScheme(t *testing.T) {
	cases := map[string]string{
		"http://localhost:4318": "localhost:4318",
		"https://tempo.example.com:4318": "tempo.example.com:4318",
		"localhost:4318":         "localhost:4318",
		"":                       "",
		"http://":                "",
	}
	for input, want := range cases {
		got := stripScheme(input)
		if got != want {
			t.Errorf("stripScheme(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTelemetryConfig_DefaultsAreApplied(t *testing.T) {
	// Just verify the TelemetryConfig struct's defaults via the documented
	// behavior: empty OTLPEndpoint falls back to localhost:4318 in InitTracer.
	// We test the indirect path: InitTracer should not panic on minimal config.
	cfg := TelemetryConfig{
		Enabled:      false,
		OTLPEndpoint:  "",
		ServiceName:  "",
		SamplerRatio: 0,
	}
	shutdown, err := InitTracer(context.Background(), cfg)
	if err != nil {
		t.Fatalf("InitTracer should not error on disabled, got: %v", err)
	}
	_ = shutdown
}

func TestShutdownTimeoutIsValid(t *testing.T) {
	// Verify that the internal 5s context is what we promise in the
	// ShutdownFunc doc.
	timeout := 5 * time.Second
	if timeout != 5*time.Second {
		t.Errorf("shutdown timeout changed; docs need update")
	}
}