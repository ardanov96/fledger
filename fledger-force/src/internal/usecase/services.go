// Package usecase — services wiring.
package usecase

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-force/internal/integration/coreclient"
	"github.com/fledger/fledger-force/internal/repository/postgres"
)

// Deps bundles repositories + adapters.
type Deps struct {
	Pool        *pgxpool.Pool
	Reps        *postgres.RepRepo
	Stores      *postgres.StoreRepo
	Visits      *postgres.VisitRepo
	Collections *postgres.CollectionRepo
	Settlements *postgres.SettlementRepo
	Outbox      *postgres.OutboxRepo
	Audit       *postgres.AuditRepo
	Core        *coreclient.Client
}

// Services is the bag of application services.
type Services struct {
	Rep        *RepService
	Store      *StoreService
	Visit      *VisitService
	Collection *CollectionService
}

func NewServices(d Deps) *Services {
	return &Services{
		Rep:        NewRepService(d.Reps),
		Store:      NewStoreService(d.Stores),
		Visit:      NewVisitService(d.Visits, d.Stores, d.Audit),
		Collection: NewCollectionService(d.Pool, d.Collections, d.Settlements, d.Reps, d.Stores, d.Visits, d.Outbox, d.Audit, d.Core),
	}
}