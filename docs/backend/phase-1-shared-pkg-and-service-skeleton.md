# Backend Phase 1 — Shared `pkg` & Service Skeleton

**Goal:** build the reusable building blocks every service needs (config,
logger, middleware, HTTP server with graceful shutdown), and a
`todo-service` that answers `GET /healthz`.
**Branch:** `phase/b1-shared-pkg` · **Tag:** `b1`

## Concepts

- **12-factor config:** config comes from **environment variables**, never
  hard-coded. Locally we use defaults; in Docker we set env vars.
- **Structured logging:** logs are JSON (`{"level":"INFO","msg":"request","status":200}`),
  so tools can search/filter them. Go's standard `log/slog` does this.
- **Request ID:** a unique ID per request, passed through every service in the
  `X-Request-ID` header, so you can find all logs for one user action.
- **Graceful shutdown:** when Docker sends `SIGTERM`, stop accepting new
  connections, let running requests finish (up to a timeout), then exit.
  Without it, users get errors during every deploy.
- **Server timeouts:** an `http.Server` without timeouts can be held open
  forever by slow clients (the "Slowloris" attack). Always set them.

## Files in this phase

```
pkg/
├── config/env.go
├── httpserver/server.go
├── logger/logger.go
├── middleware/
│   ├── logger.go
│   └── requestid.go
└── response/response.go
services/todo-service/
├── cmd/main.go
└── internal/
    ├── config/config.go
    └── router/router.go
```

## Step 1 — Dependencies for `pkg`

```bash
git checkout -b phase/b1-shared-pkg
cd pkg
go get github.com/gin-gonic/gin@latest
go get github.com/google/uuid@latest
```

## Step 2 — `pkg/config/env.go`

```go
// Package config provides small helpers to read typed environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the env var value, or fallback if it is unset or empty.
func String(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// Required returns the env var value or an error if it is missing.
// Use it for secrets: the service should refuse to start without them.
func Required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}

// Int parses an integer env var, falling back on missing or invalid values.
func Int(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

// Float parses a float env var.
func Float(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return v
}

// Duration parses values like "15m", "24h", "500ms".
func Duration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

// List splits a comma-separated env var: "a, b,c" → ["a","b","c"].
func List(key string, fallback []string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

## Step 3 — `pkg/logger/logger.go`

```go
// Package logger builds the structured JSON logger used by every service.
package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON logger that tags every line with the service name.
// level is one of: debug, info, warn, error.
func New(service, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler).With("service", service)
}
```

Usage: `log.Info("todo created", "todo_id", id)` → key/value pairs, not
formatted strings. Never `fmt.Sprintf` into a log message.

## Step 4 — `pkg/response/response.go`

One error format for the whole system:

```go
// Package response defines the JSON error envelope shared by all services.
package response

import "github.com/gin-gonic/gin"

// ErrorBody is the top-level error JSON: {"error": {...}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a machine-readable code and a human message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error writes the error JSON and ABORTS the middleware chain.
// The caller must still `return` afterwards.
func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}
```

## Step 5 — `pkg/middleware/requestid.go`

```go
// Package middleware contains Gin middleware shared by all services.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader is propagated across every service hop.
const RequestIDHeader = "X-Request-ID"

// ContextRequestID is the gin.Context key holding the request ID.
const ContextRequestID = "request_id"

