// nats_subscriber.go — Sprint 24 / Fase 4A.
//
// Wires the NATS subscriber for the API process. Logs every `transfer.posted`
// event as a smoke test that the publisher → broker → subscriber loop works.
// Production subscribers (notification dispatcher, fraud scanner, projection
// writer, etc.) would register their own handlers here or in cmd/worker.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"

	"github.com/runut/fmcg-wallet/internal/domain/outbox"
	"github.com/runut/fmcg-wallet/internal/infra"
	"github.com/runut/fmcg-wallet/internal/platform/config"
)

// startEventSubscriber connects to NATS and subscribes to the configured
// stream subjects. Returns the *nats.Conn for shutdown / readiness.
func startEventSubscriber(ctx context.Context, cfg config.NATSConfig, log *slog.Logger) (*infra.NATSClient, error) {
	client, err := infra.NewNATSClient(ctx, cfg, log)
	if err != nil {
		return nil, fmt.Errorf("start subscriber: %w", err)
	}

	subject := outbox.SubjectTransferPosted
	handler := makeTransferPostedLogHandler(log)
	if _, err := client.Subscribe(subject, handler); err != nil {
		client.Close()
		return nil, fmt.Errorf("subscribe %s: %w", subject, err)
	}
	log.Info("event subscriber started", "subject", subject)
	return client, nil
}

// makeTransferPostedLogHandler logs every received transfer.posted event.
// Real subscribers (notification dispatcher, fraud scanner, etc.) would
// register additional handlers — the handler must be idempotent because
// NATS may redeliver on consumer crash.
func makeTransferPostedLogHandler(log *slog.Logger) nats.MsgHandler {
	return func(msg *nats.Msg) {
		var payload map[string]any
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Warn("transfer.posted handler: invalid JSON",
				"subject", msg.Subject,
				"data_len", len(msg.Data),
				"error", err,
			)
			return
		}
		log.Info("transfer.posted received",
			"subject", msg.Subject,
			"transaction_id", payload["transaction_id"],
			"tenant_id", payload["tenant_id"],
			"amount_minor", payload["amount_minor"],
			"currency", payload["currency"],
			"is_cross_cur", payload["is_cross_cur"],
		)
	}
}
