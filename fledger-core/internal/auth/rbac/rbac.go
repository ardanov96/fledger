// Package rbac wires the Casbin enforcer for FMCG Wallet.
//
// We use the standard `casbin/casbin/v2` library with a model + policy file.
// The model is the classic RBAC-with-domains pattern (suits our multi-tenant
// model where each tenant_id is a Casbin domain).
//
// Policy file format (see ./policies/rbac_policy.csv):
//
//	p, <role>, <domain>, <action>, <object>
//	g, <user>, <role>, <domain>
package rbac

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
)

// Action constants — kept here so callers don't drift on typos.
const (
	ActionCreate  = "create"
	ActionRead    = "read"
	ActionUpdate  = "update"
	ActionDelete  = "delete"
	ActionList    = "list"
	ActionApprove = "approve"
	ActionReject  = "reject"
	ActionReopen  = "reopen"
	ActionRun     = "run" // trigger a job/reconciler run
)

// Object constants — match the resources exposed by the API.
const (
	ObjectAccount     = "account"
	ObjectTransfer    = "transfer"
	ObjectInvoice     = "invoice"
	ObjectPayment     = "payment"
	ObjectCreditLimit = "credit_limit"
	ObjectAgingReport = "aging_report"
	ObjectAuditLog    = "audit_log"
	ObjectWriteOff    = "write_off"
	ObjectPeriodClose = "period_close"
	ObjectReconciler  = "reconciler"
	ObjectCollectionRoute = "collection_route"
	ObjectSettlement  = "settlement"
	ObjectCurrency    = "currency" // Sprint 12 / Fase 1D
	ObjectFxRate      = "fx_rate"  // Sprint 12 / Fase 1D
)
// Role constants — match policy file.
const (
	RoleHQAdmin   = "hq_admin"
	RoleHQFinance = "hq_finance"
	RoleAuditor   = "auditor"
	RoleOutletMgr = "outlet_manager"
	RoleSalesRep  = "sales_rep"
)

// Sentinel errors.
var (
	ErrNotInitialized = errors.New("rbac: enforcer not initialized")
	ErrDeny           = errors.New("rbac: permission denied")
)

// Enforcer is a thin wrapper around casbin.Enforcer with hot-reload + thread-safety.
type Enforcer struct {
	mu     sync.RWMutex
	inner  *casbin.Enforcer
	source string
}

// New constructs an Enforcer from a model + policy file.
func New(modelPath, policyPath string) (*Enforcer, error) {
	e, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, fmt.Errorf("casbin: load model/policy: %w", err)
	}
	abs, _ := filepath.Abs(policyPath)
	return &Enforcer{inner: e, source: abs}, nil
}

// Check reports whether the principal (user, role, tenant) may perform
// the given action on the given object.
func (e *Enforcer) Check(userID, role, tenantID, action, object string) (bool, error) {
	e.mu.RLock()
	inner := e.inner
	e.mu.RUnlock()
	if inner == nil {
		return false, ErrNotInitialized
	}
	ok, err := inner.Enforce(userID, tenantID, object, action)
	if err != nil {
		return false, fmt.Errorf("casbin enforce: %w", err)
	}
	if !ok && role != "" {
		ok2, _ := inner.Enforce(role, tenantID, object, action)
		if ok2 {
			return true, nil
		}
	}
	return ok, nil
}

// MustCheck panics if not allowed — useful in tests.
func (e *Enforcer) MustCheck(userID, role, tenantID, action, object string) {
	ok, err := e.Check(userID, role, tenantID, action, object)
	if err != nil || !ok {
		panic(fmt.Sprintf("rbac: deny %s/%s on %s.%s (err=%v ok=%v)", userID, role, object, action, err, ok))
	}
}

// Reload forces a policy reload from disk.
func (e *Enforcer) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner == nil {
		return ErrNotInitialized
	}
	return e.inner.LoadPolicy()
}

// AddPolicy adds a runtime policy rule.
func (e *Enforcer) AddPolicy(role, tenant, action, object string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner == nil {
		return false, ErrNotInitialized
	}
	return e.inner.AddPolicy(role, tenant, object, action)
}

// AddRoleForUser assigns a role to a user within a tenant.
func (e *Enforcer) AddRoleForUser(userID, role, tenant string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner == nil {
		return false, ErrNotInitialized
	}
	return e.inner.AddRoleForUserInDomain(userID, role, tenant)
}

// Source returns the path to the loaded policy file (for diagnostics).
func (e *Enforcer) Source() string { return strings.TrimSpace(e.source) }
