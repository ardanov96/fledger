// Package transaction defines the payment-transaction domain (one row per
// successful gateway callback).
package transaction

import "time"

// Channel enumerates supported payment channels.
type Channel string

const (
	ChannelVABCA      Channel = "VA_BCA"
	ChannelVAMandiri  Channel = "VA_MANDIRI"
	ChannelVABRI      Channel = "VA_BRI"
	ChannelVABNI      Channel = "VA_BNI"
	ChannelVAPermata  Channel = "VA_PERMATA"
	ChannelQRIS       Channel = "QRIS"
	ChannelManual     Channel = "MANUAL_TRANSFER"
)

func (c Channel) Valid() bool {
	switch c {
	case ChannelVABCA, ChannelVAMandiri, ChannelVABRI, ChannelVABNI,
		ChannelVAPermata, ChannelQRIS, ChannelManual:
		return true
	}
	return false
}

// Transaction is one bank callback / simulator event.
type Transaction struct {
	ID                string         `json:"id"`
	TenantID          string         `json:"tenant_id"`
	PaymentRequestID  string         `json:"payment_request_id"`
	Channel           Channel        `json:"channel"`
	ExternalReference string         `json:"external_reference"`
	IdempotencyKey    string         `json:"idempotency_key"`
	GrossAmount       int64          `json:"gross_amount"`
	NetAmount         int64          `json:"net_amount"`
	FeeAmount         int64          `json:"fee_amount"`
	PaidAt            time.Time      `json:"paid_at"`
	PayerName         string         `json:"payer_name,omitempty"`
	PayerBank         string         `json:"payer_bank,omitempty"`
	RawPayload        map[string]any `json:"raw_payload,omitempty"`
	SignatureVerified bool           `json:"signature_verified"`
	CreatedAt         time.Time      `json:"created_at"`
}