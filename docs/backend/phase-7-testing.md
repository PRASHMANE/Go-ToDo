# Backend Phase 7 — Testing & Linting

**Goal:** confidence to change code. Unit tests for business logic, handler
tests over real HTTP, integration tests against real Postgres, plus a linter.
**Branch:** `phase/b7-testing` · **Tag:** `b7`

## Concepts

- Go's testing is **built in**: files ending in `_test.go`, functions
  `func TestXxx(t *testing.T)`, run with `go test ./...`. No framework needed.
- **Table-driven tests:** one test function, a slice of cases, a loop with
  `t.Run(name, ...)`. This is idiomatic Go.
- **Test pyramid:**

  ```
        /\        integration (real DB): few, slower
       /  \       handler (httptest + real service + memory repo): some
      /____\      unit (service with fake repo): many, fast
  ```

- **Fakes over mocks:** we already have `MemoryTodoRepository`, a working
  fake. For error cases we write a tiny struct that returns errors. No mocking
  library needed.
- **`httptest`:** `httptest.NewRecorder()` captures a response without opening a port.
- **Build tags:** `//go:build integration` excludes slow tests unless you
  pass `-tags=integration`.
- **Race detector:** `go test -race` finds data races. Always use it in CI.

## Step 1 — Assertions helper

```bash
cd services/todo-service && go get github.com/stretchr/testify@latest
```

`require.X` stops the test on failure; `assert.X` records the failure and continues.

## Step 2 — Let tests freeze the clock

The `now func() time.Time` field has existed since B2. Add a setter in
`service/todo.go` so tests (in another package) can replace it:

```go
// WithClock overrides the clock (tests only).
func (s *TodoService) WithClock(now func() time.Time) *TodoService {
	s.now = now
	return s
}
```

## Step 3 — Service unit tests: `internal/service/todo_test.go`

```go
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
	return service.NewTodoService(repository.NewMemoryTodoRepository()).
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

func TestUsersAreIsolated(t *testing.T) {
	svc := newService()
	ctx := context.Background()
	todo, err := svc.Create(ctx, user, service.CreateTodoInput{Title: "mine"})
	require.NoError(t, err)

	_, err = svc.Get(ctx, "someone-else", todo.ID)
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

// failingRepo is a hand-written fake that always errors.
type failingRepo struct{ service.TodoRepository } // embed interface: only override what we need

func (failingRepo) Create(context.Context, *model.Todo) error { return errors.New("db down") }

func TestCreate_RepositoryErrorIsWrapped(t *testing.T) {
	svc := service.NewTodoService(failingRepo{})
	_, err := svc.Create(context.Background(), user, service.CreateTodoInput{Title: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create todo: db down")
}
```

The **embedded interface trick** in `failingRepo`: by embedding
`service.TodoRepository` (nil), the struct satisfies the whole interface, and we
override only `Create`. Calling any other method would panic, which is fine
because this test never does.

## Step 4 — Handler tests: `internal/handler/todo_test.go`

```go
package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/router"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

func newTestServer() *gin.Engine {
	log := slog.New(slog.NewTextHandler(io.Discard, nil)) // silence logs in tests
	svc := service.NewTodoService(repository.NewMemoryTodoRepository())
	return router.New(log, gin.TestMode, handler.NewTodoHandler(svc, log))
}

func do(t *testing.T, r http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const u1 = "11111111-1111-1111-1111-111111111111"

func TestCreateAndGetTodo(t *testing.T) {
	r := newTestServer()

	w := do(t, r, http.MethodPost, "/api/v1/todos", `{"title":"write tests","priority":"high"}`, u1)
	require.Equal(t, http.StatusCreated, w.Code)

	var created dto.TodoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "write tests", created.Title)

	w = do(t, r, http.MethodGet, "/api/v1/todos/"+created.ID, "", u1)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestValidationAndAuthErrors(t *testing.T) {
	r := newTestServer()
	tests := []struct {
		name, method, path, body, user string
		wantStatus                     int
		wantCode                       string
	}{
		{"missing user header", "GET", "/api/v1/todos", "", "", 401, "unauthorized"},
		{"empty title", "POST", "/api/v1/todos", `{"title":""}`, u1, 400, "invalid_request"},
		{"bad priority", "POST", "/api/v1/todos", `{"title":"x","priority":"urgent"}`, u1, 400, "invalid_request"},
		{"bad page size", "GET", "/api/v1/todos?page_size=1000", "", u1, 400, "invalid_request"},
		{"non-uuid id", "GET", "/api/v1/todos/abc", "", u1, 404, "not_found"},
		{"unknown id", "DELETE", "/api/v1/todos/" + u1, "", u1, 404, "not_found"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, r, tc.method, tc.path, tc.body, tc.user)
			assert.Equal(t, tc.wantStatus, w.Code)
			var body struct{ Error struct{ Code string } }
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
		})
	}
}
```

