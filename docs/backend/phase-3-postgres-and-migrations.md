# Backend Phase 3 — PostgreSQL, Migrations, Filters & Pagination

**Goal:** todos survive restarts. Add a Postgres repository that implements
the **same** `TodoRepository` interface, so the service and handler stay unchanged.
**Branch:** `phase/b3-postgres` · **Tag:** `b3`

## Concepts

- **Database per service:** each service gets its *own database and DB user*
  on one Postgres server. `todo-service` physically cannot read `auth_db`.
- **Migrations:** schema changes are versioned SQL files applied in order
  (`000001_...up.sql`). Each has a `down.sql` to undo it. The migration tool keeps
  a `schema_migrations` table so it knows what has already run.
- **Connection pool (`pgxpool`):** opening a DB connection is slow, so a pool
  keeps a few open and lends them to goroutines. One pool per service, created
  once in `main`.
- **Parameterised queries (`$1, $2`):** values are sent *separately* from the SQL
  text, so user input can never become SQL. **Never** build SQL with
  `fmt.Sprintf` + user values. (We only `Sprintf` the *placeholder numbers*.)
- **Context timeouts:** every query gets `ctx`; if the client disconnects, the query is cancelled.

## Files in this phase

```
docker-compose.yml                       # Postgres only (for now)
deployments/postgres/init.sql
.env.example
pkg/database/postgres.go
services/todo-service/migrations/000001_create_todos.up.sql
services/todo-service/migrations/000001_create_todos.down.sql
services/todo-service/internal/repository/postgres.go
services/todo-service/internal/config/config.go   # + DatabaseURL
services/todo-service/cmd/main.go                 # choose repo
```

## Step 1 — `deployments/postgres/init.sql`

Runs **once**, the first time the Postgres volume is created:

```sql
-- One server, one database + one user per service.
CREATE USER auth_user WITH PASSWORD 'auth_pass';
CREATE DATABASE auth_db OWNER auth_user;

CREATE USER todo_user WITH PASSWORD 'todo_pass';
CREATE DATABASE todo_db OWNER todo_user;

CREATE USER activity_user WITH PASSWORD 'activity_pass';
CREATE DATABASE activity_db OWNER activity_user;
```

> These are **dev-only** passwords. In production they come from a secret manager.

## Step 2 — `docker-compose.yml` (repo root, infra only for now)

```yaml
name: go-todo

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "15432:5432"           # host 15432 → container 5432 (your Mac may already run Postgres on 5432/5433)
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./deployments/postgres/init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10

volumes:
  pgdata:
```

```bash
docker compose up -d postgres
docker compose ps                      # STATUS should become "healthy"
psql "postgres://todo_user:todo_pass@localhost:15432/todo_db" -c 'select current_user, current_database();'
```

> Changed `init.sql` later? It won't re-run on an existing volume. Reset with
> `docker compose down -v` (**deletes all data**) and start again.

## Step 3 — Migration files

```bash
migrate create -ext sql -dir services/todo-service/migrations -seq create_todos
```

`000001_create_todos.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS todos (
    id          UUID PRIMARY KEY,
    user_id     UUID         NOT NULL,
    title       VARCHAR(200) NOT NULL,
    description TEXT         NOT NULL DEFAULT '',
    completed   BOOLEAN      NOT NULL DEFAULT FALSE,
    priority    VARCHAR(10)  NOT NULL DEFAULT 'medium'
                CHECK (priority IN ('low', 'medium', 'high')),
    due_date    TIMESTAMPTZ  NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Almost every query is "this user's todos, newest first".
CREATE INDEX IF NOT EXISTS idx_todos_user_created ON todos (user_id, created_at DESC);
```

`000001_create_todos.down.sql`:

```sql
DROP TABLE IF EXISTS todos;
```

Notice there's **no foreign key to users**: users live in another service's
database. The link is just a UUID. This is normal in microservices.

Run it:

```bash
export TODO_DB_URL="postgres://todo_user:todo_pass@localhost:15432/todo_db?sslmode=disable"
migrate -path services/todo-service/migrations -database "$TODO_DB_URL" up
migrate -path services/todo-service/migrations -database "$TODO_DB_URL" version   # 1
psql "$TODO_DB_URL" -c '\d todos'
```

Try `down 1` then `up` again to see that migrations are reversible.

## Step 4 — `pkg/database/postgres.go`

```bash
cd pkg && go get github.com/jackc/pgx/v5@latest && cd ..
```

```go
// Package database creates connection pools to PostgreSQL.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool parses the URL, opens a pool and pings it once.
func NewPostgresPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
```

## Step 5 — `internal/repository/postgres.go`

```bash
cd services/todo-service && go get github.com/jackc/pgx/v5@latest
```

```go
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
)

const todoColumns = `id, user_id, title, description, completed, priority, due_date, created_at, updated_at`

// PostgresTodoRepository stores todos in PostgreSQL.
type PostgresTodoRepository struct {
	db *pgxpool.Pool
}

// NewPostgresTodoRepository wraps a connection pool.
func NewPostgresTodoRepository(db *pgxpool.Pool) *PostgresTodoRepository {
	return &PostgresTodoRepository{db: db}
}

// scanTodo reads one row. pgx.Row is satisfied by both QueryRow results and Rows.
func scanTodo(row pgx.Row) (*model.Todo, error) {
	var t model.Todo
	var priority string
	if err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &t.Completed,
		&priority, &t.DueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	t.Priority = model.Priority(priority)
	return &t, nil
}

func (r *PostgresTodoRepository) Create(ctx context.Context, t *model.Todo) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO todos (`+todoColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.UserID, t.Title, t.Description, t.Completed, string(t.Priority),
		t.DueDate, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

