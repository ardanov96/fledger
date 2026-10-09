// Package qris defines the dynamic-QRIS domain.
package qris

import "time"

// Code is one dynamic QRIS code bound to a payment request.
type Code struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	PaymentRequestID  string    `json:"payment_request_id"`
	QRString          string    `json:"qr_string"`
	QRImageURL        string    `json:"qr_image_url,omitempty"`
	ExpectedAmount    int64     `json:"expected_amount"`
	Status            string    `json:"status"`
	ExpiresAt         time.Time `json:"expires_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}