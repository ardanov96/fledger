// Package outbox defines the durable queue domain used to sync POD results
// to Fledger Core (issue #4 / Sprint 4). When the upstream is unavailable the
// POD is still stored locally and an outbox row is appended; the worker
// retries until success or attempts are exhausted.
package outbox

import "time"

// Status enumerates outbox row lifecycle states.
type Status string

const (
	StatusPending Status = "PENDING"
	StatusSent    Status = "SENT"
	StatusFailed  Status = "FAILED"
)

// Event is a single durable outbox row.
type Event struct {
	ID            string        `json:"id"`
	TenantID      string        `json:"tenant_id"`
	AggregateType string        `json:"aggregate_type"` // "DELIVERY_ORDER"
	AggregateID   string        `json:"aggregate_id"`   // DO UUID
	EventType     string        `json:"event_type"`     // "DO_POD_SUBMITTED"
	Subject       string        `json:"subject"`        // logical topic
	Payload       map[string]any `json:"payload"`
	Status        Status        `json:"status"`
	Attempts      int           `json:"attempts"`
	LastError     string        `json:"last_error,omitempty"`
	NextAttemptAt time.Time     `json:"next_attempt_at"`
	SentAt        *time.Time    `json:"sent_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}