// Package order defines the B2B purchase order domain.
package order

import "time"

// Status enumerates the order lifecycle.
type Status string

const (
	StatusDraft              Status = "DRAFT"
	StatusPendingCreditCheck Status = "PENDING_CREDIT_CHECK"
	StatusCreditBlocked     Status = "CREDIT_BLOCKED"
	StatusApproved           Status = "APPROVED"
	StatusDispatchedToFleet  Status = "DISPATCHED_TO_FLEET"
	StatusCompleted          Status = "COMPLETED"
	StatusCancelled          Status = "CANCELLED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPendingCreditCheck, StatusCreditBlocked,
		StatusApproved, StatusDispatchedToFleet, StatusCompleted, StatusCancelled:
		return true
	}
	return false
}

// CreditGateStatus enumerates the credit-evaluation outcome.
type CreditGateStatus string

const (
	CreditUnevaluated     CreditGateStatus = "UNEVALUATED"
	CreditPassed          CreditGateStatus = "PASSED"
	CreditLimitExceeded   CreditGateStatus = "LIMIT_EXCEEDED"
	CreditOverdueBlocked  CreditGateStatus = "OVERDUE_BLOCKED"
	CreditOverridden       CreditGateStatus = "OVERRIDDEN"
)

// Order is one B2B purchase order.
type Order struct {
	ID                  string                 `json:"id"`
	TenantID            string                 `json:"tenant_id"`
	OrderNumber         string                 `json:"order_number"`
	CustomerID          string                 `json:"customer_id"`
	CustomerName        string                 `json:"customer_name"`
	CustomerTier        string                 `json:"customer_tier"`
	CustomerPhone       string                 `json:"customer_phone,omitempty"`
	DestinationAddress  string                 `json:"destination_address"`
	TotalWeightKg       int                    `json:"total_weight_kg"`
	Subtotal            int64                  `json:"subtotal"`
	Discount            int64                  `json:"discount"`
	TotalAmount         int64                  `json:"total_amount"`
	Status              Status                 `json:"status"`
	CreditGateStatus    CreditGateStatus       `json:"credit_gate_status"`
	CreditCheckDetails  map[string]any         `json:"credit_check_details,omitempty"`
	OverrideBy          string                 `json:"override_by,omitempty"`
	OverrideReason      string                 `json:"override_reason,omitempty"`
	FledgerFleetDOID    string                 `json:"fledger_fleet_do_id,omitempty"`
	Notes               string                 `json:"notes,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// Item is one line on the order.
type Item struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	OrderID      string    `json:"order_id"`
	ProductID    string    `json:"product_id"`
	SKU          string    `json:"sku"`
	Name         string    `json:"name"`
	Quantity     int       `json:"quantity"`
	UnitPrice    int64     `json:"unit_price"`
	LineTotal    int64     `json:"line_total"`
	WeightGrams  int       `json:"weight_grams"`
	CreatedAt    time.Time `json:"created_at"`
}