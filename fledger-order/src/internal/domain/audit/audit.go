// Package audit defines the audit-log domain for Fledger Order.
package audit

import "time"

// Action enumerates recognised audit actions.
type Action string

const (
	ActionCreateOrder      Action = "CREATE_ORDER"
	ActionCreditPassed     Action = "CREDIT_PASSED"
	ActionCreditBlocked    Action = "CREDIT_BLOCKED"
	ActionOverrideApproved Action = "OVERRIDE_APPROVED"
	ActionDispatchedFleet  Action = "DISPATCHED_FLEET"
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