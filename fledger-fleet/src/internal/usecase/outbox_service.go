package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fledger/fledger-fleet/internal/domain/outbox"
	apperrors "github.com/fledger/fledger-fleet/internal/platform/errors"
	"github.com/fledger/fledger-fleet/internal/integration/coreclient"
	"github.com/fledger/fledger-fleet/internal/repository/postgres"
)

// OutboxService drains the durable outbox table to Fledger Core.
type OutboxService struct {
	repo   *postgres.OutboxRepo
	client *coreclient.Client
	log    *slog.Logger
}

func NewOutboxService(repo *postgres.OutboxRepo, c *coreclient.Client) *OutboxService {
	return &OutboxService{repo: repo, client: c, log: slog.Default()}
}

// SetLogger lets the caller inject a structured logger (mainly for main).
func (s *OutboxService) SetLogger(l *slog.Logger) { s.log = l }

// MaxAttempts returns the per-row attempt cap (used by the worker).
const DefaultMaxAttempts = 8

// DrainOnce fetches and attempts the next batch of due rows.
func (s *OutboxService) DrainOnce(ctx context.Context, batch int, maxAttempts int) (sent, failed int, err error) {
	if batch <= 0 {
		batch = 16
	}
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxAttempts
	}
	due, err := s.repo.FetchDue(ctx, batch)
	if err != nil {
		return 0, 0, err
	}
	for _, e := range due {
		if e.AggregateType != "DELIVERY_ORDER" || e.EventType != "DO_POD_SUBMITTED" {
			// Unknown event type — mark sent so we don't loop forever.
			if err := s.repo.MarkSent(ctx, e.ID); err != nil {
				return sent, failed, err
			}
			sent++
			continue
		}
		inv := coreclient.InvoiceInput{
			CustomerID:  stringField(e.Payload, "customer_id"),
			Code:        stringField(e.Payload, "code"),
			AmountMinor: int64Field(e.Payload, "amount_minor"),
			DueDate:     stringField(e.Payload, "due_date"),
			Description: stringField(e.Payload, "description"),
			// Idempotency-Key = DO UUID (e.AggregateID) so the worker's
			// retries collapse to a single invoice in Core.
			IdempotencyKey: e.AggregateID,
			Metadata: map[string]any{
				"do_id":        e.AggregateID,
				"fleet_source": "fledger-fleet",
			},
		}
		if inv.Code == "" || inv.CustomerID == "" {
			errMsg := fmt.Sprintf("invalid payload: %+v", e.Payload)
			_ = s.repo.MarkFailed(ctx, e.ID, errMsg, e.Attempts, maxAttempts, 30*time.Second)
			failed++
			continue
		}
		resp, err := s.client.CreateInvoice(ctx, inv)
		if err != nil {
			// Upstream unavailable OR validation error → keep retrying unless
			// we've exhausted attempts.
			if errors.Is(err, apperrors.ErrInvalidInput) {
				// Permanent: stop retrying.
				_ = s.repo.MarkFailed(ctx, e.ID, err.Error(), maxAttempts, maxAttempts, 0)
				failed++
				continue
			}
			backoff := backoffFor(e.Attempts)
			_ = s.repo.MarkFailed(ctx, e.ID, err.Error(), e.Attempts, maxAttempts, backoff)
			failed++
			continue
		}
		if err := s.repo.MarkSent(ctx, e.ID); err != nil {
			return sent, failed, err
		}
		s.log.Info("outbox sent",
			"event_id", e.ID,
			"do_id", e.AggregateID,
			"invoice_id", resp.ID,
		)
		sent++
	}
	return sent, failed, nil
}

// Counts returns the current dashboard view.
func (s *OutboxService) Counts(ctx context.Context, tenantID string) (postgres.OutboxCounts, error) {
	return s.repo.Count(ctx, tenantID)
}

// backoffFor returns the wait time before the Nth retry: 30s, 1m, 2m, 4m, 8m…
// capped at 30m to keep within reason for a demo.
func backoffFor(attempts int) time.Duration {
	if attempts <= 0 {
		return 30 * time.Second
	}
	d := 30 * time.Second
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= 30*time.Minute {
			return 30 * time.Minute
		}
	}
	return d
}

func stringField(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func int64Field(m map[string]any, k string) int64 {
	switch v := m[k].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	}
	return 0
}

// DrainLoop runs DrainOnce on a ticker until ctx is cancelled.
func (s *OutboxService) DrainLoop(ctx context.Context, interval time.Duration, batch, maxAttempts int) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	// First drain immediately.
	_, _, _ = s.DrainOnce(ctx, batch, maxAttempts)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sent, failed, err := s.DrainOnce(ctx, batch, maxAttempts)
			if err != nil {
				s.log.Error("outbox drain", "err", err.Error())
				continue
			}
			if sent > 0 || failed > 0 {
				s.log.Info("outbox drain tick", "sent", sent, "failed", failed)
			}
		}
	}
}

// Avoid unused import if outbox.Event isn't referenced in the public API.
var _ outbox.Event