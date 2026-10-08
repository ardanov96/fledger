// Package audit provides domain types and a repository interface for the
// audit log. Audit logs are append-only records of significant user
// actions, used for compliance (7-year retention in financial systems)
// and operational debugging.
//
// This is Fase 2 partial — the table is migrated, the type is defined,
// and a middleware records each request. Persistent storage is in-memory
// for now; swap the implementation for a Postgres repo when ready.
package audit

import (
	"context"
	"sync"
	"time"
)

// ActorType identifies who performed the action.
type ActorType string

const (
	ActorUser   ActorType = "user"
	ActorService ActorType = "service"
	ActorSystem ActorType = "system"
	ActorAdmin  ActorType = "admin"
)

// Outcome describes the result of the action.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
)

// Action is a domain.<verb> identifier, e.g. "transfer.create".
type Action string

const (
	ActionTransferCreate Action = "transfer.create"
	ActionAccountCreate  Action = "account.create"
	ActionAccountFreeze   Action = "account.freeze"
	ActionAccountClose   Action = "account.close"
	ActionAuthLogin      Action = "auth.login"
	ActionAuthLogout     Action = "auth.logout"
)

// Entry is one audit log record.
type Entry struct {
	ID           string         `json:"id"`
	TenantID     string         `json:"tenant_id"`
	ActorID      string         `json:"actor_id,omitempty"`
	ActorType    ActorType      `json:"actor_type"`
	Action       Action         `json:"action"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Outcome      Outcome        `json:"outcome"`
	RequestID    string         `json:"request_id,omitempty"`
	IPAddress    string         `json:"ip_address,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	OccurredAt   time.Time      `json:"occurred_at"`
}

// Repository is the audit log persistence interface.
type Repository interface {
	// Record persists an entry. Implementations should NOT block the
	// caller (queue async if needed for high-throughput paths).
	Record(ctx context.Context, e Entry) error

	// List returns the most recent N entries for a tenant.
	List(ctx context.Context, tenantID string, limit int) ([]Entry, error)
}

// =============================================================================
// In-memory implementation (Fase 2 scaffold)
// =============================================================================
//
// Used until the Postgres-backed implementation lands. Thread-safe via mutex.

type MemoryRepository struct {
	mu      sync.RWMutex
	entries []Entry
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{entries: make([]Entry, 0, 256)}
}

func (m *MemoryRepository) Record(_ context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

func (m *MemoryRepository) List(_ context.Context, _ string, limit int) ([]Entry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.entries) {
		limit = len(m.entries)
	}
	// Return most recent first
	out := make([]Entry, 0, limit)
	start := len(m.entries) - limit
	if start < 0 {
		start = 0
	}
	for i := len(m.entries) - 1; i >= start; i-- {
		out = append(out, m.entries[i])
	}
	return out, nil
}

// NopRepository discards all entries. Useful for tests.
type NopRepository struct{}

func (NopRepository) Record(_ context.Context, _ Entry) error { return nil }
func (NopRepository) List(_ context.Context, _ string, _ int) ([]Entry, error) {
	return nil, nil
}
