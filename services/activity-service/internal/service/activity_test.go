package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/service"
)

// memRepo mimics the UNIQUE(event_id) constraint of the real table.
type memRepo struct {
	mu   sync.Mutex
	rows []model.Activity
}

func (m *memRepo) Insert(_ context.Context, a *model.Activity) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.EventID == a.EventID {
			return false, nil
		}
	}
	m.rows = append(m.rows, *a)
	return true, nil
}

func (m *memRepo) ListByUser(_ context.Context, userID string, limit int) ([]model.Activity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Activity
	for i := len(m.rows) - 1; i >= 0 && len(out) < limit; i-- {
		if m.rows[i].UserID == userID {
			out = append(out, m.rows[i])
		}
	}
	return out, nil
}

func TestRecordIsIdempotent(t *testing.T) {
	repo := &memRepo{}
	svc := service.NewActivityService(repo)
	e := events.TodoEvent{EventID: "e1", Type: events.SubjectTodoCreated, UserID: "u1", TodoID: "t1",
		Title: "x", OccurredAt: time.Now()}

	inserted, err := svc.Record(context.Background(), e)
	require.NoError(t, err)
	assert.True(t, inserted)

	inserted, err = svc.Record(context.Background(), e) // redelivery
	require.NoError(t, err)
	assert.False(t, inserted)
	assert.Len(t, repo.rows, 1)
}

func TestRecordRejectsInvalidEvent(t *testing.T) {
	_, err := service.NewActivityService(&memRepo{}).Record(context.Background(), events.TodoEvent{Type: "x"})
	assert.ErrorIs(t, err, service.ErrInvalidEvent)
}

func TestListIsPerUser(t *testing.T) {
	svc := service.NewActivityService(&memRepo{})
	ctx := context.Background()
	for i, u := range []string{"u1", "u2", "u1"} {
		_, err := svc.Record(ctx, events.TodoEvent{EventID: string(rune('a' + i)), UserID: u, TodoID: "t"})
		require.NoError(t, err)
	}
	items, err := svc.List(ctx, "u1", 10)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}
