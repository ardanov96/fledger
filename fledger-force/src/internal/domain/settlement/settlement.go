// Package settlement defines the EOD settlement domain.
package settlement

import "time"

// Status enumerates the settlement outcome.
type Status string

const (
	StatusSettled      Status = "SETTLED"
	StatusDiscrepancy  Status = "DISCREPANCY_FLAGGED"
	StatusRejected     Status = "REJECTED"
)

// Settlement is one EOD cash deposit by a rep at the HQ cashier desk.
type Settlement struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	SettlementNumber  string    `json:"settlement_number"`
	SalesRepID        string    `json:"sales_rep_id"`
	CashierUserID     string    `json:"cashier_user_id"`
	TotalSystemCash    int64     `json:"total_system_cash"`
	TotalPhysicalCash  int64     `json:"total_physical_cash"`
	DiscrepancyAmount int64     `json:"discrepancy_amount"`
	Status            Status    `json:"status"`
	CashierNotes      string    `json:"cashier_notes,omitempty"`
	SettledAt         time.Time `json:"settled_at"`
}