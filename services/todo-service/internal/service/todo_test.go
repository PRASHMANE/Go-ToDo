package service_test // black-box: test only the exported API

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

const user = "00000000-0000-0000-0000-0000000000aa"

var fixedNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func newService() *service.TodoService {
	return service.NewTodoService(repository.NewMemoryTodoRepository(), nil).
		WithClock(func() time.Time { return fixedNow })
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name         string
		in           service.CreateTodoInput
		wantErr      error
		wantTitle    string
		wantPriority model.Priority
	}{
		{name: "defaults priority to medium", in: service.CreateTodoInput{Title: "a"},
			wantTitle: "a", wantPriority: model.PriorityMedium},
		{name: "trims title", in: service.CreateTodoInput{Title: "  b  ", Priority: model.PriorityHigh},
			wantTitle: "b", wantPriority: model.PriorityHigh},
		{name: "rejects blank title", in: service.CreateTodoInput{Title: "   "},
			wantErr: model.ErrValidation},
		{name: "rejects unknown priority", in: service.CreateTodoInput{Title: "c", Priority: "urgent"},
			wantErr: model.ErrValidation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newService().Create(context.Background(), user, tc.in)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantTitle, got.Title)
			assert.Equal(t, tc.wantPriority, got.Priority)
			assert.Equal(t, fixedNow, got.CreatedAt)
			assert.NotEmpty(t, got.ID)
		})
	}
}

func TestUpdate_PartialFieldsOnly(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	created, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "orig", Description: "keep me"})
	require.NoError(t, err)

	done := true
	updated, err := svc.Update(ctx, user, created.ID, service.UpdateTodoInput{Completed: &done})
	require.NoError(t, err)

	assert.True(t, updated.Completed)
	assert.Equal(t, "orig", updated.Title)          // untouched
	assert.Equal(t, "keep me", updated.Description) // untouched
}

func TestUpdate_Validation(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	created, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "orig"})
	require.NoError(t, err)

	blank := "   "
	_, err = svc.Update(ctx, user, created.ID, service.UpdateTodoInput{Title: &blank})
	require.ErrorIs(t, err, model.ErrValidation)

	bad := model.Priority("urgent")
	_, err = svc.Update(ctx, user, created.ID, service.UpdateTodoInput{Priority: &bad})
	require.ErrorIs(t, err, model.ErrValidation)

	_, err = svc.Update(ctx, user, "00000000-0000-0000-0000-00000000dead", service.UpdateTodoInput{})
	require.ErrorIs(t, err, model.ErrNotFound)
}

func TestUpdate_DueDateSetAndClear(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	created, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "due"})
	require.NoError(t, err)

	due := fixedNow.Add(48 * time.Hour)
	updated, err := svc.Update(ctx, user, created.ID, service.UpdateTodoInput{DueDate: &due})
	require.NoError(t, err)
	require.NotNil(t, updated.DueDate)
	assert.True(t, due.Equal(*updated.DueDate))

	updated, err = svc.Update(ctx, user, created.ID, service.UpdateTodoInput{ClearDueDate: true})
	require.NoError(t, err)
	assert.Nil(t, updated.DueDate)
}

func TestUsersAreIsolated(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	todo, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "mine"})
	require.NoError(t, err)

	_, err = svc.Get(ctx, "someone-else", todo.ID)
	assert.ErrorIs(t, err, model.ErrNotFound)

	err = svc.Delete(ctx, "someone-else", todo.ID)
	assert.ErrorIs(t, err, model.ErrNotFound)
}

func TestList_FiltersAndPaginates(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "task"})
		require.NoError(t, err)
	}
	res, err := svc.List(ctx, user, service.ListTodosInput{Status: "all", Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, res.Total)
	assert.Len(t, res.Items, 2)
}

func TestList_StatusFilter(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	a, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "a"})
	require.NoError(t, err)
	_, err = svc.Create(ctx, user, service.CreateTodoInput{Title: "b"})
	require.NoError(t, err)
	done := true
	_, err = svc.Update(ctx, user, a.ID, service.UpdateTodoInput{Completed: &done})
	require.NoError(t, err)

	for status, want := range map[string]int{"all": 2, "active": 1, "completed": 1} {
		res, err := svc.List(ctx, user, service.ListTodosInput{Status: status, Page: 1, PageSize: 10})
		require.NoError(t, err)
		assert.Equal(t, want, res.Total, status)
	}
}

// failingRepo is a hand-written fake that always errors.
type failingRepo struct{ service.TodoRepository } // embed interface: only override what we need

func (failingRepo) Create(context.Context, *model.Todo) error { return errors.New("db down") }

func TestCreate_RepositoryErrorIsWrapped(t *testing.T) {
	svc := service.NewTodoService(failingRepo{}, nil)
	_, err := svc.Create(context.Background(), user, service.CreateTodoInput{Title: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create todo: db down")
}
