// Package usecase - fraud scanner service (Sprint 31 / Fase 8).
//
// FraudScannerService is the usecase-layer orchestrator that:
//  1. Receives a NATS `fmcg.transfer.posted` event (Sprint 24)
//  2. Decodes the payload into a fraud.TransferEvent
//  3. Runs each Rule, collecting all matches
//  4. Persists one Flag per match (via fraud.Repository)
//  5. For critical flags, also creates a notification row so the user
//     sees it in GET /v1/notifications (Sprint 28)
//
// Event payload shape (mirrors transfer_service outbox payload):
//
//   {
//     "id": "<transfer-uuid>",
//     "tenant_id": "<tenant-uuid>",
//     "from_account_id": "<uuid>",
//     "to_account_id": "<uuid>",
//     "amount_minor": 5000000,
//     "currency": "IDR",
//     "occurred_at": "2026-10-01T10:30:00Z"
//   }
//
// Production extension points:
//   - Rule priority / severity-based short-circuit (e.g. skip lower-severity
//     rules if a critical rule already fired — saves DB writes)
//   - User-tunable thresholds stored in DB per tenant (currently env-only)
//   - Real-time push via WebSocket when critical flag is created
package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/runut/fmcg-wallet/internal/domain/fraud"
	"github.com/runut/fmcg-wallet/internal/domain/notification"
	"github.com/runut/fmcg-wallet/internal/platform/tenantctx"
)

// FraudDeps bundles dependencies for FraudScannerService.
type FraudDeps struct {
	Repo          fraud.Repository
	NotifNotifier NotificationCreator // Sprint 31: emit notification for critical flags
	Broker        EventSubscriber
	Subjects      []string // e.g. ["fmcg.transfer.posted"]
	// Rules returns the per-tenant ruleset for evaluation. Sprint 38:
	// closure that resolves per-tenant settings (with cache) on demand.
	Rules func(ctx context.Context, tenantID uuid.UUID) []fraud.Rule
	Logger        *slog.Logger
}

// NotificationCreator is the minimal interface FraudScannerService needs
// from notification_service (Sprint 28) to emit a notification row when
// a critical flag fires. Avoids importing the usecase package directly.
type NotificationCreator interface {
	CreateDirect(ctx context.Context, n notification.Notification) error
}

// FraudScannerService evaluates rules and persists flags.
type FraudScannerService struct {
	repo       fraud.Repository
	notifier   NotificationCreator
	broker     EventSubscriber
	subjects   []string
	rules      func(ctx context.Context, tenantID uuid.UUID) []fraud.Rule
	log        *slog.Logger
}

// NewFraudScannerService constructs a FraudScannerService.
func NewFraudScannerService(deps FraudDeps) *FraudScannerService {
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	return &FraudScannerService{
		repo:     deps.Repo,
		notifier: deps.NotifNotifier,
		broker:   deps.Broker,
		subjects: deps.Subjects,
		rules:    deps.Rules,
		log:      log,
	}
}

// Subscribe connects to NATS and starts consuming transfer events.
// Returns immediately. Caller is responsible for keeping the process alive.
func (s *FraudScannerService) Subscribe(ctx context.Context) error {
	if len(s.subjects) == 0 {
		return fmt.Errorf("fraud scanner: no subjects configured")
	}
	for _, subj := range s.subjects {
		handler := func(msg *nats.Msg) {
			if err := s.handleEvent(ctx, msg.Subject, msg.Data); err != nil {
				s.log.Warn("fraud scanner: handler error",
					"subject", msg.Subject, "error", err)
			}
		}
		if _, err := s.broker.Subscribe(subj, handler); err != nil {
			return fmt.Errorf("fraud scanner: subscribe %s: %w", subj, err)
		}
		s.log.Info("fraud scanner: subscribed", "subject", subj)
	}
	return nil
}

