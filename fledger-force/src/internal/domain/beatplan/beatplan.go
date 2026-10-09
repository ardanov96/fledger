// Package beatplan defines the daily beat-plan domain.
package beatplan

import "time"

// Status enumerates the beat-plan lifecycle.
type Status string

const (
	StatusAssigned    Status = "ASSIGNED"
	StatusInProgress  Status = "IN_PROGRESS"
	StatusCompleted   Status = "COMPLETED"
	StatusCancelled   Status = "CANCELLED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusAssigned, StatusInProgress, StatusCompleted, StatusCancelled:
		return true
	}
	return false
}

// Plan is a daily beat plan for a rep.
type Plan struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	PlanNumber        string    `json:"plan_number"`
	SalesRepID        string    `json:"sales_rep_id"`
	PlanDate          time.Time `json:"plan_date"`
	Territory         string    `json:"territory"`
	TargetStoresCount int       `json:"target_stores_count"`
	VisitedStoresCount int      `json:"visited_stores_count"`
	Status            Status    `json:"status"`
	Notes             string    `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}