package coreclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_CreateInvoice_Success confirms the happy path against a stub
// server and that the body matches the live Fledger Core contract
// (customer_id, code, amount_minor, due_date, description) and that the
// Idempotency-Key header is set per §5.1 of the brief.
func TestClient_CreateInvoice_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/invoices", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "tenant-x", r.Header.Get("X-Tenant-ID"))
		assert.Equal(t, "Bearer svc-jwt", r.Header.Get("Authorization"))
		// Idempotency-Key: brief §5.1 — value MUST be the DO UUID.
		assert.Equal(t, "do-uuid-abc", r.Header.Get("Idempotency-Key"))

		var got InvoiceInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "cust-1", got.CustomerID)
		assert.Equal(t, "INV-DO-202610-0089", got.Code)
		assert.EqualValues(t, 4_000_000, got.AmountMinor)
		assert.Equal(t, "2026-10-22", got.DueDate)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(InvoiceResponse{ID: "inv-uuid-99", Code: got.Code, AmountMinor: got.AmountMinor})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, TenantID: "tenant-x", JWT: "svc-jwt"})
	resp, err := c.CreateInvoice(context.Background(), InvoiceInput{
		CustomerID:     "cust-1",
		Code:           "INV-DO-202610-0089",
		AmountMinor:    4_000_000,
		DueDate:        "2026-10-22",
		Description:    "test",
		IdempotencyKey: "do-uuid-abc",
	})
	require.NoError(t, err)
	assert.Equal(t, "inv-uuid-99", resp.ID)
	assert.EqualValues(t, 4_000_000, resp.AmountMinor)
}

// TestClient_CreateInvoice_FallsBackToCode ensures the client sends some
// Idempotency-Key (never blank) so Core never sees an empty header.
func TestClient_CreateInvoice_FallsBackToCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "INV-DO-X", r.Header.Get("Idempotency-Key"))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(InvoiceResponse{ID: "ok"})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, TenantID: "t"})
	_, err := c.CreateInvoice(context.Background(), InvoiceInput{
		CustomerID: "c", Code: "INV-DO-X", AmountMinor: 1, DueDate: "2026-10-22",
	})
	require.NoError(t, err)
}

// TestClient_CreateInvoice_UpstreamUnavailable covers the 5xx / network failure
// path used by the POD service to decide whether to enqueue to the outbox.
func TestClient_CreateInvoice_UpstreamUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"db down"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, TenantID: "tenant-x"})
	_, err := c.CreateInvoice(context.Background(), InvoiceInput{
		CustomerID: "c", Code: "INV-X", AmountMinor: 1, DueDate: "2026-10-22",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upstream")
}

// TestClient_CreateInvoice_InvalidInput covers 4xx validation errors.
func TestClient_CreateInvoice_InvalidInput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, TenantID: "tenant-x"})
	_, err := c.CreateInvoice(context.Background(), InvoiceInput{
		CustomerID: "c", Code: "INV-X", AmountMinor: 1, DueDate: "2026-10-22",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input")
}

// TestClient_CreateInvoice_RejectsZero guards against accidentally submitting a
// zero-amount invoice (which Core also rejects via the amount_positive CHECK).
func TestClient_CreateInvoice_RejectsZero(t *testing.T) {
	c := NewClient(Config{BaseURL: "http://localhost", TenantID: "t"})
	_, err := c.CreateInvoice(context.Background(), InvoiceInput{
		CustomerID: "c", Code: "INV-X", AmountMinor: 0, DueDate: "2026-10-22",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount_minor must be > 0")
}