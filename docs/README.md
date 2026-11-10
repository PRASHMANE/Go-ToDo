# Go-ToDo — Learning Go & Gin by Building a Microservice Todo App

This folder is the **complete plan** for the project. Read the docs in order,
build one phase at a time, and make **one git commit per finished phase**.

> **Honest note:** a todo app does not *need* microservices. A single Go binary
> would be simpler. We use microservices **on purpose** because the goal is to
> learn how production Go systems are built: service boundaries, an API
> gateway, JWT auth, database-per-service, events, Docker, tests and CI.
> The business logic is kept small so all your attention goes to the engineering.

---

## 1. Read these first (concepts)

| # | Doc | What you learn |
|---|-----|----------------|
| 00 | [Architecture & patterns](00-architecture.md) | What we build, the architecture, every design pattern, full folder structure and what each folder means |
| 01 | [Setup & installation](01-setup-installation.md) | Every tool to install on macOS, with commands to verify each one |
| 02 | [Git workflow](02-git-workflow.md) | Branch per phase, commit message per phase, how to keep only *your* name on commits |
| 03 | [How Go & Gin work](03-how-go-and-gin-work.md) | `net/http` from scratch, then how Gin is built on top of it (router, context, middleware chain, binding) |

**Deploying to a server:** [AWS or GCP VM, from scratch (VPC, firewall, nginx, supervisord)](deploy-aws-gcp-vm.md) · [Hostinger VPS](deploy-hostinger-supervisor.md)

## 2. Build order (follow this exactly)

Backend and frontend phases are interleaved so the frontend always has a
real API to talk to.

| Step | Phase | Doc | Result when done |
|------|-------|-----|------------------|
| 1 | Backend 0 | [Foundations & repo setup](backend/phase-0-foundations-and-repo-setup.md) | Go basics understood, monorepo skeleton, `go.work` |
| 2 | Backend 1 | [Shared `pkg` & service skeleton](backend/phase-1-shared-pkg-and-service-skeleton.md) | `todo-service` answers `GET /healthz`, logs JSON, shuts down gracefully |
| 3 | Backend 2 | [Todo CRUD (in-memory)](backend/phase-2-todo-service-crud.md) | Full REST CRUD with clean architecture layers |
| 4 | Frontend 0 | [React setup](frontend/phase-0-setup.md) | Vite + React + TS + Tailwind running |
| 5 | Frontend 1 | [UI with local state](frontend/phase-1-ui-with-local-state.md) | Todo UI working with fake data |
| 6 | Backend 3 | [PostgreSQL & migrations](backend/phase-3-postgres-and-migrations.md) | Todos stored in Postgres, filters + pagination |
| 7 | Frontend 2 | [Connect to the API](frontend/phase-2-connect-api.md) | UI reads/writes real todos via TanStack Query |
| 8 | Backend 4 | [Auth service](backend/phase-4-auth-service.md) | Register / login / JWT / bcrypt |
| 9 | Backend 5 | [API gateway](backend/phase-5-api-gateway.md) | Single entry point: routing, JWT check, CORS, rate limit; todos are per-user |
| 10 | Frontend 3 | [Authentication](frontend/phase-3-authentication.md) | Login/register pages, protected routes, token handling |
| 11 | Backend 6 | [Dockerize everything](backend/phase-6-docker.md) | `docker compose up` runs the whole backend |
| 12 | Frontend 4 | [Advanced todo features](frontend/phase-4-advanced-todo-features.md) | Edit, filters in URL, search, pagination, optimistic updates |
| 13 | Frontend 5 | [Docker + Nginx](frontend/phase-5-docker-nginx.md) | Frontend served by Nginx in compose |
| 14 | Backend 7 | [Testing](backend/phase-7-testing.md) | Unit, handler and integration tests, linting |
| 15 | Backend 8 | [Events & activity service](backend/phase-8-events-activity-service.md) | NATS JetStream events, a 4th service consuming them |
| 16 | Frontend 6 | [Testing, activity feed & polish](frontend/phase-6-testing-activity-polish.md) | Vitest + Testing Library, activity feed, error boundary |
| 17 | Backend 9 | [Observability & CI](backend/phase-9-observability-and-ci.md) | Prometheus metrics, readiness checks, GitHub Actions CI |

