// Package inventory defines the warehouse stock domain.
package inventory

import "time"

// Stock is the per-warehouse stock row for a product.
type Stock struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	ProductID    string    `json:"product_id"`
	WarehouseID  string    `json:"warehouse_id"`
	OnHandQty    int       `json:"on_hand_qty"`
	ReservedQty  int       `json:"reserved_qty"`
	AvailableQty int       `json:"available_qty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}