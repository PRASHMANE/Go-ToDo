# 00 — Architecture, Patterns & File Structure

## 1. What we are building

A multi-user todo application.

### Features (final product)

| Area | Feature |
|------|---------|
| Accounts | Register, log in, see "my profile" (`/me`), log out |
| Todos | Create, list, view, edit, delete, mark complete/incomplete |
| Todo fields | Title, description, priority (low/medium/high), due date, completed flag, timestamps |
| Listing | Filter by status (all/active/completed) and priority, text search, pagination |
| Activity | A feed of "what happened" (created/completed/updated/deleted), built from events |
| Platform | API gateway, JWT auth, rate limiting, CORS, request IDs, structured logs, metrics, health checks |
| Ops | Everything runs with `docker compose up`; CI runs tests, lint and Docker builds |

## 2. The architecture

We are building a **microservice architecture** with an **API gateway**,
**database-per-service**, and **event-driven** communication for side effects.
*Inside* every service we use **clean (layered) architecture**.

```
                         ┌──────────────────────────┐
   Browser               │  frontend (React + Vite) │  served by Nginx (:3000)
   ───────────────────▶  └────────────┬─────────────┘
                                      │  /api/v1/*  (HTTP + JSON, Bearer JWT)
                                      ▼
                         ┌──────────────────────────┐
                         │       api-gateway        │  :8080   (the ONLY public backend port)
                         │  - routing / reverse proxy│
                         │  - JWT validation         │
                         │  - CORS, rate limit       │
                         │  - request ID, logging    │
                         └──┬──────────┬─────────┬──┘
              /auth/*       │          │ /todos/* │ /activity/*
                            ▼          ▼          ▼
                 ┌──────────────┐ ┌─────────────┐ ┌──────────────────┐
                 │ auth-service │ │ todo-service│ │ activity-service │
                 │    :8081     │ │    :8082    │ │      :8083       │
                 └──────┬───────┘ └──┬───────┬──┘ └───▲──────────┬───┘
                        │            │       │ publish │ consume  │
                        ▼            ▼       ▼         │          ▼
                   ┌─────────┐ ┌─────────┐ ┌──────────────┐ ┌─────────────┐
                   │ auth_db │ │ todo_db │ │ NATS         │ │ activity_db │
                   └─────────┘ └─────────┘ │ JetStream    │ └─────────────┘
                     (one Postgres server,  │ stream TODOS │
                      separate databases)   └──────────────┘
```

### Services and their single responsibility

| Service | Port | Owns | Responsibility |
|---------|------|------|----------------|
| `api-gateway` | 8080 | nothing (stateless) | Single entry point. Verifies JWTs, forwards requests to the right service, applies cross-cutting concerns (CORS, rate limit, request IDs). |
| `auth-service` | 8081 | `auth_db.users` | User accounts, password hashing, issuing JWTs. |
| `todo-service` | 8082 | `todo_db.todos` | Todo CRUD and querying. Publishes `todos.*` events. |
| `activity-service` | 8083 | `activity_db.activities` | Listens to todo events and builds a per-user activity feed. |

### A request, step by step: "create a todo"

1. React calls `POST /api/v1/todos` with header `Authorization: Bearer <jwt>`.
2. **Nginx** (in Docker) forwards `/api/*` to `api-gateway:8080`.
3. **Gateway middleware chain** runs: recover from panics → assign `X-Request-ID`
   → log → CORS → rate limit → **strip any client-sent `X-User-ID`** →
   validate JWT → set `X-User-ID: <user id from token>`.
4. Gateway **reverse-proxies** the request to `todo-service:8082`.
5. todo-service: **handler** binds and validates JSON → calls **service** (business
   rules) → calls **repository** (SQL INSERT into `todo_db`).
6. The service **publishes** a `todos.created` event to NATS.
7. todo-service returns `201 Created` + JSON. Gateway streams it back to React.
8. Later, **activity-service** receives the event and stores an activity row.
   (Step 8 does not slow down step 7. This is the point of events.)

## 3. Architecture styles & design patterns we use

