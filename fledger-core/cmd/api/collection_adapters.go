// collection_adapters.go — adapters to wire CollectionService → handler.CollectionAPI.
//
// Mirrors the pattern from period_adapters.go and reconciler_adapters.go.
// CollectionService has narrow dependency interfaces (collection.InvoiceLookup
// and collection.CollectionTxRunner); we satisfy them here against the concrete
// Postgres repos + DB.
package main

import (
	"context"
	"time"

	"github.com/runut/fmcg-wallet/internal/domain/collection"
	"github.com/runut/fmcg-wallet/internal/handler"
	"github.com/runut/fmcg-wallet/internal/repository/postgres"
	"github.com/runut/fmcg-wallet/internal/usecase"
)

// collectionTxAdapter — wraps DB.RunInTxCollectionDomain → usecase.CollectionTxRunner.
type collectionTxAdapter struct {
	db *postgres.DB
}

func (a *collectionTxAdapter) ExecuteTx(ctx context.Context, fn func(collection.Tx) error) error {
	return a.db.RunInTxCollectionDomain(ctx, fn)
}

// invoiceLookupAdapter — implements collection.InvoiceLookup using InvoiceRepository.
type invoiceLookupAdapter struct {
	invoiceRepo *postgres.InvoiceRepository
}

// OutstandingByCustomer returns outstanding (open/partial) invoices for the
// given customer IDs within a tenant. If customerIDs is empty, returns empty.
//
// MVP: delegates to InvoiceRepository.GetAging per customer. Aggregates all
// buckets (current/1-7/8-30/.../90+) into OutstandingInvoiceRef entries.
// Sprint 12 backlog: add a dedicated OutstandingByTenant query with partial index.
func (a *invoiceLookupAdapter) OutstandingByCustomer(ctx context.Context, tenantID string, customerIDs []string) ([]collection.OutstandingInvoiceRef, error) {
	if len(customerIDs) == 0 {
		return []collection.OutstandingInvoiceRef{}, nil
	}
	out := make([]collection.OutstandingInvoiceRef, 0)
	for _, cid := range customerIDs {
		buckets, err := a.invoiceRepo.GetAging(ctx, tenantID, cid)
		if err != nil {
			return nil, err
		}
		for _, b := range buckets {
			if b.OutstandingMinor <= 0 {
				continue
			}
			out = append(out, collection.OutstandingInvoiceRef{
				ID:          b.CustomerID + "-bucket-" + string(b.Bucket),
				CustomerID:  cid,
				AmountMinor: b.OutstandingMinor,
				DueDate:     time.Time{},
			})
		}
	}
	return out, nil
}

// collectionAPIAdapter — adapts usecase.CollectionService → handler.CollectionAPI.
type collectionAPIAdapter struct {
	svc *usecase.CollectionService
}

func (a *collectionAPIAdapter) PlanRoute(ctx context.Context, in collection.PlanRouteInput) (collection.CollectionRoute, []collection.RouteStop, error) {
	return a.svc.PlanRoute(ctx, in)
}

func (a *collectionAPIAdapter) StartRoute(ctx context.Context, routeID string) (collection.CollectionRoute, error) {
	return a.svc.StartRoute(ctx, routeID)
}

func (a *collectionAPIAdapter) CompleteRoute(ctx context.Context, routeID string) (collection.CollectionRoute, error) {
	return a.svc.CompleteRoute(ctx, routeID)
}

func (a *collectionAPIAdapter) SettleRoute(ctx context.Context, in collection.SettleRouteInput) (collection.Settlement, collection.CollectionRoute, error) {
	return a.svc.SettleRoute(ctx, in)
}

func (a *collectionAPIAdapter) ApproveSettlement(ctx context.Context, in collection.ApproveSettlementInput) (collection.Settlement, error) {
	return a.svc.ApproveSettlement(ctx, in)
}

func (a *collectionAPIAdapter) RecordVisit(ctx context.Context, in collection.RecordVisitInput) (collection.CollectionEvent, collection.RouteStop, error) {
	return a.svc.RecordVisit(ctx, in)
}

func (a *collectionAPIAdapter) CloseStop(ctx context.Context, in collection.CloseStopInput) (collection.RouteStop, error) {
	return a.svc.CloseStop(ctx, in)
}

func (a *collectionAPIAdapter) GetRoute(ctx context.Context, id string) (collection.CollectionRoute, error) {
	return a.svc.GetRoute(ctx, id)
}

func (a *collectionAPIAdapter) ListStopsByRoute(ctx context.Context, routeID string) ([]collection.RouteStop, error) {
	return a.svc.ListStopsByRoute(ctx, routeID)
}

func (a *collectionAPIAdapter) ListEventsByStop(ctx context.Context, stopID string) ([]collection.CollectionEvent, error) {
	return a.svc.ListEventsByStop(ctx, stopID)
}

func (a *collectionAPIAdapter) GetSettlementByRoute(ctx context.Context, routeID string) (collection.Settlement, error) {
	return a.svc.GetSettlementByRoute(ctx, routeID)
}

func (a *collectionAPIAdapter) ListRoutesBySalesRep(ctx context.Context, tenantID, salesRepID string, limit int) ([]collection.CollectionRoute, error) {
	return a.svc.ListRoutesBySalesRep(ctx, tenantID, salesRepID, limit)
}

func (a *collectionAPIAdapter) ListRoutesByDate(ctx context.Context, tenantID string, date time.Time) ([]collection.CollectionRoute, error) {
	return a.svc.ListRoutesByDate(ctx, tenantID, date)
}

// Compile-time guards.
var (
	_ handler.CollectionAPI      = (*collectionAPIAdapter)(nil)
	_ usecase.CollectionTxRunner = (*collectionTxAdapter)(nil)
	_ collection.InvoiceLookup   = (*invoiceLookupAdapter)(nil)
)

