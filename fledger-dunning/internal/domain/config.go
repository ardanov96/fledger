// Package domain: per-tenant dunning configuration.
package domain

import (
	"encoding/json"
	"time"
)

// DunningConfig holds the cadence + WhatsApp settings for one tenant.
type DunningConfig struct {
	ID                    string    `json:"id"`
	TenantID              string    `json:"tenant_id"`
	CadenceDays           []int     `json:"cadence_days"`
	WAProvider            string    `json:"wa_provider"` // MOCK | BAILEYS | META_CLOUD
	StatementDayOfMonth   int       `json:"statement_day_of_month"`
	JitterMinSeconds      int       `json:"jitter_min_seconds"`
	JitterMaxSeconds      int       `json:"jitter_max_seconds"`
	AutoCancelOnPayment   bool      `json:"auto_cancel_on_payment"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// DefaultCadence is the brief's spec: 5 stages, [-3, 0, 3, 7, 14] days.
var DefaultCadence = []int{-3, 0, 3, 7, 14}

// MarshalCadence returns a JSON []int suitable for the JSONB column.
func MarshalCadence(days []int) ([]byte, error) {
	if days == nil {
		days = DefaultCadence
	}
	return json.Marshal(days)
}