| Pattern | Where | Why we use it |
|---------|-------|---------------|
| **Microservices** | whole system | Each service is deployed, scaled and failed independently, with its own code and data. |
| **API Gateway** | `api-gateway` | Clients talk to one URL. Auth, CORS and rate limits live in one place, not copied into every service. |
| **Database per service** | Postgres: `auth_db`, `todo_db`, `activity_db` | A service never reads another service's tables. That keeps services loosely coupled. (We use one Postgres *server* to save resources but separate *databases* and *users*.) |
| **Clean / layered architecture** | inside each service | `handler → service → repository`. HTTP details don't leak into business logic, and SQL doesn't leak into handlers. |
| **Repository pattern** | `internal/repository` | The service depends on an *interface* (`TodoRepository`), so we can swap in-memory ↔ Postgres, and use fakes in tests. |
| **Dependency injection (constructor injection)** | every `New...()` + `main.go` | Dependencies are passed in, not created inside. `main.go` is the "composition root" that wires everything. No DI framework needed in Go. |
| **DTO (Data Transfer Object)** | `internal/dto` | JSON request/response shapes are separate from domain models, so the API can change without changing the core. |
| **Middleware (chain of responsibility)** | Gin middleware, `pkg/middleware` | Cross-cutting logic (logging, auth, request ID) wraps handlers in a chain. |
| **Reverse proxy** | gateway | Forwards requests to upstream services using `net/http/httputil.ReverseProxy`. |
| **Stateless JWT authentication** | auth-service issues, gateway verifies | No session store needed; any gateway instance can verify a token. |
| **Publish / subscribe (event-driven)** | NATS JetStream | todo-service doesn't know or care who listens. New features (emails, analytics) can subscribe later without changing todo-service. |
| **Idempotent consumer** | activity-service | Events may be delivered more than once; a unique `event_id` makes processing them twice harmless. |
| **12-factor config** | `pkg/config`, `.env` | All config comes from environment variables. The same Docker image runs in dev and prod. |
| **Graceful shutdown** | `pkg/httpserver` | On SIGTERM, stop accepting requests and finish in-flight ones before exiting. |
| **Health checks (liveness/readiness)** | `/healthz`, `/readyz` | Docker/Kubernetes know when a service is alive and when it is ready for traffic. |
| **Multi-stage Docker builds** | every `Dockerfile` | Compile in a big image, ship a small image (~25–40 MB) with only the binary. |
| **Monorepo + Go workspace** | `go.work` | All services live in one git repo, but each is an independent Go module. |
| **Feature-based folders (frontend)** | `frontend/src/features/*` | Code is grouped by feature (auth, todos), not by file type. |
| **Server-state caching (frontend)** | TanStack Query | API data is cached, re-fetched and invalidated automatically instead of being hand-managed in `useState`. |

### Clean architecture inside one service

```
          HTTP request
               │
   ┌───────────▼────────────┐   knows: Gin, JSON, status codes
   │  handler  (delivery)   │   does: bind + validate DTO, call service, map errors → HTTP
   └───────────┬────────────┘
               │ calls (Go structs, context)
   ┌───────────▼────────────┐   knows: business rules only
   │  service  (use case)   │   does: defaults, validation rules, orchestration, events
   └───────────┬────────────┘
               │ via interface TodoRepository
   ┌───────────▼────────────┐   knows: SQL / storage
   │  repository            │   does: INSERT/SELECT/UPDATE/DELETE
   └───────────┬────────────┘
               ▼
           PostgreSQL

   model: plain domain structs + domain errors, used by all layers
```

**The dependency rule:** arrows only point *down*. `repository` never imports
`handler`. `service` never imports `gin`. This is what makes each layer
testable on its own.

> In the food-delivery example you saw, the layers were called
> `delivery / usecase / repository / models`. We use the friendlier names
> `handler / service / repository / model`. **Same idea.**

## 4. Complete file structure (final state after all phases)

