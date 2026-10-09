// Package delivery_order defines the Surat Jalan (Delivery Order) domain.
package delivery_order

import "time"

// Status enumerates DO lifecycle states.
type Status string

const (
	StatusPending            Status = "PENDING"
	StatusLoaded             Status = "LOADED"
	StatusOutForDelivery     Status = "OUT_FOR_DELIVERY"
	StatusDeliveredFull      Status = "DELIVERED_FULL"
	StatusDeliveredPartial   Status = "DELIVERED_PARTIAL"
	StatusDeliveryFailed     Status = "DELIVERY_FAILED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusLoaded, StatusOutForDelivery,
		StatusDeliveredFull, StatusDeliveredPartial, StatusDeliveryFailed:
		return true
	}
	return false
}

// DeliveryOrder is one Surat Jalan to a single toko/outlet.
type DeliveryOrder struct {
	ID                     string     `json:"id"`
	TenantID               string     `json:"tenant_id"`
	TripID                 *string    `json:"trip_id,omitempty"`
	DoNumber               string     `json:"do_number"`
	CustomerID             string     `json:"customer_id"`
	CustomerName           string     `json:"customer_name"`
	DestinationAddress     string     `json:"destination_address"`
	DestinationLat         *float64   `json:"destination_lat,omitempty"`
	DestinationLng         *float64   `json:"destination_lng,omitempty"`
	TotalItemsOrdered      int        `json:"total_items_ordered"`
	TotalItemsDelivered    int        `json:"total_items_delivered"`
	TotalItemsRejected     int        `json:"total_items_rejected"`
	NominalOrderedCents    int64      `json:"nominal_ordered_cents"`
	NominalDeliveredCents  int64      `json:"nominal_delivered_cents"`
	Status                 Status     `json:"status"`
	FledgerInvoiceID       *string    `json:"fledger_invoice_id,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// DOItem is one SKU line on a Surat Jalan.
type DOItem struct {
	ID               string  `json:"id"`
	DOID             string  `json:"do_id"`
	ProductSKU       string  `json:"product_sku"`
	ProductName      string  `json:"product_name"`
	QtyOrdered       int     `json:"qty_ordered"`
	QtyDelivered     int     `json:"qty_delivered"`
	QtyRejected      int     `json:"qty_rejected"`
	UnitPriceCents   int64   `json:"unit_price_cents"`
	RejectionReason  *string `json:"rejection_reason,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ComputeNominalOrdered returns the sum of (qty_ordered * unit_price_cents).
func ComputeNominalOrdered(items []DOItem) int64 {
	var sum int64
	for _, it := range items {
		if it.QtyOrdered <= 0 {
			continue
		}
		sum += int64(it.QtyOrdered) * it.UnitPriceCents
	}
	return sum
}

// ComputeNominalDelivered returns the sum of (qty_delivered * unit_price_cents).
func ComputeNominalDelivered(items []DOItem) int64 {
	var sum int64
	for _, it := range items {
		if it.QtyDelivered <= 0 {
			continue
		}
		sum += int64(it.QtyDelivered) * it.UnitPriceCents
	}
	return sum
}