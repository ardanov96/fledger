// Package vehicle defines the master-data domain of the fleet (truk armada).
package vehicle

import "time"

// Status enumerates vehicle lifecycle states.
type Status string

const (
	StatusAvailable   Status = "AVAILABLE"
	StatusOnTrip      Status = "ON_TRIP"
	StatusMaintenance Status = "MAINTENANCE"
)

// Valid reports whether s is one of the known statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusAvailable, StatusOnTrip, StatusMaintenance:
		return true
	}
	return false
}

// Vehicle is the master-data record of a single armada piece.
type Vehicle struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	PlateNumber string    `json:"plate_number"`
	VehicleType string    `json:"vehicle_type"` // CDE_BOX, CDD_BOX, BLIND_VAN, MOTOR_CARGO
	BrandModel  string    `json:"brand_model"`
	CapacityKg  float64   `json:"capacity_kg"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}