```
Go-ToDo/
├── .github/
│   └── workflows/
│       ├── ci-backend.yml            # Go vet + test + lint for every module, Docker build
│       └── ci-frontend.yml           # npm ci + lint + test + build
├── api/
│   └── openapi/
│       └── gateway.yaml              # OpenAPI (Swagger) spec of the public API
├── deployments/
│   ├── postgres/
│   │   └── init.sql                  # Creates auth_db, todo_db, activity_db + users
│   └── prometheus/
│       └── prometheus.yml            # Scrape config for /metrics (Phase B9)
├── docs/                             # ← you are here
├── frontend/                         # React app (see section 5)
├── pkg/                              # Shared Go library module. NO business logic.
│   ├── auth/
│   │   └── jwt.go                    # TokenManager: generate & parse JWTs
│   ├── config/
│   │   └── env.go                    # Helpers to read env vars with defaults
│   ├── database/
│   │   └── postgres.go               # pgx connection pool constructor
│   ├── events/
│   │   ├── events.go                 # Event structs + subject names (the "contract")
│   │   └── nats.go                   # Connect to NATS, ensure stream exists
│   ├── httpserver/
│   │   └── server.go                 # http.Server with timeouts + graceful shutdown
│   ├── logger/
│   │   └── logger.go                 # slog JSON logger
│   ├── middleware/
│   │   ├── auth.go                   # RequireJWT (Bearer token → user id)
│   │   ├── logger.go                 # Request logging
│   │   ├── metrics.go                # Prometheus metrics (B9)
│   │   ├── requestid.go              # X-Request-ID
│   │   └── user.go                   # RequireUserHeader (X-User-ID from gateway)
│   ├── response/
│   │   └── response.go               # Standard JSON error envelope
│   ├── go.mod
│   └── go.sum
├── services/
│   ├── api-gateway/
│   │   ├── cmd/
│   │   │   └── main.go               # Entry point: wires config, proxies, router
│   │   ├── internal/
│   │   │   ├── config/config.go      # Gateway env config
│   │   │   ├── middleware/           # Gateway-only middleware (rate limit, header strip)
│   │   │   ├── proxy/proxy.go        # httputil.ReverseProxy factory
│   │   │   └── router/router.go      # Which path goes to which service
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── auth-service/
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── config/               # Env config
│   │   │   ├── dto/                  # Register/Login request & response JSON
│   │   │   ├── handler/              # Gin handlers (HTTP layer)
│   │   │   ├── model/                # User struct + domain errors
│   │   │   ├── repository/           # Postgres user storage
│   │   │   ├── router/               # Builds the gin.Engine
│   │   │   └── service/              # Register/Login business logic
│   │   ├── migrations/               # 000001_create_users.up.sql / .down.sql
│   │   ├── Dockerfile
│   │   └── go.mod
│   ├── todo-service/
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   ├── dto/                  # CreateTodoRequest, TodoResponse, ...
│   │   │   ├── events/               # JetStream publisher implementation
│   │   │   ├── handler/
│   │   │   ├── model/                # Todo, Priority, ListFilter, errors
│   │   │   ├── repository/           # memory.go (B2) + postgres.go (B3)
│   │   │   ├── router/
│   │   │   └── service/              # TodoService + TodoRepository interface
│   │   ├── migrations/
│   │   ├── Dockerfile
│   │   └── go.mod
│   └── activity-service/
│       ├── cmd/main.go
│       ├── internal/
│       │   ├── config/
│       │   ├── consumer/             # JetStream consumer (reads todo events)
│       │   ├── handler/              # GET /api/v1/activity
│       │   ├── model/
│       │   ├── repository/
│       │   ├── router/
│       │   └── service/
│       ├── migrations/
│       ├── Dockerfile
│       └── go.mod
├── .editorconfig
├── .env.example                      # Template for secrets/config; copy to .env
├── .gitignore
├── .golangci.yml                     # Linter config
├── docker-compose.yml                # Runs the whole system locally
├── go.work                           # Go workspace: ties pkg + all services together
├── Makefile                          # make up / down / test / lint / migrate
└── README.md
```

### What each Go folder means (Go conventions)

| Folder / file | Meaning |
|---------------|---------|
| `go.mod` | Declares a **module**: its import path (e.g. `github.com/PRASHMANE/Go-ToDo/services/todo-service`), Go version and dependencies. Each service has its own, so each can be built and versioned independently. |
| `go.sum` | Checksums of every dependency. Commit it. It guarantees everyone downloads byte-identical code. |
| `go.work` | A **workspace**: tells the Go tool "these local modules belong together", so `todo-service` uses your local `pkg/` folder instead of downloading it. |
| `cmd/` | Holds `main` packages, the **entry points** that compile to binaries. Keep `main.go` thin: read config, wire dependencies, start the server. |
| `internal/` | **Special to the Go compiler:** packages under `internal/` can only be imported by code in the parent folder tree. `auth-service` *cannot* import `todo-service/internal/...`. The compiler enforces service boundaries for you. |
| `pkg/` | Code that is *meant* to be shared. Only generic, infrastructure-level things (logging, config, JWT). If you're tempted to put a `Todo` struct here, stop: that's business logic and belongs to a service. |
| `migrations/` | Versioned SQL files (`000001_x.up.sql` / `.down.sql`). The DB schema evolves through these, never by hand. |
| `Dockerfile` | How to build this service's container image. |

### Why `replace` + `go.work`?

`pkg` is not published anywhere, so each service's `go.mod` contains:

```go
require github.com/PRASHMANE/Go-ToDo/pkg v0.0.0
replace github.com/PRASHMANE/Go-ToDo/pkg => ../../pkg
```

`replace` makes Docker builds work (no workspace inside the container);
`go.work` makes your editor and `go` commands at the repo root work smoothly.

