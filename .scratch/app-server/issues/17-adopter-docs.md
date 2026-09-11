# 17 — Adopter docs & GraphQL manifest

Status: ready-for-agent
Blocked by: 05-redis-caching, 06-nats-events-activity-projector, 13-api-tokens-sso

Parent: `.scratch/app-server/PRD.md`

## What to build

The adopter-facing payoff of the example. Make the generated GraphQL manifest and schema documentation available so an adopter can understand the API surface, and provide a short README that walks the blessed path — how the generated client, tenancy, cache, events, hooks, and observability are wired — so an adopter can copy it into their own project. Depends on the cross-cutting wiring being in place so the walkthrough describes the real, finished path.

## Acceptance criteria

- [x] The generated GraphQL manifest and schema documentation are surfaced/available to adopters (story 60)
- [x] A README walks the blessed path: generated client construction, tenancy, cache, events, hooks, and observability wiring (story 59)
- [x] The walkthrough points at the idiomatic, unedited-generated-code boundary (hand-written code lives only in `cmd/` and non-generated `internal/` packages)

## Blocked by

- 05-redis-caching
- 06-nats-events-activity-projector
- 13-api-tokens-sso
