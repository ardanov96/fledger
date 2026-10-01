// Package fraud defines the fraud-detection domain (Sprint 31 / Fase 8).
//
// Architecture:
//   - `Rule` is a pluggable interface: each rule examines a TransferEvent
//     and returns zero or more Flags. Pure functions, easy to unit test.
//   - `Flag` is the persisted artifact (one row in `fraud_flags`).
//   - `Repository` persists flags and provides read access for ops/admin views.
//
// Pure domain (zero infra deps). Repository is an interface so the
// fraud_service (usecase layer) can be unit-tested with an in-memory fake.
//
// Event flow:
//   NATS fmcg.transfer.posted → FraudScannerWorker → for each Rule →
//   if rule matches → persist Flag (postgres) → if severity >= critical →
//   also create a Notification row (notification dispatcher, Sprint 28).
package fraud

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// Enums
// =============================================================================

// Severity mirrors fraud_flags_severity_valid CHECK.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityCritical Severity = "critical"
)

// Status mirrors fraud_flags_status_valid CHECK.
type Status string

const (
	StatusOpen      Status = "open"
	StatusDismissed Status = "dismissed"
	StatusConfirmed Status = "confirmed"
)

// RuleName mirrors fraud_flags_rule_name_valid CHECK.
type RuleName string

const (
	RuleLargeAmount         RuleName = "large_amount"
	RuleOffHours            RuleName = "off_hours"
	RuleVelocity            RuleName = "velocity"
	RuleFirstTimeRecipient  RuleName = "first_time_recipient"
)

// =============================================================================
// Entities
// =============================================================================

// Flag is one row in the fraud_flags table.
type Flag struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	AccountID   uuid.UUID
	TransferID  uuid.UUID
	RuleName    RuleName
	Severity    Severity
	Status      Status
	Evidence    map[string]any
	DetectedAt  time.Time
	ResolvedAt  *time.Time
}

// TransferEvent is the input to rules. Decoded from the NATS outbox payload
// (`fmcg.transfer.posted`). Kept narrow to avoid pulling domain types from
// the ledger package into this pure domain.
type TransferEvent struct {
	TenantID    uuid.UUID
	TransferID  uuid.UUID
	FromAccount uuid.UUID
	ToAccount   uuid.UUID
	AmountMinor int64
	Currency    string
	OccurredAt  time.Time
	InitiatorID uuid.UUID
}

// Match is the output of a single rule evaluation. Zero or more matches
// per rule (though in practice most rules produce 0 or 1).
type Match struct {
	RuleName RuleName
	Severity Severity
	Evidence map[string]any
}

// =============================================================================
// Tx abstraction (minimal — only Create uses tx; reads use the pool via
// RunInReadTx / RunInAdminTx)
// =============================================================================

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (CommandTag, error)
}

type CommandTag interface {
	RowsAffected() int64
}

// =============================================================================
// Repository interface
// =============================================================================

// Repository persists fraud flags and supports the velocity rule's
// per-account lookup.
//
// Two write methods (intentional):
//   - Create(ctx, tx, flag): for in-tx writes where the caller already
//     holds a tx (cross-domain atomic in future)
//   - PersistDirect(ctx, flag): fire-and-forget from the worker. The
//     postgres impl opens its own tx + binds GUC. Used by FraudScannerService
//     since each flag creation is independent of the originating transfer's
//     tx (which has long since committed).
type Repository interface {
	// Create inserts a flag row inside caller's tx.
	Create(ctx context.Context, tx Tx, f Flag) error

	// PersistDirect inserts a flag row using the repo's own tx (with
	// tenant GUC bound from ctx). Use for fire-and-forget writes from
	// background workers.
	PersistDirect(ctx context.Context, f Flag) error

	// ListByAccount returns recent flags for one account, newest first.
	// Limit is enforced (default 50, max 200). Tenant-scoped.
	ListByAccount(ctx context.Context, tenantID, accountID uuid.UUID, limit int) ([]Flag, error)

	// CountRecentByAccount returns the number of transfers posted from
	// `accountID` in the last `window` duration. Sprint 31: implemented
	// via direct COUNT on transactions table (cross-domain read).
	// This is intentionally NOT scoped to the transactions Repository to
	// avoid cross-domain coupling; runs inside RunInAdminTx so it
	// bypasses RLS (worker has no tenant scope).
	CountRecentByAccount(ctx context.Context, tenantID, accountID uuid.UUID, window time.Duration) (int, error)

	// HasRecipientHistory returns true if `fromAccount` has previously
	// transferred to `toAccount`. Used by FirstTimeRecipientRule.
	HasRecipientHistory(ctx context.Context, tenantID, fromAccount, toAccount uuid.UUID) (bool, error)
}

// =============================================================================
// Rule interface
// =============================================================================

// Rule evaluates a TransferEvent and produces zero or more matches.
// Rules are pure (no I/O) so they can be unit-tested without DB setup.
// They receive a `Context` to support time-of-day lookups via clock injection.
type Rule interface {
	// Name returns the canonical rule name (mirrors DB CHECK).
	Name() RuleName

	// Evaluate returns 0 or 1 match (most rules are binary). Multiple
	// matches are supported for future compound rules.
	Evaluate(ctx context.Context, ev TransferEvent) ([]Match, error)
}