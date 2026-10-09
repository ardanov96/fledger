// Package pod defines the Proof of Delivery (POD) domain.
package pod

import (
	"time"

	"github.com/fledger/fledger-fleet/internal/domain/delivery_order"
)

// ProofOfDelivery is the digital bukti serah terima captured at the destination.
type ProofOfDelivery struct {
	ID                string                       `json:"id"`
	DOID              string                       `json:"do_id"`
	RecipientName     string                       `json:"recipient_name"`
	RecipientPhone    string                       `json:"recipient_phone,omitempty"`
	SignatureDataURL  string                       `json:"signature_data_url"`
	PhotoEvidenceURLs []string                     `json:"photo_evidence_urls"`
	DeliveredLat      float64                      `json:"delivered_lat"`
	DeliveredLng      float64                      `json:"delivered_lng"`
	DeliveredAt       time.Time                    `json:"delivered_at"`
	DriverNotes       string                       `json:"driver_notes,omitempty"`
	CreatedAt         time.Time                    `json:"created_at"`
	// ItemResults maps SKU → qty delivered/rejected + reason.
	// One entry per DOItem. Stored as JSON on the proof row to keep the
	// schema flat; the use case hydrates DOItems with these values after
	// the proof is accepted.
	Items []ItemResult `json:"items"`
}

// ItemResult describes how one SKU line was resolved at delivery time.
type ItemResult struct {
	ProductSKU      string `json:"product_sku"`
	QtyDelivered    int    `json:"qty_delivered"`
	QtyRejected     int    `json:"qty_rejected"`
	RejectionReason string `json:"rejection_reason,omitempty"`
}

// RejectionReason codes — matches the DATABASE-SCHEMA.sql comment block.
const (
	ReasonDamagedLeak  = "DAMAGED_LEAK"
	ReasonExpired      = "EXPIRED"
	ReasonWrongItem    = "WRONG_ITEM"
	ReasonStoreRejected = "STORE_REJECTED"
)

// ValidRejectionReasons enumerates accepted values for ItemResult.RejectionReason.
func ValidRejectionReasons() map[string]struct{} {
	return map[string]struct{}{
		ReasonDamagedLeak:   {},
		ReasonExpired:       {},
		ReasonWrongItem:     {},
		ReasonStoreRejected: {},
	}
}

// ComputeDOStatus derives the post-POD DO status from the result vs the ordered
// expected rows.
func ComputeDOStatus(ordered, delivered, rejected int) delivery_order.Status {
	switch {
	case rejected == 0 && delivered == ordered:
		return delivery_order.StatusDeliveredFull
	case rejected == 0 && delivered < ordered:
		return delivery_order.StatusDeliveryFailed
	case rejected > 0 && delivered > 0:
		return delivery_order.StatusDeliveredPartial
	case rejected > 0 && delivered == 0:
		return delivery_order.StatusDeliveryFailed
	default:
		return delivery_order.StatusDeliveredPartial
	}
}