# 14 — Error mapping & query limits

Status: ready-for-agent
Blocked by: 07-projects-tasks-core

Parent: `.scratch/app-server/PRD.md`

## What to build

Predictable errors and query safety. A not-found entity returns a clear GraphQL error clients can handle. A unique/foreign-key/check-constraint violation (e.g. duplicate Label name, duplicate slug) surfaces as a typed, meaningful GraphQL error via the generated `mapErrorToGQL`. GraphQL query depth and complexity are limited so expensive nested queries are rejected.

## Acceptance criteria

- [ ] A not-found entity returns a clear GraphQL error (story 48)
- [ ] A unique-constraint violation returns a typed, meaningful GraphQL error via `mapErrorToGQL` (story 49)
- [ ] Foreign-key and check-constraint violations also surface as meaningful typed errors
- [ ] Query depth is limited and over-deep queries are rejected (story 51)
- [ ] Query complexity is limited and over-complex queries are rejected (story 51)
- [ ] Integration tests assert each error and limit through the HTTP seam

## Blocked by

- 07-projects-tasks-core
