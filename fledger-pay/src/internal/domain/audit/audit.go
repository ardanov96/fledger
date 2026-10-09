// Package audit defines the audit-log domain (Sprint guardrail §7.3 equivalent
// for Fledger Pay).
package audit

import "time"

// Action enumerates recognised audit actions.
type Action string

const (
	ActionCreatePayment   Action = "CREATE_PAYMENT"
	ActionCancelPayment   Action = "CANCEL_PAYMENT"
	ActionWebhookReceived Action = "WEBHOOK_RECEIVED"
	ActionSettleTransaction Action = "SETTLE_TRANSACTION"
)

// Log is one audit row.
type Log struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id"`
	ActorID      string         `json:"actor_id"`
	ActorRole    string         `json:"actor_role"`
	Action       Action         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Details      map[string]any `json:"details,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}