These tests go through **the real router, middleware, handler, service and
repository**. Only the network is fake.

## Step 5 — Postgres integration test: `internal/repository/postgres_integration_test.go`

```go
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
	userID := uuid.NewString() // unique per run → no clashes with other data
	now := time.Now().UTC().Truncate(time.Microsecond) // Postgres stores µs precision

	todo := &model.Todo{ID: uuid.NewString(), UserID: userID, Title: "integration",
		Priority: model.PriorityLow, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, repo.Create(ctx, todo))
	t.Cleanup(func() { _ = repo.Delete(ctx, userID, todo.ID) })

	got, err := repo.GetByID(ctx, userID, todo.ID)
	require.NoError(t, err)
	assert.Equal(t, todo.Title, got.Title)
	assert.True(t, now.Equal(got.CreatedAt))

	items, total, err := repo.List(ctx, model.ListFilter{UserID: userID, Search: "integ", Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, items, 1)

	_, err = repo.GetByID(ctx, uuid.NewString(), todo.ID) // other user
	assert.ErrorIs(t, err, model.ErrNotFound)
}
```

Run it against the compose database:

```bash
make db-up migrate-todo
cd services/todo-service
TEST_DATABASE_URL="postgres://todo_user:todo_pass@localhost:5432/todo_db?sslmode=disable" \
  go test -tags=integration -race ./internal/repository/...
```

> Next level: **testcontainers-go** starts a throwaway Postgres container per test run, with no manual setup.

## Step 6 — Auth tests (do these yourself)

Write `services/auth-service/internal/service/auth_test.go` with an in-memory
fake `UserRepository` (a map + mutex, ~30 lines). Cover:

- register then login succeeds; token parses with the same `TokenManager`
- wrong password → `ErrInvalidCredentials`
- unknown email → `ErrInvalidCredentials` (same error!)
- email is lower-cased (`"A@X.com"` can log in as `"a@x.com"`)

And `pkg/auth/jwt_test.go`: expired token fails, token signed with a different secret fails, wrong issuer fails.

## Step 7 — Linter: `.golangci.yml` (repo root)

```yaml
version: "2"
linters:
  default: standard        # errcheck, govet, ineffassign, staticcheck, unused
  enable:
    - bodyclose            # forgot resp.Body.Close()
    - errorlint            # use errors.Is/As, not ==
    - misspell
    - revive               # style
    - gosec                # security smells
formatters:
  enable:
    - gofmt
    - goimports
```

## Step 8 — Makefile

```makefile
test:            ## Unit + handler tests (race detector on)
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go test -race -cover ./...); done

test-integration: ## Integration tests (needs `make db-up migrate-todo`)
	cd services/todo-service && TEST_DATABASE_URL="$(TODO_DB_URL)" go test -tags=integration -race ./...

lint:            ## golangci-lint on every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && golangci-lint run ./...); done

cover:           ## HTML coverage report for todo-service
	cd services/todo-service && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Verify

```bash
make test               # all PASS, coverage % printed per package
make test-integration
make lint               # fix everything it reports
go test -run TestCreate/trims -v ./services/todo-service/internal/service/   # run one sub-test
```

## Exercises

1. Break the code on purpose (e.g. forget to trim the title). Which test fails?
2. Get `internal/service` to ≥ 90% coverage. Look at `make cover` to see which lines are red.
3. Write a test for the gateway's `StripIdentityHeaders` + `ForwardUserID` using `httptest`
   and a fake upstream (`httptest.NewServer`) that echoes `X-User-ID`.

## Commit

```bash
git add -A
git commit -m "test: unit, handler and integration tests with linting (B7)"
```

**Next:** [Phase 8 — Events & activity service](phase-8-events-activity-service.md)
