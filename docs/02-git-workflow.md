# 02 — Git Workflow & Commit per Phase

## Rules

1. **One branch per phase**, merged into `main` when the phase is done.
2. **One main commit per phase** using the message from the table below.
   Small "wip" commits on the branch are fine; squash them when merging.
3. Commits are authored **only by you**: `prashanth v <prashanthvprashanthv46@gmail.com>`.
   **No `Co-Authored-By` lines**, no tool attribution.
4. Never commit secrets. `.env` is git-ignored; only `.env.example` is committed.
5. Tag each finished phase (`b2`, `f1`, ...), so you can always `git checkout b2`
   to see exactly how the code looked at that point.

## Before every commit: check who you are

```bash
git config user.name     # → prashanth v
git config user.email    # → prashanthvprashanthv46@gmail.com
```

## The phase loop (copy this every time)

```bash
# 1. start
git checkout main && git pull
git checkout -b phase/b2-todo-crud

# 2. work ... commit as often as you like
git add -A
git commit -m "wip: handler skeleton"

# 3. finish: squash-merge into main with the phase message
git checkout main
git merge --squash phase/b2-todo-crud
git commit -m "feat(todo-service): in-memory CRUD with clean architecture layers (B2)"
git tag b2
git push origin main --tags
git branch -D phase/b2-todo-crud
```

Check the author of the last commit:

```bash
git log -1 --format='%an <%ae>%n%n%B'
```

## Commit message format (Conventional Commits)

```
<type>(<scope>): <short summary in present tense>

<optional body: WHY you did it, not what — the diff shows what>
```

Types: `feat` (new feature), `fix`, `refactor`, `test`, `docs`, `build`
(Docker, deps), `ci`, `chore`.

## Commit message for each phase

| Order | Tag | Branch | Commit message |
|-------|-----|--------|----------------|
| — | — | `main` | `docs: add project plan, architecture and phase guides` |
| 1 | `b0` | `phase/b0-repo-setup` | `chore: initialise monorepo layout, go workspace and tooling (B0)` |
| 2 | `b1` | `phase/b1-shared-pkg` | `feat(pkg): shared config, logger, middleware and graceful http server (B1)` |
| 3 | `b2` | `phase/b2-todo-crud` | `feat(todo-service): in-memory CRUD with clean architecture layers (B2)` |
| 4 | `f0` | `phase/f0-frontend-setup` | `chore(frontend): scaffold Vite React TypeScript app with Tailwind (F0)` |
| 5 | `f1` | `phase/f1-ui-local-state` | `feat(frontend): todo UI components with local state (F1)` |
| 6 | `b3` | `phase/b3-postgres` | `feat(todo-service): PostgreSQL repository, migrations, filters and pagination (B3)` |
| 7 | `f2` | `phase/f2-connect-api` | `feat(frontend): connect todo UI to API with TanStack Query (F2)` |
| 8 | `b4` | `phase/b4-auth-service` | `feat(auth-service): user registration, login and JWT issuing (B4)` |
| 9 | `b5` | `phase/b5-api-gateway` | `feat(api-gateway): reverse proxy with JWT auth, CORS and rate limiting (B5)` |
| 10 | `f3` | `phase/f3-auth` | `feat(frontend): login, register and protected routes (F3)` |
| 11 | `b6` | `phase/b6-docker` | `build: multi-stage Dockerfiles and docker compose for all services (B6)` |
| 12 | `f4` | `phase/f4-advanced-todos` | `feat(frontend): editing, URL filters, search, pagination and optimistic updates (F4)` |
| 13 | `f5` | `phase/f5-frontend-docker` | `build(frontend): nginx image and compose integration (F5)` |
| 14 | `b7` | `phase/b7-testing` | `test: unit, handler and integration tests with linting (B7)` |
| 15 | `b8` | `phase/b8-events` | `feat: todo events over NATS JetStream and activity-service (B8)` |
| 16 | `f6` | `phase/f6-testing-polish` | `feat(frontend): activity feed, error boundary and component tests (F6)` |
| 17 | `b9` | `phase/b9-observability-ci` | `feat: prometheus metrics, readiness checks and GitHub Actions CI (B9)` |

## Useful commands

```bash
git status                       # what changed
git diff                         # unstaged changes
git diff --staged                # what will be committed
git log --oneline --graph --all  # history
git show b3                      # what phase B3 changed
git checkout b3                  # look at the code as of B3 (read-only; `git checkout main` to return)
git restore <file>               # throw away local changes to a file
```

## Fixing a commit made with the wrong identity

If it is the **last** commit and **not pushed** yet:

```bash
git commit --amend --reset-author --no-edit
```

If it is already pushed, amend as above and then
`git push --force-with-lease` (only on branches nobody else uses).
