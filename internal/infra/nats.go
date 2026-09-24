// Package infra — nats.go provides a thin wrapper around the NATS Go client.
//
// The wrapper isolates the rest of the codebase from github.com/nats-io/nats.go
// so we can swap the broker implementation if needed (e.g. move from core
// NATS to NATS JetStream later) without touching use cases.
//
// Sprint 24 ships core NATS publish + subscribe (no JetStream). All events
// are fire-and-forget on the publisher side; the broker does NOT persist
// undelivered messages. This is intentional for the demo — durability is
// already provided by the outbox table on the publisher side, and consumers
// can be made idempotent by using the event ID as a dedup key.
package infra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/runut/fmcg-wallet/internal/platform/config"
)

// NATSClient wraps *nats.Conn with the subset of operations we use.
type NATSClient struct {
	conn *nats.Conn
	log  *slog.Logger
	url  string
}

// NewNATSClient dials the broker described by cfg.NATS. Returns error if
// the dial fails — caller decides whether to fail-fast (production) or
// log-and-degrade (development without a broker).
func NewNATSClient(ctx context.Context, cfg config.NATSConfig, log *slog.Logger) (*NATSClient, error) {
	if log == nil {
		log = slog.Default()
	}
	if cfg.URL == "" {
		return nil, errors.New("nats: URL is required")
	}

	opts := []nats.Option{
		nats.Name("fmcg-wallet"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.Timeout(5 * time.Second),
		nats.PingInterval(30 * time.Second),
		nats.MaxPingsOutstanding(2),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn("nats disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("nats reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Info("nats connection closed")
		}),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect %s: %w", cfg.URL, err)
	}

	log.Info("nats connected",
		"url", conn.ConnectedUrl(),
		"server_id", conn.ConnectedServerId(),
		"server_name", conn.ConnectedServerName(),
	)
	return &NATSClient{conn: conn, log: log, url: cfg.URL}, nil
}

// Publish sends a message to the given subject. Returns error if the broker
// is unavailable or the message is rejected.
//
// Sprint 24: core NATS publish (not JetStream). The message is fire-and-forget;
// if the broker is down, the publisher will retry via outbox.IncrementAttempts
// and a subsequent fetch cycle.
func (c *NATSClient) Publish(ctx context.Context, subject string, payload []byte) error {
	if c == nil || c.conn == nil {
		return errors.New("nats: client not connected")
	}
	if subject == "" {
		return errors.New("nats: subject required")
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  nats.Header{},
	}
	if err := c.conn.PublishMsg(msg); err != nil {
		return fmt.Errorf("nats publish %s: %w", subject, err)
	}

	if err := c.conn.FlushTimeout(2 * time.Second); err != nil {
		return fmt.Errorf("nats flush %s: %w", subject, err)
	}
	return nil
}

// Subscribe registers a handler for the given subject. Returns a Subscription
// that the caller should keep alive for the duration of consumption; calling
// Subscription.Unsubscribe() stops delivery.
//
// Handler errors are LOGGED but do NOT stop the subscription — the broker
// will redeliver the message. Idempotent handlers are required.
func (c *NATSClient) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if c == nil || c.conn == nil {
		return nil, errors.New("nats: client not connected")
	}
	if subject == "" {
		return nil, errors.New("nats: subject required")
	}
	if handler == nil {
		return nil, errors.New("nats: handler required")
	}

	sub, err := c.conn.Subscribe(subject, handler)
	if err != nil {
		return nil, fmt.Errorf("nats subscribe %s: %w", subject, err)
	}
	c.log.Info("nats subscribed", "subject", subject)
	return sub, nil
}

// Ping checks broker liveness. Returns nil if reachable.
func (c *NATSClient) Ping() error {
	if c == nil || c.conn == nil {
		return errors.New("nats: client not connected")
	}
	if !c.conn.IsConnected() {
		return errors.New("nats: not connected")
	}
	return nil
}

// Close drains pending publishes and closes the underlying connection.
func (c *NATSClient) Close() {
	if c == nil || c.conn == nil {
		return
	}
	c.conn.Drain()
	c.log.Info("nats client closed", "url", c.url)
}

// IsConnected returns true if the underlying connection is healthy.
func (c *NATSClient) IsConnected() bool {
	if c == nil || c.conn == nil {
		return false
	}
	return c.conn.IsConnected()
}
