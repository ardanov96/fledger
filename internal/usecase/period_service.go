// Package usecase — PeriodService implements the period-close workflow:
//
//   RequestClose  → status = closing, request = pending
//   ApproveClose  → trial balance check → snapshots → status = closed, request = approved
//   RejectClose   → status = open, request = rejected
//   Reopen        → status = open, request baru (admin only)
//
// All mutations run inside one DB transaction (via TxRunner) so the period
// status flip, the request row write, and (on approve) the N snapshot rows
// are atomic — no torn writes.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/runut/fmcg-wallet/internal/domain/period"
	apperrors "github.com/runut/fmcg-wallet/internal/platform/errors"
	"github.com/runut/fmcg-wallet/internal/platform/money"
)

// =============================================================================
// TxRunner — period-flavored
// =============================================================================

// PeriodTxRunner is the period-flavored transaction runner. Mirrors the
// ledger/invoice TxRunner pattern but exposes period.Tx to the closure.
type PeriodTxRunner interface {
	ExecuteTx(ctx context.Context, fn func(period.Tx) error) error
}

// =============================================================================
// PeriodService
// =============================================================================

// PeriodService orchestrates period-close workflows.
//
// Dependencies:
//   - repo        — Postgres-backed period repo (all reads/writes)
//   - db          — Tx runner (so we can wrap multi-step writes in one tx)
//   - log         — structured logger
type PeriodService struct {
	repo period.Repository
	db   PeriodTxRunner
	log  *slog.Logger
	now  func() time.Time // injectable for tests
}

// PeriodServiceDeps bundles dependencies.
type PeriodServiceDeps struct {
	Repo    period.Repository
	DB      PeriodTxRunner
	Logger  *slog.Logger
	NowFunc func() time.Time // optional; defaults to time.Now
}

// NewPeriodService constructs a PeriodService.
func NewPeriodService(deps PeriodServiceDeps) *PeriodService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	nowFn := deps.NowFunc
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &PeriodService{repo: deps.Repo, db: deps.DB, log: log, now: nowFn}
}

// =============================================================================
// Use case: RequestClose
// =============================================================================

// RequestCloseInput is the input for requesting a period close.
type RequestCloseInput struct {
	TenantID    string
	PeriodID    string
	RequesterID string
	Metadata    map[string]any
}

// RequestClose validates period is open, locks the period, inserts a pending
// close request, and flips period.status → 'closing'. Caller can later call
// ApproveClose to finalize.
func (s *PeriodService) RequestClose(ctx context.Context, in RequestCloseInput) (period.CloseRequest, error) {
	if err := validateRequestClose(in); err != nil {
		return period.CloseRequest{}, err
	}

	now := s.now()
	requestID := uuid.NewString()
	out := period.CloseRequest{
		ID:          requestID,
		TenantID:    in.TenantID,
		PeriodID:    in.PeriodID,
		RequesterID: in.RequesterID,
		Status:      period.CloseRequestPending,
		RequestedAt: now,
		Metadata:    in.Metadata,
	}

	err := s.db.ExecuteTx(ctx, func(tx period.Tx) error {
		p, err := s.repo.LockPeriod(ctx, tx, in.PeriodID)
		if err != nil {
			return fmt.Errorf("lock period: %w", err)
		}
		if p.Status != period.PeriodStatusOpen {
			return fmt.Errorf("%w: cannot request close for period in status %q", apperrors.ErrInvalidInput, p.Status)
		}
		if err := s.repo.InsertCloseRequest(ctx, tx, out); err != nil {
			return fmt.Errorf("insert close request: %w", err)
		}
		if err := s.repo.UpdatePeriodStatus(ctx, tx, in.PeriodID, period.PeriodStatusClosing); err != nil {
			return fmt.Errorf("update period status: %w", err)
		}
		return nil
	})
	if err != nil {
		return period.CloseRequest{}, err
	}

	s.log.Info("period close requested",
		"period_id", in.PeriodID,
		"requester_id", in.RequesterID,
		"request_id", requestID,
	)
	return out, nil
}

