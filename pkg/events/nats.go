package events

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Connect opens a NATS connection (auto-reconnects) and a JetStream handle.
func Connect(url, clientName string) (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(url,
		nats.Name(clientName),
		nats.MaxReconnects(-1), // reconnect forever
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("connect nats: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("jetstream: %w", err)
	}
	return nc, js, nil
}

// EnsureStream creates or updates the TODOS stream. Safe to call on every start.
func EnsureStream(ctx context.Context, js jetstream.JetStream) error {
	_, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       StreamName,
		Subjects:   []string{SubjectTodoAll},
		Storage:    jetstream.FileStorage,
		MaxAge:     7 * 24 * time.Hour, // keep a week of history
		Duplicates: 2 * time.Minute,    // dedupe window for Nats-Msg-Id
	})
	if err != nil {
		return fmt.Errorf("ensure stream %s: %w", StreamName, err)
	}
	return nil
}

// ConnPinger adapts a NATS connection to health.Pinger for /readyz.
type ConnPinger struct{ NC *nats.Conn }

// Ping reports an error unless the connection is currently up.
func (p ConnPinger) Ping(context.Context) error {
	if !p.NC.IsConnected() {
		return fmt.Errorf("nats not connected (status %s)", p.NC.Status())
	}
	return nil
}
