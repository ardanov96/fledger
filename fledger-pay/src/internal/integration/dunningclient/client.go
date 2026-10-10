// Package dunningclient is the HTTP client for the Fledger Dunning
// payment.settled webhook endpoint. It is invoked from the Fledger Pay
// settlement path once a payment has been confirmed and the dunning
// queue for the invoice needs to be cancelled (self-healing loop).
package dunningclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PaymentSettledPayload is the body POSTed to the dunning webhook.
type PaymentSettledPayload struct {
	Event          string `json:"event"`
	InvoiceID      string `json:"invoice_id"`
	PaidAmountMinor int64 `json:"paid_amount_minor"`
	PaidAt         string `json:"paid_at"`
	SettlementRef  string `json:"settlement_ref"`
}

// Client is the HTTP client for the Fledger Dunning webhook.
type Client struct {
	baseURL       string
	webhookSecret string
	tenantID      string
	httpClient    *http.Client
}

// NewClient returns a configured client. baseURL is the Fledger Dunning
// base URL (e.g. http://dunning:8086 inside the docker-compose network).
func NewClient(baseURL, webhookSecret, tenantID string) *Client {
	return &Client{
		baseURL:       baseURL,
		webhookSecret: webhookSecret,
		tenantID:      tenantID,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
	}
}

// NotifyPaymentSettled POSTs a payment.settled event to the dunning webhook.
// A nil error indicates the request reached the server and was acknowledged
// (200 OK). A non-nil error means the request failed or returned a 4xx/5xx —
// callers should treat this as best-effort: a failed dunning cancel does
// not block the payment settlement.
func (c *Client) NotifyPaymentSettled(ctx context.Context, invoiceID string, amount int64, ref string) error {
	if c.baseURL == "" || invoiceID == "" {
		return nil
	}
	payload := PaymentSettledPayload{
		Event:          "payment.settled",
		InvoiceID:      invoiceID,
		PaidAmountMinor: amount,
		PaidAt:         time.Now().UTC().Format(time.RFC3339),
		SettlementRef:  ref,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/dunning/webhooks/pay", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.tenantID != "" {
		req.Header.Set("X-Tenant-ID", c.tenantID)
	}
	if c.webhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", c.webhookSecret)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("dunning webhook returned status %d", resp.StatusCode)
	}
	return nil
}