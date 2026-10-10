// Package coreclient is the HTTP adapter that talks to Fledger Core
// (POST /v1/transfers, GET /v1/invoices, GET /v1/aging) for the
// Hard Credit Gate pre-flight check.
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

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
)

// ARSummary is the aggregate AR / aging snapshot returned from Core.
type ARSummary struct {
	CustomerID         string         `json:"customer_id"`
	OutstandingARMinor int64          `json:"outstanding_ar_minor"`
	CreditLimitMinor   int64          `json:"credit_limit_minor"`
	OverdueBucket      OverdueBucket  `json:"overdue_bucket"`
	OverdueInvoiceID   string         `json:"overdue_invoice_id,omitempty"`
	OverdueDays        int            `json:"overdue_days"`
}

// OverdueBucket is the aging-bucket summary.
type OverdueBucket struct {
	HasOverdue30d bool `json:"has_overdue_30d"`
	HasOverdue60d bool `json:"has_overdue_60d"`
}

// Client is the HTTP client for the Fledger Core API.
type Client struct {
	baseURL    string
	tenantID   string
	apiKey     string
	httpClient *http.Client
	log        *slog.Logger
}

type Config struct {
	BaseURL  string
	TenantID string
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
		DisableKeepAlives: true,
	}
	return &Client{
		baseURL:    c.BaseURL,
		tenantID:   c.TenantID,
		apiKey:     c.APIKey,
		httpClient: &http.Client{Timeout: c.Timeout, Transport: transport},
		log:        c.Log,
	}
}

// GetARSummary reads outstanding AR + overdue aging for a customer.
func (c *Client) GetARSummary(ctx context.Context, customerID string) (*ARSummary, error) {
	if customerID == "" {
		return nil, fmt.Errorf("%w: customer_id required", apperrors.ErrInvalidInput)
	}
	// Try /v1/customers/{id}/ar-summary (preferred) then fall back to
	// /v1/aging + /v1/invoices aggregation.
	url := c.baseURL + "/v1/customers/" + customerID + "/ar-summary"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("core ar-summary failed; falling back to invoices+aging", "err", err.Error())
		return c.fallbackARSummary(ctx, customerID)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var out ARSummary
		if err := json.Unmarshal(raw, &out); err == nil {
			return &out, nil
		}
	}
	// Fallback path on 404 / non-JSON.
	return c.fallbackARSummary(ctx, customerID)
}

// fallbackARSummary computes the AR snapshot from /v1/invoices + /v1/aging.
func (c *Client) fallbackARSummary(ctx context.Context, customerID string) (*ARSummary, error) {
	invoices, err := c.listOpenInvoices(ctx, customerID)
	if err != nil {
		return nil, err
	}
	var total int64
	var overdue30 bool
	var oldestOverdue int
	var overdueInvoiceID string
	now := time.Now().UTC()
	for _, inv := range invoices {
		total += inv.OutstandingMinor
		if inv.DueDate == "" {
			continue
		}
		due, perr := time.Parse("2006-01-02", inv.DueDate)
		if perr != nil {
			continue
		}
		days := int(now.Sub(due).Hours() / 24)
		if days > 30 {
			overdue30 = true
		}
		if days > oldestOverdue {
			oldestOverdue = days
			overdueInvoiceID = inv.ID
		}
	}
	limit, _ := c.creditLimit(ctx, customerID)
	return &ARSummary{
		CustomerID:         customerID,
		OutstandingARMinor: total,
		CreditLimitMinor:   limit,
		OverdueBucket:      OverdueBucket{HasOverdue30d: overdue30},
		OverdueInvoiceID:   overdueInvoiceID,
		OverdueDays:        oldestOverdue,
	}, nil
}

type coreInvoice struct {
	ID               string `json:"id"`
	OutstandingMinor int64  `json:"amount_minor"`
	Status           string `json:"status"`
	DueDate          string `json:"due_date"`
}

func (c *Client) listOpenInvoices(ctx context.Context, customerID string) ([]coreInvoice, error) {
	url := c.baseURL + "/v1/invoices?customer_id=" + customerID + "&status=ISSUED"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%w: core %d %s", apperrors.ErrUpstreamUnavailable, resp.StatusCode, string(raw))
	}
	var out struct {
		Data []coreInvoice `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		// Accept flat array too.
		var flat []coreInvoice
		if err2 := json.Unmarshal(raw, &flat); err2 == nil {
			return flat, nil
		}
		return nil, fmt.Errorf("decode invoices: %w (raw=%s)", err, string(raw))
	}
	return out.Data, nil
}

func (c *Client) creditLimit(ctx context.Context, customerID string) (int64, error) {
	url := c.baseURL + "/v1/customers/" + customerID + "/credit-limit"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil // non-fatal; just return 0 if upstream unreachable
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return 0, nil
	}
	var out struct {
		CreditLimitMinor int64 `json:"credit_limit_minor"`
	}
	_ = json.Unmarshal(raw, &out)
	return out.CreditLimitMinor, nil
}

// CreateTransfer posts a transfer to Core (used for the
// order-fulfillment double-entry bridge — optional here, present for
// parity with the brief).
func (c *Client) CreateTransfer(ctx context.Context, from, to string, amountMinor int64, currency, description, idempotencyKey string) error {
	if amountMinor <= 0 {
		return fmt.Errorf("%w: amount_minor must be > 0", apperrors.ErrInvalidInput)
	}
	body, _ := json.Marshal(map[string]any{
		"from_account_id": from, "to_account_id": to,
		"amount_minor": amountMinor, "currency": currency,
		"description": description,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/transfers", bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setHeadersWithIdem(req, idempotencyKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: core %d %s", apperrors.ErrUpstreamUnavailable, resp.StatusCode, string(raw))
	}
	return nil
}

func (c *Client) setHeaders(req *http.Request) { c.setHeadersWithIdem(req, "") }

func (c *Client) setHeadersWithIdem(req *http.Request, idem string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
}