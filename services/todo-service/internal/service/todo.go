// Package service contains todo business rules.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
)

// TodoRepository is what the service needs from storage.
// Declared HERE (by the consumer), implemented in package repository.
type TodoRepository interface {
	Create(ctx context.Context, t *model.Todo) error
	GetByID(ctx context.Context, userID, id string) (*model.Todo, error)
	List(ctx context.Context, f model.ListFilter) ([]model.Todo, int, error)
	Update(ctx context.Context, t *model.Todo) error
	Delete(ctx context.Context, userID, id string) error
}

// CreateTodoInput is the data needed to create a todo.
type CreateTodoInput struct {
	Title       string
	Description string
	Priority    model.Priority
	DueDate     *time.Time
}

// UpdateTodoInput is a partial update: nil fields are left unchanged.
type UpdateTodoInput struct {
	Title        *string
	Description  *string
	Completed    *bool
	Priority     *model.Priority
	DueDate      *time.Time
	ClearDueDate bool
}

// ListTodosInput is a list request in business terms.
type ListTodosInput struct {
	Status   string // all | active | completed
	Priority model.Priority
	Search   string
	Page     int
	PageSize int
}

// ListResult is one page of todos.
type ListResult struct {
	Items []model.Todo
	Total int
}

// EventPublisher sends domain events. Declared by the consumer (this package).
type EventPublisher interface {
	Publish(ctx context.Context, e events.TodoEvent)
}

// NoopPublisher discards events (tests, or when NATS isn't configured).
type NoopPublisher struct{}

// Publish does nothing.
func (NoopPublisher) Publish(context.Context, events.TodoEvent) {}

// TodoService implements todo use cases.
type TodoService struct {
	repo   TodoRepository
	events EventPublisher
	now    func() time.Time // injectable clock → deterministic tests
}

// NewTodoService wires the service with its repository and event publisher
// (nil publisher = events disabled).
func NewTodoService(repo TodoRepository, pub EventPublisher) *TodoService {
	if pub == nil {
		pub = NoopPublisher{}
	}
	return &TodoService{repo: repo, events: pub, now: time.Now}
}

// emit publishes a todo event. Called only AFTER a successful write.
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

// WithClock overrides the clock (tests only).
func (s *TodoService) WithClock(now func() time.Time) *TodoService {
	s.now = now
	return s
}

// Create validates input, applies defaults and stores a new todo.
func (s *TodoService) Create(ctx context.Context, userID string, in CreateTodoInput) (*model.Todo, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", model.ErrValidation)
	}
	priority := in.Priority
	if priority == "" {
		priority = model.PriorityMedium
	}
	if !priority.Valid() {
		return nil, fmt.Errorf("%w: invalid priority %q", model.ErrValidation, priority)
	}

	now := s.now().UTC()
	t := &model.Todo{
		ID:          uuid.NewString(),
		UserID:      userID,
		Title:       title,
		Description: strings.TrimSpace(in.Description),
		Priority:    priority,
		DueDate:     in.DueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create todo: %w", err)
	}
	s.emit(ctx, events.SubjectTodoCreated, t)
	return t, nil
}

// Get returns one todo owned by userID.
func (s *TodoService) Get(ctx context.Context, userID, id string) (*model.Todo, error) {
	return s.repo.GetByID(ctx, userID, id)
}

// List returns a filtered, paginated list.
func (s *TodoService) List(ctx context.Context, userID string, in ListTodosInput) (ListResult, error) {
	f := model.ListFilter{
		UserID:   userID,
		Priority: in.Priority,
		Search:   strings.TrimSpace(in.Search),
		Limit:    in.PageSize,
		Offset:   (in.Page - 1) * in.PageSize,
	}
	switch in.Status {
	case "active":
		f.Completed = new(bool) // pointer to false
	case "completed":
		done := true
		f.Completed = &done
	}

	items, total, err := s.repo.List(ctx, f)
	if err != nil {
		return ListResult{}, fmt.Errorf("list todos: %w", err)
	}
	return ListResult{Items: items, Total: total}, nil
}

// Update applies a partial update.
func (s *TodoService) Update(ctx context.Context, userID, id string, in UpdateTodoInput) (*model.Todo, error) {
	t, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	wasCompleted := t.Completed // decide the event type BEFORE applying changes

	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: title cannot be empty", model.ErrValidation)
		}
		t.Title = title
	}
	if in.Description != nil {
		t.Description = strings.TrimSpace(*in.Description)
	}
	if in.Completed != nil {
		t.Completed = *in.Completed
	}
	if in.Priority != nil {
		if !in.Priority.Valid() {
			return nil, fmt.Errorf("%w: invalid priority %q", model.ErrValidation, *in.Priority)
		}
		t.Priority = *in.Priority
	}
	if in.ClearDueDate {
		t.DueDate = nil
	} else if in.DueDate != nil {
		t.DueDate = in.DueDate
	}
	t.UpdatedAt = s.now().UTC()

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("update todo: %w", err)
	}
	if !wasCompleted && t.Completed {
		s.emit(ctx, events.SubjectTodoCompleted, t)
	} else {
		s.emit(ctx, events.SubjectTodoUpdated, t)
	}
	return t, nil
}

// Delete removes a todo. It reads it first so the event can carry the title.
func (s *TodoService) Delete(ctx context.Context, userID, id string) error {
	t, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		return err
	}
	s.emit(ctx, events.SubjectTodoDeleted, t)
	return nil
}
