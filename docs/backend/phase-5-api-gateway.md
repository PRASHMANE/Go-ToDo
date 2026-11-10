# Backend Phase 5 — API Gateway

**Goal:** one public entry point (`:8080`) that routes `/api/v1/auth/*` to
auth-service and `/api/v1/todos/*` to todo-service, validates JWTs, and applies
CORS + rate limiting. todo-service now serves **each user their own todos**.
**Branch:** `phase/b5-api-gateway` · **Tag:** `b5`

## Concepts

- **Reverse proxy:** the client talks to the gateway; the gateway opens a
  *new* request to the upstream service and streams the response back. Go's
  standard library ships one: `net/http/httputil.ReverseProxy`.
- **Edge authentication:** the gateway verifies the JWT once, then tells
  internal services who the user is via a trusted header `X-User-ID`.
- **🔒 Header spoofing:** a malicious client could send `X-User-ID: someone-else`.
  The gateway **must delete** that header from every incoming request before
  setting its own. Internal services must not be reachable from outside
  (Docker network isolation in B6).
- **CORS:** browsers block JavaScript from calling a different origin unless the
  server allows it. The gateway is the only place that sets CORS headers.
- **Rate limiting (token bucket):** each client IP gets a "bucket" that refills
  `N` tokens per second up to a `burst` size. Each request uses one token. Empty
  bucket → `429 Too Many Requests`.
- **Trusted proxies:** `c.ClientIP()` reads `X-Forwarded-For` only from
  proxies you trust; otherwise clients could fake their IP to dodge rate limits.

## Files in this phase

```
services/api-gateway/
├── cmd/main.go
├── internal/
│   ├── config/config.go
│   ├── middleware/
│   │   ├── headers.go          # strip / forward identity headers
│   │   └── ratelimit.go        # per-IP token bucket
│   ├── proxy/proxy.go
│   └── router/router.go
└── go.mod
services/todo-service/internal/router/router.go   # FixedUser → RequireUserHeader
```

## Step 1 — Module

```bash
git checkout -b phase/b5-api-gateway
mkdir -p services/api-gateway/{cmd,internal/{config,middleware,proxy,router}}
cd services/api-gateway
go mod init github.com/PRASHMANE/Go-ToDo/services/api-gateway
# add require/replace for pkg in go.mod, then:
go get github.com/gin-gonic/gin@latest github.com/gin-contrib/cors@latest golang.org/x/time/rate@latest
cd ../.. && go work use ./services/api-gateway
```

## Step 2 — `internal/config/config.go`

```go
package config

import pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"

type Config struct {
	Port           string
	LogLevel       string
	GinMode        string
	AuthServiceURL string
	TodoServiceURL string
	JWTSecret      string
	JWTIssuer      string
	CORSOrigins    []string
	TrustedProxies []string
	RateLimitRPS   float64
	RateLimitBurst int
}

func Load() (Config, error) {
	secret, err := pkgconfig.Required("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:           pkgconfig.String("PORT", "8080"),
		LogLevel:       pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:        pkgconfig.String("GIN_MODE", "debug"),
		AuthServiceURL: pkgconfig.String("AUTH_SERVICE_URL", "http://localhost:8081"),
		TodoServiceURL: pkgconfig.String("TODO_SERVICE_URL", "http://localhost:8082"),
		JWTSecret:      secret,
		JWTIssuer:      pkgconfig.String("JWT_ISSUER", "go-todo-auth"),
		CORSOrigins:    pkgconfig.List("CORS_ORIGINS", []string{"http://localhost:5173"}),
		TrustedProxies: pkgconfig.List("TRUSTED_PROXIES", nil),
		RateLimitRPS:   pkgconfig.Float("RATE_LIMIT_RPS", 10),
		RateLimitBurst: pkgconfig.Int("RATE_LIMIT_BURST", 20),
	}, nil
}
```

## Step 3 — `internal/proxy/proxy.go`

