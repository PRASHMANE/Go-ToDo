// Package repository implements todo storage.
package repository

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
)

// MemoryTodoRepository stores todos in a map. Safe for concurrent use.
type MemoryTodoRepository struct {
	mu    sync.RWMutex
	todos map[string]model.Todo
}

// NewMemoryTodoRepository returns an empty in-memory repository.
func NewMemoryTodoRepository() *MemoryTodoRepository {
	return &MemoryTodoRepository{todos: make(map[string]model.Todo)}
}

// Create stores a new todo.
func (r *MemoryTodoRepository) Create(_ context.Context, t *model.Todo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.todos[t.ID] = *t // store a COPY, so callers can't mutate our data later
	return nil
}

// GetByID returns a todo owned by userID.
func (r *MemoryTodoRepository) GetByID(_ context.Context, userID, id string) (*model.Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.todos[id]
	if !ok || t.UserID != userID { // other users' todos are "not found", not "forbidden"
		return nil, model.ErrNotFound
	}
	return &t, nil
}

// List returns a filtered, paginated slice plus the total match count.
func (r *MemoryTodoRepository) List(_ context.Context, f model.ListFilter) ([]model.Todo, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	search := strings.ToLower(f.Search)
	var items []model.Todo
	for _, t := range r.todos {
		if t.UserID != f.UserID {
			continue
		}
		if f.Completed != nil && t.Completed != *f.Completed {
			continue
		}
		if f.Priority != "" && t.Priority != f.Priority {
			continue
		}
		if search != "" &&
			!strings.Contains(strings.ToLower(t.Title), search) &&
			!strings.Contains(strings.ToLower(t.Description), search) {
			continue
		}
		items = append(items, t)
	}

	// Maps have random order in Go, so sort: newest first.
	slices.SortFunc(items, func(a, b model.Todo) int { return b.CreatedAt.Compare(a.CreatedAt) })

	total := len(items)
	start := min(f.Offset, total)
	end := min(start+f.Limit, total)
	return items[start:end], total, nil
}

// Update replaces an existing todo.
func (r *MemoryTodoRepository) Update(_ context.Context, t *model.Todo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.todos[t.ID]
	if !ok || existing.UserID != t.UserID {
		return model.ErrNotFound
	}
	r.todos[t.ID] = *t
	return nil
}

// Delete removes a todo.
func (r *MemoryTodoRepository) Delete(_ context.Context, userID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.todos[id]
	if !ok || t.UserID != userID {
		return model.ErrNotFound
	}
	delete(r.todos, id)
	return nil
}
