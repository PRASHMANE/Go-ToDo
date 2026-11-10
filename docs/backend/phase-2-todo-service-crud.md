# Backend Phase 2 — Todo CRUD with Clean Architecture (in-memory)

**Goal:** a complete REST API for todos with proper layers. Storage is an
in-memory map for now; in Phase B3 we swap it for Postgres **without touching
the service or handler**. That swap is the proof that the architecture works.
**Branch:** `phase/b2-todo-crud` · **Tag:** `b2`

## Concepts

| Layer | Package | Knows about | Must NOT know about |
|-------|---------|-------------|---------------------|
| Model | `internal/model` | Domain structs, domain errors | JSON, HTTP, SQL |
| DTO | `internal/dto` | JSON shapes + validation tags | SQL |
| Repository | `internal/repository` | Storage | HTTP |
| Service | `internal/service` | Business rules | Gin, HTTP status codes, SQL |
| Handler | `internal/handler` | Gin, HTTP, DTOs | SQL |

- **Interfaces belong to the consumer.** The `TodoRepository` interface is
  declared in the `service` package (the one that *uses* it), not in `repository`.
  This is idiomatic Go and keeps `service` independent.
- **Sentinel errors** (`model.ErrNotFound`) let the handler map domain errors
  to HTTP codes with `errors.Is`.
- **Pointers for optional fields** (`*string`, `*bool`) in PATCH requests:
  `nil` means "not sent"; a value means "change it". A plain `bool` can't tell
  "not sent" from `false`.
- **Concurrency:** Gin runs each request in its own goroutine, so the in-memory
  map must be protected with `sync.RWMutex` (many readers OR one writer).

## Files in this phase

```
pkg/middleware/user.go                      # ContextUserID + RequireUserHeader (used fully in B5)
services/todo-service/internal/
├── model/todo.go
├── dto/todo.go
├── repository/memory.go
├── service/todo.go
├── handler/todo.go
└── router/router.go                         # updated
services/todo-service/cmd/main.go            # updated: wiring
services/todo-service/requests.http          # manual API tests
```

## Step 1 — `pkg/middleware/user.go`

```go
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

// ContextUserID is the gin.Context key that holds the authenticated user's ID.
const ContextUserID = "user_id"

// UserIDHeader is set by the API gateway after it validates the JWT.
const UserIDHeader = "X-User-ID"

// FixedUser sets a hard-coded user ID. Development only (Phase B2–B4),
// until the gateway exists.
func FixedUser(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextUserID, userID)
		c.Next()
	}
}

// RequireUserHeader trusts the X-User-ID header set by the gateway (Phase B5).
func RequireUserHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(UserIDHeader)
		if id == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user identity")
			return
		}
		c.Set(ContextUserID, id)
		c.Next()
	}
}
```

Handlers always read `c.GetString(middleware.ContextUserID)`. Today
`FixedUser` fills it; in B5 we swap to `RequireUserHeader` and **handlers
don't change**.

## Step 1b — Friendly validation errors: `pkg/response/bind.go`

Gin's raw validation error looks like
`Key: 'CreateTodoRequest.Title' Error:Field validation for 'Title' failed on the 'required' tag`.
That's fine for developers but bad for a UI. `response.BindError(c, err)` turns
`validator.ValidationErrors` into messages like `title is required; priority must be one of: low medium high`
(field names converted to snake_case, the same names the client sends).
Read the real file in the repo: [pkg/response/bind.go](../../pkg/response/bind.go). It uses
`errors.As` to detect validation errors, and a `switch` on `fe.Tag()` for the message.
(This was added in phase F2, when the UI first showed these errors.)

## Step 2 — `internal/model/todo.go`

```go
// Package model holds todo-service domain types. No JSON, no SQL here.
package model

import (
	"errors"
	"time"
)

// Domain errors. Handlers map these to HTTP status codes.
var (
	ErrNotFound   = errors.New("todo not found")
	ErrValidation = errors.New("validation failed")
)

// Priority is a typed string: the compiler stops you passing a random string by accident.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	}
	return false
}

// Todo is the core domain entity.
type Todo struct {
	ID          string
	UserID      string
	Title       string
	Description string
	Completed   bool
	Priority    Priority
	DueDate     *time.Time // nil = no due date
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListFilter describes a list query. Zero values mean "no filter".
type ListFilter struct {
	UserID    string
	Completed *bool // nil = all, true = completed, false = active
	Priority  Priority
	Search    string
	Limit     int
	Offset    int
}
```

## Step 3 — `internal/dto/todo.go`

