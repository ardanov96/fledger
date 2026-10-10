// Package product defines the FMCG product catalog domain.
package product

import "time"

// Category enumerates product categories.
type Category string

const (
	CategorySembako     Category = "SEMBAKO"
	CategoryMieInstan   Category = "MIE_INSTAN"
	CategoryMinuman     Category = "MINUMAN"
	CategoryToiletries  Category = "TOILETRIES"
	CategoryRokok       Category = "ROKOK"
)

// Unit enumerates the unit of measure.
type Unit string

const (
	UnitDus      Unit = "DUS"
	UnitKarton   Unit = "KARTON"
	UnitPack     Unit = "PACK"
	UnitRenceng  Unit = "RENCENG"
	UnitPcs      Unit = "PCS"
)

// Product is a single FMCG SKU.
type Product struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	SKU          string    `json:"sku"`
	Barcode      string    `json:"barcode,omitempty"`
	Name         string    `json:"name"`
	Category     Category  `json:"category"`
	Unit         Unit      `json:"unit"`
	WeightGrams  int       `json:"weight_grams"`
	IsActive     bool      `json:"is_active"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}