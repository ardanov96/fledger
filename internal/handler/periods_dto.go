// periods_dto.go — DTO + converter untuk period-close workflow (Fase 1A).
//
// Berisi:
//   - HTTP response DTOs (CloseRequestResponse, PeriodResponse, PeriodSnapshotResponse)
//   - Converter dari domain → DTO
//   - helper principalIDFromContext (extract user ID dari JWT principal)
package handler

import (
	"net/http"

	"github.com/runut/fmcg-wallet/internal/domain/period"
	"github.com/runut/fmcg-wallet/internal/middleware"
)

// =============================================================================
// DTOs
// =============================================================================

// CloseRequestResponse is the public representation of a period close request.
type CloseRequestResponse struct {
	ID              string `json:"id"`
	TenantID        string `json:"tenant_id"`
	PeriodID        string `json:"period_id"`
	RequesterID     string `json:"requester_id"`
	ApproverID      string `json:"approver_id,omitempty"`
	Status          string `json:"status"`
	TrialBalanceOK  bool   `json:"trial_balance_ok"`
	TotalDebitMinor int64  `json:"total_debit_minor"`
	TotalCreditMinor int64 `json:"total_credit_minor"`
	ImbalanceMinor  int64  `json:"imbalance_minor"`
	RejectionReason string `json:"rejection_reason,omitempty"`
	RequestedAt     string `json:"requested_at"`
	DecidedAt       string `json:"decided_at,omitempty"`
}

// PeriodResponse is the public representation of an accounting period.
type PeriodResponse struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Status      string `json:"status"`
}

// PeriodSnapshotResponse is the public representation of one frozen snapshot row.
type PeriodSnapshotResponse struct {
	ID           string `json:"id"`
	PeriodID     string `json:"period_id"`
	RequestID    string `json:"request_id"`
	AccountID    string `json:"account_id"`
	BalanceMinor int64  `json:"balance_minor"` // signed: debit positive, credit negative
	Currency     string `json:"currency"`
	EntryCount   int    `json:"entry_count"`
	SnapshotAt   string `json:"snapshot_at"`
}

// =============================================================================
// Converters
// =============================================================================

// ToCloseRequestResponse converts a domain CloseRequest to its HTTP response DTO.
func ToCloseRequestResponse(r period.CloseRequest) CloseRequestResponse {
	out := CloseRequestResponse{
		ID:              r.ID,
		TenantID:        r.TenantID,
		PeriodID:        r.PeriodID,
		RequesterID:     r.RequesterID,
		ApproverID:      r.ApproverID,
		Status:          string(r.Status),
		TrialBalanceOK:  r.TrialBalanceOK,
		TotalDebitMinor: r.TotalDebit.Minor(),
		TotalCreditMinor: r.TotalCredit.Minor(),
		ImbalanceMinor:  r.Imbalance.Minor(),
		RejectionReason: r.RejectionReason,
		RequestedAt:     r.RequestedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if r.DecidedAt != nil {
		out.DecidedAt = r.DecidedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return out
}

// ToPeriodResponse converts a domain Period to its HTTP response DTO.
func ToPeriodResponse(p period.Period) PeriodResponse {
	return PeriodResponse{
		ID:          p.ID,
		TenantID:    p.TenantID,
		PeriodStart: p.PeriodStart.UTC().Format("2006-01-02T15:04:05Z07:00"),
		PeriodEnd:   p.PeriodEnd.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Status:      string(p.Status),
	}
}

// ToPeriodSnapshotResponse converts a domain PeriodSnapshot to its HTTP response DTO.
func ToPeriodSnapshotResponse(s period.PeriodSnapshot) PeriodSnapshotResponse {
	return PeriodSnapshotResponse{
		ID:           s.ID,
		PeriodID:     s.PeriodID,
		RequestID:    s.RequestID,
		AccountID:    s.AccountID,
		BalanceMinor: s.BalanceMinor,
		Currency:     s.Currency,
		EntryCount:   s.EntryCount,
		SnapshotAt:   s.SnapshotAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// =============================================================================
// Helpers
// =============================================================================

// principalIDFromContext extracts the authenticated user ID from the request
// context (set by JWT middleware). Returns empty string if no principal present.
func principalIDFromContext(r *http.Request) string {
	p := middleware.PrincipalFromContext(r.Context())
	if p == nil {
		return ""
	}
	return p.UserID
}
