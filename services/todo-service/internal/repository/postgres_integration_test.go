//go:build integration

package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/database"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
)

func TestPostgresRepository_CRUD(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := database.NewPostgresPool(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	repo := repository.NewPostgresTodoRepository(pool)
	userID := uuid.NewString()                         // unique per run → no clashes with other data
	now := time.Now().UTC().Truncate(time.Microsecond) // Postgres stores µs precision

	todo := &model.Todo{ID: uuid.NewString(), UserID: userID, Title: "integration 100%",
		Priority: model.PriorityLow, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.Create(ctx, todo))
	t.Cleanup(func() { _ = repo.Delete(ctx, userID, todo.ID) })

	got, err := repo.GetByID(ctx, userID, todo.ID)
	require.NoError(t, err)
	assert.Equal(t, todo.Title, got.Title)
	assert.True(t, now.Equal(got.CreatedAt))
	assert.Equal(t, time.UTC, got.CreatedAt.Location())

	items, total, err := repo.List(ctx, model.ListFilter{UserID: userID, Search: "integ", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)

	_, total, err = repo.List(ctx, model.ListFilter{UserID: userID, Search: "0%", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total, "literal %% must match")
	_, total, err = repo.List(ctx, model.ListFilter{UserID: userID, Search: "%", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	_, total, err = repo.List(ctx, model.ListFilter{UserID: userID, Search: "_ntegration", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 0, total, "_ must not act as a wildcard")

	todo.Completed = true
	todo.UpdatedAt = now.Add(time.Second)
	require.NoError(t, repo.Update(ctx, todo))
	done := true
	_, total, err = repo.List(ctx, model.ListFilter{UserID: userID, Completed: &done, Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	_, err = repo.GetByID(ctx, uuid.NewString(), todo.ID) // other user
	assert.ErrorIs(t, err, model.ErrNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, uuid.NewString(), todo.ID), model.ErrNotFound)
}
