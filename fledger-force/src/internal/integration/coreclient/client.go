// Package coreclient is the HTTP adapter that posts transfers to Fledger
// Core (POST /v1/transfers). The contract follows the real Fledger Core
// shape (CreateTransferRequest) — NOT the simplified sample in the brief.
package coreclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	apperrors "github.com/fledger/fledger-force/internal/platform/errors"
)

// TransferInput is the shape sent to /v1/transfers.
type TransferInput struct {
	FromAccountID  string `json:"from_account_id"`
	ToAccountID    string `json:"to_account_id"`
	AmountMinor    int64  `json:"amount_minor"`
	Currency       string `json:"currency"`
	Description   string `json:"description,omitempty"`
	IdempotencyKey string `json:"-"`
}

// TransferResponse is the response shape.
type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	AmountMinor   int64  `json:"amount_minor"`
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

type Config struct {
	BaseURL  string
	TenantID string
	JWT      string
	APIKey   string
	Timeout  time.Duration
	Log      *slog.Logger
}

func NewClient(c Config) *Client {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	transport := &http.Transport{
		DisableKeepAlives: true, // critical for sequential test calls
	}
	return &Client{
		baseURL:    c.BaseURL,
		tenantID:   c.TenantID,
		jwt:        c.JWT,
		apiKey:     c.APIKey,
		httpClient: &http.Client{Timeout: c.Timeout, Transport: transport},
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
	url := c.baseURL + "/v1/transfers"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	req.Header.Set("Idempotency-Key", idem)
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("core call failed", "url", url, "err", err.Error())
		return nil, fmt.Errorf("%w: %v", apperrors.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var out TransferResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode: %w (raw=%s)", err, string(raw))
		}
		return &out, nil
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: core %d %s", apperrors.ErrUpstreamUnavailable, resp.StatusCode, string(raw))
	default:
		return nil, fmt.Errorf("%w: core %d %s", apperrors.ErrInvalidInput, resp.StatusCode, string(raw))
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
		return fmt.Errorf("core ping non-2xx")
	}
	return nil
}