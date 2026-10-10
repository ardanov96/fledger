// Package domain: store contact (WhatsApp recipient) master.
package domain

import "time"

// StoreContact is one customer recipient for dunning messages.
type StoreContact struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	StoreID   string    `json:"store_id"`
	StoreName string    `json:"store_name"`
	OwnerName string    `json:"owner_name"`
	Phone     string    `json:"phone_number"` // E.164, e.g. 6281234567890
	Email     string    `json:"email,omitempty"`
	IsActive  bool      `json:"is_active"`
	OptOut    bool      `json:"opt_out"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}