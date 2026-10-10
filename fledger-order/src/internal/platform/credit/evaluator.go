// Package credit implements the Hard Credit Gate evaluation engine.
//
// The gate enforces TWO rules before an order can be APPROVED:
//   Rule 1 (Plafon): current_outstanding_AR + new_order_amount <= customer credit_limit
//   Rule 2 (Aging): customer must have NO invoices overdue > 30 days
//
// If either rule fails the order is locked to CREDIT_BLOCKED. Only the
// managerial override endpoint (with a valid PIN) can flip it to APPROVED.
package credit

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/fledger/fledger-order/internal/platform/errors"
	"github.com/fledger/fledger-order/internal/integration/coreclient"
)

// Decision is the result of evaluating a candidate order against Core.
type Decision struct {
	Pass                  bool           `json:"pass"`
	Reason                string         `json:"reason,omitempty"`
	CreditLimitMinor      int64          `json:"credit_limit_minor"`
	OutstandingARMMinor   int64          `json:"outstanding_ar_minor"`
	NewOrderMinor         int64          `json:"new_order_minor"`
	ProjectedARMMinor     int64          `json:"projected_ar_minor"`
	HasOverdue30d         bool           `json:"has_overdue_30d"`
	OldestOverdueDays      int            `json:"oldest_overdue_days"`
	OverdueInvoiceID      string         `json:"overdue_invoice_id,omitempty"`
}

// Evaluator runs the Hard Credit Gate checks.
type Evaluator struct {
	core *coreclient.Client
}

// NewEvaluator creates an Evaluator.
func NewEvaluator(core *coreclient.Client) *Evaluator { return &Evaluator{core: core} }

// Evaluate runs the two-rule check via the Fledger Core adapter.
func (e *Evaluator) Evaluate(ctx context.Context, customerID string, newOrderMinor int64) (Decision, error) {
	if newOrderMinor <= 0 {
		return Decision{}, fmt.Errorf("%w: new_order_minor must be > 0", apperrors.ErrInvalidInput)
	}
	ar, err := e.core.GetARSummary(ctx, customerID)
	if err != nil {
		return Decision{}, fmt.Errorf("fetch AR summary: %w", err)
	}
	d := Decision{
		CreditLimitMinor:    ar.CreditLimitMinor,
		OutstandingARMMinor: ar.OutstandingARMinor,
		NewOrderMinor:       newOrderMinor,
		ProjectedARMMinor:   ar.OutstandingARMinor + newOrderMinor,
		HasOverdue30d:       ar.OverdueBucket.HasOverdue30d,
		OldestOverdueDays:    ar.OverdueDays,
		OverdueInvoiceID:    ar.OverdueInvoiceID,
	}

	// Rule 2: aging > 30d.
	if ar.OverdueBucket.HasOverdue30d {
		d.Pass = false
		d.Reason = fmt.Sprintf("Toko memiliki faktur menunggak %d hari (>= 30 hari): %s",
			ar.OverdueDays, ar.OverdueInvoiceID)
		return d, nil
	}

	// Rule 1: plafond kredit.
	if d.CreditLimitMinor > 0 && d.ProjectedARMMinor > d.CreditLimitMinor {
		d.Pass = false
		d.Reason = fmt.Sprintf(
			"Proyeksi AR (Rp %d) melebihi plafon toko (Rp %d) [piutang berjalan Rp %d + order baru Rp %d]",
			d.ProjectedARMMinor, d.CreditLimitMinor, d.OutstandingARMMinor, d.NewOrderMinor,
		)
		return d, nil
	}

	d.Pass = true
	return d, nil
}

// IsOrderExpired is a small helper used by the order engine to reject if the
// evaluation took more than 5 minutes.
func IsOrderExpired(evaluatedAt time.Time) bool {
	return time.Since(evaluatedAt) > 5*time.Minute
}