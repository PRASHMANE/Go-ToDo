// Package consumer reads todo events from JetStream.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/service"
)

// Durable is the consumer name: JetStream remembers its position across restarts.
const Durable = "activity-service"

// Run consumes until ctx is cancelled.
func Run(ctx context.Context, js jetstream.JetStream, svc *service.ActivityService, log *slog.Logger) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, events.StreamName, jetstream.ConsumerConfig{
		Durable:       Durable, // survives restarts
		FilterSubject: events.SubjectTodoAll,
		AckPolicy:     jetstream.AckExplicitPolicy, // we ack each message ourselves
		AckWait:       30 * time.Second,            // no ack in 30s → redeliver
		MaxDeliver:    5,                           // then give up
	})
	if err != nil {
		return err
	}

	cc, err := cons.Consume(func(msg jetstream.Msg) {
		var e events.TodoEvent
		if err := json.Unmarshal(msg.Data(), &e); err != nil {
			log.Error("bad event payload, dropping", "subject", msg.Subject(), "err", err)
			_ = msg.Term() // poison message: never redeliver
			return
		}
		hctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		inserted, err := svc.Record(hctx, e)
		if errors.Is(err, service.ErrInvalidEvent) {
			log.Error("invalid event, dropping", "subject", msg.Subject(), "err", err)
			_ = msg.Term() // retrying can never fix it
			return
		}
		if err != nil { // e.g. database down: temporary, so retry later
			log.Error("record activity failed, will retry", "event_id", e.EventID, "err", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Ack()
		if inserted {
			log.Info("activity recorded", "type", e.Type, "event_id", e.EventID)
		} else {
			log.Info("duplicate event ignored", "type", e.Type, "event_id", e.EventID)
		}
	})
	if err != nil {
		return err
	}
	log.Info("consumer started", "stream", events.StreamName, "durable", Durable)

	<-ctx.Done()
	cc.Stop()
	log.Info("consumer stopped")
	return nil
}
