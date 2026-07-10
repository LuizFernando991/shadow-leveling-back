# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, opencode, …) working in this repository.

## What this is

**Shadow Leveling — backend.** A Go REST API for a gamified workout tracker themed after Solo Leveling. Go module: `github.com/LuizFernando991/gym-api`. The Expo/React Native client is a separate repo (`shadow-leveling-front`).

Reference docs in this repo: `API_GUIDE.md` (API contract), `internal/infra/http/docs/openapi.yaml` (OpenAPI), `PRD.md` (product spec), `SCREENS_GUIDE.md`. XP/leveling code comments cite PRD sections (e.g. `PRD §4.4`).

## Stack

Go 1.25, `gorilla/mux` router, `pgx` (via `database/sql`), PostgreSQL, Redis, Prometheus metrics, bearer-token auth.

## Commands

All commands run via the Makefile, which sources `.env`.

```bash
make infra/up          # start postgres + redis (docker)
make run               # build to bin/gym and run the API
make build             # build only
make test/unit         # unit tests (excludes integration_tests)
make test/integration  # integration tests (needs a running DB + .env.test)
make test              # both
go test ./internal/features/leveling/...                  # a single package
go test -run TestXPForLevel ./internal/features/leveling/ # a single test
make migrate/install   # install golang-migrate CLI (one-time)
make migrate/new name=create_foo
make migrate/up        # migrate/up N=1, migrate/down, migrate/status also exist
```

Config comes from env (`.env`, see `.env.example`) via `internal/config`. Auth token TTL from `AUTH_TOKEN_TTL`.

## Architecture

`cmd/api/main.go` is the composition root: it loads config, connects the DB, constructs each feature **Module**, and hands them to `router.NewRouter`. Modules are wired explicitly — e.g. the `workout` module receives `leveling`'s `Awarder()` so completing a workout grants XP. There is no DI framework; dependencies are passed by hand in `main.go`.

Each feature under `internal/features/<name>/` is a self-contained vertical slice with the same shape:
- `module.go` — builds `repository → service → handler`, exposes `RegisterRoutes` (and `Middleware()` for auth).
- `handler.go` — HTTP handlers, route registration.
- `service.go` — business logic (unit-tested; `service_test.go`).
- `repository.go` — SQL access.
- `entity.go` — domain types (pure logic like leveling math is unit-tested in `entity_test.go`).
- `dto.go` — request/response shapes.

Current features: `auth`, `task`, `usermetrics`, `workout`, `leveling`.

Cross-cutting code:
- `internal/infra/` — HTTP `server`, `router`, `middleware` (CORS, JSON, logger, metrics), `docs` (serves OpenAPI), `cache` (Redis rate limiter), `email` (sender + templates).
- `internal/shared/` — auth middleware, `httputil` (responses + request context), `validate`, shared entities.
- `internal/database/` — connection + `migrations/` (numbered `.up.sql`/`.down.sql`).

**Adding a feature:** create `internal/features/<name>/` following the slice shape above, add its `Module` to `router.Modules` and construct it in `main.go`, then register routes in `router.registerRoutes` (pass `modules.Auth.Middleware()` for protected routes).

Integration tests (`internal/testutil/setup.go`) reset the schema, run all migrations, and spin up a full `httptest` server against a real DB using a NoopSender — tests read email verification codes straight from the DB. They require `.env.test` and a running postgres.
