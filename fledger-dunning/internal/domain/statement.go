// Package domain: monthly e-statement (rekening koran).
package domain

import "time"

// StatementDispatchStatus enumerates the lifecycle.
type StatementDispatchStatus string

const (
	StatementPending   StatementDispatchStatus = "PENDING"
	StatementGenerated StatementDispatchStatus = "GENERATED"
	StatementSent      StatementDispatchStatus = "SENT"
	StatementFailed    StatementDispatchStatus = "FAILED"
)

// Statement is one monthly rekening-koran PDF row.
type Statement struct {
	ID                  string                `json:"id"`
	TenantID            string                `json:"tenant_id"`
	StoreID             string                `json:"store_id"`
	StatementMonth      string                `json:"statement_month"` // YYYY-MM
	TotalInvoicedMinor  int64                 `json:"total_invoiced_minor"`
	TotalPaidMinor      int64                 `json:"total_paid_minor"`
	TotalReturnedMinor  int64                 `json:"total_returned_minor"`
	ClosingBalanceMinor int64                 `json:"closing_balance_minor"`
	PDFFilePath         string                `json:"pdf_file_path"`
	PhoneNumber         string                `json:"phone_number"`
	DispatchStatus      StatementDispatchStatus `json:"dispatch_status"`
	SentAt              *time.Time            `json:"sent_at,omitempty"`
	FailureReason       string                `json:"failure_reason,omitempty"`
	CreatedAt           time.Time             `json:"created_at"`
}

// StatementLineItem is one mutation row inside a statement.
type StatementLineItem struct {
	Date        time.Time
	Reference   string
	Description string
	DebitMinor  int64 // + invoice
	CreditMinor int64 // - payment / - return
	BalanceMinor int64
}