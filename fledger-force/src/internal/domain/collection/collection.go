// Package collection defines the cash-collection domain.
package collection

import "time"

// Status enumerates the cash-collection lifecycle.
type Status string

const (
	StatusHeldBySales Status = "HELD_BY_SALES"
	StatusSettledToHQ  Status = "SETTLED_TO_HQ"
	StatusCancelled    Status = "CANCELLED"
)

// Collection is one cash receipt from a store.
type Collection struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	VisitID           *string   `json:"visit_id,omitempty"`
	SalesRepID        string    `json:"sales_rep_id"`
	StoreID           string    `json:"store_id"`
	FledgerInvoiceID  string    `json:"fledger_invoice_id"`
	ReceiptNumber     string    `json:"receipt_number"`
	Amount            int64     `json:"amount"`
	CollectedAt       time.Time `json:"collected_at"`
	PayerName         string    `json:"payer_name"`
	PayerPhone        string    `json:"payer_phone,omitempty"`
	WAReceiptSent     bool      `json:"wa_receipt_sent"`
	Status            Status    `json:"status"`
	SettlementID      *string   `json:"settlement_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}