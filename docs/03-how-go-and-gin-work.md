# 03 — How Go's `net/http` and Gin Work (from scratch)

Gin is **not magic**. It's a thin, fast layer on top of Go's standard
`net/http` package. If you understand `net/http`, you understand 80% of Gin.

---

## Part 1 — `net/http` from zero

### 1.1 The one interface that runs every Go web server

```go
// from the standard library (package net/http)
type Handler interface {
    ServeHTTP(w ResponseWriter, r *Request)
}
```

That's it. **Anything** with a `ServeHTTP(w, r)` method can handle HTTP requests.

- `r *http.Request` is **what came in**: method, URL, headers, body, context.
- `w http.ResponseWriter` is **how you answer**: set headers, write status, write body.

### 1.2 The smallest possible server

```go
package main

import (
    "fmt"
    "net/http"
)

type hello struct{}

func (hello) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "hello from %s %s\n", r.Method, r.URL.Path)
}

func main() {
    http.ListenAndServe(":8080", hello{}) // every request → hello.ServeHTTP
}
```

`ListenAndServe` opens a TCP socket and, **for every incoming connection,
starts a new goroutine** that parses HTTP and calls your `ServeHTTP`.
That's why Go servers handle thousands of concurrent requests: goroutines are
cheap (a few KB each), and the Go runtime schedules them across CPU cores.

> ⚠️ Consequence: your handlers run **concurrently**. Any shared data (a map
> of todos in memory, say) must be protected with a `sync.Mutex`. You'll
> do exactly this in Phase B2.

### 1.3 `HandlerFunc`: turning a function into a Handler

Writing a struct for every endpoint is tedious, so the standard library has an adapter:

```go
type HandlerFunc func(ResponseWriter, *Request)

func (f HandlerFunc) ServeHTTP(w ResponseWriter, r *Request) { f(w, r) }
```

A *function type with a method*. Now any function with the right signature
can become a Handler: `http.HandlerFunc(myFunc)`. This trick (methods on
function types) shows up everywhere in Go, including Gin.

### 1.4 Routing with `ServeMux`

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /todos", listTodos)        // method + path patterns (Go 1.22+)
mux.HandleFunc("GET /todos/{id}", getTodo)     // r.PathValue("id")
http.ListenAndServe(":8080", mux)
```

`ServeMux` is itself a `Handler`: its `ServeHTTP` looks at the URL and calls
the matching handler.

### 1.5 Middleware = a Handler that wraps a Handler

```go
func logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)                       // call the wrapped handler
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}

http.ListenAndServe(":8080", logging(mux))
```

Code **before** `next.ServeHTTP` runs on the way in; code **after** runs on
the way out. Stack several and you get an onion:

```
request → recover → requestID → logging → auth → handler
response ←   ...   ←    ...    ←   ...   ←  ... ←
```

### 1.6 `context.Context`: cancellation and request-scoped values

Every request has `r.Context()`. It is **cancelled** when the client disconnects
or the server shuts down. You pass it down to the DB:

```go
rows, err := db.Query(r.Context(), "SELECT ...")  // stops if the client goes away
```

**Rule:** any function that does I/O takes `ctx context.Context` as its
**first parameter**. You'll see this in every service and repository method.

### 1.7 Why do we need a framework at all?

Raw `net/http` makes you hand-write: JSON decoding + validation, path
params in groups, route groups with shared middleware, consistent error
responses, panic recovery. Gin gives you all of that, with very little overhead.

---

## Part 2 — How Gin is built

### 2.1 `gin.Engine` is an `http.Handler`

```go
r := gin.New()                  // *gin.Engine
http.ListenAndServe(":8080", r) // works, because Engine has ServeHTTP
```

That's why in Phase B1 we put a Gin engine inside our own `http.Server`
(for timeouts and graceful shutdown) instead of using `r.Run()`.

### 2.2 What happens on every request inside Gin

```
net/http goroutine
      │
      ▼
Engine.ServeHTTP(w, req)
  1. c := engine.pool.Get().(*Context)     ← reuse a Context from a sync.Pool (no allocation)
  2. c.reset(w, req)                       ← clear it for this request
  3. engine.handleHTTPRequest(c)
        a. find the radix tree for req.Method
        b. tree.getValue(path) → HandlersChain + path params (:id)
        c. c.handlers = chain; c.Next()    ← run middleware + handler
        d. no match → 404 / 405 handlers
  4. engine.pool.Put(c)                    ← return Context to the pool
```

Key consequence of step 4: **never keep `*gin.Context` after the handler
returns** (e.g. in a goroutine). It gets reused by another request. If you
need it in a goroutine, use `c.Copy()`.

### 2.3 The router: a radix tree per HTTP method

Gin stores routes in a **compressed prefix tree** (radix tree), one per method:

```
GET tree
/api/v1/
 ├─ todos            → [Recovery, RequestID, Logger, list]
 │   └─ /:id         → [Recovery, RequestID, Logger, get]
 └─ activity         → [...]
