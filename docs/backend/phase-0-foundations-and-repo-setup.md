# Backend Phase 0 — Go Foundations & Repo Setup

**Goal:** learn just enough Go to start, and create the monorepo skeleton.
**Branch:** `phase/b0-repo-setup` · **Tag:** `b0`

---

## Part A — Go crash course (do this in a scratch folder, NOT in the repo)

```bash
mkdir -p ~/go-playground && cd ~/go-playground
go mod init playground
```

Create `main.go` and run each snippet with `go run .`.

### A1. Packages, imports, functions, multiple return values

```go
package main            // "main" package + main() = executable

import (
    "errors"
    "fmt"
)

func divide(a, b float64) (float64, error) {  // functions can return several values
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil                          // nil = "no error"
}

func main() {
    result, err := divide(10, 2)               // := declares + infers type
    if err != nil {                            // ALWAYS check errors right away
        fmt.Println("error:", err)
        return
    }
    fmt.Println(result)
}
```

### A2. Structs, methods, pointers

```go
type Todo struct {
    ID        string
    Title     string
    Completed bool
}

// value receiver: gets a COPY; can't change the original
func (t Todo) Summary() string { return fmt.Sprintf("[%v] %s", t.Completed, t.Title) }

// pointer receiver: gets the ADDRESS; can change the original
func (t *Todo) Complete() { t.Completed = true }

func main() {
    t := Todo{ID: "1", Title: "learn go"}
    t.Complete()            // Go auto-takes &t for you
    fmt.Println(t.Summary())

    p := &t                 // p is *Todo (pointer)
    p.Title = "learn gin"   // same as (*p).Title
    fmt.Println(t.Title)    // "learn gin": same struct
}
```

**Rule of thumb:** use pointer receivers for structs with state
(`*TodoService`, `*Repository`). Use value receivers for small immutable types.

### A3. Slices and maps

```go
todos := []Todo{}                       // slice: dynamic array
todos = append(todos, Todo{ID: "1"})
for i, t := range todos { fmt.Println(i, t.ID) }

byID := make(map[string]Todo)           // map: key → value
byID["1"] = Todo{ID: "1"}
t, ok := byID["2"]                      // "comma ok": ok=false if missing
fmt.Println(t, ok)
delete(byID, "1")
```

### A4. Interfaces (the most important Go concept for this project)

```go
type TodoStore interface {
    Save(t Todo) error
}

type MemoryStore struct{ data map[string]Todo }

// MemoryStore satisfies TodoStore automatically: no "implements" keyword
func (m *MemoryStore) Save(t Todo) error { m.data[t.ID] = t; return nil }

type FileStore struct{}
func (FileStore) Save(t Todo) error { fmt.Println("saving to file", t.ID); return nil }

func createTodo(store TodoStore, title string) error {   // depends on BEHAVIOUR, not a concrete type
    return store.Save(Todo{ID: "x", Title: title})
}

func main() {
    _ = createTodo(&MemoryStore{data: map[string]Todo{}}, "a")
    _ = createTodo(FileStore{}, "b")
}
```

This is the whole trick behind the **repository pattern** we use: the
service takes an interface, and we plug in memory, Postgres, or a test fake.

> Go proverb: *"Accept interfaces, return structs."* Constructors return
> concrete types (`*TodoService`); functions that *use* dependencies accept interfaces.

### A5. Errors: wrapping and checking

```go
var ErrNotFound = errors.New("not found")   // a "sentinel" error

func find(id string) error {
    return fmt.Errorf("find todo %s: %w", id, ErrNotFound)  // %w WRAPS the error
}

func main() {
    err := find("42")
    fmt.Println(err)                         // find todo 42: not found
    fmt.Println(errors.Is(err, ErrNotFound)) // true: looks through the wrapping
}
```

Our handlers use exactly this: `errors.Is(err, model.ErrNotFound)` → HTTP 404.

### A6. JSON and struct tags

```go
import "encoding/json"

type TodoJSON struct {
    ID        string `json:"id"`
    Title     string `json:"title"`
    Completed bool   `json:"completed"`
    Secret    string `json:"-"`                   // never serialised
    Note      string `json:"note,omitempty"`      // omitted when empty
}

b, _ := json.Marshal(TodoJSON{ID: "1", Title: "x"})
fmt.Println(string(b)) // {"id":"1","title":"x","completed":false}
```

Only **exported** (Capitalised) fields are encoded.

### A7. Goroutines, channels, mutex, context

