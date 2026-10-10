// Package usecase — receipt / order-number generators (Sprint 2 + 3).
package usecase

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateOrderNumber returns `ORD-YYYYMM-XXXXX`.
func GenerateOrderNumber(now time.Time) string {
	ym := now.UTC().Format("200601")
	u := uuid.NewString()
	hex := strings.ReplaceAll(u, "-", "")
	return fmt.Sprintf("ORD-%s-%s", ym, strings.ToUpper(hex[:5]))
}

// TotalWeightKg computes the cumulated shipment weight in kilograms.
func TotalWeightKg(items []Weighted) int {
	var grams int
	for _, it := range items {
		grams += it.WeightGrams() * it.Quantity()
	}
	return grams / 1000
}

// Weighted is the read-only contract needed by TotalWeightKg.
type Weighted interface {
	WeightGrams() int
	Quantity() int
}
