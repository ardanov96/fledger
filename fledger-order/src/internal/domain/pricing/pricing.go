// Package pricing defines the multi-tier pricing domain.
package pricing

import "time"

// Tier enumerates customer/store tiers.
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

// PriceTier is one price-tuple for a product at a given tier & minimum qty.
type PriceTier struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	ProductID   string    `json:"product_id"`
	Tier        Tier      `json:"tier"`
	MinQuantity int       `json:"min_quantity"`
	UnitPrice   int64     `json:"unit_price"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}