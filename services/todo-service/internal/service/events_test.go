package service_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

// recordingPublisher is a fake EventPublisher that remembers what was published.
type recordingPublisher struct {
	mu     sync.Mutex
	events []events.TodoEvent
}

func (p *recordingPublisher) Publish(_ context.Context, e events.TodoEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
}

func (p *recordingPublisher) types() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.events))
	for i, e := range p.events {
		out[i] = e.Type
	}
	return out
}

func TestEventsAreEmittedForEachChange(t *testing.T) {
	pub := &recordingPublisher{}
	svc := service.NewTodoService(repository.NewMemoryTodoRepository(), pub)
	ctx := context.Background()

	todo, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "evented"})
	require.NoError(t, err)

	title := "renamed"
	_, err = svc.Update(ctx, user, todo.ID, service.UpdateTodoInput{Title: &title})
	require.NoError(t, err)

	done := true
	_, err = svc.Update(ctx, user, todo.ID, service.UpdateTodoInput{Completed: &done})
	require.NoError(t, err)

	_, err = svc.Update(ctx, user, todo.ID, service.UpdateTodoInput{Completed: &done}) // already done → "updated"
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, user, todo.ID))

	assert.Equal(t, []string{
		events.SubjectTodoCreated, events.SubjectTodoUpdated, events.SubjectTodoCompleted,
		events.SubjectTodoUpdated, events.SubjectTodoDeleted,
	}, pub.types())

	last := pub.events[len(pub.events)-1]
	assert.Equal(t, todo.ID, last.TodoID)
	assert.Equal(t, user, last.UserID)
	assert.Equal(t, "renamed", last.Title)
	assert.NotEqual(t, pub.events[0].EventID, last.EventID, "each event gets a unique id")
}

func TestNoEventWhenWriteFails(t *testing.T) {
	pub := &recordingPublisher{}
	svc := service.NewTodoService(repository.NewMemoryTodoRepository(), pub)
	err := svc.Delete(context.Background(), user, "00000000-0000-0000-0000-00000000dead")
	require.Error(t, err)
	assert.Empty(t, pub.types())
}
