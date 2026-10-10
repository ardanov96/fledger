// Package domain: dunning queue row.
package domain

import "time"

// DunningStage enumerates the 5-stage cadence plus MANUAL_REMINDER.
type DunningStage string

const (
	StagePreDueH3    DunningStage = "PRE_DUE_H3"
	StageDueDate     DunningStage = "DUE_DATE"
	StageOverdueH3   DunningStage = "OVERDUE_H3"
	StageOverdueH7   DunningStage = "OVERDUE_H7"
	StageOverdueH14  DunningStage = "OVERDUE_H14"
	StageManual      DunningStage = "MANUAL_REMINDER"
)

// DunningStatus enumerates the row lifecycle.
type DunningStatus string

const (
	StatusQueued             DunningStatus = "QUEUED"
	StatusProcessing         DunningStatus = "PROCESSING"
	StatusSent               DunningStatus = "SENT"
	StatusFailed             DunningStatus = "FAILED"
	StatusCancelledByPayment DunningStatus = "CANCELLED_BY_PAYMENT"
)

// QueueItem is one scheduled message in the dunning pipeline.
type QueueItem struct {
	ID              string        `json:"id"`
	TenantID        string        `json:"tenant_id"`
	InvoiceID       string        `json:"invoice_id"`
	InvoiceNumber   string        `json:"invoice_number"`
	StoreID         string        `json:"store_id"`
	PhoneNumber     string        `json:"phone_number"`
	Stage           DunningStage  `json:"stage"`
	DueDate         time.Time     `json:"due_date"`
	AmountDueMinor  int64         `json:"amount_due_minor"`
	PaymentLinkURL  string        `json:"payment_link_url"`
	MessageBody     string        `json:"message_body"`
	Status         DunningStatus   `json:"status"`
	ScheduledAt     time.Time     `json:"scheduled_at"`
	SentAt         *time.Time     `json:"sent_at,omitempty"`
	FailureReason  string        `json:"failure_reason,omitempty"`
	RetryCount      int           `json:"retry_count"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}