// aging_snapshot_adapter.go — Sprint 25 / Fase 4D.
//
// Bridges handler.AgingAPI to usecase.TenantSnapshotService. The handler
// reads from aging_snapshots via this adapter; falls back to live view
// automatically (handled inside TenantSnapshotService).
package main

import (
	"context"

	"github.com/runut/fmcg-wallet/internal/domain/invoice"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// agingSnapshotAPIAdapter implements handler.AgingAPI.
type agingSnapshotAPIAdapter struct {
	svc *usecase.TenantSnapshotService
}

func newAgingSnapshotAPIAdapter(snapRepo *postgres.AgingSnapshotRepository, liveRepo *postgres.InvoiceRepository) *agingSnapshotAPIAdapter {
	svc := usecase.NewTenantSnapshotService(snapRepo, liveRepo, nil)
	return &agingSnapshotAPIAdapter{svc: svc}
}

// GetAgingSnapshot returns aging for the customer. Second return is true when
// data came from the snapshot table; false when it fell back to live view
// (caller may log/metric this for ops visibility).
func (a *agingSnapshotAPIAdapter) GetAgingSnapshot(ctx context.Context, tenantID, customerID string) ([]invoice.AgingSummary, bool, error) {
	return a.svc.GetAgingSnapshot(ctx, tenantID, customerID)
}
