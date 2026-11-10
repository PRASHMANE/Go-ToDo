# Backend Phase 8 — Event-Driven: NATS JetStream & Activity Service

**Goal:** todo-service **publishes events** (`todos.created`, `todos.completed`, ...)
and a new **activity-service** consumes them to build a per-user activity feed.
todo-service doesn't know activity-service exists.
**Branch:** `phase/b8-events` · **Tag:** `b8`

## Concepts

- **Synchronous vs asynchronous communication:**
  - *Sync (HTTP):* gateway → todo-service. Caller waits for an answer.
  - *Async (events):* todo-service → broker → anyone interested. Publisher doesn't wait.
- **Why events here?** Writing an activity log is a *side effect*. If
  activity-service is down, creating a todo must still work. With events, the
  messages wait in the stream until the consumer comes back.
- **NATS JetStream:** NATS is a fast message system; JetStream adds
  **persistence**. Vocabulary:
  - **Subject:** a topic name like `todos.created`. Wildcard `todos.>` = everything under `todos.`.
  - **Stream:** stores messages for some subjects (our stream `TODOS` stores `todos.>`).
  - **Consumer (durable):** a named cursor in a stream. It remembers what was
    acknowledged, so a restarted service continues where it left off.
  - **Ack / Nak / Term:** "done", "retry later", "never retry (bad message)".
- **At-least-once delivery:** a message can arrive **twice** (e.g. crash before
  ack). The consumer must be **idempotent**: we store `event_id` with a UNIQUE
  constraint, so a duplicate insert does nothing.
- **Event = contract:** the event struct lives in `pkg/events`, shared by
  publisher and consumer. Change it carefully (only *add* fields).
- **⚠️ Dual-write problem:** we write to Postgres, *then* publish. If the process
  dies between the two, the event is lost. The production fix is the
  **transactional outbox pattern** (write the event into an `outbox` table in the
  same DB transaction, and a relay publishes it). We note it and move on; it's a
  great stretch exercise.

```
todo-service ──publish todos.created──▶ ┌────────────── NATS JetStream ───────────────┐
                                        │ stream TODOS (subjects: todos.>)             │
                                        │   msg1 msg2 msg3 ...                         │
                                        │ durable consumer "activity-service" ─────────┼──▶ activity-service
                                        └──────────────────────────────────────────────┘        │
                                                                                     INSERT ... ON CONFLICT DO NOTHING
                                                                                                 ▼
                                                                                            activity_db
```

## Files in this phase

```
pkg/events/events.go
pkg/events/nats.go
services/todo-service/internal/events/publisher.go
services/todo-service/internal/service/todo.go          # + EventPublisher
services/activity-service/                              # NEW (same layout as others)
├── cmd/main.go
├── internal/{config,consumer,handler,model,repository,router,service}/
├── migrations/000001_create_activities.{up,down}.sql
└── Dockerfile
services/api-gateway/...                                # + /activity route
docker-compose.yml                                      # + nats, activity-migrate, activity-service
```

## Step 1 — NATS in compose

```yaml
  nats:
    image: nats:2.11-alpine
    command: ["-js", "-sd", "/data", "-m", "8222"]   # JetStream on, storage dir, monitoring port
    ports:
      - "4222:4222"     # client port (for running services locally)
      - "8222:8222"     # monitoring http
    volumes:
      - natsdata:/data
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8222/healthz"]
      interval: 5s
      timeout: 3s
      retries: 10
```

Add `natsdata:` under `volumes:`. Then `docker compose up -d nats` and open
http://localhost:8222/jsz.

## Step 2 — `pkg/events/events.go` (the contract)

```bash
cd pkg && go get github.com/nats-io/nats.go@latest && cd ..
```

