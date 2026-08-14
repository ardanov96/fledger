// Package tenantctx sets Postgres session-level GUC variables used by the
// RLS policies created in migration 000014:
//
//   app.current_tenant_id   UUID  - the tenant the request belongs to
//   app.current_user_id     UUID  - the authenticated user (for sales_rep scope)
//   app.is_sales_rep        TEXT  - 'true' when role is sales_rep
//
// Callers (use case layer) MUST call SetTenantContext within the same
// transaction as the queries you want to scope. The `is_local=true`
// flag scopes the setting to the current transaction (auto-reverts on
// COMMIT/ROLLBACK).
package tenantctx

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// TxExec is the minimal interface needed to issue raw SQL. We define it
// here (instead of depending on pgx.Tx directly) to avoid type-collision
// with domain.Tx which also has an Exec method.
type TxExec interface {
	Exec(ctx context.Context, sql string, args ...any) (commandTag, error)
}

// commandTag is an empty interface; pgconn.CommandTag satisfies it
// structurally.
type commandTag interface{}

// SetTenantContext issues SELECT set_config(...) to bind GUC variables
// for the current transaction.
func SetTenantContext(ctx context.Context, tx any, tenantID, userID uuid.UUID, isSalesRep bool) error {
	exec, ok := tx.(TxExec)
	if !ok {
		return errors.New("tenantctx: tx must satisfy TxExec interface (pgx.Tx or wrapper)")
	}

	salesRep := "false"
	if isSalesRep {
		salesRep = "true"
	}

	if _, err := exec.Exec(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID.String()); err != nil {
		return err
	}
	if _, err := exec.Exec(ctx, "SELECT set_config('app.current_user_id', $1, true)", userID.String()); err != nil {
		return err
	}
	if _, err := exec.Exec(ctx, "SELECT set_config('app.is_sales_rep', $1, true)", salesRep); err != nil {
		return err
	}
	return nil
}
