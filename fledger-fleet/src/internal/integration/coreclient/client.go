// Package coreclient is the HTTP adapter that talks to Fledger Core
// (POST /v1/invoices) to settle POD-driven invoices.
//
// The contract here is the **real** Fledger Core contract observed in
// fledger-core/internal/handler/dto.go::CreateInvoiceRequest, NOT the
// sample payload from the original brief (which used field names like
// "amount" / "invoice_number" that don't exist on the live endpoint).
//
// Real payload shape (PostgreSQL `invoices` table also reflects this):
//   customer_id  string  (uuid of a `customer`-type account in Core)
//   code         string  (human-readable invoice number; unique per tenant)
//   amount_minor int64   (nominal in IDR minor units)
//   due_date     string  (YYYY-MM-DD)
//   description  string
//   metadata     map
//
// Required headers:
//   Authorization: Bearer <JWT>
//   X-Tenant-ID:   <tenant-uuid>
package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
)

// InvoiceInput is the shape we send to Core.
type InvoiceInput struct {
	CustomerID    string         `json:"customer_id"`
	Code          string         `json:"code"`
	AmountMinor   int64          `json:"amount_minor"`
	DueDate       string         `json:"due_date"` // YYYY-MM-DD
	Description   string         `json:"description,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	// IdempotencyKey is forwarded as the `Idempotency-Key` header. If empty
	// the client falls back to the invoice `code`. This guarantees re-POSTs
	// with the same key collapse to a single invoice in Core.
	IdempotencyKey string `json:"-"`
}

// InvoiceResponse is the published response shape from Core.
type InvoiceResponse struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	Code           string    `json:"code"`
	AmountMinor    int64     `json:"amount_minor"`
	PaidMinor      int64     `json:"paid_minor"`
	DueDate        time.Time `json:"due_date"`
	IssuedAt       time.Time `json:"issued_at"`
	Status         string    `json:"status"`
	Description    string    `json:"description,omitempty"`
	PeriodID       string    `json:"period_id"`
}

// Client is the HTTP client for the Fledger Core API.
type Client struct {
	baseURL    string
	tenantID   string
	jwt        string
	apiKey     string
	httpClient *http.Client
	log        *slog.Logger
}

// Config is the dependency bundle.
type Config struct {
	BaseURL  string
	TenantID string
	JWT      string
	APIKey   string
	Timeout  time.Duration
	Log      *slog.Logger
}

// NewClient constructs a Client. Timeout defaults to 10s when zero.
func NewClient(c Config) *Client {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	return &Client{
		baseURL:    c.BaseURL,
		tenantID:   c.TenantID,
		jwt:        c.JWT,
		apiKey:     c.APIKey,
		httpClient: &http.Client{Timeout: c.Timeout},
		log:        c.Log,
	}
}

// CreateInvoice posts an invoice to Core. On 5xx, network errors, or timeout
// it returns apperrors.ErrUpstreamUnavailable so the caller can enqueue.
//
// Per Fledger Fleet / AGENT-EXECUTION-BRIEF.md §5.1, every call carries an
// Idempotency-Key header (defaulting to the caller-supplied IdempotencyKey,
// otherwise the invoice `code`) so retried POSTs do not produce duplicate
// invoices in Core.
func (c *Client) CreateInvoice(ctx context.Context, in InvoiceInput) (*InvoiceResponse, error) {
	if in.AmountMinor <= 0 {
		return nil, fmt.Errorf("%w: amount_minor must be > 0", apperrors.ErrInvalidInput)
	}
	if in.CustomerID == "" || in.Code == "" || in.DueDate == "" {
		return nil, fmt.Errorf("%w: customer_id, code, due_date are required", apperrors.ErrInvalidInput)
	}

	body, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal invoice: %w", err)
	}

	idemKey := in.IdempotencyKey
	if idemKey == "" {
		idemKey = in.Code
	}

	url := c.baseURL + "/v1/invoices"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	req.Header.Set("Idempotency-Key", idemKey)
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("core invoice call failed",
			"url", url,
			"err", err.Error(),
		)
		return nil, fmt.Errorf("%w: %v", apperrors.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode == http.StatusCreated, resp.StatusCode == http.StatusOK:
		var out InvoiceResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode invoice response: %w (raw=%s)", err, string(raw))
		}
		return &out, nil
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: core 5xx %d %s", apperrors.ErrUpstreamUnavailable, resp.StatusCode, string(raw))
	default:
		// 4xx — caller bug or Core validation failure. Surface verbatim.
		return nil, fmt.Errorf("%w: core %d %s", apperrors.ErrInvalidInput, resp.StatusCode, string(raw))
	}
}

// Ping returns nil if /v1/ping on Core responds OK. Useful for the worker to
// decide whether to retry.
func (c *Client) Ping(ctx context.Context) error {
	url := c.baseURL + "/v1/ping"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return errors.New("core ping non-2xx")
	}
	return nil
}