```go
// Package events defines the messages exchanged between services.
package events

import "time"

// StreamName is the JetStream stream holding all todo events.
const StreamName = "TODOS"

// Subjects.
const (
	SubjectTodoCreated   = "todos.created"
	SubjectTodoUpdated   = "todos.updated"
	SubjectTodoCompleted = "todos.completed"
	SubjectTodoDeleted   = "todos.deleted"
	SubjectTodoAll       = "todos.>"
)

// TodoEvent is published whenever a todo changes.
type TodoEvent struct {
	EventID    string    `json:"event_id"`    // unique → idempotency + dedupe
	Type       string    `json:"type"`        // same as the subject
	OccurredAt time.Time `json:"occurred_at"`
	UserID     string    `json:"user_id"`
	TodoID     string    `json:"todo_id"`
	Title      string    `json:"title"`
}
```

## Step 3 — `pkg/events/nats.go`

```go
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
		nats.MaxReconnects(-1),          // reconnect forever
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
		MaxAge:     7 * 24 * time.Hour,    // keep a week of history
		Duplicates: 2 * time.Minute,        // dedupe window for Nats-Msg-Id
	})
	return err
}
```

## Step 4 — todo-service publishes

In `internal/service/todo.go` add the dependency:

```go
// EventPublisher sends domain events. Declared by the consumer (this package).
type EventPublisher interface {
	Publish(ctx context.Context, e events.TodoEvent)
}

// NoopPublisher discards events (tests, or when NATS isn't configured).
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, events.TodoEvent) {}

type TodoService struct {
	repo   TodoRepository
	events EventPublisher
	now    func() time.Time
}

func NewTodoService(repo TodoRepository, pub EventPublisher) *TodoService {
	if pub == nil {
		pub = NoopPublisher{}
	}
	return &TodoService{repo: repo, events: pub, now: time.Now}
}

func (s *TodoService) emit(ctx context.Context, subject string, t *model.Todo) {
	s.events.Publish(ctx, events.TodoEvent{
		EventID:    uuid.NewString(),
		Type:       subject,
		OccurredAt: s.now().UTC(),
		UserID:     t.UserID,
		TodoID:     t.ID,
		Title:      t.Title,
	})
}
```

Call it **after** each successful write:

```go
// Create: after repo.Create succeeds
s.emit(ctx, events.SubjectTodoCreated, t)

// Update: decide BEFORE applying changes whether this completes the todo
wasCompleted := t.Completed
// ... apply changes, repo.Update ...
if !wasCompleted && t.Completed {
	s.emit(ctx, events.SubjectTodoCompleted, t)
} else {
	s.emit(ctx, events.SubjectTodoUpdated, t)
}

// Delete: fetch first so we know the title, then delete, then emit
t, err := s.repo.GetByID(ctx, userID, id)
if err != nil { return err }
if err := s.repo.Delete(ctx, userID, id); err != nil { return err }
s.emit(ctx, events.SubjectTodoDeleted, t)
return nil
```

Update every `service.NewTodoService(repo)` call: `NewTodoService(repo, nil)`
in tests, and the real publisher in `main`.

`internal/events/publisher.go`:

```go
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

func NewJetStreamPublisher(js jetstream.JetStream, log *slog.Logger) *JetStreamPublisher {
	return &JetStreamPublisher{js: js, log: log}
}

// Publish is best-effort: a failure is logged, never returned, so a broker
// outage cannot break todo CRUD. (See "dual-write problem" above.)
func (p *JetStreamPublisher) Publish(ctx context.Context, e pkgevents.TodoEvent) {
	data, err := json.Marshal(e)
	if err != nil {
		p.log.Error("marshal event", "err", err)
		return
	}
	// Don't let a slow broker slow down the HTTP response too much.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// WithMsgID → JetStream drops duplicates of the same event within the window.
	if _, err := p.js.Publish(ctx, e.Type, data, jetstream.WithMsgID(e.EventID)); err != nil {
		p.log.Error("publish event", "subject", e.Type, "event_id", e.EventID, "err", err)
	}
}
```

Wire in todo-service `main.go` (config gains `NATSURL`, env `NATS_URL`, empty = disabled):

