// Package events publishes todo events to JetStream.
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	pkgevents "github.com/PRASHMANE/Go-ToDo/pkg/events"
)

// JetStreamPublisher implements service.EventPublisher.
type JetStreamPublisher struct {
	js  jetstream.JetStream
	log *slog.Logger
}

// NewJetStreamPublisher builds a publisher.
func NewJetStreamPublisher(js jetstream.JetStream, log *slog.Logger) *JetStreamPublisher {
	return &JetStreamPublisher{js: js, log: log}
}

// Publish is best-effort: a failure is logged, never returned, so a broker
// outage cannot break todo CRUD. (See the "dual-write problem" in the B8 doc;
// the production fix is the transactional outbox pattern.)
func (p *JetStreamPublisher) Publish(ctx context.Context, e pkgevents.TodoEvent) {
	data, err := json.Marshal(e)
	if err != nil {
		p.log.Error("marshal event", "err", err)
		return
	}
	// Don't let a slow broker slow down the HTTP response too much.
	// WithoutCancel: if the client disconnects right after the write, still publish.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()

	// WithMsgID → JetStream drops duplicates of the same event within the window.
	if _, err := p.js.Publish(ctx, e.Type, data, jetstream.WithMsgID(e.EventID)); err != nil {
		p.log.Error("publish event", "subject", e.Type, "event_id", e.EventID, "err", err)
		return
	}
	p.log.Debug("event published", "subject", e.Type, "event_id", e.EventID)
}
