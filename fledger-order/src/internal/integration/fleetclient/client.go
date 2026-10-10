// Package fleetclient is the HTTP adapter that posts delivery orders to
// Fledger Fleet (POST /v1/fleet/delivery-orders).
package fleetclient

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

// DeliveryOrderInput is the shape we POST to Fleet.
type DeliveryOrderInput struct {
	DoNumber           string         `json:"do_number"`
	CustomerID         string         `json:"customer_id"`
	CustomerName       string         `json:"customer_name"`
	DestinationAddress string         `json:"destination_address"`
	CustomerPhone      string         `json:"customer_phone,omitempty"`
	TotalNominal       int64          `json:"total_nominal,omitempty"`
	TotalWeightKg      int            `json:"total_weight_kg,omitempty"`
	Items              []DeliveryItem `json:"items"`
}

// DeliveryItem is one SKU on a delivery order.
type DeliveryItem struct {
	ProductSKU     string `json:"product_sku"`
	ProductName    string `json:"product_name"`
	QtyOrdered     int    `json:"qty_ordered"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	SkuID          string `json:"sku_id,omitempty"`
	Name           string `json:"name,omitempty"`
	Quantity       int    `json:"quantity,omitempty"`
	UnitPrice      int64  `json:"unit_price,omitempty"`
}

// DeliveryOrderResponse is the Fleet response.
type DeliveryOrderResponse struct {
	ID          string `json:"id"`
	DoNumber    string `json:"do_number"`
	Status      string `json:"status"`
	DispatchedAt string `json:"dispatched_at"`
}

// Client is the HTTP client for the Fledger Fleet API.
type Client struct {
	baseURL    string
	tenantID   string
	apiKey     string
	jwt        string
	httpClient *http.Client
	log        *slog.Logger
}

type Config struct {
	BaseURL  string
	TenantID string
	APIKey   string
	JWT      string
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
	transport := &http.Transport{DisableKeepAlives: true}
	return &Client{
		baseURL: c.BaseURL, tenantID: c.TenantID, apiKey: c.APIKey, jwt: c.JWT,
		httpClient: &http.Client{Timeout: c.Timeout, Transport: transport},
		log: c.Log,
	}
}

// CreateDeliveryOrder posts a DO to Fleet.
func (c *Client) CreateDeliveryOrder(ctx context.Context, in DeliveryOrderInput, idempotencyKey string) (*DeliveryOrderResponse, error) {
	if in.DoNumber == "" {
		return nil, fmt.Errorf("%w: do_number required", apperrors.ErrInvalidInput)
	}
	if in.TotalNominal <= 0 {
		return nil, fmt.Errorf("%w: total_nominal must be > 0", apperrors.ErrInvalidInput)
	}
	body, _ := json.Marshal(in)
	url := c.baseURL + "/v1/fleet/delivery-orders"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantID)
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if c.jwt != "" {
		req.Header.Set("Authorization", "Bearer "+c.jwt)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Warn("fleet call failed", "err", err.Error())
		return nil, fmt.Errorf("%w: %v", apperrors.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		var out DeliveryOrderResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("decode: %w (raw=%s)", err, string(raw))
		}
		return &out, nil
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: fleet %d %s", apperrors.ErrUpstreamUnavailable, resp.StatusCode, string(raw))
	default:
		return nil, fmt.Errorf("%w: fleet %d %s", apperrors.ErrInvalidInput, resp.StatusCode, string(raw))
	}
}

// Ping returns nil if /v1/ping on Fleet responds OK.
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
		return fmt.Errorf("fleet ping non-2xx")
	}
	return nil
}