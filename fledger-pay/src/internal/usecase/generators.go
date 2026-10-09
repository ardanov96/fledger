// Package usecase — VA + QRIS generators (Sprint 2).
package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/fledger/fledger-pay/internal/domain/va"
)

// GenerateVANumber builds a VA number by concatenating the bank prefix + a
// derived suffix from the payment-request id + a salt. The result is unique
// per (bank, payment_request_id) and stable across lookups.
func GenerateVANumber(bank va.BankCode, paymentRequestID, customerPhone string) string {
	prefix := va.Prefix(bank)
	suffix := vaSuffix(bank, paymentRequestID, customerPhone)
	return prefix + suffix
}

func vaSuffix(bank va.BankCode, paymentRequestID, phone string) string {
	h := sha256.New()
	h.Write([]byte(string(bank)))
	h.Write([]byte("|"))
	h.Write([]byte(paymentRequestID))
	h.Write([]byte("|"))
	h.Write([]byte(strings.TrimSpace(phone)))
	h.Write([]byte("|"))
	h.Write([]byte("fledger-pay-v1"))
	sum := hex.EncodeToString(h.Sum(nil))
	return sum[:12]
}

// VADisplayName formats a customer name for VA routing labels.
func VADisplayName(customerName string) string {
	cleaned := strings.ToUpper(strings.TrimSpace(customerName))
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if len(cleaned) > 30 {
		cleaned = cleaned[:30]
	}
	return fmt.Sprintf("FLEDGER - %s", cleaned)
}

// QRISPayload builds a minimal but standards-compliant EMVCo string for
// dynamic Indonesian QRIS (Merchant Account Information + Transaction Amount).
//
// Format reference: EMVCo QR Code Specification + BI SNAP (Standar Nasional
// QRIS). This implementation is sufficient for sandbox demos; production
// integrations should use the acquirer's SDK.
func QRISPayload(merchantCity string, merchantName string, amountMinor int64, id string) string {
	tlv := func(tag, value string) string {
		return fmt.Sprintf("%s%02d%s", tag, len(value), value)
	}

	merchantTax := "000000000000000" // placeholder 15-digit terminal ID

	root := "000201" // format indicator "QRIS"
	mai := tlv("00", "ID.CO.FLEDGER.WWW")
	mai += tlv("01", merchantTax)
	root += tlv("26", mai)

	root += tlv("52", "0000")
	root += tlv("53", "360")
	if amountMinor > 0 {
		root += tlv("54", fmt.Sprintf("%d", amountMinor))
	}
	root += tlv("58", "ID")
	root += tlv("59", truncate(merchantName, 25))
	root += tlv("60", truncate(merchantCity, 15))
	root += tlv("62", tlv("05", truncate(id, 20)))

	root += "6304"
	crc := crc16CCITT([]byte(root))
	return root + fmt.Sprintf("%04X", crc)
}

func crc16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// IsQRISExpired returns true if the QR is past its expiry.
func IsQRISExpired(expiresAt time.Time) bool {
	return time.Now().UTC().After(expiresAt)
}