//go:build !windows
// +build !windows

package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	modelPath  = "policies/rbac_model.conf"
	policyPath = "policies/rbac_policy.csv"
)

func newEnforcer(t *testing.T) *Enforcer {
	t.Helper()
	e, err := New(modelPath, policyPath)
	require.NoError(t, err, "policy file must load; run tests from internal/auth/rbac")
	return e
}

func TestRBAC_HQAdmin_CanCreateInvoice(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, err := e.Check("u-admin", RoleHQAdmin, "tenant-1", ActionCreate, ObjectInvoice)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRBAC_HQAdmin_CanApproveWriteOff(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-admin", RoleHQAdmin, "tenant-1", ActionApprove, ObjectWriteOff)
	assert.True(t, ok)
}

func TestRBAC_HQFinance_CanApproveWriteOff(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-fin", RoleHQFinance, "tenant-1", ActionApprove, ObjectWriteOff)
	assert.True(t, ok)
}

func TestRBAC_HQFinance_CannotCreateAccount(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-fin", RoleHQFinance, "tenant-1", ActionCreate, ObjectAccount)
	assert.False(t, ok, "hq_finance should NOT create accounts")
}

func TestRBAC_Auditor_ReadOnly(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	readActions := []string{ActionRead, ActionList}
	readObjects := []string{ObjectAccount, ObjectInvoice, ObjectTransfer, ObjectPayment, ObjectAgingReport, ObjectAuditLog}
	for _, obj := range readObjects {
		for _, act := range readActions {
			ok, err := e.Check("u-aud", RoleAuditor, "tenant-1", act, obj)
			require.NoError(t, err)
			assert.True(t, ok, "auditor should %s %s", act, obj)
		}
	}
	for _, obj := range readObjects {
		for _, act := range []string{ActionCreate, ActionUpdate, ActionDelete} {
			ok, _ := e.Check("u-aud", RoleAuditor, "tenant-1", act, obj)
			assert.False(t, ok, "auditor should NOT %s %s", act, obj)
		}
	}
}

func TestRBAC_SalesRep_OnlyCreatePayment(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-rep", RoleSalesRep, "tenant-1", ActionCreate, ObjectPayment)
	assert.True(t, ok)
	for _, obj := range []string{ObjectAccount, ObjectInvoice, ObjectTransfer, ObjectCreditLimit} {
		ok, _ := e.Check("u-rep", RoleSalesRep, "tenant-1", ActionCreate, obj)
		assert.False(t, ok, "sales_rep should NOT create %s", obj)
	}
}

func TestRBAC_OutletManager_NoAdmin(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-om", RoleOutletMgr, "tenant-1", ActionDelete, ObjectAccount)
	assert.False(t, ok, "outlet_manager cannot delete accounts")
	ok, _ = e.Check("u-om", RoleOutletMgr, "tenant-1", ActionRead, ObjectAgingReport)
	assert.True(t, ok)
}

func TestRBAC_UnknownRole_Deny(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-x", "nonexistent_role", "tenant-1", ActionRead, ObjectInvoice)
	assert.False(t, ok)
}

func TestRBAC_UnknownObject_Deny(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-admin", RoleHQAdmin, "tenant-1", ActionRead, "nonexistent_object")
	assert.False(t, ok)
}

func TestRBAC_HotReload_AddPolicy(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("u-rep", RoleSalesRep, "tenant-1", ActionCreate, ObjectCreditLimit)
	assert.False(t, ok)
	added, err := e.AddPolicy(RoleSalesRep, "tenant-1", ActionCreate, ObjectCreditLimit)
	require.NoError(t, err)
	assert.True(t, added)
	ok, _ = e.Check("u-rep", RoleSalesRep, "tenant-1", ActionCreate, ObjectCreditLimit)
	assert.True(t, ok)
}

func TestRBAC_AddRoleForUser(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	ok, _ := e.Check("new-user", "no-such-role", "tenant-1", ActionRead, ObjectInvoice)
	assert.False(t, ok)
	added, err := e.AddRoleForUser("new-user", RoleAuditor, "tenant-1")
	require.NoError(t, err)
	assert.True(t, added)
	ok, _ = e.Check("new-user", RoleAuditor, "tenant-1", ActionRead, ObjectInvoice)
	assert.True(t, ok)
}

func TestRBAC_Source(t *testing.T) {
	t.Parallel()
	e := newEnforcer(t)
	s := e.Source()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "rbac_policy.csv")
}

// TestRBAC_ModelString omitted (ModelString method removed in rbac.go).
<task_progress>- [x] All Sprint 9A files
- [x] Fix rbac.go + rbac_test.go (removed ModelString)
- [ ] Verify build (go build ./...)