// handleEvent decodes the payload, runs all rules, persists flags.
func (s *FraudScannerService) handleEvent(ctx context.Context, subject string, payload []byte) error {
	var ev struct {
		ID            uuid.UUID `json:"id"`
		TenantID      uuid.UUID `json:"tenant_id"`
		FromAccountID uuid.UUID `json:"from_account_id"`
		ToAccountID uuid.UUID `json:"to_account_id"`
		AmountMinor   int64     `json:"amount_minor"`
		Currency      string    `json:"currency"`
		OccurredAt    time.Time `json:"occurred_at"`
		InitiatorID   uuid.UUID `json:"initiator_id"`
	}
	if err := json.Unmarshal(payload, &ev); err != nil {
		s.log.Warn("fraud scanner: malformed payload", "subject", subject, "error", err)
		return nil
	}
	if ev.TenantID == uuid.Nil || ev.FromAccountID == uuid.Nil {
		s.log.Warn("fraud scanner: missing tenant_id/from_account_id", "transfer_id", ev.ID)
		return nil
	}

	te := fraud.TransferEvent{
		TenantID:    ev.TenantID,
		TransferID:  ev.ID,
		FromAccount: ev.FromAccountID,
		ToAccount:   ev.ToAccountID,
		AmountMinor: ev.AmountMinor,
		Currency:    ev.Currency,
		OccurredAt:  ev.OccurredAt,
		InitiatorID: ev.InitiatorID,
	}

	// Attach tenant ctx so the repo's Create can bind GUC via RunInTxFraudDomain
	txCtx := tenantctx.WithInfo(ctx, &tenantctx.Info{
		TenantID:   ev.TenantID,
		UserID:     ev.InitiatorID,
		IsSalesRep: false,
	})

	flagsCreated := 0
	// Sprint 38: resolve rules per-tenant (each tenant may have different
	// thresholds + disabled rules).
	rules := s.rules(txCtx, te.TenantID)
	for _, rule := range rules {
		matches, err := rule.Evaluate(txCtx, te)
		if err != nil {
			s.log.Warn("fraud rule evaluation failed",
				"rule", rule.Name(), "transfer_id", te.TransferID, "error", err)
			continue
		}
		for _, m := range matches {
			flag := fraud.Flag{
				ID:         uuid.New(),
				TenantID:   te.TenantID,
				AccountID:  te.FromAccount,
				TransferID: te.TransferID,
				RuleName:   m.RuleName,
				Severity:   m.Severity,
				Status:     fraud.StatusOpen,
				Evidence:   m.Evidence,
				DetectedAt: time.Now().UTC(),
			}
			if err := s.persistFlag(txCtx, flag); err != nil {
				s.log.Warn("fraud scanner: persist flag failed",
					"rule", m.RuleName, "transfer_id", te.TransferID, "error", err)
				continue
			}
			flagsCreated++

			// Critical → emit notification so user sees it via Sprint 28 feed.
			if m.Severity == fraud.SeverityCritical && s.notifier != nil {
				if err := s.emitCriticalNotification(txCtx, flag, te); err != nil {
					s.log.Warn("fraud scanner: notification emit failed",
						"rule", m.RuleName, "error", err)
				}
			}
		}
	}

	if flagsCreated > 0 {
		s.log.Info("fraud scanner: flags created",
			"transfer_id", te.TransferID, "count", flagsCreated)
	}
	return nil
}

// persistFlag writes one flag row inside a tx that binds tenant GUC.
// Mirrors notification_service.persistNotification pattern.
func (s *FraudScannerService) persistFlag(ctx context.Context, f fraud.Flag) error {
	return s.repo.PersistDirect(ctx, f)
}

// emitCriticalNotification creates a notification row so the user sees
// the critical fraud alert via GET /v1/notifications (Sprint 28).
func (s *FraudScannerService) emitCriticalNotification(ctx context.Context, f fraud.Flag, te fraud.TransferEvent) error {
	n := notification.Notification{
		ID:       uuid.New(),
		TenantID: f.TenantID,
		UserID:   te.InitiatorID,
		Type:     "fraud.flagged",
		Title:    fmt.Sprintf("Fraud alert: %s", humanReadableRuleName(f.RuleName)),
		Body: map[string]any{
			"flag_id":     f.ID.String(),
			"rule_name":   string(f.RuleName),
			"transfer_id": te.TransferID.String(),
			"severity":    string(f.Severity),
			"evidence":    f.Evidence,
		},
		Severity:  notification.SeverityCritical,
		Status:    notification.StatusUnread,
		CreatedAt: time.Now().UTC(),
	}
	return s.notifier.CreateDirect(ctx, n)
}

func humanReadableRuleName(r fraud.RuleName) string {
	switch r {
	case fraud.RuleLargeAmount:
		return "Large transfer amount"
	case fraud.RuleOffHours:
		return "Off-hours transfer"
	case fraud.RuleVelocity:
		return "High-velocity transfers"
	case fraud.RuleFirstTimeRecipient:
		return "First-time recipient"
	default:
		return string(r)
	}
}