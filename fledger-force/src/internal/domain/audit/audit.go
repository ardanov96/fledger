// Package audit defines the audit-log domain for the Fledger Force service.
package audit

import "time"

// Action enumerates recognised audit actions.
type Action string

const (
	ActionCashCollected   Action = "CASH_COLLECTED"
	ActionEODSettled      Action = "EOD_SETTLED"
	ActionGeofenceOverride Action = "GEOFENCE_OVERRIDE"
	ActionDiscrepancyFlagged Action = "DISCREPANCY_FLAGGED"
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