```go
	var pub service.EventPublisher = service.NoopPublisher{}
	if cfg.NATSURL != "" {
		nc, js, err := pkgevents.Connect(cfg.NATSURL, "todo-service")
		if err != nil {
			return err
		}
		defer nc.Drain() // flush pending messages on shutdown
		if err := pkgevents.EnsureStream(ctx, js); err != nil {
			return err
		}
		pub = todoevents.NewJetStreamPublisher(js, log)
	}
	svc := service.NewTodoService(repo, pub)
```

## Step 5 — activity-service

Create it exactly like the other services (B4 Step 1): module
`github.com/PRASHMANE/Go-ToDo/services/activity-service`, `go work use`, `pkg` replace.

**Migration** `000001_create_activities.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS activities (
    id          UUID PRIMARY KEY,
    event_id    UUID        NOT NULL UNIQUE,   -- idempotency key
    user_id     UUID        NOT NULL,
    todo_id     UUID        NOT NULL,
    type        VARCHAR(50) NOT NULL,
    title       VARCHAR(200) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activities_user_time ON activities (user_id, occurred_at DESC);
```

**Model** (`internal/model/activity.go`):

```go
package model

import "time"

type Activity struct {
	ID         string
	EventID    string
	UserID     string
	TodoID     string
	Type       string
	Title      string
	OccurredAt time.Time
}
```

**Repository** (`internal/repository/postgres.go`), key parts:

```go
// Insert stores an activity. Returns inserted=false for a duplicate event.
func (r *ActivityRepository) Insert(ctx context.Context, a *model.Activity) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO activities (id, event_id, user_id, todo_id, type, title, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (event_id) DO NOTHING`,
		a.ID, a.EventID, a.UserID, a.TodoID, a.Type, a.Title, a.OccurredAt)
	if err != nil {
		return false, fmt.Errorf("insert activity: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListByUser returns the latest activities for a user.
func (r *ActivityRepository) ListByUser(ctx context.Context, userID string, limit int) ([]model.Activity, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, event_id, user_id, todo_id, type, title, occurred_at
		   FROM activities WHERE user_id = $1 ORDER BY occurred_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	// pgx.CollectRows + RowToStructByPos maps columns to fields in order.
	return pgx.CollectRows(rows, pgx.RowToStructByPos[model.Activity])
}
```

**Service** (`internal/service/activity.go`):

```go
type ActivityRepository interface {
	Insert(ctx context.Context, a *model.Activity) (bool, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]model.Activity, error)
}

type ActivityService struct{ repo ActivityRepository }

func NewActivityService(repo ActivityRepository) *ActivityService { return &ActivityService{repo: repo} }

// Record turns an event into an activity row (idempotent).
func (s *ActivityService) Record(ctx context.Context, e events.TodoEvent) error {
	_, err := s.repo.Insert(ctx, &model.Activity{
		ID: uuid.NewString(), EventID: e.EventID, UserID: e.UserID,
		TodoID: e.TodoID, Type: e.Type, Title: e.Title, OccurredAt: e.OccurredAt,
	})
	return err
}

func (s *ActivityService) List(ctx context.Context, userID string, limit int) ([]model.Activity, error) {
	return s.repo.ListByUser(ctx, userID, limit)
}
```

**Consumer** (`internal/consumer/consumer.go`), the heart of this phase:

```go
// Package consumer reads todo events from JetStream.
package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/service"
)

