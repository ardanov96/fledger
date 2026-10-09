// Package coreclient is the HTTP adapter that posts to Fledger Core
// (POST /v1/transfers, POST /v1/invoices/:id/pay) on settlement.
//
// The contract is the **real** Fledger Core contract observed in
// fledger-core/internal/handler/handlers.go and dto.go (CreateInvoiceRequest +
// CreateTransferRequest). It is NOT the sample payload from the brief.
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

	apperrors "github.com/fledger/fledger-pay/internal/platform/errors"
)

// TransferInput is the shape sent to /v1/transfers.
type TransferInput struct {
	FromAccountID     string `json:"from_account_id"`
	ToAccountID       string `json:"to_account_id"`
	AmountMinor       int64  `json:"amount_minor"`
	Currency          string `json:"currency"`
	Description       string `json:"description,omitempty"`
	IdempotencyKey    string `json:"-"`
}

// TransferResponse is the response shape.
type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	AmountMinor   int64  `json:"amount_minor"`
}

// InvoicePayResponse is the response shape from POST /v1/invoices/:id/pay.
type InvoicePayResponse struct {
	InvoiceID string `json:"invoice_id"`
	Status    string `json:"status"`
	PaidMinor int64  `json:"paid_minor"`
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

// CreateTransfer posts a transfer to Core.
func (c *Client) CreateTransfer(ctx context.Context, in TransferInput) (*TransferResponse, error) {
	if in.AmountMinor <= 0 {
		return nil, fmt.Errorf("%w: amount_minor must be > 0", apperrors.ErrInvalidInput)
	}
	if in.FromAccountID == "" || in.ToAccountID == "" || in.Currency == "" {
		return nil, fmt.Errorf("%w: from/to/currency required", apperrors.ErrInvalidInput)
	}
	body, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	idem := in.IdempotencyKey
	if idem == "" {
		idem = in.FromAccountID + "->" + in.ToAccountID
	}
	var out TransferResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/transfers", body, idem, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MarkInvoicePaid calls POST /v1/invoices/:id/pay.
func (c *Client) MarkInvoicePaid(ctx context.Context, invoiceID, idempotencyKey string) (*InvoicePayResponse, error) {
	body := []byte(`{}`)
	var out InvoicePayResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/invoices/"+invoiceID+"/pay", body, idempotencyKey, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body []byte, idem string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("core call failed", "path", path, "err", err.Error())
		return fmt.Errorf("%w: %v", apperrors.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if out != nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, out); err != nil {
				return fmt.Errorf("decode response: %w (raw=%s)", err, string(raw))
			}
		}
		return nil
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: core %d %s", apperrors.ErrUpstreamUnavailable, resp.StatusCode, string(raw))
	default:
		return fmt.Errorf("%w: core %d %s", apperrors.ErrInvalidInput, resp.StatusCode, string(raw))
	}
}

// Ping returns nil if /v1/ping on Core responds OK.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/ping", nil)
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