```go
// Package proxy builds reverse proxies to upstream services.
package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// New creates a reverse proxy to target (e.g. "http://todo-service:8082").
// The incoming path is kept as-is: /api/v1/todos → target/api/v1/todos.
func New(target string, log *slog.Logger) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse upstream url %q: %w", target, err)
	}

	return &httputil.ReverseProxy{
		// Rewrite (Go 1.20+) is the modern, safer hook. It receives the
		// incoming request (In) and the outgoing copy (Out).
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)      // scheme + host of upstream, keep the path
			pr.SetXForwarded() // X-Forwarded-For / -Host / -Proto
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error("upstream error", "upstream", u.Host, "path", r.URL.Path, "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"code":"bad_gateway","message":"upstream service unavailable"}}`))
		},
	}, nil
}

// Handler adapts a ReverseProxy (an http.Handler) into a gin.HandlerFunc.
func Handler(p *httputil.ReverseProxy) gin.HandlerFunc {
	return func(c *gin.Context) {
		p.ServeHTTP(c.Writer, c.Request)
	}
}
```

Read `Handler` slowly: `ReverseProxy` is just an `http.Handler`, and
`c.Writer` / `c.Request` are the plain `net/http` values Gin wraps. That's
the whole integration.

## Step 4 — `internal/middleware/headers.go`

```go
// Package middleware holds gateway-only middleware.
package middleware

import (
	"github.com/gin-gonic/gin"

	pkgmw "github.com/PRASHMANE/Go-ToDo/pkg/middleware"
)

// StripIdentityHeaders removes identity headers a client might try to forge.
// MUST run on every request, before anything else reads them.
func StripIdentityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(pkgmw.UserIDHeader)
		c.Next()
	}
}