## 5. Frontend structure (final state)

```
frontend/
├── public/                         # Static files copied as-is
├── src/
│   ├── main.tsx                    # Entry: mounts <App/> into index.html
│   ├── App.tsx                     # Route table
│   ├── index.css                   # Tailwind import + global styles
│   ├── app/
│   │   └── providers.tsx           # QueryClientProvider, AuthProvider, Router
│   ├── lib/
│   │   ├── api-client.ts           # axios instance + interceptors (token, 401)
│   │   ├── query-client.ts         # TanStack Query defaults
│   │   └── token-storage.ts        # Read/write the JWT safely
│   ├── components/
│   │   ├── ui/                     # Dumb, reusable: Button, Input, Spinner, ErrorMessage
│   │   └── layout/AppLayout.tsx    # Header + <Outlet/>
│   ├── hooks/
│   │   └── useDebounce.ts          # Generic hooks
│   ├── features/
│   │   ├── auth/                   # api.ts, types.ts, AuthContext, AuthProvider, useAuth, ProtectedRoute, forms
│   │   ├── todos/                  # api.ts, types.ts, hooks.ts, components/*
│   │   └── activity/               # api.ts, hooks.ts, ActivityFeed.tsx (F6)
│   ├── pages/                      # Route-level screens: TodosPage, LoginPage, RegisterPage, NotFoundPage
│   └── test/                       # Test setup + MSW handlers
├── .env.example
├── Dockerfile                      # node build → nginx runtime
├── nginx.conf                      # SPA fallback + /api proxy to gateway
├── index.html
├── package.json
├── tsconfig*.json
└── vite.config.ts
```

**Rule of thumb:** `components/ui` knows nothing about todos. `features/todos`
knows nothing about routing. `pages` glue features together for a URL.

## 6. Public API (through the gateway, base `/api/v1`)

| Method | Path | Auth | Service | Description |
|--------|------|------|---------|-------------|
| POST | `/auth/register` | no | auth | `{email, name, password}` → `201 {user}` |
| POST | `/auth/login` | no | auth | `{email, password}` → `{access_token, token_type, expires_at, user}` |
| GET | `/auth/me` | yes | auth | Current user |
| GET | `/todos` | yes | todo | Query: `status`, `priority`, `q`, `page`, `page_size` → `{items, page, page_size, total}` |
| POST | `/todos` | yes | todo | `{title, description?, priority?, due_date?}` → `201 {todo}` |
| GET | `/todos/:id` | yes | todo | One todo |
| PATCH | `/todos/:id` | yes | todo | Partial update: any of `title, description, completed, priority, due_date, clear_due_date` |
| DELETE | `/todos/:id` | yes | todo | `204 No Content` |
| GET | `/activity` | yes | activity | Query: `limit` → `{items}` |

Every service also has `GET /healthz` (liveness) and, from B9, `GET /readyz` and `GET /metrics`.

**Error format (every service, every error):**

```json
{ "error": { "code": "not_found", "message": "todo not found" } }
```

## 7. Tech stack and why

| Tech | Why |
|------|-----|
| Go 1.27 | Fast, simple, single static binary, great concurrency, the standard for cloud services. |
| Gin | The most used Go web framework: fast radix-tree router, middleware, JSON binding + validation. |
| `log/slog` | Structured JSON logging from the **standard library**. No extra dependency. |
| PostgreSQL 16 + `pgx/v5` | Reliable relational DB; `pgx` is the fastest, most complete Postgres driver for Go. |
| `golang-migrate` | Versioned SQL migrations, used both as CLI and in Docker. |
| `golang-jwt/jwt/v5` + `bcrypt` | JWT signing/verification and safe password hashing. |
| NATS JetStream | Lightweight message broker with durable streams. Much simpler to run than Kafka; same concepts. |
| Prometheus client | Industry-standard metrics. |
| Docker + Compose | Reproducible local environment for the whole system. |
| React 19 + Vite + TypeScript | Modern, fast dev server, type safety. |
| TanStack Query | Server-state caching, retries, invalidation. |
| React Router | Client-side routing. |
| Tailwind CSS v4 | Styling without writing lots of CSS files. |
| Vitest + Testing Library + MSW | Fast tests that use the app the way a user would, with a mocked network. |

## 8. Deliberately left for "later" (good next steps after B9)

- gRPC between services (instead of HTTP for internal calls)
- Refresh tokens + RS256 keys with a JWKS endpoint
- Transactional **outbox pattern** (guaranteed event publishing)
- OpenTelemetry distributed tracing
- Kubernetes manifests / Helm chart
