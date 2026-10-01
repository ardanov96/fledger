// Package usecase - fraud scanner service tests (Sprint 31).
//
// Tests the orchestration logic end-to-end with a fake repo + broker.
// No DB / NATS dependencies.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/runut/fmcg-wallet/internal/domain/fraud"
	"github.com/runut/fmcg-wallet/internal/domain/notification"
)

// =============================================================================
// Fakes
// =============================================================================

type fakeFraudRepo struct {
	mu     sync.Mutex
	flags  []fraud.Flag
	createError error
}

func (r *fakeFraudRepo) Create(_ context.Context, _ fraud.Tx, f fraud.Flag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createError != nil {
		return r.createError
	}
	r.flags = append(r.flags, f)
	return nil
}

func (r *fakeFraudRepo) PersistDirect(_ context.Context, f fraud.Flag) error {
	return r.Create(context.Background(), nil, f)
}

func (r *fakeFraudRepo) ListByAccount(_ context.Context, _, _ uuid.UUID, _ int) ([]fraud.Flag, error) {
	return r.flags, nil
}

func (r *fakeFraudRepo) CountRecentByAccount(_ context.Context, _, _ uuid.UUID, _ time.Duration) (int, error) {
	return 0, nil
}

func (r *fakeFraudRepo) HasRecipientHistory(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
	return false, nil
}

type fakeNotifCreator struct {
	mu      sync.Mutex
	calls   []notification.Notification
	err     error
}

func (n *fakeNotifCreator) CreateDirect(_ context.Context, notif notification.Notification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.err != nil {
		return n.err
	}
	n.calls = append(n.calls, notif)
	return nil
}

// fakeFraudBroker captures subscriptions; for tests we we don't actually need
// nats.Msg — handleEvent takes raw payload bytes.
type fakeFraudBroker struct{}

func (b *fakeFraudBroker) Subscribe(_ string, _ nats.MsgHandler) (*nats.Subscription, error) {
	return nil, nil
}

// =============================================================================
// Tests
// =============================================================================

// sampleTransferPayload builds a NATS-style payload for transfer.posted.
func sampleTransferPayload(t *testing.T) ([]byte, fraud.TransferEvent) {
	t.Helper()
	ev := fraud.TransferEvent{
		TenantID:    uuid.New(),
		TransferID:  uuid.New(),
		FromAccount: uuid.New(),
		ToAccount:   uuid.New(),
		AmountMinor: 100_000_000, // 100M IDR — will trigger large_amount rule
		Currency:    "IDR",
		OccurredAt:  time.Now(),
		InitiatorID: uuid.New(),
	}
	payload, err := json.Marshal(map[string]any{
		"id":             ev.TransferID,
		"tenant_id":      ev.TenantID,
		"from_account_id": ev.FromAccount,
		"to_account_id":  ev.ToAccount,
		"amount_minor":   ev.AmountMinor,
		"currency":       ev.Currency,
		"occurred_at":    ev.OccurredAt,
		"initiator_id":   ev.InitiatorID,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return payload, ev
}

func TestFraudScanner_LargeAmount_CreatesFlag_AndCriticalNotification(t *testing.T) {
	repo := &fakeFraudRepo{}
	notif := &fakeNotifCreator{}
	svc := NewFraudScannerService(FraudDeps{
		Repo:          repo,
		NotifNotifier: notif,
		Broker:        &fakeFraudBroker{},
		Subjects:      []string{"fmcg.transfer.posted"},
		Rules: []fraud.Rule{
			fraud.LargeAmountRule{ThresholdMinor: 50_000_000},
		},
		Logger: slog.Default(),
	})

	payload, _ := sampleTransferPayload(t)
	if err := svc.handleEvent(context.Background(), "fmcg.transfer.posted", payload); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}

	if len(repo.flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(repo.flags))
	}
	if repo.flags[0].RuleName != fraud.RuleLargeAmount {
		t.Errorf("rule = %q, want large_amount", repo.flags[0].RuleName)
	}
	if repo.flags[0].Severity != fraud.SeverityCritical {
		t.Errorf("severity = %q, want critical", repo.flags[0].Severity)
	}

	// Critical severity should trigger a notification
	if len(notif.calls) != 1 {
		t.Fatalf("expected 1 notification for critical flag, got %d", len(notif.calls))
	}
	if notif.calls[0].Severity != notification.SeverityCritical {
		t.Errorf("notification severity = %q, want critical", notif.calls[0].Severity)
	}
	if notif.calls[0].Type != "fraud.flagged" {
		t.Errorf("notification type = %q, want fraud.flagged", notif.calls[0].Type)
	}
}

