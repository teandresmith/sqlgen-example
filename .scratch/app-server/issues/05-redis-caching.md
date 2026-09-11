# 05 — Redis caching

Status: ready-for-human
Blocked by: 03-workspaces-membership-tenant-scoping, 07-projects-tasks-core

Parent: `.scratch/app-server/PRD.md`

## What to build

A Redis-backed `cache.Backend` adapter attached to the client via `WithCache` (JSON serializer, key prefix `taskr`, per `sqlgen.yml`). Single-entity `Get` reads are served from cache; a mutation invalidates the cached entry so a subsequent read is never stale. A request can bypass the cache via a header to force a fresh read. Cache behavior is exercised against a real Task (from ticket 07), so hits/invalidation are observable on a rich tenant-scoped entity.

## Acceptance criteria

- [x] A Redis `cache.Backend` is attached via `WithCache` using the configured serializer and key prefix
- [x] A repeated single-entity `Get` is served from cache (story 46)
- [x] A mutation to an entity invalidates its cache entry, so the next read reflects the change (story 46)
- [x] A cache-bypass request header forces a fresh read (story 47)
- [x] Integration test drives the running server against a real Redis (testcontainer) and asserts hit then invalidation on a Task

## Notes

- `internal/cache` builds the Redis-backed `cache.Backend` (`cache/redis`, `WithOwnedClient`) and the generated `*database.Cache` (serializer JSON, prefix `taskr` baked in from `sqlgen.yml`), attached via `WithCache` in both `cmd/server` and the test harness — one construction path so caching is identical under test and in production.
- Wiring the cache exposed a latent stale-cache bug: `acceptInvitation` updated the tenanted `invitations` table under `SkipTenancy` (unresolved tenant), so the cache could not build the per-tenant key to invalidate and the sender kept reading a stale `PENDING`. Fixed by running the accept writes tenant-resolved under a new, unforgeable `authz.WithSystemGrant` context marker (which replaces `SkipTenancy` as the trust signal for guarded-table bootstrap writes without giving up tenant resolution). Guarded by `TestAcceptInvitationInvalidatesCachedInvitation`.
- Note: cache invalidation for mutations run inside a transaction (e.g. `acceptInvitation`) is deferred to an async post-commit callback and is therefore eventually consistent; single mutations outside a transaction invalidate synchronously.
- Adding the `cache/redis` module dependency transitively bumped `testcontainers-go` 0.41→0.42 (and its indirect deps); validated end-to-end by the green integration suite.

## Blocked by

- 03-workspaces-membership-tenant-scoping
- 07-projects-tasks-core
