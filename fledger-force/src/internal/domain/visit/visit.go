// Package visit defines the store-visit domain (GPS check-in).
package visit

import "time"

// VisitType enumerates supported visit purposes.
type VisitType string

const (
	VisitTakingOrder             VisitType = "TAKING_ORDER"
	VisitCashCollection          VisitType = "CASH_COLLECTION"
	VisitTakingOrderAndCollection VisitType = "TAKING_ORDER_AND_COLLECTION"
	VisitCanvassing              VisitType = "CANVASSING"
	VisitNoOrder                 VisitType = "NO_ORDER"
)

func (v VisitType) Valid() bool {
	switch v {
	case VisitTakingOrder, VisitCashCollection, VisitTakingOrderAndCollection,
		VisitCanvassing, VisitNoOrder:
		return true
	}
	return false
}

// Status enumerates the visit lifecycle.
type Status string

const (
	StatusCheckedIn Status = "CHECKED_IN"
	StatusCompleted  Status = "COMPLETED"
	StatusSkipped    Status = "SKIPPED"
)

// Visit is one store visit with its GPS check-in record.
type Visit struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	BeatPlanID        string    `json:"beat_plan_id"`
	StoreID           string    `json:"store_id"`
	SalesRepID        string    `json:"sales_rep_id"`
	CheckInAt         time.Time `json:"check_in_at"`
	CheckOutAt        *time.Time `json:"check_out_at,omitempty"`
	CheckInLat        float64   `json:"check_in_lat"`
	CheckInLong       float64   `json:"check_in_long"`
	DistanceMeters    int       `json:"distance_meters"`
	GeofenceVerified  bool      `json:"geofence_verified"`
	VisitType         VisitType `json:"visit_type"`
	Status            Status    `json:"status"`
	SkipReason        string    `json:"skip_reason,omitempty"`
	Notes             string    `json:"notes,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}