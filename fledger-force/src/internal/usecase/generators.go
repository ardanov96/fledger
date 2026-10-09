// Package usecase — receipt number + WA payload generators (Sprint 3).
package usecase

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateReceiptNumber returns a unique receipt number in the
// `RCP-YYYYMM-XXXXX` format (Sprint 3 of the brief).
func GenerateReceiptNumber(now time.Time) string {
	ym := now.UTC().Format("200601")
	// Take 5 hex chars (10 bit) from a v7 UUID — high entropy in a short form.
	u := uuid.NewString()
	hex := strings.ReplaceAll(u, "-", "")
	return fmt.Sprintf("RCP-%s-%s", ym, strings.ToUpper(hex[:5]))
}

// GenerateEODSettlementNumber returns `EOD-YYYYMM-XXXXX`.
func GenerateEODSettlementNumber(now time.Time) string {
	ym := now.UTC().Format("200601")
	u := uuid.NewString()
	hex := strings.ReplaceAll(u, "-", "")
	return fmt.Sprintf("EOD-%s-%s", ym, strings.ToUpper(hex[:5]))
}

// GenerateBeatPlanNumber returns `BEAT-YYYYMM-XXXXX`.
func GenerateBeatPlanNumber(now time.Time) string {
	ym := now.UTC().Format("200601")
	u := uuid.NewString()
	hex := strings.ReplaceAll(u, "-", "")
	return fmt.Sprintf("BEAT-%s-%s", ym, strings.ToUpper(hex[:5]))
}

// WAResceiptPayload is the official WhatsApp message sent to the store
// owner (Sprint 3 of the brief).
type WAResceiptPayload struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

// BuildWAResceiptPayload assembles the WA message body.
func BuildWAResceiptPayload(
	storeName, payerName, receiptNumber, invoiceNumber, employeeCode, salesName string,
	amountMinor int64,
) WAResceiptPayload {
	rupiah := formatRupiah(amountMinor)
	msg := fmt.Sprintf(
		"Terima kasih %s. Pembayaran kas senilai %s untuk faktur %s telah diterima oleh Salesman %s (%s). No Kwitansi: %s.",
		storeName, rupiah, invoiceNumber, salesName, employeeCode, receiptNumber,
	)
	return WAResceiptPayload{To: payerName, Message: msg}
}

func formatRupiah(n int64) string {
	// Simple formatter; supports negative.
	neg := n < 0
	if neg {
		n = -n
	}
	digits := fmt.Sprintf("%d", n)
	out := []byte{}
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	prefix := "Rp "
	if neg {
		prefix = "-Rp "
	}
	return prefix + string(out)
}