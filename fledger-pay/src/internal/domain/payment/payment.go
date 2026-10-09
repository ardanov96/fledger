// Package payment defines the payment-request domain.
package payment

import "time"

// Status enumerates the lifecycle of a payment request.
type Status string

const (
	StatusPending         Status = "PENDING"
	StatusPartiallyPaid   Status = "PARTIALLY_PAID"
	StatusSettled         Status = "SETTLED"
	StatusExpired         Status = "EXPIRED"
	StatusCancelled       Status = "CANCELLED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusPartiallyPaid, StatusSettled, StatusExpired, StatusCancelled:
		return true
	}
	return false
}

// Request is one payment request bound to a Fledger Core invoice.
type Request struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	FledgerInvoiceID string         `json:"fledger_invoice_id"`
	RequestNumber    string         `json:"request_number"`
	CustomerID       string         `json:"customer_id"`
	CustomerName     string         `json:"customer_name"`
	CustomerPhone    string         `json:"customer_phone,omitempty"`
	Currency         string         `json:"currency"`
	Amount           int64          `json:"amount"`
	FeeAmount        int64          `json:"fee_amount"`
	TotalAmount      int64          `json:"total_amount"`
	Status           Status         `json:"status"`
	ExpiresAt        time.Time      `json:"expires_at"`
	SettledAt        *time.Time     `json:"settled_at,omitempty"`
	SettledAmount    int64          `json:"settled_amount"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}