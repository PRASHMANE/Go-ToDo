# Backend Phase 6 — Dockerize Everything

**Goal:** `docker compose up --build` starts Postgres, runs migrations, and
starts auth-service, todo-service and the gateway, in the right order, with
health checks. Only the gateway is reachable from your Mac.
**Branch:** `phase/b6-docker` · **Tag:** `b6`

## Concepts

- **Image vs container:** an *image* is a read-only template (your compiled
  app + OS files). A *container* is a running instance of an image.
- **Multi-stage build:** stage 1 uses the big `golang` image (~800 MB) to
  compile; stage 2 copies **only the binary** into a tiny `distroless` image
  (~25–40 MB total vs ~800 MB). Smaller = faster deploys and fewer vulnerabilities.
- **Static binary:** `CGO_ENABLED=0` makes Go produce a binary with no C
  library dependencies, so it runs on a minimal image with no OS packages.
- **Layer caching:** Docker caches each instruction. We copy `go.mod`/`go.sum`
  and run `go mod download` **before** copying source code, so changing a
  `.go` file doesn't re-download all dependencies.
- **Non-root:** containers run as user `nonroot`, never root.
- **Compose networking:** all services join a private network and reach each
  other by **service name** (`http://todo-service:8082`). Services without
  `ports:` are invisible from your Mac. That's what makes the `X-User-ID`
  trust model safe.
- **Startup order:** `depends_on` with `condition: service_healthy` /
  `service_completed_successfully` waits until a dependency is *actually ready*, not just started.

## Step 1 — A healthcheck command inside the binary

Distroless images have no `curl`/`wget`, so the binary checks itself.

Add to `pkg/httpserver/server.go`:

```go
// Healthcheck GETs http://127.0.0.1:<port>/healthz and returns an exit code.
// Used by Docker: `CMD ["/app", "healthcheck"]`.
func Healthcheck(port string) int {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
```

At the very top of **every** service's `main()`:

```go
func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(httpserver.Healthcheck(config.String("PORT", "8082"))) // use each service's default port
	}
	if err := run(); err != nil { ... }
}
```

(Import `github.com/PRASHMANE/Go-ToDo/pkg/config` as `pkgconfig` if the name clashes.)

## Step 2 — Dockerfile (same pattern for each service)

`services/todo-service/Dockerfile`:

```dockerfile
# syntax=docker/dockerfile:1

############################
# Stage 1: build
############################
FROM golang:1.27-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOWORK=off

# 1) dependencies first (cached until go.mod/go.sum change)
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY services/todo-service/go.mod services/todo-service/go.sum ./services/todo-service/
RUN cd services/todo-service && go mod download

# 2) source code
COPY pkg/ ./pkg/
COPY services/todo-service/ ./services/todo-service/

RUN cd services/todo-service && \
    go build -trimpath -ldflags="-s -w" -o /out/app ./cmd

############################
# Stage 2: runtime
############################
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/app /app
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/app"]
```

Line by line:

| Line | Why |
|------|-----|
| `GOWORK=off` | No `go.work` in the container; use `go.mod` + the `replace ../../pkg` directive. |
| `COPY pkg/...` | The `replace` path `../../pkg` must exist relative to the service. |
| `-trimpath` | Removes your Mac's file paths from the binary. |
| `-ldflags="-s -w"` | Strips debug symbols: smaller binary. |
| `distroless/static:nonroot` | No shell, no package manager, runs as UID 65532. |

Copy this file to `auth-service` and `api-gateway`, replacing the folder
name and `EXPOSE` port (8081 / 8080).

**Build context is the repo root**, because the Dockerfile needs `pkg/`:

```bash
docker build -f services/todo-service/Dockerfile -t go-todo/todo-service .
docker images | grep go-todo     # look at the size!
```

## Step 3 — `.dockerignore` (repo root)

```
.git
**/node_modules
**/dist
**/tmp
**/*.test
.env
docs
frontend
```

(`frontend` gets its own context in F5.)

## Step 4 — Full `docker-compose.yml`

