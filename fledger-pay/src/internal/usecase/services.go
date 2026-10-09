package usecase

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fledger/fledger-pay/internal/integration/coreclient"
	"github.com/fledger/fledger-pay/internal/repository/postgres"
)

// Deps bundles the repositories + adapters shared by all services.
type Deps struct {
	Pool         *pgxpool.Pool
	Payments     *postgres.PaymentRepo
	Transactions *postgres.TransactionRepo
	Outbox       *postgres.OutboxRepo
	Audit        *postgres.AuditRepo
	Core         *coreclient.Client
}

// Services is the bag of application services instantiated at startup.
type Services struct {
	Payment    *PaymentService
	Settlement *SettlementService
}

func NewServices(d Deps) *Services {
	return &Services{
		Payment:    NewPaymentService(d.Payments, d.Audit),
		Settlement: NewSettlementService(d.Payments, d.Transactions, d.Outbox, d.Audit, d.Core),
	}
}