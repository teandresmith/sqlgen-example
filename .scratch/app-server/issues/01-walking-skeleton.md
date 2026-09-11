# 01 — Walking skeleton & test harness

Status: done
Blocked by: None — can start immediately

Parent: `.scratch/app-server/PRD.md`

## What to build

A runnable, wired-but-featureless server: `docker compose up` starts Postgres, Redis, NATS, and an OpenTelemetry collector; the server reads its configuration from environment variables, applies the `./migrations/*.up.sql` files at startup, builds a single `*database.Client` on a pgx pool, and serves the generated gqlgen handler with the generated `Resolver{Client, Q, M}` wrapped by `WithCallOptionsMiddleware`. A health/readiness endpoint reports live. Alongside it, stand up the full-stack integration harness (per ADR-0005): a shared `TestMain` that boots Postgres + Redis + NATS via testcontainers and a helper that POSTs GraphQL operations to the wired server over `httptest`.

This is the tracer bullet that establishes the whole vertical — infra → config → migrations → client → GraphQL handler → tests — before any feature rides on it.

## Acceptance criteria

- [ ] `docker compose up` brings up Postgres, Redis, NATS, and an OTel collector
- [ ] Configuration (DB/Redis/NATS URLs, JWT secret, OTel endpoint, listen address) is read and validated from environment variables at boot
- [ ] Migrations are applied from `./migrations` at startup (or an explicit migrate step) before serving
- [ ] One `*database.Client` is constructed on a pgx pool via `database.New(dbpgx.New(pool), …)` and shared into the resolver
- [ ] The gqlgen handler serves with the generated `Resolver{Client, Q, M}` and `WithCallOptionsMiddleware` installed
- [ ] A health/readiness endpoint returns a live status
- [ ] A shared `TestMain` boots Postgres + Redis + NATS via testcontainers with startup amortized across the package
- [ ] A GraphQL-POST test helper exists and a smoke integration test (schema introspection or an untenanted `users` read) passes against the wired server

## Blocked by

- None — can start immediately
