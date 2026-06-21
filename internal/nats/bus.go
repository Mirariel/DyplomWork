// Package nats provides a thin wrapper around nats.go for inter-service messaging.
package nats

import (
	"encoding/json"
	"log/slog"
	"time"

	natsgo "github.com/nats-io/nats.go"
)

// ─── Subjects ─────────────────────────────────────────────────────────────────

const (
	// SubjPricesUpdated is published by market-data every 30 s after refreshing
	// prices in Redis. Trading subscribes to trigger smart-order checks.
	SubjPricesUpdated = "market.prices.updated"
)

// ─── Message types ────────────────────────────────────────────────────────────

// PricesMsg carries a snapshot of symbol→price at a given moment.
type PricesMsg struct {
	Prices map[string]float64 `json:"prices"`
	At     time.Time          `json:"at"`
}

// ─── Bus ──────────────────────────────────────────────────────────────────────

// Bus is a thin, reconnect-aware wrapper around a NATS connection.
type Bus struct {
	nc     *natsgo.Conn
	logger *slog.Logger
}

// Connect opens a NATS connection with automatic reconnect.
func Connect(url string, logger *slog.Logger) (*Bus, error) {
	if url == "" {
		url = natsgo.DefaultURL
	}
	nc, err := natsgo.Connect(
		url,
		natsgo.RetryOnFailedConnect(true),
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(2*time.Second),
		natsgo.DisconnectErrHandler(func(_ *natsgo.Conn, err error) {
			if err != nil {
				logger.Warn("nats: disconnected", "error", err)
			}
		}),
		natsgo.ReconnectHandler(func(_ *natsgo.Conn) {
			logger.Info("nats: reconnected")
		}),
	)
	if err != nil {
		return nil, err
	}
	logger.Info("nats: connected", "url", url)
	return &Bus{nc: nc, logger: logger}, nil
}

// Publish marshals v as JSON and publishes it to subj.
func (b *Bus) Publish(subj string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return b.nc.Publish(subj, data)
}

// Subscribe registers a handler for subj. The handler receives raw JSON bytes.
func (b *Bus) Subscribe(subj string, handler func([]byte)) (*natsgo.Subscription, error) {
	return b.nc.Subscribe(subj, func(msg *natsgo.Msg) {
		handler(msg.Data)
	})
}

// Close drains pending messages and closes the connection.
func (b *Bus) Close() {
	if err := b.nc.Drain(); err != nil {
		b.logger.Warn("nats: drain error", "error", err)
	}
}
