// Package midtransclient provides an HTTP client for Midtrans Core API (v2/charge).
package midtransclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	ServerKey  string
	ClientKey  string
	MerchantID string
	IsProd     bool
	HTTPClient *http.Client
}

type Client struct {
	cfg     Config
	baseURL string
}

func NewClient(cfg Config) *Client {
	baseURL := "https://api.sandbox.midtrans.com"
	if cfg.IsProd {
		baseURL = "https://api.midtrans.com"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 8 * time.Second}
	}
	return &Client{
		cfg:     cfg,
		baseURL: baseURL,
	}
}

type ChargeVARequest struct {
	OrderID       string
	GrossAmount   int64
	Bank          string
	CustomerName  string
	CustomerPhone string
}

type ChargeVAResponse struct {
	TransactionID     string
	OrderID           string
	GrossAmount       string
	VANumber          string
	Bank              string
	TransactionStatus string
}

// ChargeVA creates a bank transfer charge with Midtrans.
func (c *Client) ChargeVA(ctx context.Context, req ChargeVARequest) (*ChargeVAResponse, error) {
	if c.cfg.ServerKey == "" {
		return nil, fmt.Errorf("midtrans: server key not configured")
	}

	payload := map[string]any{
		"payment_type": "bank_transfer",
		"transaction_details": map[string]any{
			"order_id":     req.OrderID,
			"gross_amount": req.GrossAmount,
		},
		"bank_transfer": map[string]any{
			"bank": strings.ToLower(req.Bank),
		},
		"customer_details": map[string]any{
			"first_name": req.CustomerName,
			"phone":      req.CustomerPhone,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal midtrans payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/charge", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create midtrans req: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.cfg.ServerKey + ":"))
	httpReq.Header.Set("Authorization", "Basic "+auth)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.cfg.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call midtrans api: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read midtrans response: %w", err)
	}

	var raw struct {
		StatusCode        string `json:"status_code"`
		StatusMessage     string `json:"status_message"`
		TransactionID     string `json:"transaction_id"`
		OrderID           string `json:"order_id"`
		GrossAmount       string `json:"gross_amount"`
		TransactionStatus string `json:"transaction_status"`
		VANumbers         []struct {
			Bank     string `json:"bank"`
			VANumber string `json:"va_number"`
		} `json:"va_numbers"`
		PermataVANumber string `json:"permata_va_number"`
		BillKey         string `json:"bill_key"`
	}

	if err := json.Unmarshal(respBytes, &raw); err != nil {
		return nil, fmt.Errorf("parse midtrans response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("midtrans error %s: %s", raw.StatusCode, raw.StatusMessage)
	}

	vaNum := ""
	if len(raw.VANumbers) > 0 {
		vaNum = raw.VANumbers[0].VANumber
	} else if raw.PermataVANumber != "" {
		vaNum = raw.PermataVANumber
	} else if raw.BillKey != "" {
		vaNum = raw.BillKey
	}

	if vaNum == "" {
		return nil, fmt.Errorf("midtrans: no VA number returned in response")
	}

	return &ChargeVAResponse{
		TransactionID:     raw.TransactionID,
		OrderID:           raw.OrderID,
		GrossAmount:       raw.GrossAmount,
		VANumber:          vaNum,
		Bank:              strings.ToUpper(req.Bank),
		TransactionStatus: raw.TransactionStatus,
	}, nil
}