```go
// Package dto defines the JSON request/response shapes of the HTTP API.
package dto

import (
	"time"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
)

// CreateTodoRequest is the body of POST /todos.
type CreateTodoRequest struct {
	Title       string     `json:"title" binding:"required,min=1,max=200"`
	Description string     `json:"description" binding:"max=2000"`
	Priority    string     `json:"priority" binding:"omitempty,oneof=low medium high"`
	DueDate     *time.Time `json:"due_date"` // RFC3339, e.g. "2026-12-01T00:00:00Z"
}

// UpdateTodoRequest is the body of PATCH /todos/:id. All fields optional.
type UpdateTodoRequest struct {
	Title        *string    `json:"title" binding:"omitempty,min=1,max=200"`
	Description  *string    `json:"description" binding:"omitempty,max=2000"`
	Completed    *bool      `json:"completed"`
	Priority     *string    `json:"priority" binding:"omitempty,oneof=low medium high"`
	DueDate      *time.Time `json:"due_date"`
	ClearDueDate bool       `json:"clear_due_date"` // true = remove the due date
}

// ListTodosQuery is the query string of GET /todos.
type ListTodosQuery struct {
	Status   string `form:"status" binding:"omitempty,oneof=all active completed"`
	Priority string `form:"priority" binding:"omitempty,oneof=low medium high"`
	Q        string `form:"q" binding:"max=100"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

// TodoResponse is how a todo looks in JSON.
type TodoResponse struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Completed   bool       `json:"completed"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ListTodosResponse wraps a page of todos.
type ListTodosResponse struct {
	Items    []TodoResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
}

// FromModel maps a domain Todo to its JSON form.
func FromModel(t *model.Todo) TodoResponse {
	return TodoResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Completed:   t.Completed,
		Priority:    string(t.Priority),
		DueDate:     t.DueDate,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
```

Why not just put JSON tags on `model.Todo`? Because then `UserID` would leak
into responses unless you remember `json:"-"`, and every API change would touch
the domain. Separate DTOs = the API is an explicit contract.

## Step 4 — `internal/repository/memory.go`

```go
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

func (r *MemoryTodoRepository) Create(_ context.Context, t *model.Todo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.todos[t.ID] = *t // store a COPY, so callers can't mutate our data later
	return nil
}

func (r *MemoryTodoRepository) GetByID(_ context.Context, userID, id string) (*model.Todo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.todos[id]
	if !ok || t.UserID != userID { // other users' todos are "not found", not "forbidden"
		return nil, model.ErrNotFound
	}
	return &t, nil
}

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
```

## Step 5 — `internal/service/todo.go` (business logic)

```go
// Package service contains todo business rules.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

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

// UpdateTodoInput: nil fields are left unchanged.
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

// TodoService implements todo use cases.
type TodoService struct {
	repo TodoRepository
	now  func() time.Time // injectable clock → deterministic tests
}

// NewTodoService wires the service with its repository.
func NewTodoService(repo TodoRepository) *TodoService {
	return &TodoService{repo: repo, now: time.Now}
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
	return t, nil
}

// Delete removes a todo.
func (s *TodoService) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}
```

Notice: **no Gin, no HTTP status codes, no JSON** in this file. You could call
this service from a CLI or a gRPC server unchanged.

## Step 6 — `internal/handler/todo.go` (HTTP layer)

```go
// Package handler exposes todo use cases over HTTP.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

// TodoHandler holds HTTP handlers for /todos.
type TodoHandler struct {
	svc *service.TodoService
	log *slog.Logger
}

// NewTodoHandler builds a TodoHandler.
func NewTodoHandler(svc *service.TodoService, log *slog.Logger) *TodoHandler {
	return &TodoHandler{svc: svc, log: log}
}

// RegisterRoutes mounts the todo routes on a router group.
func (h *TodoHandler) RegisterRoutes(rg *gin.RouterGroup) {
	todos := rg.Group("/todos")
	todos.GET("", h.list)
	todos.POST("", h.create)
	todos.GET("/:id", h.get)
	todos.PATCH("/:id", h.update)
	todos.DELETE("/:id", h.delete)
}

func (h *TodoHandler) create(c *gin.Context) {
	var req dto.CreateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err) // friendly message, e.g. "title is required"
		return
	}
	todo, err := h.svc.Create(c.Request.Context(), userID(c), service.CreateTodoInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    model.Priority(req.Priority),
		DueDate:     req.DueDate,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.FromModel(todo))
}

func (h *TodoHandler) list(c *gin.Context) {
	var q dto.ListTodosQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BindError(c, err) // friendly message, e.g. "title is required"
		return
	}
	res, err := h.svc.List(c.Request.Context(), userID(c), service.ListTodosInput{
		Status:   q.Status,
		Priority: model.Priority(q.Priority),
		Search:   q.Q,
		Page:     q.Page,
		PageSize: q.PageSize,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]dto.TodoResponse, 0, len(res.Items)) // non-nil → JSON [] not null
	for i := range res.Items {
		items = append(items, dto.FromModel(&res.Items[i]))
	}
	c.JSON(http.StatusOK, dto.ListTodosResponse{
		Items: items, Page: q.Page, PageSize: q.PageSize, Total: res.Total,
	})
}

func (h *TodoHandler) get(c *gin.Context) {
	id, ok := todoID(c)
	if !ok {
		return
	}
	todo, err := h.svc.Get(c.Request.Context(), userID(c), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromModel(todo))
}

func (h *TodoHandler) update(c *gin.Context) {
	id, ok := todoID(c)
	if !ok {
		return
	}
	var req dto.UpdateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err) // friendly message, e.g. "title is required"
		return
	}
	in := service.UpdateTodoInput{
		Title:        req.Title,
		Description:  req.Description,
		Completed:    req.Completed,
		DueDate:      req.DueDate,
		ClearDueDate: req.ClearDueDate,
	}
	if req.Priority != nil {
		p := model.Priority(*req.Priority)
		in.Priority = &p
	}
	todo, err := h.svc.Update(c.Request.Context(), userID(c), id, in)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromModel(todo))
}

func (h *TodoHandler) delete(c *gin.Context) {
	id, ok := todoID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), userID(c), id); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleError maps domain errors to HTTP responses in ONE place.
func (h *TodoHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		response.Error(c, http.StatusNotFound, "not_found", "todo not found")
	case errors.Is(err, model.ErrValidation):
		response.Error(c, http.StatusBadRequest, "validation_error", err.Error())
	default:
		// Log the real error, but never leak internals to the client.
		h.log.Error("internal error", "err", err, "request_id", c.GetString(middleware.ContextRequestID))
		response.Error(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

func userID(c *gin.Context) string { return c.GetString(middleware.ContextUserID) }

// todoID reads and validates the :id path param (must be a UUID).
func todoID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.Error(c, http.StatusNotFound, "not_found", "todo not found")
		return "", false
	}
	return id, true
}
```

