// Package tenantctx sets Postgres session-level GUC variables used by the
// RLS policies created in migration 000014:
//
//	app.current_tenant_id   UUID  - the tenant the request belongs to
//	app.current_user_id     UUID  - the authenticated user (for sales_rep scope)
//	app.is_sales_rep        TEXT  - 'true' when role is sales_rep
//
// Callers (use case layer) MUST run SetTenantContext within the same
// transaction as the queries they want to scope. The `is_local=true`
// flag scopes the setting to the current transaction (auto-reverts on
// COMMIT/ROLLBACK).
//
// Helper pattern:
//
//  1. The HTTP middleware (TenantContextMiddleware) extracts Principal from
//     JWT claims and stores tenant_id + user_id + role on the request context.
//  2. Each use case service, at the start of an ExecuteTx / RunInTx*Domain
//     closure, calls SetTenantContext(ctx, tx, info) to bind GUC variables
//     before any SELECT/INSERT/UPDATE/DELETE.
//  3. RLS policies (migration 000014) read these GUC variables to filter
//     rows by tenant_id (and by sales_rep_id when app.is_sales_rep='true').
//
// Sprint 23 (22B.5): SetTenantContext also INSERTs a forensic row into
// guc_bind_audit for every bind attempt. Operators can query
// SELECT ... FROM guc_bind_audit WHERE tenant_id = '<uuid>'
// to see "who accessed this tenant at what request".
package tenantctx

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// TxExec is the minimal interface needed to issue raw SQL. We define it
// here (instead of depending on pgx.Tx directly) to avoid type-collision
// with domain.Tx (which also has an Exec method, satisfying TxExec
// structurally via duck typing).
type TxExec interface {
	Exec(ctx context.Context, sql string, args ...any) (commandTag, error)
}

// commandTag is an empty interface; pgconn.CommandTag satisfies it
// structurally.
type commandTag interface{ RowsAffected() int64 }

// ContextKey is the key used to attach `*Info` (request_id) to context.
// Sub-package setters use this; production code never references it
// directly. Exported so `httpx.WithRequestID` can put the W3C request
// ID under the same key the auditor reads.
type ContextKey struct{}

// Info holds the tenant + user + role context for RLS binding.
// Populated from Principal in middleware, consumed by SetTenantContext at
// the start of every transaction.
type Info struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	IsSalesRep bool
}

// GetRequestID extracts the W3C request_id (or any other ID set under
// ContextKey) from the context. Returns empty string if absent.
// Used by SetTenantContext so the audit row carries the same correlation
// ID as the HTTP request_id middleware echo.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKey{}).(string); ok {
		return v
	}
	return ""
}

// withRequestID returns a child context carrying requestID under
// ContextKey. Wired by HTTP middleware (mirrors httpx.WithRequestID).
func withRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, ContextKey{}, requestID)
}

// SetTenantContext issues SELECT set_config(...) to bind GUC variables
// for the current transaction. All three GUC settings use is_local=true
// so they auto-revert on COMMIT/ROLLBACK.
//
// The tx argument must be one of the domain.Tx interfaces (ledger.Tx,
// invoice.Tx, period.Tx, reconciler.Tx, collection.Tx, currency.Tx,
// auth.Tx) — they all structurally satisfy TxExec.
//
// If info is nil (e.g. request from a public endpoint), this function
// returns nil and does nothing. The use case layer is responsible for
// rejecting endpoints that REQUIRE a tenant context (per the post-Sprint-15
// convention, only /auth/* uses no-tenant tx; all /v1/* routes do).
//
// Sprint 23 (22B.5): after binding GUC variables, INSERTs a row into
// guc_bind_audit for forensic trail. Failure to write the audit row is
// non-fatal (logged via fmt.Errorf returned to caller) — we do NOT
// want a missing audit trail to fail the user's transaction. If you
// need audit-strict mode, call SetTenantContextStrict instead.
func SetTenantContext(ctx context.Context, tx any, info *Info) error {
	if info == nil {
		return nil
	}
	exec, ok := tx.(TxExec)
	if !ok {
		return fmt.Errorf("tenantctx: tx %T does not satisfy TxExec (need Exec method)", tx)
	}

	salesRep := "false"
	if info.IsSalesRep {
		salesRep = "true"
	}

	if _, err := exec.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", info.TenantID.String()); err != nil {
		return fmt.Errorf("tenantctx: set tenant_id: %w", err)
	}
	if _, err := exec.Exec(ctx, "SELECT set_config('app.current_user_id', $1, true)", info.UserID.String()); err != nil {
		return fmt.Errorf("tenantctx: set user_id: %w", err)
	}
	if _, err := exec.Exec(ctx, "SELECT set_config('app.is_sales_rep', $1, true)", salesRep); err != nil {
		return fmt.Errorf("tenantctx: set is_sales_rep: %w", err)
	}

	// Sprint 23 (22B.5) audit INSERT — failure non-fatal.
	requestID := GetRequestID(ctx)
	if _, err := exec.Exec(ctx,
		`INSERT INTO guc_bind_audit (tenant_id, user_id, is_sales_rep, request_id)
		 VALUES ($1, $2, $3, NULLIF($4, ''))`,
		info.TenantID, info.UserID, info.IsSalesRep, requestID,
	); err != nil {
		// Log but don't fail the tx — audit is best-effort, not a precondition
		// for the user's actual operation. Operators monitoring the bind count
		// against actual tx count will catch drift.
		return fmt.Errorf("tenantctx: audit insert: %w", err)
	}

	return nil
}

// ErrTxMismatch is returned by SetTenantContext when the supplied tx does
// not satisfy TxExec (mainly useful for explicit failure mode in tests).
var ErrTxMismatch = errors.New("tenantctx: tx does not satisfy TxExec")

// WithInfo stores *Info on the context. Caller usually the middleware; the
// use case layer reads via InfoFromContext.
func WithInfo(ctx context.Context, info *Info) context.Context {
	return context.WithValue(ctx, ContextKey{}, info)
}

// InfoFromContext returns the *Info set by WithInfo, or nil if absent.
// Used by usecase services to retrieve tenant binding metadata.
func InfoFromContext(ctx context.Context) *Info {
	v, _ := ctx.Value(ContextKey{}).(*Info)
	return v
}
