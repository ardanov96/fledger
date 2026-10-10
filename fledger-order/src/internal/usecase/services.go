// Package usecase — services wiring.
package usecase

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-order/internal/integration/coreclient"
	"github.com/fledger/fledger-order/internal/integration/fleetclient"
	"github.com/fledger/fledger-order/internal/platform/credit"
	"github.com/fledger/fledger-order/internal/repository/postgres"
)

// Deps bundles repositories + adapters.
type Deps struct {
	Pool         *pgxpool.Pool
	Products     *postgres.ProductRepo
	Prices       *postgres.PricingRepo
	Inventory    *postgres.InventoryRepo
	Orders       *postgres.OrderRepo
	Outbox       *postgres.OutboxRepo
	Audit        *postgres.AuditRepo
	Core         *coreclient.Client
	Fleet        *fleetclient.Client
	OverridePIN  string
}

// Services is the bag of application services.
type Services struct {
	Product *ProductService
	Pricing *PricingService
	Inv     *InventoryService
	Order   *OrderService
}

func NewServices(d Deps) *Services {
	evaluator := credit.NewEvaluator(d.Core)
	return &Services{
		Product: NewProductService(d.Products),
		Pricing: NewPricingService(d.Prices),
		Inv:     NewInventoryService(d.Inventory),
		Order: NewOrderService(
			d.Orders, d.Products, d.Prices, d.Inventory, d.Outbox, d.Audit,
			d.Core, d.Fleet, evaluator, d.OverridePIN,
		),
	}
}