## 3. "Definition of done" for every phase

A phase is finished only when **all** of these are true:

1. The "Verify" section at the end of the phase doc passes.
2. `go vet ./...` (backend) or `npm run lint` (frontend) shows no errors.
3. You can explain, in your own words, every file you created in that phase.
4. You made the phase commit (see [02-git-workflow.md](02-git-workflow.md)).

## 4. How to study each phase

1. Read the **"Concepts"** section first. Don't skip it.
2. Type the code yourself. Don't copy-paste. Typing is how Go syntax sticks.
3. After each file, run `go build ./...` and fix errors yourself before
   looking back at the doc.
4. Do the **"Exercises"** at the end. They are where you actually learn.

---

## 5. "As built" notes (read this!)

All 17 phases have been **implemented, tested and committed** (tags `b0` … `b9`,
`f0` … `f6`). The code in the repo is the source of truth. Where building
it for real taught us something, the code differs from the first draft of these
docs, as listed below:

| Topic | What the code actually does | Why |
|-------|-----------------------------|-----|
| Postgres host port | `15432` (container still listens on 5432) | This Mac already runs Postgres on 5432 *and* something on 5433. |
| Postgres image | `postgres:16-alpine` | Was already available locally; 16 vs 17 makes no difference here. |
| Go version | Go **1.27.3**, `golang:1.27-alpine` builder | The installed toolchain. `go.mod` says `go 1.27.3`. |
| `make tidy` | runs `GOWORK=off go mod tidy` per module | With `go.work` on, a missing `go.sum` entry is invisible on your Mac but **breaks the Docker build** (`missing go.sum entry ... golang-jwt`). Tidying with the workspace off reproduces Docker exactly. CI also runs with `GOWORK=off`. |
| Validation errors | `response.BindError` → `"title is required"` | Raw validator messages aren't fit for a UI. |
| Search | `%` and `_` are escaped in `ILIKE` | So searching `100%` is literal (B3 exercise 2, done). |
| Timestamps | Postgres repos convert to UTC on read | pgx returns the machine's local zone (`+05:30`); the API always speaks UTC. |
| Event publishing | uses `context.WithoutCancel` | An event is still published if the client disconnects right after the write. |
| Invalid events | consumer **Terms** events with missing IDs (`service.ErrInvalidEvent`) | Retrying can never fix them; only temporary errors (DB down) are Nak'd for retry. |
| `/healthz` `/readyz` `/metrics` | one shared package: `pkg/health.Register(r, deps)` | Same code in all 4 services; readiness checks Postgres **and** NATS. |
| Gateway tests | run behind `httptest.NewServer` | `httputil.ReverseProxy` needs `CloseNotify`, which `httptest.ResponseRecorder` lacks. |
| Frontend lint | oxlint (not ESLint), `useNow()` hook for time | Current Vite template. |
| Test libraries | MSW **3** (`onUnhandledFrame`), Vitest 5, react-router 8 | Latest versions at build time. |
| Healthchecks | `127.0.0.1`, never `localhost`, inside alpine containers | `localhost` resolves to IPv6 `::1`; nginx/NATS listen on IPv4 → false "unhealthy". |
| NATS CLI | `nats --server nats://127.0.0.1:4222 ...` | The bare `localhost:4222` form didn't connect. |
| Image sizes | Go services 27–41 MB, frontend ~93 MB | Real numbers, not estimates. |

### What is verified

- **Backend:** `make test` (race detector), `make test-integration` (real Postgres) and `make lint` (0 issues, 5 modules) all pass.
- **Frontend:** `npm run test:run` (23 tests), `npm run lint`, `npm run typecheck` and `npm run build` all pass.
- **Full stack:** `docker compose up --build` gives 7 healthy containers. Each phase's "Verify" section was run end to end against it.

### Not done (intentionally)

The **Exercises** at the end of each phase are left for you. They're how
you learn. Two were done because they fixed real bugs (LIKE escaping) or
security gaps (gateway header-spoofing tests).