func (r *PostgresTodoRepository) GetByID(ctx context.Context, userID, id string) (*model.Todo, error) {
	row := r.db.QueryRow(ctx,
		`SELECT `+todoColumns+` FROM todos WHERE id = $1 AND user_id = $2`, id, userID)
	t, err := scanTodo(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get todo: %w", err)
	}
	return t, nil
}

func (r *PostgresTodoRepository) List(ctx context.Context, f model.ListFilter) ([]model.Todo, int, error) {
	// Build WHERE dynamically, but ONLY with placeholders. Values go in args.
	where := []string{"user_id = $1"}
	args := []any{f.UserID}

	if f.Completed != nil {
		args = append(args, *f.Completed)
		where = append(where, fmt.Sprintf("completed = $%d", len(args)))
	}
	if f.Priority != "" {
		args = append(args, string(f.Priority))
		where = append(where, fmt.Sprintf("priority = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		n := len(args)
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR description ILIKE $%d)", n, n))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM todos WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count todos: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(
		"SELECT %s FROM todos WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		todoColumns, whereSQL, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close() // ALWAYS close rows, or the connection never returns to the pool

	var todos []model.Todo
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan todo: %w", err)
		}
		todos = append(todos, *t)
	}
	if err := rows.Err(); err != nil { // errors that happened during iteration
		return nil, 0, fmt.Errorf("iterate todos: %w", err)
	}
	return todos, total, nil
}

func (r *PostgresTodoRepository) Update(ctx context.Context, t *model.Todo) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE todos
		    SET title = $3, description = $4, completed = $5, priority = $6,
		        due_date = $7, updated_at = $8
		  WHERE id = $1 AND user_id = $2`,
		t.ID, t.UserID, t.Title, t.Description, t.Completed, string(t.Priority),
		t.DueDate, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *PostgresTodoRepository) Delete(ctx context.Context, userID, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM todos WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
```

**Compile-time interface check.** Add this to both repository files to make
the compiler prove they satisfy the interface:

```go
// in postgres.go (needs import of the service package)
var _ service.TodoRepository = (*PostgresTodoRepository)(nil)
```

(It's safe: `service` doesn't import `repository`, so there's no import cycle.)

## Step 6 — Config & wiring

`internal/config/config.go`: add a field and load it:

```go
type Config struct {
	Port        string
	LogLevel    string
	GinMode     string
	DatabaseURL string // empty → in-memory repository
}

// in Load():
		DatabaseURL: pkgconfig.String("DATABASE_URL", ""),
```

`cmd/main.go`: choose the repository. **This is the only file that changes
to switch storage:**

```go
	var repo service.TodoRepository
	if cfg.DatabaseURL != "" {
		pool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		repo = repository.NewPostgresTodoRepository(pool)
		log.Info("using postgres repository")
	} else {
		repo = repository.NewMemoryTodoRepository()
		log.Warn("DATABASE_URL not set, using in-memory repository")
	}

	svc := service.NewTodoService(repo)
```

## Step 7 — `.env.example` (repo root) and Makefile

```bash
# Copy to .env and adjust. .env is git-ignored.
TODO_DATABASE_URL=postgres://todo_user:todo_pass@localhost:15432/todo_db?sslmode=disable
AUTH_DATABASE_URL=postgres://auth_user:auth_pass@localhost:15432/auth_db?sslmode=disable
```

```makefile
TODO_DB_URL ?= postgres://todo_user:todo_pass@localhost:15432/todo_db?sslmode=disable

db-up:           ## Start Postgres
	docker compose up -d postgres

migrate-todo:    ## Apply todo-service migrations
	migrate -path services/todo-service/migrations -database "$(TODO_DB_URL)" up

run-todo:        ## Run todo-service locally (Postgres)
	cd services/todo-service && DATABASE_URL="$(TODO_DB_URL)" go run ./cmd
```

(Replace the old `run-todo` target.)

## Verify

```bash
make db-up migrate-todo run-todo
# another terminal: repeat the curl checks from Phase B2, then:
for i in $(seq 1 25); do curl -s -X POST localhost:8082/api/v1/todos \
  -H 'Content-Type: application/json' -d "{\"title\":\"task $i\"}" > /dev/null; done
curl -s "localhost:8082/api/v1/todos?page=2&page_size=10" | jq '{page, page_size, total, n: (.items|length)}'
curl -s "localhost:8082/api/v1/todos?q=task%201" | jq '.total'
```

Now **restart todo-service**: your todos are still there. 🎉
Look at the data directly: `psql "$TODO_DB_URL" -c 'select title, completed from todos limit 5;'`.

## Exercises

1. Run `EXPLAIN ANALYZE SELECT ... WHERE user_id = '...' ORDER BY created_at DESC LIMIT 20` in psql. Is the index used?
2. Search for `%` or `_`. What happens, and why? (Hint: they are `LIKE` wildcards.) Escape them in the repository.
3. Write migration `000002_add_todo_position` (an `INT` column for manual ordering), then roll it back with `down 1`.

## Commit

```bash
git add -A
git commit -m "feat(todo-service): PostgreSQL repository, migrations, filters and pagination (B3)"
```

**Next:** [Frontend Phase 2 — Connect the API](../frontend/phase-2-connect-api.md), then
[Backend Phase 4 — Auth service](phase-4-auth-service.md).