// Run consumes until ctx is cancelled.
func Run(ctx context.Context, js jetstream.JetStream, svc *service.ActivityService, log *slog.Logger) error {
	cons, err := js.CreateOrUpdateConsumer(ctx, events.StreamName, jetstream.ConsumerConfig{
		Durable:       "activity-service",         // survives restarts
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
		if err := svc.Record(hctx, e); err != nil {
			log.Error("record activity failed, will retry", "event_id", e.EventID, "err", err)
			_ = msg.NakWithDelay(5 * time.Second)
			return
		}
		_ = msg.Ack()
		log.Info("activity recorded", "type", e.Type, "event_id", e.EventID)
	})
	if err != nil {
		return err
	}
	log.Info("consumer started")

	<-ctx.Done()
	cc.Stop()
	return nil
}
```

**Handler**: `GET /api/v1/activity?limit=20` behind `middleware.RequireUserHeader()`,
returning `{"items":[{"id","todo_id","type","title","occurred_at"}]}`
(write the DTO + handler yourself, using todo-service as the template: good practice!).

**main.go** runs the HTTP server **and** the consumer concurrently with
`errgroup`. If either fails, both stop:

```go
	g, gctx := errgroup.WithContext(ctx)   // go get golang.org/x/sync/errgroup
	g.Go(func() error { return consumer.Run(gctx, js, svc, log) })
	g.Go(func() error { return httpserver.Run(gctx, log, ":"+cfg.Port, r) })
	return g.Wait()
```

## Step 6 — Gateway, compose, Dockerfile

Gateway: add `ActivityServiceURL` (env `ACTIVITY_SERVICE_URL`, default
`http://localhost:8083`), a third proxy, and:

```go
activity := v1.Group("/activity", requireAuth...)
activity.GET("", activityProxy)
```

Compose additions:

```yaml
  activity-migrate:
    image: migrate/migrate:v4.18.1
    command: ["-path=/migrations", "-database=postgres://activity_user:activity_pass@postgres:5432/activity_db?sslmode=disable", "up"]
    volumes: ["./services/activity-service/migrations:/migrations:ro"]
    depends_on: { postgres: { condition: service_healthy } }

  activity-service:
    <<: *go-service
    build: { context: ., dockerfile: services/activity-service/Dockerfile }
    environment:
      <<: *go-env
      PORT: "8083"
      DATABASE_URL: postgres://activity_user:activity_pass@postgres:5432/activity_db?sslmode=disable
      NATS_URL: nats://nats:4222
    healthcheck: { test: ["CMD", "/app", "healthcheck"], interval: 10s, timeout: 3s, retries: 5 }
    depends_on:
      activity-migrate: { condition: service_completed_successfully }
      nats: { condition: service_healthy }
```

Also add `NATS_URL: nats://nats:4222` + `depends_on: nats` to **todo-service**,
and `ACTIVITY_SERVICE_URL: http://activity-service:8083` + `depends_on: activity-service` to the **gateway**.
Dockerfile: copy todo-service's, change names/port to 8083.

## Verify

```bash
make up
# log in as A (Phase B5), then:
ID=$(curl -s -X POST $G/todos -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' -d '{"title":"evented"}' | jq -r .id)
curl -s -X PATCH $G/todos/$ID -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' -d '{"completed":true}' > /dev/null
curl -s $G/activity -H "Authorization: Bearer $TA" | jq      # created + completed

# resilience: events wait while the consumer is down
docker compose stop activity-service
curl -s -X POST $G/todos -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' -d '{"title":"while down"}' | jq .id   # still works!
nats --server localhost:4222 stream info TODOS                # messages stored
nats --server localhost:4222 consumer info TODOS activity-service   # "Unprocessed Messages: 1"
docker compose start activity-service
sleep 3; curl -s $G/activity -H "Authorization: Bearer $TA" | jq '.items[0].title'  # "while down"

# watch live events
nats --server localhost:4222 sub 'todos.>'
```

## Exercises

1. Publish the same event twice by hand (`nats pub todos.created '<json with same event_id>'`). Confirm only one row exists. That's idempotency.
2. Publish invalid JSON to `todos.created`. Find the "dropping" log; confirm the consumer keeps working.
3. *(Stretch)* Implement the **outbox pattern** in todo-service: an `outbox` table written in the same transaction as the todo (`pool.BeginTx`), and a background goroutine that publishes unsent rows and marks them sent.

## Commit

```bash
git add -A
git commit -m "feat: todo events over NATS JetStream and activity-service (B8)"
```

**Next:** [Frontend Phase 6](../frontend/phase-6-testing-activity-polish.md), then
[Backend Phase 9 — Observability & CI](phase-9-observability-and-ci.md).
