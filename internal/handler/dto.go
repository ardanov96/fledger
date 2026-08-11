// Package handler contains HTTP handlers that translate REST requests into
// use case calls and back.
//
// Handlers are thin — they only:
//   1. Parse + validate the request (using validator tags)
//   2. Call the appropriate use case
//   3. Translate the result/error to an HTTP response
//
// All business logic stays in internal/usecase.
package handler

import (
	"time"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	"github.com/runut/fmcg-wallet/internal/domain/ledger"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// Transfer endpoints
// =============================================================================

// TransferRequest is the body of POST /v1/transfers.
type TransferRequest struct {
	FromAccountID  string `json:"from_account_id" validate:"required,uuid"`
	ToAccountID    string `json:"to_account_id"   validate:"required,uuid"`
	AmountMinor    int64  `json:"amount_minor"     validate:"required,gt=0"`
	Currency       string `json:"currency"         validate:"required,len=3"`
	Description    string `json:"description"      validate:"required,max=500"`
	IdempotencyKey string `json:"-"                validate:"required,min=1,max=200"`
	RefType        string `json:"ref_type,omitempty"  validate:"omitempty,max=50"`
	RefID          string `json:"ref_id,omitempty"    validate:"omitempty,uuid"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// TransferResponse is the body of a successful transfer response.
type TransferResponse struct {
	TransactionID string         `json:"transaction_id"`
	Status        string         `json:"status"`
	FromAccountID string         `json:"from_account_id"`
	ToAccountID   string         `json:"to_account_id"`
	AmountMinor   int64          `json:"amount_minor"`
	Currency      string         `json:"currency"`
	Description   string         `json:"description"`
	PostedAt      time.Time      `json:"posted_at"`
	Entries       []EntryDTO     `json:"entries"`
}

// EntryDTO is the public representation of a ledger entry.
type EntryDTO struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	AmountMinor   int64     `json:"amount_minor"`
	Type          string    `json:"type"` // "debit" | "credit"
	Description   string    `json:"description,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// =============================================================================
// Account endpoints
// =============================================================================

// CreateAccountRequest is the body of POST /v1/accounts.
type CreateAccountRequest struct {
	Code     string         `json:"code"     validate:"required,min=1,max=50"`
	Name     string         `json:"name"     validate:"required,min=1,max=200"`
	Type     string         `json:"type"     validate:"required,oneof=hq outlet sales_rep customer revenue receivable payable cash suspense"`
	Currency string         `json:"currency" validate:"required,len=3"`
	OwnerID  string         `json:"owner_id,omitempty" validate:"omitempty,uuid"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// AccountResponse is the body of account endpoints.
type AccountResponse struct {
	ID            string         `json:"id"`
	Code          string         `json:"code"`
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Status        string         `json:"status"`
	Currency      string         `json:"currency"`
	BalanceMinor  int64          `json:"balance_minor"`
	OwnerID       string         `json:"owner_id,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// =============================================================================
// Invoice endpoints
// =============================================================================

// CreateInvoiceRequest is the body of POST /v1/invoices.
type CreateInvoiceRequest struct {
	CustomerID  string         `json:"customer_id"  validate:"required,uuid"`
	Code        string         `json:"code"         validate:"required,min=1,max=50"`
	AmountMinor int64          `json:"amount_minor" validate:"required,gt=0"`
	DueDate     string         `json:"due_date"     validate:"required,datetime=2006-01-02"`
	Description string         `json:"description"  validate:"omitempty,max=500"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ToCreateInvoiceInput converts the HTTP request to the use case input.
func (r *CreateInvoiceRequest) ToCreateInvoiceInput(tenantID string) invoice.CreateInvoiceInput {
	due, _ := time.Parse("2006-01-02", r.DueDate)
	return invoice.CreateInvoiceInput{
		TenantID:    tenantID,
		CustomerID:  r.CustomerID,
		Code:        r.Code,
		Amount:      money.NewFromMinor(r.AmountMinor),
		DueDate:     due,
		Description: r.Description,
		Metadata:    r.Metadata,
	}
}

// InvoiceResponse is the public representation of an invoice.
type InvoiceResponse struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	Code           string    `json:"code"`
	AmountMinor    int64     `json:"amount_minor"`
	PaidMinor      int64     `json:"paid_minor"`
	OutstandingMinor int64   `json:"outstanding_minor"`
	Currency       string    `json:"currency"`
	DueDate        string    `json:"due_date"`
	Status         string    `json:"status"`
	IssuedAt       time.Time `json:"issued_at"`
	Description    string    `json:"description,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RecordPaymentRequest is the body of POST /v1/customers/{id}/payments.
//
// In manual mode (`X-Allocation-Mode: manual`), `Allocations` is required
// and must sum to `AmountMinor`. In FIFO mode (default), `Allocations`
// is ignored.
type RecordPaymentRequest struct {
	AmountMinor  int64                  `json:"amount_minor"  validate:"required,gt=0"`
	Method       string                 `json:"method"       validate:"omitempty,oneof=cash transfer qris manual_adjust"`
	Description  string                 `json:"description"  validate:"omitempty,max=500"`
	Allocations  []PaymentAllocationDTO `json:"allocations"  validate:"omitempty,dive"`
}

// PaymentAllocationDTO is one slice of a manual payment.
type PaymentAllocationDTO struct {
	InvoiceID   string `json:"invoice_id"   validate:"required,uuid"`
	AmountMinor int64  `json:"amount_minor" validate:"required,gt=0"`
}

// PaymentResultResponse is the response of POST /v1/customers/{id}/payments.
type PaymentResultResponse struct {
	PaymentID    string                       `json:"payment_id"`
	CustomerID   string                       `json:"customer_id"`
	Method       string                       `json:"method"`
	TotalMinor   int64                        `json:"total_minor"`
	Allocations  []PaymentAllocationResponse `json:"allocations"`
}

// PaymentAllocationResponse is one applied allocation.
type PaymentAllocationResponse struct {
	InvoiceID   string `json:"invoice_id"`
	AmountMinor int64  `json:"amount_minor"`
}

// AgingSummaryResponse is the per-bucket aggregation for a customer.
type AgingSummaryResponse struct {
	Bucket           string `json:"bucket"`
	Count            int    `json:"count"`
	OutstandingMinor int64  `json:"outstanding_minor"`
}

// SetCreditLimitRequest is the body of POST /v1/customers/{id}/credit-limit.
type SetCreditLimitRequest struct {
	LimitAmountMinor int64 `json:"limit_amount_minor" validate:"gte=0"`
}

// =============================================================================
// Conversion helpers
// =============================================================================

// ToTransferInput converts the HTTP request to a use case input.
func (r *TransferRequest) ToTransferInput(tenantID string) ledger.TransferInput {
	return ledger.TransferInput{
		FromAccountID:  r.FromAccountID,
		ToAccountID:    r.ToAccountID,
		Amount:         money.NewFromMinor(r.AmountMinor),
		Description:    r.Description,
		IdempotencyKey: r.IdempotencyKey,
		InitiatorID:    "", // set from auth context in Fase 2
		RefType:        r.RefType,
		RefID:          r.RefID,
		Metadata:       r.Metadata,
		// TenantID comes from the JWT in Fase 2; for now, use the
		// source account's tenant (resolved by the use case).
	}
}

// ToTransferResponse converts a use case result to the HTTP response.
func ToTransferResponse(t ledger.Transaction) TransferResponse {
	entries := make([]EntryDTO, 0, len(t.Entries))
	for _, e := range t.Entries {
		entries = append(entries, ToEntryDTO(e))
	}
	postedAt := time.Time{}
	if t.PostedAt != nil {
		postedAt = *t.PostedAt
	}
	return TransferResponse{
		TransactionID: t.ID,
		Status:        string(t.Status),
		// FromAccountID/ToAccountID: derived from entries
		AmountMinor: 0, // filled in by handler
		Currency:     "", // filled in by handler
		Description:  t.Description,
		PostedAt:     postedAt,
		Entries:      entries,
	}
}

// ToEntryDTO converts a domain entry to its public DTO.
func ToEntryDTO(e ledger.Entry) EntryDTO {
	return EntryDTO{
		ID:            e.ID,
		TransactionID: e.TransactionID,
		AccountID:     e.AccountID,
		AmountMinor:   e.Amount.Minor(),
		Type:          string(e.Type),
		Description:   e.Description,
		CreatedAt:     e.CreatedAt,
	}
}

// ToAccountResponse converts a domain account to the HTTP response.
func ToAccountResponse(a ledger.Account) AccountResponse {
	return AccountResponse{
		ID:           a.ID,
		Code:         a.Code,
		Name:         a.Name,
		Type:         string(a.Type),
		Status:       string(a.Status),
		Currency:     a.Currency,
		BalanceMinor: a.CachedBalance.Minor(),
		OwnerID:      a.OwnerID,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// ToInvoiceResponse converts a domain invoice to the HTTP response.
func ToInvoiceResponse(i invoice.Invoice) InvoiceResponse {
	return InvoiceResponse{
		ID:               i.ID,
		CustomerID:       i.CustomerID,
		Code:             i.Code,
		AmountMinor:      i.Amount.Minor(),
		PaidMinor:        i.PaidAmount.Minor(),
		OutstandingMinor: i.Outstanding().Minor(),
		Currency:         "IDR",
		DueDate:          i.DueDate.Format("2006-01-02"),
		Status:           string(i.Status),
		IssuedAt:         i.IssuedAt,
		Description:      i.Description,
		CreatedAt:        i.CreatedAt,
		UpdatedAt:        i.UpdatedAt,
	}
}

// ToPaymentResultResponse converts a domain payment result to HTTP.
func ToPaymentResultResponse(p invoice.PaymentResult) PaymentResultResponse {
	allocs := make([]PaymentAllocationResponse, 0, len(p.Allocations))
	for _, a := range p.Allocations {
		allocs = append(allocs, PaymentAllocationResponse{
			InvoiceID:   a.InvoiceID,
			AmountMinor: a.Amount.Minor(),
		})
	}
	return PaymentResultResponse{
		PaymentID:   p.PaymentID,
		CustomerID:  p.CustomerID,
		Method:      string(p.Method),
		TotalMinor:  p.TotalMinor,
		Allocations: allocs,
	}
}

// ToAgingSummaryResponse converts a domain aging summary to HTTP.
func ToAgingSummaryResponse(s invoice.AgingSummary) AgingSummaryResponse {
	return AgingSummaryResponse{
		Bucket:           string(s.Bucket),
		Count:            s.Count,
		OutstandingMinor: s.OutstandingMinor,
	}
}
