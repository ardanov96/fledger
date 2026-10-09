// Package usecase holds the application services for Fledger Fleet. Each
// service is a thin orchestrator that wraps repositories + the Fledger Core
// adapter, enforcing business rules.
package usecase

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-fleet/internal/integration/coreclient"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// Deps bundles the repositories / adapters shared by all services.
type Deps struct {
	Pool       *pgxpool.Pool
	Vehicles   *postgres.VehicleRepo
	Drivers    *postgres.DriverRepo
	Trips      *postgres.TripRepo
	DOs        *postgres.DORepo
	PODs       *postgres.PODRepo
	Outbox     *postgres.OutboxRepo
	Audit      *postgres.AuditRepo
	CoreClient *coreclient.Client
}

// Services is the bag of application services instantiated at startup.
type Services struct {
	Vehicle *VehicleService
	Driver  *DriverService
	Trip    *TripService
	DO      *DeliveryOrderService
	POD     *ProofOfDeliveryService
	Outbox  *OutboxService
}

// NewServices wires the bag.
func NewServices(d Deps) *Services {
	return &Services{
		Vehicle: NewVehicleService(d.Vehicles, d.Pool),
		Driver:  NewDriverService(d.Drivers, d.Pool),
		Trip:    NewTripService(d.Trips, d.Vehicles, d.Drivers, d.Pool),
		DO:      NewDeliveryOrderService(d.DOs, d.Pool),
		POD:     NewProofOfDeliveryService(d.PODs, d.DOs, d.Outbox, d.Audit, d.CoreClient, d.Pool),
		Outbox:  NewOutboxService(d.Outbox, d.CoreClient),
	}
}