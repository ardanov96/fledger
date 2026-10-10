// Package usecase ??? services wiring.
package usecase

import (
	"github.com/fledger/fledger-dunning/internal/platform/whatsapp"
	"github.com/fledger/fledger-dunning/internal/repository/postgres"
)

// Deps bundles repositories + adapters.
type Deps struct {
	Configs  *postgres.ConfigRepo
	Sessions *postgres.SessionRepo
	Contacts *postgres.StoreContactRepo
	Queues   *postgres.QueueRepo
	Stats    *postgres.StatementRepo
	Logs     *postgres.MessageLogRepo
	Audits   *postgres.AuditRepo
	Provider whatsapp.Provider
	JitterMinSeconds int
	JitterMaxSeconds int
}

// Services bundles all application services.
type Services struct {
	Dunning   *DunningService
	Statement *StatementService
}

func NewServices(d Deps) *Services {
	dunning := NewDunningService(
		d.Configs, d.Sessions, d.Contacts, d.Queues, d.Logs, d.Audits,
		d.Provider, d.JitterMinSeconds, d.JitterMaxSeconds,
	)
	statement := NewStatementService(
		d.Stats, d.Contacts, d.Configs, d.Queues, d.Logs, d.Audits, d.Provider,
	)
	return &Services{Dunning: dunning, Statement: statement}
}