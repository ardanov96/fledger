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
	ErrDeny            = errors.New("rbac: permission denied")
)

// Enforcer is a thin wrapper around casbin.Enforcer with hot-reload + thread-safety.
//
// Hot-reload is implemented as a single RWMutex guarding the underlying
// casbin.Enforcer pointer; readers (Enforce calls) take a read lock while
// Reload takes a write lock and atomically swaps the pointer.
type Enforcer struct {
	mu     sync.RWMutex
	inner  *casbin.Enforcer
	source string // path to the policy file
}

// New constructs an Enforcer from a model + policy file.
//
//	modelPath:  path to RBAC .conf file (Casbin model definition)
//	policyPath: path to .csv policy file
//
// Both files must exist; New returns a clear error if either is missing.
func New(modelPath, policyPath string) (*Enforcer, error) {
	e, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, fmt.Errorf("casbin: load model/policy: %w", err)
	}
	// casbin v2 auto-reloads policy file on changes by default.
	abs, _ := filepath.Abs(policyPath)
	return &Enforcer{inner: e, source: abs}, nil
}

// Check reports whether the principal (user, role, tenant) may perform
// the given action on the given object.
//
// Mapping to Casbin's RBAC-with-domains model:
//   sub = user_id
//   dom = tenant_id
//   obj = object
//   act = action
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

// Reload forces a policy reload from disk. Call after editing the policy
// file manually. Auto-reload also picks up file changes (best-effort).
func (e *Enforcer) Reload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner == nil {
		return ErrNotInitialized
	}
	return e.inner.LoadPolicy()
}

// AddPolicy adds a runtime policy rule. Persists to disk if auto-save is enabled.
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
