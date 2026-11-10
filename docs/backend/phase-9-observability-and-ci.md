# Backend Phase 9 — Observability, Readiness & CI

**Goal:** know what the system is doing in production (metrics, readiness), and
let GitHub check every push automatically.
**Branch:** `phase/b9-observability-ci` · **Tag:** `b9`

## Concepts

- **The three pillars of observability:**
  - **Logs:** what happened (done since B1: JSON + request ID).
  - **Metrics:** numbers over time: requests/sec, error rate, latency (this phase).
  - **Traces:** one request's path across services (next step: OpenTelemetry).
- **RED method** for every service: **R**ate, **E**rrors, **D**uration.
- **Prometheus** *pulls* (scrapes) `/metrics` from each service every few seconds.
  - *Counter*: only goes up (`http_requests_total`).
  - *Histogram*: distribution in buckets (`http_request_duration_seconds`), giving p50/p95/p99.
  - **Labels** must have **low cardinality**: use the route *pattern*
    (`/api/v1/todos/:id`), never the raw path with IDs.
- **Liveness vs readiness:**
  - `/healthz` (liveness): "the process is alive". If it fails, restart me.
  - `/readyz` (readiness): "my dependencies (DB) are reachable; send me traffic."
- **CI (Continuous Integration):** every push runs vet, tests, lint and Docker
  builds. Broken code can't silently reach `main`.

## Step 1 — `pkg/middleware/metrics.go`

```bash
cd pkg && go get github.com/prometheus/client_golang@latest && cd ..
```

```go
package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests.",
	}, []string{"service", "method", "route", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets, // 5ms … 10s
	}, []string{"service", "method", "route"})
)

// Metrics records RED metrics for every request.
func Metrics(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath() // pattern, e.g. /api/v1/todos/:id
		if route == "" {
			route = "unmatched" // 404s: don't explode label cardinality with random paths
		}
		httpRequests.WithLabelValues(service, c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
		httpDuration.WithLabelValues(service, c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}
```

In each router:

```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log), middleware.Metrics("todo-service"))
r.GET("/metrics", gin.WrapH(promhttp.Handler())) // gin.WrapH adapts an http.Handler
```

> In production `/metrics` must not be public. Internal services are already
> hidden (B6). The gateway is public, though, so locally its `/metrics` is
> reachable on :8080. That's handy for learning. In production, serve the
> gateway's metrics on a separate internal port (a second `http.Server` on
> e.g. `:9100`) that only Prometheus can reach.

## Step 2 — Readiness endpoint

In services with a DB, pass a "pinger" to the router:

```go
// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface{ Ping(ctx context.Context) error }

r.GET("/readyz", func(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "db": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
})
```

Note how `*pgxpool.Pool` satisfies `Pinger` without knowing it exists.
That's Go interfaces again: the router depends only on what it needs.

## Step 3 — Prometheus + Grafana in compose (optional but fun)

`deployments/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 5s
scrape_configs:
  - job_name: go-todo
    static_configs:
      - targets: ["api-gateway:8080", "auth-service:8081", "todo-service:8082", "activity-service:8083"]
```

`docker-compose.yml`:

```yaml
  prometheus:
    image: prom/prometheus:v3.0.1
    volumes: ["./deployments/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro"]
    ports: ["9090:9090"]
    profiles: ["observability"]

  grafana:
    image: grafana/grafana:11.3.0
    ports: ["3001:3000"]
    profiles: ["observability"]
```

`profiles` means they only start with `docker compose --profile observability up -d`.

Useful PromQL queries (http://localhost:9090):

```promql
sum by (service) (rate(http_requests_total[1m]))                                   # Rate
sum by (service) (rate(http_requests_total{status=~"5.."}[5m]))                     # Errors
histogram_quantile(0.95, sum by (le, route) (rate(http_request_duration_seconds_bucket[5m])))   # p95 Duration
```

## Step 4 — GitHub Actions: `.github/workflows/ci-backend.yml`

```yaml
name: backend-ci

on:
  push:
    branches: [main]
    paths: ["pkg/**", "services/**", "go.work", ".github/workflows/ci-backend.yml"]
  pull_request:
    paths: ["pkg/**", "services/**", "go.work"]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        module: [pkg, services/api-gateway, services/auth-service, services/todo-service, services/activity-service]
    defaults:
      run:
        working-directory: ${{ matrix.module }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: ${{ matrix.module }}/go.mod
          cache-dependency-path: ${{ matrix.module }}/go.sum
      - run: go vet ./...
      - run: go test -race -cover ./...
      - uses: golangci/golangci-lint-action@v8
        with:
          working-directory: ${{ matrix.module }}

  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: todo_user
          POSTGRES_PASSWORD: todo_pass
          POSTGRES_DB: todo_db
        ports: ["5432:5432"]
        options: >-
          --health-cmd "pg_isready -U todo_user" --health-interval 5s --health-retries 10
    env:
      TEST_DATABASE_URL: postgres://todo_user:todo_pass@localhost:5432/todo_db?sslmode=disable
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: services/todo-service/go.mod }
      - run: go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
      - run: migrate -path services/todo-service/migrations -database "$TEST_DATABASE_URL" up
      - run: go test -tags=integration -race ./internal/repository/...
        working-directory: services/todo-service

  docker:
    runs-on: ubuntu-latest
    needs: test
    strategy:
      matrix:
        service: [api-gateway, auth-service, todo-service, activity-service]
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/build-push-action@v6
        with:
          context: .
          file: services/${{ matrix.service }}/Dockerfile
          push: false
          tags: go-todo/${{ matrix.service }}:ci
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

## Step 5 — Production checklist (read and tick off)

- [ ] Secrets come from env/secret manager; `.env` never committed
- [ ] Every `http.Server` has timeouts (B1) and graceful shutdown
- [ ] Request bodies are size-limited at the gateway (B5 exercise)
- [ ] Pagination has a max page size (B2: `max=100`)
- [ ] Internal services are not publicly reachable (B6)
- [ ] Containers run as non-root, from minimal images (B6)
- [ ] Migrations are versioned and run before the app starts (B6)
- [ ] Errors never leak internals to clients (`internal_error`)
- [ ] Logs are structured and carry `request_id`
- [ ] `/healthz`, `/readyz`, `/metrics` on every service
- [ ] CI runs vet, race tests, lint, integration tests and image builds
- [ ] Postgres backups (outside this project's scope, but don't forget in real life)

## Verify

```bash
make up
curl -s localhost:8080/metrics | grep http_requests_total | head
docker compose exec api-gateway /app healthcheck; echo "exit=$?"
docker compose --profile observability up -d && open http://localhost:9090
git push origin phase/b9-observability-ci   # open a PR on GitHub and watch the checks go green
```

## Exercises

1. Add a business metric in todo-service: `todos_created_total` counter, incremented in the service. (Hint: inject it, don't use a global, so tests stay clean.)
2. Stop Postgres and watch `/readyz` go to 503 while `/healthz` stays 200. Why is that the right behaviour?
3. Build a Grafana dashboard with the three RED panels for the gateway.

## Commit

```bash
git add -A
git commit -m "feat: prometheus metrics, readiness checks and GitHub Actions CI (B9)"
```

🎉 **Backend complete.** Where to go next: see "Deliberately left for later" in
[00-architecture.md](../00-architecture.md#8-deliberately-left-for-later-good-next-steps-after-b9).