```yaml
name: go-todo

x-go-service: &go-service            # YAML anchor: shared settings
  restart: unless-stopped
  environment: &go-env
    LOG_LEVEL: info
    GIN_MODE: release

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "15432:5432"           # host 15432 (your Mac may already run Postgres on 5432/5433); dev only
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./deployments/postgres/init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10

  # ---------- one-off migration jobs ----------
  auth-migrate:
    image: migrate/migrate:v4.20.1
    command: ["-path=/migrations", "-database=postgres://auth_user:auth_pass@postgres:5432/auth_db?sslmode=disable", "up"]
    volumes:
      - ./services/auth-service/migrations:/migrations:ro
    depends_on:
      postgres: { condition: service_healthy }

  todo-migrate:
    image: migrate/migrate:v4.20.1
    command: ["-path=/migrations", "-database=postgres://todo_user:todo_pass@postgres:5432/todo_db?sslmode=disable", "up"]
    volumes:
      - ./services/todo-service/migrations:/migrations:ro
    depends_on:
      postgres: { condition: service_healthy }

  # ---------- services ----------
  auth-service:
    <<: *go-service
    build:
      context: .
      dockerfile: services/auth-service/Dockerfile
    environment:
      <<: *go-env
      PORT: "8081"
      DATABASE_URL: postgres://auth_user:auth_pass@postgres:5432/auth_db?sslmode=disable
      JWT_SECRET: ${JWT_SECRET:?set JWT_SECRET in .env}
      JWT_ISSUER: go-todo-auth
    healthcheck:
      test: ["CMD", "/app", "healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
    depends_on:
      auth-migrate: { condition: service_completed_successfully }

  todo-service:
    <<: *go-service
    build:
      context: .
      dockerfile: services/todo-service/Dockerfile
    environment:
      <<: *go-env
      PORT: "8082"
      DATABASE_URL: postgres://todo_user:todo_pass@postgres:5432/todo_db?sslmode=disable
    healthcheck:
      test: ["CMD", "/app", "healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
    depends_on:
      todo-migrate: { condition: service_completed_successfully }

  api-gateway:
    <<: *go-service
    build:
      context: .
      dockerfile: services/api-gateway/Dockerfile
    environment:
      <<: *go-env
      PORT: "8080"
      AUTH_SERVICE_URL: http://auth-service:8081
      TODO_SERVICE_URL: http://todo-service:8082
      JWT_SECRET: ${JWT_SECRET:?set JWT_SECRET in .env}
      JWT_ISSUER: go-todo-auth
      CORS_ORIGINS: http://localhost:5173,http://localhost:3000
    ports:
      - "8080:8080"                   # the ONLY backend port published
    healthcheck:
      test: ["CMD", "/app", "healthcheck"]
      interval: 10s
      timeout: 3s
      retries: 5
    depends_on:
      auth-service: { condition: service_healthy }
      todo-service: { condition: service_healthy }

volumes:
  pgdata:
```

`${JWT_SECRET:?...}` reads from the `.env` file next to the compose file and
**refuses to start** if it's missing. Create `.env`:

```bash
cp .env.example .env
echo "JWT_SECRET=$(openssl rand -base64 48)" >> .env
```

## Step 5 — Makefile

```makefile
up:              ## Build and start everything
	docker compose up -d --build

down:            ## Stop everything (keeps data)
	docker compose down

reset:           ## Stop everything and DELETE the database volume
	docker compose down -v

logs:            ## Follow logs (make logs s=todo-service)
	docker compose logs -f $(s)

ps:              ## Show container status
	docker compose ps
```

## Verify

```bash
make up
make ps                       # all "healthy"; *-migrate show "exited (0)"
curl -s localhost:8080/healthz
curl -s localhost:8082/healthz || echo "todo-service is NOT reachable from the host ✅"
# re-run the full Verify block from Phase B5 against :8080
make logs s=api-gateway       # JSON logs with request_id
docker compose stop todo-service && curl -s localhost:8080/api/v1/todos -H "Authorization: Bearer $TA" | jq   # 502
docker compose start todo-service
```

Change a line in todo-service, run `make up` again, and see how fast the
rebuild is thanks to layer caching.

## Common problems

| Symptom | Fix |
|---------|-----|
| `replacement directory ../../pkg does not exist` | Build context must be `.` (repo root), not the service folder. |
| `missing go.sum entry` (e.g. for `golang-jwt` while building todo-service) | The workspace hid it locally. Run `make tidy` (it uses `GOWORK=off go mod tidy` per module) and commit the `go.sum` files. |
| migrate: `no change` | Fine: migrations already applied. |
| `password authentication failed` | `init.sql` only runs on a fresh volume: `make reset`. |
| Port 5432 in use | A local Postgres is running: `brew services stop postgresql` or change the host port. |

## Exercises

1. Run `docker run --rm -it go-todo/todo-service sh`. Why does it fail? (There's no shell in distroless. Good!)
2. Scale todo-service: `docker compose up -d --scale todo-service=3`. Send requests and watch `make logs s=todo-service`: Docker DNS spreads them across replicas.
3. Run `docker compose stop -t 30 todo-service` while a slow request is running. Find the graceful shutdown log lines.

## Commit

```bash
git add -A
git commit -m "build: multi-stage Dockerfiles and docker compose for all services (B6)"
```

**Next:** [Frontend Phase 4](../frontend/phase-4-advanced-todo-features.md) →
[Frontend Phase 5](../frontend/phase-5-docker-nginx.md) →
[Backend Phase 7 — Testing](phase-7-testing.md)
