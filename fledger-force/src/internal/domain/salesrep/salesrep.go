// Package salesrep defines the sales-representative domain.
package salesrep

import "time"

// Role enumerates known rep roles.
type Role string

const (
	RoleCanvasser Role = "CANVASSER"
	RoleMotoris   Role = "MOTORIS"
	RoleSupervisor Role = "SUPERVISOR"
	RoleCollector Role = "COLLECTOR"
)

func (r Role) Valid() bool {
	switch r {
	case RoleCanvasser, RoleMotoris, RoleSupervisor, RoleCollector:
		return true
	}
	return false
}

// Status enumerates rep lifecycle states.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusLocked   Status = "SETTLEMENT_LOCKED"
	StatusSuspended Status = "SUSPENDED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusLocked, StatusSuspended:
		return true
	}
	return false
}

// Rep is a single sales representative / canvasser.
type Rep struct {
	ID                     string    `json:"id"`
	TenantID               string    `json:"tenant_id"`
	EmployeeCode           string    `json:"employee_code"`
	Name                   string    `json:"name"`
	Phone                  string    `json:"phone"`
	Role                   Role      `json:"role"`
	FledgerWalletAccountID string    `json:"fledger_wallet_account_id"`
	CurrentCashHeld        int64     `json:"current_cash_held"`
	MaxCashLimit           int64     `json:"max_cash_limit"`
	Status                 Status    `json:"status"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}