```go
import ("context"; "sync"; "time")

func main() {
    var mu sync.Mutex
    counter := 0
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {                // 1000 goroutines at once
            defer wg.Done()
            mu.Lock()              // without the lock: data race, wrong result
            counter++
            mu.Unlock()
        }()
    }
    wg.Wait()
    fmt.Println(counter)           // 1000

    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    select {
    case <-time.After(time.Second):
        fmt.Println("work done")
    case <-ctx.Done():
        fmt.Println("cancelled:", ctx.Err()) // context deadline exceeded
    }
}
```

Run with the race detector to see why the mutex matters: remove the lock and run
`go run -race .`.

### A8. Your first Gin server (still in the playground)

```bash
go get github.com/gin-gonic/gin@latest
```

```go
package main

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type createReq struct {
    Title string `json:"title" binding:"required"`
}

func main() {
    r := gin.Default()

    r.GET("/ping", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"message": "pong"})
    })

    r.POST("/todos", func(c *gin.Context) {
        var req createReq
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
            return
        }
        c.JSON(http.StatusCreated, gin.H{"title": req.Title})
    })

    r.Run(":8080")
}
```

```bash
go run .
curl localhost:8080/ping
curl -X POST localhost:8080/todos -d '{"title":"hi"}' -H 'Content-Type: application/json'
curl -X POST localhost:8080/todos -d '{}' -H 'Content-Type: application/json'   # 400
```

Now read [03-how-go-and-gin-work.md](../03-how-go-and-gin-work.md) again. It
will make much more sense.

---

## Part B — Create the monorepo skeleton (in the real repo)

```bash
cd ~/Documents/Go_ToDo_Project
git checkout -b phase/b0-repo-setup

mkdir -p pkg services/{api-gateway,auth-service,todo-service} \
         deployments/postgres api/openapi frontend
```

### B1. `.gitignore` (repo root)

```gitignore
# Go
bin/
*.exe
*.test
*.out
coverage.*
tmp/

# Env / secrets
.env
.env.*
!.env.example

# Node
node_modules/
dist/

# OS / editor
.DS_Store
.idea/
.vscode/*
!.vscode/settings.json
```

### B2. `.editorconfig`

```ini
root = true

[*]
end_of_line = lf
insert_final_newline = true
charset = utf-8
trim_trailing_whitespace = true

[*.go]
indent_style = tab

[*.{ts,tsx,js,json,yml,yaml,md,css}]
indent_style = space
indent_size = 2

[Makefile]
indent_style = tab
```

### B3. Create the shared `pkg` module and the workspace

```bash
cd pkg
go mod init github.com/PRASHMANE/Go-ToDo/pkg
cd ..

cd services/todo-service
go mod init github.com/PRASHMANE/Go-ToDo/services/todo-service
cd ../..

go work init ./pkg ./services/todo-service
cat go.work
```

You should see:

```go
go 1.27.3

use (
	./pkg
	./services/todo-service
)
```

(We add `auth-service` and `api-gateway` to the workspace in their own phases.)

**What is the module path?** It's the *name* other code uses to import your
packages. By convention it's the repo URL + folder, so it's globally unique.

### B4. `Makefile` (starter; it grows every phase)

```makefile
.PHONY: help tidy fmt vet

MODULES := pkg $(wildcard services/*)

help:            ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

tidy:            ## go mod tidy in every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go mod tidy); done

fmt:             ## gofmt everything
	gofmt -s -w .

vet:             ## go vet every module
	@for m in $(MODULES); do [ -f $$m/go.mod ] && (echo "→ $$m" && cd $$m && go vet ./...); done
```

> Makefile recipes **must** be indented with a TAB, not spaces.

### B5. Update the root `README.md`

```markdown
# Go-ToDo

A microservice todo app built to learn Go + Gin (backend) and React (frontend).

- Plan & docs: [docs/README.md](docs/README.md)
- Run everything (from Phase B6): `docker compose up --build`
```

---

## Verify

```bash
go work sync           # no errors
make help              # shows targets
tree -L 2 -I node_modules   # (brew install tree) structure matches
```

## Exercises

1. In the playground, write a `Stack` struct with `Push`, `Pop` (returning `(int, bool)`).
2. Make `MemoryStore` safe for concurrent use with `sync.RWMutex` and prove it with `go run -race`.
3. Add a `GET /todos/:id` route to the playground Gin server that returns `404` for any id except `"1"`.

## Commit

```bash
git add -A
git commit -m "chore: initialise monorepo layout, go workspace and tooling (B0)"
git checkout main && git merge --squash phase/b0-repo-setup
git commit -m "chore: initialise monorepo layout, go workspace and tooling (B0)"
git tag b0 && git push origin main --tags
```

(See [02-git-workflow.md](../02-git-workflow.md) for the full loop.)

**Next:** [Phase 1 — Shared pkg & service skeleton](phase-1-shared-pkg-and-service-skeleton.md)