func TestFraudScanner_InfoSeverity_NoNotification(t *testing.T) {
	repo := &fakeFraudRepo{}
	notif := &fakeNotifCreator{}
	svc := NewFraudScannerService(FraudDeps{
		Repo:          repo,
		NotifNotifier: notif,
		Broker:        &fakeFraudBroker{},
		Subjects:      []string{"fmcg.transfer.posted"},
		Rules: []fraud.Rule{
			fraud.FirstTimeRecipientRule{
				HasHistory: func(_ context.Context, _, _, _ uuid.UUID) (bool, error) {
					return false, nil // always first-time
				},
			},
		},
		Logger: slog.Default(),
	})

	payload, _ := sampleTransferPayload(t)
	if err := svc.handleEvent(context.Background(), "fmcg.transfer.posted", payload); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}

	if len(repo.flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(repo.flags))
	}
	if repo.flags[0].Severity != fraud.SeverityInfo {
		t.Errorf("severity = %q, want info", repo.flags[0].Severity)
	}

	// Info severity should NOT trigger a notification
	if len(notif.calls) != 0 {
		t.Errorf("info severity should not notify, got %d calls", len(notif.calls))
	}
}

func TestFraudScanner_NoMatchingRules_NoFlags(t *testing.T) {
	repo := &fakeFraudRepo{}
	notif := &fakeNotifCreator{}
	svc := NewFraudScannerService(FraudDeps{
		Repo:          repo,
		NotifNotifier: notif,
		Broker:        &fakeFraudBroker{},
		Subjects:      []string{"fmcg.transfer.posted"},
		Rules: []fraud.Rule{
			fraud.LargeAmountRule{ThresholdMinor: 1_000_000_000}, // 1B IDR — too high
		},
		Logger: slog.Default(),
	})

	payload, _ := sampleTransferPayload(t)
	if err := svc.handleEvent(context.Background(), "fmcg.transfer.posted", payload); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}

	if len(repo.flags) != 0 {
		t.Errorf("expected 0 flags (no rule matched), got %d", len(repo.flags))
	}
}

func TestFraudScanner_MalformedPayload_NoError(t *testing.T) {
	repo := &fakeFraudRepo{}
	notif := &fakeNotifCreator{}
	svc := NewFraudScannerService(FraudDeps{
		Repo: repo, NotifNotifier: notif, Broker: &fakeFraudBroker{},
		Subjects: []string{"fmcg.transfer.posted"},
		Rules:    []fraud.Rule{fraud.LargeAmountRule{ThresholdMinor: 1}},
		Logger:   slog.Default(),
	})

	// Garbage payload — handler should swallow (logged warn) and return nil
	if err := svc.handleEvent(context.Background(), "fmcg.transfer.posted", []byte("not json")); err != nil {
		t.Errorf("malformed payload should be swallowed, got error: %v", err)
	}
	if len(repo.flags) != 0 {
		t.Errorf("expected 0 flags on malformed payload, got %d", len(repo.flags))
	}
}

func TestFraudScanner_RepoError_DoesNotPanic(t *testing.T) {
	repo := &fakeFraudRepo{createError: errors.New("disk full")}
	notif := &fakeNotifCreator{}
	svc := NewFraudScannerService(FraudDeps{
		Repo: repo, NotifNotifier: notif, Broker: &fakeFraudBroker{},
		Subjects: []string{"fmcg.transfer.posted"},
		Rules:    []fraud.Rule{fraud.LargeAmountRule{ThresholdMinor: 1}},
		Logger:   slog.Default(),
	})

	payload, _ := sampleTransferPayload(t)
	// Should NOT return error to caller — handler swallows repo errors
	if err := svc.handleEvent(context.Background(), "fmcg.transfer.posted", payload); err != nil {
		t.Errorf("handleEvent should swallow repo errors, got: %v", err)
	}
}

func TestFraudScanner_MultipleRulesMatching_CreatesMultipleFlags(t *testing.T) {
	repo := &fakeFraudRepo{}
	notif := &fakeNotifCreator{}
	svc := NewFraudScannerService(FraudDeps{
		Repo: repo, NotifNotifier: notif, Broker: &fakeFraudBroker{},
		Subjects: []string{"fmcg.transfer.posted"},
		Rules: []fraud.Rule{
			fraud.LargeAmountRule{ThresholdMinor: 1},
			fraud.FirstTimeRecipientRule{
				HasHistory: func(_ context.Context, _, _, _ uuid.UUID) (bool, error) { return false, nil },
			},
		},
		Logger: slog.Default(),
	})

	payload, _ := sampleTransferPayload(t)
	if err := svc.handleEvent(context.Background(), "fmcg.transfer.posted", payload); err != nil {
		t.Fatalf("handleEvent: %v", err)
	}

	if len(repo.flags) != 2 {
		t.Errorf("expected 2 flags (large_amount + first_time_recipient), got %d", len(repo.flags))
	}
	// 1 critical + 1 info = 1 notification (only critical triggers)
	if len(notif.calls) != 1 {
		t.Errorf("expected 1 notification (critical only), got %d", len(notif.calls))
	}
}