func validateRequestClose(in RequestCloseInput) error {
	if in.TenantID == "" {
		return fmt.Errorf("%w: tenant_id required", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.PeriodID); err != nil {
		return fmt.Errorf("%w: invalid period_id", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.RequesterID); err != nil {
		return fmt.Errorf("%w: invalid requester_id", apperrors.ErrInvalidInput)
	}
	return nil
}

// =============================================================================
// Use case: ApproveClose
// =============================================================================

// ApproveCloseInput is the input for approving a pending close request.
type ApproveCloseInput struct {
	RequestID  string
	ApproverID string
}

// ApproveClose locks the pending request, computes trial balance, generates
// per-account snapshots, flips period → 'closed', and marks request → 'approved'.
// All inside ONE tx so it's atomic.
//
// If trial balance doesn't balance (debit != credit), the request is marked
// rejected (NOT approved) and period stays in 'closing' state — caller must
// re-request after fixing the imbalance.
func (s *PeriodService) ApproveClose(ctx context.Context, in ApproveCloseInput) (period.CloseRequest, error) {
	if err := validateDecideClose(in.RequestID, in.ApproverID); err != nil {
		return period.CloseRequest{}, err
	}

	now := s.now()
	var out period.CloseRequest

	err := s.db.ExecuteTx(ctx, func(tx period.Tx) error {
		req, err := s.repo.LockCloseRequest(ctx, tx, in.RequestID)
		if err != nil {
			return fmt.Errorf("lock close request: %w", err)
		}
		if req.Status != period.CloseRequestPending {
			return fmt.Errorf("%w: cannot approve request in status %q", apperrors.ErrInvalidInput, req.Status)
		}

		// 1. Compute trial balance.
		td, tc, imb, err := s.repo.ComputeTrialBalance(ctx, tx, req.PeriodID)
		if err != nil {
			return fmt.Errorf("compute trial balance: %w", err)
		}

		// 2. Validate: must be balanced (debit == credit → imbalance == 0).
		if imb != 0 {
			// Mark rejected, keep period in 'closing' for operator review.
			if err := s.repo.DecideCloseRequest(ctx, tx, req.ID,
				period.CloseRequestRejected, in.ApproverID,
				td, tc, imb,
				fmt.Sprintf("trial balance imbalance: debit=%d credit=%d imbalance=%d", td, tc, imb),
				now,
			); err != nil {
				return fmt.Errorf("mark rejected: %w", err)
			}
			s.log.Warn("period close rejected: trial balance imbalance",
				"period_id", req.PeriodID,
				"request_id", req.ID,
				"total_debit", td, "total_credit", tc, "imbalance", imb,
			)
			return fmt.Errorf("%w: debit=%d credit=%d imbalance=%d", apperrors.ErrDoubleEntryViolation, td, tc, imb)
		}

		// 3. Generate per-account snapshots.
		accounts, err := s.repo.ListAccountsByTenant(ctx, req.TenantID)
		if err != nil {
			return fmt.Errorf("list accounts for snapshot: %w", err)
		}
		for _, a := range accounts {
			bal, cnt, err := s.repo.ComputeAccountBalanceAtPeriod(ctx, tx, a.ID, req.PeriodID)
			if err != nil {
				return fmt.Errorf("compute balance for account %s: %w", a.ID, err)
			}
			if cnt == 0 {
				continue // skip accounts with no entries in this period
			}
			snap := period.PeriodSnapshot{
				ID:           uuid.NewString(),
				TenantID:     req.TenantID,
				PeriodID:     req.PeriodID,
				RequestID:    req.ID,
				AccountID:    a.ID,
				BalanceMinor: bal,
				Currency:     a.Currency,
				EntryCount:   cnt,
				SnapshotAt:   now,
				Metadata:     map[string]any{"source": "period_approve"},
			}
			if err := s.repo.InsertSnapshot(ctx, tx, snap); err != nil {
				return fmt.Errorf("insert snapshot for account %s: %w", a.ID, err)
			}
		}

		// 4. Mark request approved + period closed.
		if err := s.repo.DecideCloseRequest(ctx, tx, req.ID,
			period.CloseRequestApproved, in.ApproverID,
			td, tc, imb,
			"", now,
		); err != nil {
			return fmt.Errorf("mark approved: %w", err)
		}
		if err := s.repo.UpdatePeriodStatus(ctx, tx, req.PeriodID, period.PeriodStatusClosed); err != nil {
			return fmt.Errorf("close period: %w", err)
		}

		// Refresh out with approved values.
		req.Status = period.CloseRequestApproved
		req.ApproverID = in.ApproverID
		req.TrialBalanceOK = true
		req.TotalDebit = money.NewFromMinor(td)
		req.TotalCredit = money.NewFromMinor(tc)
		req.Imbalance = money.NewFromMinor(imb)
		req.DecidedAt = &now
		out = req
		return nil
	})
	if err != nil {
		return period.CloseRequest{}, err
	}

	s.log.Info("period close approved",
		"period_id", out.PeriodID,
		"approver_id", in.ApproverID,
		"request_id", out.ID,
		"total_debit", out.TotalDebit.Minor(),
		"total_credit", out.TotalCredit.Minor(),
	)
	return out, nil
}

func validateDecideClose(requestID, userID string) error {
	if _, err := uuid.Parse(requestID); err != nil {
		return fmt.Errorf("%w: invalid request_id", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(userID); err != nil {
		return fmt.Errorf("%w: invalid approver_id", apperrors.ErrInvalidInput)
	}
	return nil
}

// =============================================================================
// Use case: RejectClose
// =============================================================================

// RejectCloseInput is the input for rejecting a pending close request.
type RejectCloseInput struct {
	RequestID       string
	ApproverID      string
	RejectionReason string
}

// RejectClose rejects a pending close request and returns the period to 'open'.
// Rejection does NOT require trial balance to pass — operator can reject for
// any reason (e.g. missing entries, wrong cutoff, etc.).
func (s *PeriodService) RejectClose(ctx context.Context, in RejectCloseInput) (period.CloseRequest, error) {
	if err := validateDecideClose(in.RequestID, in.ApproverID); err != nil {
		return period.CloseRequest{}, err
	}
	if in.RejectionReason == "" {
		return period.CloseRequest{}, fmt.Errorf("%w: rejection_reason required", apperrors.ErrInvalidInput)
	}

	now := s.now()
	var out period.CloseRequest

	err := s.db.ExecuteTx(ctx, func(tx period.Tx) error {
		req, err := s.repo.LockCloseRequest(ctx, tx, in.RequestID)
		if err != nil {
			return fmt.Errorf("lock close request: %w", err)
		}
		if req.Status != period.CloseRequestPending {
			return fmt.Errorf("%w: cannot reject request in status %q", apperrors.ErrInvalidInput, req.Status)
		}

		if err := s.repo.DecideCloseRequest(ctx, tx, req.ID,
			period.CloseRequestRejected, in.ApproverID,
			req.TotalDebit.Minor(), req.TotalCredit.Minor(), req.Imbalance.Minor(),
			in.RejectionReason, now,
		); err != nil {
			return fmt.Errorf("mark rejected: %w", err)
		}
		if err := s.repo.UpdatePeriodStatus(ctx, tx, req.PeriodID, period.PeriodStatusOpen); err != nil {
			return fmt.Errorf("reopen period: %w", err)
		}

		req.Status = period.CloseRequestRejected
		req.ApproverID = in.ApproverID
		req.RejectionReason = in.RejectionReason
		req.DecidedAt = &now
		out = req
		return nil
	})
	if err != nil {
		return period.CloseRequest{}, err
	}

	s.log.Info("period close rejected",
		"period_id", out.PeriodID,
		"approver_id", in.ApproverID,
		"request_id", out.ID,
		"reason", in.RejectionReason,
	)
	return out, nil
}

// =============================================================================
// Use case: Reopen
// =============================================================================

// ReopenInput is the input for reopening a closed period (admin only).
//
// Reopening a closed period is normally NOT allowed (that's the whole point of
// closing it!). This use case exists for emergency corrections (e.g. found a
// missing entry that affects the period). The caller MUST:
//   - have admin permission (enforced by RBAC middleware)
//   - provide a reason (audit trail)
//
// Reopening does NOT delete snapshots; it just flips status back to 'open'
// so new entries can be posted. Snapshots remain as "what we thought at close
// time" for audit comparison vs. the new state.
type ReopenInput struct {
	PeriodID string
	AdminID  string
	Reason   string
}

// Reopen flips a closed period back to 'open'. Admin-only (RBAC enforced at handler).
func (s *PeriodService) Reopen(ctx context.Context, in ReopenInput) (period.Period, error) {
	if _, err := uuid.Parse(in.PeriodID); err != nil {
		return period.Period{}, fmt.Errorf("%w: invalid period_id", apperrors.ErrInvalidInput)
	}
	if _, err := uuid.Parse(in.AdminID); err != nil {
		return period.Period{}, fmt.Errorf("%w: invalid admin_id", apperrors.ErrInvalidInput)
	}
	if in.Reason == "" {
		return period.Period{}, fmt.Errorf("%w: reason required for reopen", apperrors.ErrInvalidInput)
	}

	var out period.Period
	err := s.db.ExecuteTx(ctx, func(tx period.Tx) error {
		p, err := s.repo.LockPeriod(ctx, tx, in.PeriodID)
		if err != nil {
			return fmt.Errorf("lock period: %w", err)
		}
		if p.Status != period.PeriodStatusClosed {
			return fmt.Errorf("%w: cannot reopen period in status %q", apperrors.ErrInvalidInput, p.Status)
		}
		if err := s.repo.UpdatePeriodStatus(ctx, tx, in.PeriodID, period.PeriodStatusOpen); err != nil {
			return fmt.Errorf("reopen: %w", err)
		}
		p.Status = period.PeriodStatusOpen
		out = p
		return nil
	})
	if err != nil {
		return period.Period{}, err
	}

	s.log.Warn("period reopened",
		"period_id", in.PeriodID,
		"admin_id", in.AdminID,
		"reason", in.Reason,
	)
	return out, nil
}

// =============================================================================
// Query helpers
// =============================================================================

// GetRequest returns one close request by id (read-only).
func (s *PeriodService) GetRequest(ctx context.Context, id string) (period.CloseRequest, error) {
	return s.repo.GetCloseRequest(ctx, id)
}

// ListSnapshotsByPeriod returns all frozen snapshots for a period.
func (s *PeriodService) ListSnapshotsByPeriod(ctx context.Context, periodID string) ([]period.PeriodSnapshot, error) {
	return s.repo.ListSnapshotsByPeriod(ctx, periodID)
}

// ListRequestsByPeriod returns the full audit trail of close attempts for a period.
func (s *PeriodService) ListRequestsByPeriod(ctx context.Context, periodID string) ([]period.CloseRequest, error) {
	return s.repo.ListRequestsByPeriod(ctx, periodID)
}

// Compile-time guard: ensure PeriodTxRunner interface is satisfied by common adapters.
var _ PeriodTxRunner = (*periodTxRunnerAdapter)(nil)

type periodTxRunnerAdapter struct {
	exec func(ctx context.Context, fn func(period.Tx) error) error
}

func (a *periodTxRunnerAdapter) ExecuteTx(ctx context.Context, fn func(period.Tx) error) error {
	return a.exec(ctx, fn)
}

// NewPeriodTxRunnerAdapter wraps a closure into a PeriodTxRunner.
// Used in main.go to wire RunInTxPeriodDomain into the use case.
func NewPeriodTxRunnerAdapter(exec func(ctx context.Context, fn func(period.Tx) error) error) PeriodTxRunner {
	return &periodTxRunnerAdapter{exec: exec}
}

// Sentinel for error matching in tests (errors.Is compatibility).
var ErrPeriodNotClosable = errors.New("period not closable in current status")