// RequestID reuses an incoming X-Request-ID or generates a new one,
// stores it in the context, and echoes it in the response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ContextRequestID, id)
		c.Request.Header.Set(RequestIDHeader, id) // so a reverse proxy forwards it
		c.Header(RequestIDHeader, id)             // so the client sees it
		c.Next()
	}
}
```

## Step 6 — `pkg/middleware/logger.go`

```go
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs one structured line per request AFTER the handler has run.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next() // run the rest of the chain (handlers) first

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(), // "/api/v1/todos/:id" (pattern, good for grouping)
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", c.GetString(ContextRequestID),
		}
		switch {
		case status >= 500:
			log.Error("request", attrs...)
		case status >= 400:
			log.Warn("request", attrs...)
		default:
			log.Info("request", attrs...)
		}
	}
}
```

Notice the `c.Next()` in the middle: code before it runs **on the way in**,
code after it **on the way out**. That's how we can measure latency and read
the final status code.

## Step 7 — `pkg/httpserver/server.go`

```go
// Package httpserver runs an http.Server with sane timeouts and graceful shutdown.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Run starts the server and blocks until ctx is cancelled (SIGINT/SIGTERM)
// or the server fails to start. On cancellation it shuts down gracefully.
func Run(ctx context.Context, log *slog.Logger, addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,  // protects against slowloris
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", addr)
		// ListenAndServe blocks. It always returns a non-nil error;
		// ErrServerClosed just means "Shutdown was called", which is normal.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh: // failed to start (e.g. port already in use)
		return err
	case <-ctx.Done(): // got a shutdown signal
	}

	log.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("http server stopped")
	return nil
}
```

**Why a goroutine + channel + select?** `ListenAndServe` blocks forever, so we
run it in a goroutine and use `select` to wait for *whichever happens first*:
a startup error or a shutdown signal.

```bash
cd ..   # back to repo root
cd pkg && go mod tidy && go build ./... && cd ..
```

## Step 8 — todo-service: config

```bash
cd services/todo-service
go get github.com/gin-gonic/gin@latest
```

Add to `services/todo-service/go.mod` (manually, below the `module`/`go` lines):

```go
require github.com/PRASHMANE/Go-ToDo/pkg v0.0.0

replace github.com/PRASHMANE/Go-ToDo/pkg => ../../pkg
```

`services/todo-service/internal/config/config.go`:

```go
// Package config loads todo-service configuration from the environment.
package config

import pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"

// Config holds everything the service needs to start.
type Config struct {
	Port     string
	LogLevel string
	GinMode  string
}

// Load reads env vars, applying local-development defaults.
func Load() (Config, error) {
	return Config{
		Port:     pkgconfig.String("PORT", "8082"),
		LogLevel: pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:  pkgconfig.String("GIN_MODE", "debug"),
	}, nil
}
```

`pkgconfig` is an **import alias**, needed because both packages are named `config`.

## Step 9 — todo-service: router

`services/todo-service/internal/router/router.go`:

```go
// Package router builds the HTTP routing table for todo-service.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
)

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(
		gin.Recovery(),           // turn panics into 500s instead of crashing
		middleware.RequestID(),   // must run before Logger so the ID gets logged
		middleware.Logger(log),
	)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
```

## Step 10 — todo-service: `cmd/main.go`

```go
// Command todo-service runs the todo HTTP API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/router"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New("todo-service", cfg.LogLevel)

	// ctx is cancelled when we receive Ctrl+C (SIGINT) or docker stop (SIGTERM).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := router.New(log, cfg.GinMode)
	return httpserver.Run(ctx, log, ":"+cfg.Port, r)
}
```

```bash
go mod tidy
go run ./cmd
```

## Step 11 — Hot reload with `air` (optional, recommended)

```bash
cd services/todo-service
air init        # creates .air.toml
```

In `.air.toml` set `cmd = "go build -o ./tmp/main ./cmd"`. Then just run `air`;
it rebuilds on save. (`tmp/` is already git-ignored.)

## Step 12 — Makefile targets

Append to the root `Makefile`:

```makefile
run-todo:        ## Run todo-service locally
	cd services/todo-service && go run ./cmd
```

## Verify

```bash
make run-todo
# in another terminal:
curl -i localhost:8082/healthz
```

Expect `HTTP/1.1 200 OK`, an `X-Request-ID` header and `{"status":"ok"}`.
In the server terminal: one JSON log line. Press **Ctrl+C**: you should see
`shutting down http server` → `http server stopped`.

```bash
curl -i -H 'X-Request-ID: my-test-id' localhost:8082/healthz   # echoes your ID
PORT=9000 make run-todo                                         # config from env works
```

## Exercises

1. Start the service twice. Read the error from the second one: where in `httpserver.Run` did it come from?
2. Add a `GET /panic` route that calls `panic("boom")`. Confirm you get a 500 and the server keeps running. Then remove it.
3. Set `LOG_LEVEL=warn` and check that 200 requests are no longer logged.

## Commit

```bash
git add -A
git commit -m "feat(pkg): shared config, logger, middleware and graceful http server (B1)"
# then squash-merge + tag b1 (see 02-git-workflow.md)
```

**Next:** [Phase 2 — Todo CRUD](phase-2-todo-service-crud.md)
