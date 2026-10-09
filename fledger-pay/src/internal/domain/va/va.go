// Package va defines the virtual-account domain.
package va

import "time"

// BankCode enumerates supported Indonesian banks.
type BankCode string

const (
	BankBCA     BankCode = "BCA"
	BankMandiri BankCode = "MANDIRI"
	BankBRI     BankCode = "BRI"
	BankBNI     BankCode = "BNI"
	BankPermata BankCode = "PERMATA"
	BankCIMB    BankCode = "CIMB"
)

func (b BankCode) Valid() bool {
	switch b {
	case BankBCA, BankMandiri, BankBRI, BankBNI, BankPermata, BankCIMB:
		return true
	}
	return false
}

// VA prefix registry per bank (real-world convention, demo values).
var prefixes = map[BankCode]string{
	BankBCA:     "89012",
	BankMandiri: "88701",
	BankBRI:     "88810",
	BankBNI:     "8808",
	BankPermata: "8504",
	BankCIMB:    "8001",
}

// Account is one virtual account for a payment request.
type Account struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	PaymentRequestID  string    `json:"payment_request_id"`
	BankCode          BankCode  `json:"bank_code"`
	VANumber          string    `json:"va_number"`
	VAName            string    `json:"va_name"`
	ExpectedAmount    int64     `json:"expected_amount"`
	Status            string    `json:"status"`
	ExpiresAt         time.Time `json:"expires_at"`
	PaidAt            *time.Time `json:"paid_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// Prefix returns the numeric prefix used for this bank's VA numbers.
func Prefix(b BankCode) string { return prefixes[b] }