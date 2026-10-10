// Package outbox defines the durable outbox events for fleet dispatching.
package outbox

import "time"

// Status enumerates the outbox row lifecycle.
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusSent       Status = "SENT"
	StatusFailed     Status = "FAILED"
)

// EventType enumerates the dispatched event types.
type EventType string

const (
	EventDispatchedFleet EventType = "order.dispatched_fleet"
)

// Event is one outbox row.
type Event struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenant_id"`
	EventType     EventType      `json:"event_type"`
	AggregateID   string         `json:"aggregate_id"`
	Payload       map[string]any `json:"payload"`
	Status        Status         `json:"status"`
	RetryCount    int            `json:"retry_count"`
	MaxRetries    int            `json:"max_retries"`
	LastError     string         `json:"last_error,omitempty"`
	NextRetryAt   time.Time      `json:"next_retry_at"`
	SentAt        *time.Time     `json:"sent_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}