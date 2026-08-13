// period_adapters.go — adapters to wire PeriodService → handler.PeriodAPI.
//
// Two pieces:
//
//  1. periodTxAdapter — adapts DB.RunInTxPeriodDomain → usecase.PeriodTxRunner
//  2. periodAPIAdapter — adapts usecase.PeriodService → handler.PeriodAPI
//     (translates input types from handler package to usecase package)
package main

import (
	"context"

	"github.com/runut/fmcg-wallet/internal/domain/period"
	"github.com/runut/fmcg-wallet/internal/handler"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// =============================================================================
// periodTxAdapter — wraps *postgres.DB.RunInTxPeriodDomain as usecase.PeriodTxRunner
// =============================================================================

type periodTxAdapter struct {
	db *postgres.DB
}

func (a *periodTxAdapter) ExecuteTx(ctx context.Context, fn func(period.Tx) error) error {
	return a.db.RunInTxPeriodDomain(ctx, fn)
}

// =============================================================================
// periodAPIAdapter — adapts *usecase.PeriodService to handler.PeriodAPI
// =============================================================================

// periodAPIAdapter wraps a *usecase.PeriodService and translates the input
// types between packages (handler.PeriodXInput ↔ usecase.XInput). The two
// types have the same field layout; we just copy fields.
type periodAPIAdapter struct {
	svc *usecase.PeriodService
}

func (a *periodAPIAdapter) RequestClose(ctx context.Context, in handler.PeriodRequestCloseInput) (period.CloseRequest, error) {
	return a.svc.RequestClose(ctx, usecase.RequestCloseInput{
		TenantID:    in.TenantID,
		PeriodID:    in.PeriodID,
		RequesterID: in.RequesterID,
		Metadata:    in.Metadata,
	})
}

func (a *periodAPIAdapter) ApproveClose(ctx context.Context, in handler.PeriodApproveCloseInput) (period.CloseRequest, error) {
	return a.svc.ApproveClose(ctx, usecase.ApproveCloseInput{
		RequestID:  in.RequestID,
		ApproverID: in.ApproverID,
	})
}

func (a *periodAPIAdapter) RejectClose(ctx context.Context, in handler.PeriodRejectCloseInput) (period.CloseRequest, error) {
	return a.svc.RejectClose(ctx, usecase.RejectCloseInput{
		RequestID:       in.RequestID,
		ApproverID:      in.ApproverID,
		RejectionReason: in.RejectionReason,
	})
}

func (a *periodAPIAdapter) Reopen(ctx context.Context, in handler.PeriodReopenInput) (period.Period, error) {
	return a.svc.Reopen(ctx, usecase.ReopenInput{
		PeriodID: in.PeriodID,
		AdminID:  in.AdminID,
		Reason:   in.Reason,
	})
}

func (a *periodAPIAdapter) GetRequest(ctx context.Context, id string) (period.CloseRequest, error) {
	return a.svc.GetRequest(ctx, id)
}

func (a *periodAPIAdapter) ListSnapshotsByPeriod(ctx context.Context, periodID string) ([]period.PeriodSnapshot, error) {
	return a.svc.ListSnapshotsByPeriod(ctx, periodID)
}

// Compile-time guard: ensure periodAPIAdapter satisfies handler.PeriodAPI.
var _ handler.PeriodAPI = (*periodAPIAdapter)(nil)