## Step 7 — Update `internal/router/router.go`

```go
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
)

// DemoUserID is used until the API gateway exists (Phase B5).
const DemoUserID = "00000000-0000-0000-0000-000000000001"

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string, todos *handler.TodoHandler) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1", middleware.FixedUser(DemoUserID)) // B5: RequireUserHeader()
	todos.RegisterRoutes(v1)

	return r
}
```

## Step 8 — Wire it in `cmd/main.go`

Replace the body of `run()` after `defer stop()`:

```go
	repo := repository.NewMemoryTodoRepository()
	svc := service.NewTodoService(repo)
	todoHandler := handler.NewTodoHandler(svc, log)

	r := router.New(log, cfg.GinMode, todoHandler)
	return httpserver.Run(ctx, log, ":"+cfg.Port, r)
```

…and add the imports for `handler`, `repository` and `service`. This is
**dependency injection by hand**: `main` builds the graph bottom-up
(repo → service → handler → router). Nothing else creates its own dependencies.

```bash
cd services/todo-service
go get github.com/google/uuid@latest
go mod tidy && go vet ./... && go run ./cmd
```

## Step 9 — `requests.http` (manual tests in VS Code REST Client)

```http
@base = http://localhost:8082/api/v1

### Create
POST {{base}}/todos
Content-Type: application/json

{ "title": "Learn Gin", "priority": "high", "due_date": "2026-12-01T00:00:00Z" }

### List (filters + pagination)
GET {{base}}/todos?status=active&priority=high&q=gin&page=1&page_size=10

### Get one (paste an id)
GET {{base}}/todos/REPLACE_ID

### Complete it
PATCH {{base}}/todos/REPLACE_ID
Content-Type: application/json

{ "completed": true }

### Delete
DELETE {{base}}/todos/REPLACE_ID
```

## Verify (with curl)

```bash
B=localhost:8082/api/v1
ID=$(curl -s -X POST $B/todos -H 'Content-Type: application/json' \
      -d '{"title":"Learn Gin","priority":"high"}' | jq -r .id)
curl -s $B/todos | jq
curl -s -X PATCH $B/todos/$ID -H 'Content-Type: application/json' -d '{"completed":true}' | jq
curl -s "$B/todos?status=completed" | jq '.total'          # 1
curl -s -X POST $B/todos -H 'Content-Type: application/json' -d '{"title":""}' | jq   # 400
curl -s $B/todos/not-a-uuid | jq                             # 404
curl -s -o /dev/null -w '%{http_code}\n' -X DELETE $B/todos/$ID   # 204
curl -s -X PATCH $B/todos/$ID -H 'Content-Type: application/json' -d '{}' | jq   # 404
```

Restart the service: all todos are gone. That's expected (memory). Phase B3 fixes it.

## Exercises

1. Run 100 concurrent creates and check the race detector stays quiet:
   `go run -race ./cmd`, then
   `seq 100 | xargs -P 20 -I{} curl -s -X POST localhost:8082/api/v1/todos -H 'Content-Type: application/json' -d '{"title":"t{}"}' > /dev/null`
2. Add a rule in the **service**: a due date in the past is a validation error. Which layer did you change, and which didn't you touch?
3. Send `{"priority":"urgent"}` in a create. Which layer rejected it: the DTO `binding` tag or the service? Why keep both checks?

## Commit

```bash
git add -A
git commit -m "feat(todo-service): in-memory CRUD with clean architecture layers (B2)"
```

**Next:** [Frontend Phase 0 — React setup](../frontend/phase-0-setup.md), then come back for
[Backend Phase 3 — PostgreSQL](phase-3-postgres-and-migrations.md).
