// Package trip defines the trip (surat tugas perjalanan) domain.
package trip

import "time"

// Status enumerates trip lifecycle states.
type Status string

const (
	StatusDraft      Status = "DRAFT"
	StatusDispatched Status = "DISPATCHED"
	StatusInTransit  Status = "IN_TRANSIT"
	StatusCompleted  Status = "COMPLETED"
	StatusCancelled  Status = "CANCELLED"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusDispatched, StatusInTransit, StatusCompleted, StatusCancelled:
		return true
	}
	return false
}

// Trip is one trip pengantaran (satu truk, satu supir, satu set DO).
type Trip struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	TripNumber     string     `json:"trip_number"`
	VehicleID      string     `json:"vehicle_id"`
	DriverID       string     `json:"driver_id"`
	DepartureTime  *time.Time `json:"departure_time,omitempty"`
	CompletedTime  *time.Time `json:"completed_time,omitempty"`
	Status         Status     `json:"status"`
	TotalStops     int        `json:"total_stops"`
	TotalDelivered int        `json:"total_delivered"`
	Notes          string     `json:"notes,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}