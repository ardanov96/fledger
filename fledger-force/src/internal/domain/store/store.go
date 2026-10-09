// Package store defines the retail-store domain.
package store

import "time"

// Tier enumerates store tiers.
type Tier string

const (
	TierGrosir     Tier = "GROSIR"
	TierSemiGrosir Tier = "SEMI_GROSIR"
	TierRetail     Tier = "RETAIL"
	TierStarOutlet Tier = "STAR_OUTLET"
)

func (t Tier) Valid() bool {
	switch t {
	case TierGrosir, TierSemiGrosir, TierRetail, TierStarOutlet:
		return true
	}
	return false
}

// Store is a single retail outlet.
type Store struct {
	ID                   string    `json:"id"`
	TenantID             string    `json:"tenant_id"`
	StoreCode            string    `json:"store_code"`
	Name                 string    `json:"name"`
	OwnerName            string    `json:"owner_name,omitempty"`
	Phone                string    `json:"phone,omitempty"`
	Address              string    `json:"address"`
	Latitude             float64   `json:"latitude"`
	Longitude            float64   `json:"longitude"`
	GeofenceRadiusMeters int       `json:"geofence_radius_meters"`
	Tier                 Tier      `json:"tier"`
	CreditLimit          int64     `json:"credit_limit"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}