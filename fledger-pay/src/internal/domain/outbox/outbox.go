// Package outbox defines the durable settlement outbox used to push
// "fledger.pay.settled" events to Fledger Core.
package outbox

import "time"

// Status enumerates the outbox-row lifecycle.
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusSent       Status = "SENT"
	StatusFailed     Status = "FAILED"
)

// Event is one outbox row.
type Event struct {
	ID               string         `json:"id"`
	TenantID         string         `json:"tenant_id"`
	TransactionID    string         `json:"transaction_id"`
	FledgerInvoiceID string         `json:"fledger_invoice_id"`
	EventType        string         `json:"event_type"`
	Payload          map[string]any `json:"payload"`
	Status           Status         `json:"status"`
	RetryCount       int            `json:"retry_count"`
	MaxRetries       int            `json:"max_retries"`
	LastError        string         `json:"last_error,omitempty"`
	NextRetryAt      time.Time      `json:"next_retry_at"`
	SentAt           *time.Time     `json:"sent_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}