// ForwardUserID copies the verified user ID (set by RequireJWT) into the
// X-User-ID header that upstream services trust.
func ForwardUserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Set(pkgmw.UserIDHeader, c.GetString(pkgmw.ContextUserID))
		c.Next()
	}
}
```

## Step 5 — `internal/middleware/ratelimit.go`

```go
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanup forgets IPs not seen for 3 minutes, so memory doesn't grow forever.
func (l *ipRateLimiter) cleanup() {
	for range time.Tick(time.Minute) {
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit allows rps requests/second per client IP with bursts up to burst.
// NOTE: state is per gateway instance. With several replicas you'd move this
// to Redis so all instances share one budget.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	l := &ipRateLimiter{visitors: make(map[string]*visitor), rps: rate.Limit(rps), burst: burst}
	go l.cleanup()

	return func(c *gin.Context) {
		if !l.get(c.ClientIP()).Allow() {
			c.Header("Retry-After", "1")
			response.Error(c, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		c.Next()
	}
}
```

## Step 6 — `internal/router/router.go`

```go
// Package router maps public paths to upstream services.
package router

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	pkgmw "github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/config"
	gwmw "github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/proxy"
)

// Upstreams groups the reverse proxies.
type Upstreams struct {
	Auth *httputil.ReverseProxy
	Todo *httputil.ReverseProxy
}

func New(cfg config.Config, log *slog.Logger, tm *auth.TokenManager, up Upstreams) (*gin.Engine, error) {
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil { // nil = trust nobody
		return nil, err
	}

	// ORDER MATTERS: this is the chain every request goes through.
	r.Use(
		gin.Recovery(),
		pkgmw.RequestID(),
		pkgmw.Logger(log),
		cors.New(cors.Config{
			AllowOrigins:  cfg.CORSOrigins,
			AllowMethods:  []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:  []string{"Authorization", "Content-Type", pkgmw.RequestIDHeader},
			ExposeHeaders: []string{pkgmw.RequestIDHeader},
			MaxAge:        12 * time.Hour,
		}),
		gwmw.RateLimit(cfg.RateLimitRPS, cfg.RateLimitBurst),
		gwmw.StripIdentityHeaders(),
	)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	authProxy := proxy.Handler(up.Auth)
	todoProxy := proxy.Handler(up.Todo)
	requireAuth := []gin.HandlerFunc{pkgmw.RequireJWT(tm), gwmw.ForwardUserID()}

	v1 := r.Group("/api/v1")

	// public
	v1.POST("/auth/register", authProxy)
	v1.POST("/auth/login", authProxy)

	// protected
	v1.GET("/auth/me", append(requireAuth, authProxy)...)

	todos := v1.Group("/todos", requireAuth...)
	todos.Any("", todoProxy)       // /api/v1/todos
	todos.Any("/*path", todoProxy) // /api/v1/todos/<anything>

	return r, nil
}
```

## Step 7 — `cmd/main.go`

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/httpserver"
	"github.com/PRASHMANE/Go-ToDo/pkg/logger"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/proxy"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/router"
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
	log := logger.New("api-gateway", cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authProxy, err := proxy.New(cfg.AuthServiceURL, log)
	if err != nil {
		return err
	}
	todoProxy, err := proxy.New(cfg.TodoServiceURL, log)
	if err != nil {
		return err
	}

	// The gateway only VERIFIES tokens, so the TTL value here is irrelevant.
	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer, time.Hour)

	r, err := router.New(cfg, log, tm, router.Upstreams{Auth: authProxy, Todo: todoProxy})
	if err != nil {
		return err
	}
	return httpserver.Run(ctx, log, ":"+cfg.Port, r)
}
```

## Step 8 — todo-service trusts the gateway

In `services/todo-service/internal/router/router.go`, replace:

```go
v1 := r.Group("/api/v1", middleware.FixedUser(DemoUserID))
```

with:

```go
v1 := r.Group("/api/v1", middleware.RequireUserHeader())
```

…and delete the `DemoUserID` constant. **The handler, service and repository
are untouched.** Each user now only sees their own todos, because every
repository query already filters by `user_id`.

Old demo todos belong to user `...0001` and will simply be invisible. Clean up
with `psql "$TODO_DB_URL" -c "delete from todos;"` if you like.

## Step 9 — Makefile

```makefile
run-gateway:     ## Run api-gateway locally
	cd services/api-gateway && JWT_SECRET="$(JWT_SECRET)" go run ./cmd
```

## Verify (three terminals: `make run-auth`, `make run-todo`, `make run-gateway`)

```bash
G=localhost:8080/api/v1
curl -s -X POST $G/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"a@x.com","name":"A","password":"password1"}' > /dev/null
curl -s -X POST $G/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"b@x.com","name":"B","password":"password1"}' > /dev/null
TA=$(curl -s -X POST $G/auth/login -H 'Content-Type: application/json' -d '{"email":"a@x.com","password":"password1"}' | jq -r .access_token)
TB=$(curl -s -X POST $G/auth/login -H 'Content-Type: application/json' -d '{"email":"b@x.com","password":"password1"}' | jq -r .access_token)

curl -s $G/todos | jq                                                     # 401: no token
curl -s -X POST $G/todos -H "Authorization: Bearer $TA" -H 'Content-Type: application/json' -d '{"title":"A only"}' | jq
curl -s $G/todos -H "Authorization: Bearer $TA" | jq '.total'             # 1
curl -s $G/todos -H "Authorization: Bearer $TB" | jq '.total'             # 0  ← isolation works
curl -s $G/todos -H "Authorization: Bearer $TB" -H "X-User-ID: whatever" | jq '.total'   # still 0 ← spoofing blocked
curl -s $G/auth/me -H "Authorization: Bearer $TA" | jq

# CORS preflight
curl -s -i -X OPTIONS $G/todos -H 'Origin: http://localhost:5173' \
  -H 'Access-Control-Request-Method: POST' | grep -i access-control

# rate limit: expect some 429s
for i in $(seq 1 40); do curl -s -o /dev/null -w '%{http_code} ' $G/auth/login -X POST; done; echo

# stop todo-service, then:
curl -s $G/todos -H "Authorization: Bearer $TA" | jq                     # 502 bad_gateway
```

## Exercises

1. Draw the middleware chain for `GET /api/v1/todos` from memory, in order. Why must `StripIdentityHeaders` come before `ForwardUserID`?
2. Add a stricter rate limit (e.g. 1 rps, burst 5) **only** for `/auth/login` to slow down password guessing.
3. Add a request body size limit (1 MB) with `http.MaxBytesReader` as gateway middleware.

## Commit

```bash
git add -A
git commit -m "feat(api-gateway): reverse proxy with JWT auth, CORS and rate limiting (B5)"
```

**Next:** [Frontend Phase 3 — Authentication](../frontend/phase-3-authentication.md), then
[Backend Phase 6 — Docker](phase-6-docker.md).
