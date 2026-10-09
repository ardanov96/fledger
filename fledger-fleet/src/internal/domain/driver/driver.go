// Package driver defines the master-data domain of the supir armada.
package driver

import "time"

// Status enumerates driver lifecycle states.
type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusOnDuty   Status = "ON_DUTY"
	StatusInactive Status = "INACTIVE"
)

// Valid reports whether s is one of the known statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusActive, StatusOnDuty, StatusInactive:
		return true
	}
	return false
}

// Driver is the master-data record of a single supir.
type Driver struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	FullName      string    `json:"full_name"`
	PhoneNumber   string    `json:"phone_number"`
	LicenseNumber string    `json:"license_number"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}