```

Lookup cost depends on the **path length**, not the number of routes. This is
why Gin is fast. The trade-off is strict rules: you can't register both
`/todos/:id` and `/todos/new` as siblings in older versions, and wildcards
(`*path`) must be at the end.

### 2.4 `HandlersChain` and `c.Next()`

In Gin, middleware and handlers have the **same type**:

```go
type HandlerFunc func(*gin.Context)
type HandlersChain []HandlerFunc
```

When you write:

```go
r.Use(A, B)
r.GET("/x", C)
```

Gin stores `[A, B, C]` for `GET /x`. Running it is just an index:

```go
// simplified from gin's source
func (c *Context) Next() {
    c.index++
    for c.index < int8(len(c.handlers)) {
        c.handlers[c.index](c)
        c.index++
    }
}
```

- If middleware `A` **calls `c.Next()`**, `B` and `C` run *inside* that call,
  and then the rest of `A` runs afterwards (that's how a logger measures latency).
- If `A` **doesn't** call `c.Next()`, Gin still continues to `B` after `A`
  returns (the loop above).
- `c.Abort()` sets the index to the end → nothing after it runs.
  `c.AbortWithStatusJSON(401, ...)` = abort + write response. **Always
  `return` right after aborting**, otherwise the rest of *your* function still runs.

### 2.5 `gin.Context`: everything about one request

| Need | Method |
|------|--------|
| Path param `/todos/:id` | `c.Param("id")` |
| Query `?page=2` | `c.Query("page")`, `c.DefaultQuery("page","1")` |
| Header | `c.GetHeader("Authorization")` |
| Bind + validate JSON body | `c.ShouldBindJSON(&req)` |
| Bind + validate query string | `c.ShouldBindQuery(&q)` |
| Store a value for later middleware/handlers | `c.Set("user_id", id)` / `c.GetString("user_id")` |
| Respond with JSON | `c.JSON(200, obj)` |
| Respond with no body | `c.Status(204)` |
| Standard `context.Context` | `c.Request.Context()` ← pass THIS to services/DB |
| Client IP (respects trusted proxies) | `c.ClientIP()` |

### 2.6 Binding & validation

Gin uses `encoding/json` to decode and **go-playground/validator** to
validate, driven by **struct tags**:

```go
type CreateTodoRequest struct {
    Title    string `json:"title"    binding:"required,min=1,max=200"`
    Priority string `json:"priority" binding:"omitempty,oneof=low medium high"`
}

var req CreateTodoRequest
if err := c.ShouldBindJSON(&req); err != nil {
    c.JSON(400, gin.H{"error": err.Error()})
    return
}
```

- `json:"title"`: the JSON key name.
- `binding:"..."`: validation rules. `required`, `min`, `max`, `oneof`,
  `email`, `omitempty` (skip the other rules if empty).
- `ShouldBind*` returns the error to you. `Bind*` (without "Should")
  automatically writes a 400 and aborts. We always use `ShouldBind*` so we
  control the error format.

**Struct tags** are just strings attached to fields. Packages read them at
runtime using **reflection** (`reflect` package). Nothing else magic happens.

### 2.7 Route groups

```go
v1 := r.Group("/api/v1")
todos := v1.Group("/todos", requireUser)   // middleware only for this group
todos.GET("", list)                        // GET /api/v1/todos
todos.GET("/:id", get)                     // GET /api/v1/todos/:id
```

A group is just a **path prefix + a list of middleware** that gets prepended to
each route's chain when it's registered.

### 2.8 `gin.Default()` vs `gin.New()`

`gin.Default()` = `gin.New()` + `Logger()` + `Recovery()`. We use
`gin.New()` and add our **own** JSON logger, so logs are structured and include
the request ID.

### 2.9 Gin modes

`gin.SetMode(gin.ReleaseMode)` turns off debug route printing and warnings.
We set this from the `GIN_MODE` env var (`debug` locally, `release` in Docker).

---

## Part 3 — How a Go service starts (what our `main.go` does)

```
main()
 └─ run() error
     1. load config from env           (fail fast if a required var is missing)
     2. create logger
     3. ctx := signal.NotifyContext(SIGINT, SIGTERM)   ← cancelled on Ctrl+C / docker stop
     4. open infrastructure            (DB pool, NATS)  + defer Close()
     5. build layers bottom-up         repo → service → handler
     6. build router                   gin.Engine + middleware + routes
     7. httpserver.Run(ctx, ...)       blocks until ctx is cancelled, then graceful shutdown
     8. deferred Close() calls run     (DB pool closed last)
```

Why `run() error` instead of putting everything in `main`? Because `os.Exit`
skips deferred functions. With `run()`, all `defer`s run, and only `main`
decides the exit code.

## Part 4 — Mini glossary

| Term | Meaning |
|------|---------|
| package | A folder of `.go` files with the same `package x` line. Unit of compilation and import. |
| module | A tree of packages with a `go.mod`. Unit of versioning and dependency management. |
| exported | Starts with a capital letter (`TodoService`) → visible to other packages. Lowercase = private to the package. |
| interface | A set of method signatures. Types satisfy it **implicitly**, with no `implements` keyword. |
| pointer receiver | `func (s *TodoService) Create(...)`: method can modify `s`, and avoids copying the struct. |
| goroutine | A lightweight thread started with `go f()`. |
| channel | Typed pipe between goroutines: `ch := make(chan error, 1)`. |
| `defer` | Run this when the function returns, in reverse order. Used for `Close()`, `Unlock()`. |
| `error` | An interface: `Error() string`. Errors are values, returned, never thrown. |
| `errors.Is` / `%w` | Wrap errors with context (`fmt.Errorf("create: %w", err)`) and still check the root cause. |
