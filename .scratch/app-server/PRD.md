# Spec: Task-tracker application code (GraphQL server on the generated client)

Status: ready-for-agent

_Ubiquitous language follows `CONTEXT.md`; architecture follows `docs/adr/0001–0005`._

## Problem Statement

As a developer evaluating or adopting sqlgen, I can see the generated
`internal/database` client compiles, but I have no reference for how the generated
pieces compose into a real, runnable, multi-tenant GraphQL API — how auth,
tenant scoping, caching, events, hooks, and observability are wired the blessed
way, and how it's all tested against real infrastructure. The generated code is
only half the story; without a worked example, every adopter re-derives the
integration from scratch and the project has no living, end-to-end proof that
the generated surface behaves correctly in a realistic application.

## Solution

Build the full application layer on top of the generated client: a GraphQL
server for a multi-tenant SaaS task tracker, wired end to end — JWT auth with
per-request Active Workspace selection, structural tenant scoping, Redis cache,
NATS-backed events feeding an Activity projector, logging/authz hooks, OpenTelemetry
metrics, a data seeder, and configuration — all runnable via `docker compose` and
covered by full-stack integration tests. The application doubles as the project's
adopter-facing reference and its live integration test bed.

## User Stories

### Identity, workspaces, and access

1. As a person, I want to register with an email and password, so that I have a global User identity.
2. As a User, I want to log in and receive a JWT, so that I can authenticate subsequent requests.
3. As a User, I want my JWT to identify me but not bind me to one Workspace, so that I can belong to many Workspaces.
4. As a User, I want to create a Workspace, so that I have an isolated space to organize work; I become its `owner` via a Membership.
5. As a Workspace owner, I want to invite a person by email with a Role, so that they can join my Workspace.
6. As an invited person, I want to accept an Invitation, so that a Membership is created and the Invitation becomes `accepted`.
7. As a Workspace owner, I want to revoke a pending Invitation, so that it can no longer be accepted.
8. As a User, I want to select my Active Workspace per request via the `X-Workspace-Id` header, so that I can switch Workspaces without re-authenticating.
9. As a User, I want a request whose `X-Workspace-Id` I am not a Member of to be rejected, so that I cannot reach another tenant's data.
10. As a User, I want a request with no valid Active Workspace to be rejected for tenant-scoped operations, so that tenant isolation is fail-closed.
11. As a Workspace admin, I want to manage Memberships and Roles, so that I can control who has access and at what level.
12. As a Workspace owner, I want to configure an SSO Connection (provider + config), so that my organization can use single sign-on.
13. As a User, I want to create an API Token with scopes, so that a service or script can call the API on my behalf.
14. As a User, I want to create a service (userless) API Token, so that automation can authenticate without a personal account.
15. As a User, I want to revoke an API Token, so that a leaked credential can be disabled.

### Teams and structure

16. As a Workspace admin, I want to create Teams within a Workspace, so that I can group Users.
17. As a Workspace admin, I want to add and remove Users on a Team, so that Team membership reflects reality.
18. As a Workspace member, I want to assign a Project to a Team, so that ownership is clear.

### Projects, tasks, and collaboration

19. As a Workspace member, I want to create a Project, so that I can organize related Tasks.
20. As a Workspace member, I want to archive a Project (soft-delete), so that it disappears from active views without losing history.
21. As a Workspace member, I want to create a Task in a Project with a status and priority, so that work is tracked.
22. As a Workspace member, I want to create a Subtask under a Task, so that I can break work down.
23. As a Workspace member, I want to declare that a Task depends on another Task, so that ordering constraints are explicit.
24. As a Workspace member, I want to see a Task's dependencies and dependents, so that I understand what blocks it and what it blocks.
25. As a Workspace member, I want to assign multiple Users to a Task, so that shared work has clear owners.
26. As a Workspace member, I want to watch a Task, so that I follow updates without being an Assignee.
27. As a Workspace member, I want to apply Labels to a Task, so that I can categorize and filter work.
28. As a Workspace admin, I want to create and manage Labels for the Workspace, so that categorization is consistent.
29. As a Workspace member, I want to comment on a Task, so that discussion stays with the work.
30. As a Workspace member, I want to set a Task's Reporter to the creating User, so that provenance is recorded.
31. As a Workspace member, I want to schedule a Task into a Cycle, so that it is planned into an iteration.
32. As a Workspace admin, I want to create Cycles with a date range and status, so that iterations are defined.
33. As a Workspace member, I want to attach a file to a Task or a Comment, so that supporting material lives with the work.
34. As a Workspace member, I want to log Time Entries against a Task in decimal hours, so that effort is captured.

### Querying, filtering, pagination

35. As a Workspace member, I want to fetch a single entity by id, so that I can view its details.
36. As a Workspace member, I want to filter Tasks by status, priority, assignee, label, and other fields, so that I can find relevant work.
37. As a Workspace member, I want to combine filter conditions with and/or, so that I can express precise queries.
38. As a Workspace member, I want to sort Task lists, so that I can order by relevance.
39. As a Workspace member, I want cursor-paginated Task connections, so that I can page through large lists stably.
40. As a Workspace member, I want offset-paginated lists where a total count is useful, so that I can show page counts.
41. As a Workspace member, I want to load a Task with its Comments, Assignees, Labels, and Subtasks in one query, so that I avoid N round-trips.
42. As a Workspace member, I want per-project task statistics via the Project Stats view, so that I can see progress without computing it client-side.
43. As a Workspace member, I want every list and read I perform to be automatically scoped to my Active Workspace, so that I never see another tenant's rows.

### Activity, cache, events (observable behavior)

44. As a Workspace member, I want an Activity feed of mutations in my Workspace, so that I can see what changed and who changed it.
45. As a Workspace member, I want the Activity feed itself to not generate further Activity, so that the feed does not recurse.
46. As a Workspace member, I want repeated reads of the same entity to be served from cache, so that the API is fast; and I want a mutation to invalidate that cache so I never read stale data.
47. As an operator, I want a request to be able to bypass cache or events via headers, so that I can force fresh reads or suppress side effects during bulk operations.

### Errors and safety

48. As a Workspace member, I want a not-found entity to return a clear GraphQL error, so that clients can handle it.
49. As a Workspace member, I want a unique-constraint violation (e.g. duplicate Label name, duplicate slug) to return a typed, meaningful GraphQL error, so that I can correct the input.
50. As a Workspace member, I want an operation forbidden by my Role to be rejected by an authorization hook, so that permissions are enforced server-side.
51. As a Workspace member, I want GraphQL query depth and complexity limited, so that expensive nested queries are rejected.

### Operations and developer experience

52. As an operator, I want `docker compose up` to start Postgres, Redis, NATS, and an OpenTelemetry collector, so that I can run the app locally with one command.
53. As an operator, I want database migrations applied from the same `./migrations` files sqlgen parses, so that the schema and generated code never drift.
54. As an operator, I want a seed command that populates a realistic multi-workspace dataset, so that I can explore the API immediately.
55. As an operator, I want the server configured entirely via environment variables, so that it follows twelve-factor conventions.
56. As an operator, I want OpenTelemetry metrics exported to the collector, so that I can observe request and query behavior.
57. As an operator, I want a health/readiness check, so that orchestration can tell when the server is live.
58. As an operator, I want graceful shutdown, so that in-flight requests complete and connections close cleanly.
59. As an adopter, I want the wiring of the generated client, cache, events, tenancy, and hooks to be idiomatic and readable, so that I can copy the blessed path into my own project.
60. As an adopter, I want the generated GraphQL manifest and schema documentation available, so that I can understand the API surface.

## Implementation Decisions

- **Module layout.** Standard Go layout: `cmd/server` (API entrypoint) and `cmd/seed` (data seeder); `internal/{database(generated), auth, tenancy, cache, events, hooks, observability, config}`. The generated client and graph package live under `internal/database` and are never hand-edited.
- **API surface.** GraphQL only (per ADR-0003). The server bootstraps the gqlgen handler with the generated `graph.Resolver{Client, Q, M}` initialized against a single `*database.Client`, wrapped by the generated `WithCallOptionsMiddleware` so per-request HTTP → `CallOptions` mapping works.
- **Client construction.** One `*database.Client` built on a pgx pool via `database.New(dbpgx.New(pool), …opts)`, sharing the same instance across resolvers, the Activity projector, and the seeder. Cache, event publisher, hooks, and tenant resolver are attached as client options at construction.
- **Authentication.** A dev `login` mutation verifies email + password hash and issues a JWT carrying the User identity only (per ADR-0004). A stdlib `net/http` middleware validates the bearer JWT and reads the `X-Workspace-Id` header.
- **Active Workspace / tenancy.** The middleware resolves the Active Workspace by validating the caller's Membership in the requested Workspace, then places User + Active Workspace into the request context. The generated `TenantResolver` reads the Active Workspace from context and feeds sqlgen's structural tenant scoping (`workspace_id`, fail-closed). `users` and `workspaces` are shared (untenanted) so login and membership validation work before a tenant is established.
- **Authorization.** Enforced in a mutation/query hook keyed on the caller's Role for the Active Workspace (e.g. only `owner`/`admin` may manage Memberships, Labels, SSO Connections). Read/write authz beyond tenant scoping is a hook concern, not schema.
- **Caching.** Redis-backed `cache.Backend` adapter attached via `WithCache`; single-entity `Get` reads are cached and invalidated on mutation. Serializer JSON, key prefix `taskr`, per ADR/`sqlgen.yml`.
- **Events and the Activity projector.** Mutations publish events via a NATS `event.Publisher` attached with `WithEventPublisher`. A subscriber (the Activity projector) consumes them and writes rows to the `activity` table via the same client; `activity` has events disabled to prevent recursion. The projector records actor (from event metadata / context), entity table + id, action, and a payload snapshot.
- **Error mapping.** The generated `mapErrorToGQL` translates sentinel/`ConstraintError` values to GraphQL errors; unique/foreign-key/check violations surface as meaningful, typed messages.
- **Observability.** OpenTelemetry `MetricsRecorder` from `metrics/otel` attached to the client and HTTP server; metrics exported to a collector defined in `docker-compose`.
- **Migrations = single source of truth.** `golang-migrate` applies `./migrations/*.up.sql` (per ADR-0002); the same files feed sqlgen. Applied at server startup (or via an explicit migrate step) before serving.
- **Configuration.** Environment variables only (DB/Redis/NATS URLs, JWT secret, OTel endpoint, listen address). A small `config` package parses and validates them at boot.
- **Local dependencies.** `replace` directives point the runtime, `cache/redis`, `event/natsbus`, and `metrics/otel` modules at the local `../sqlgen` tree (per ADR); the CLI is built from `../sqlgen/cmd/sqlgen`.
- **Infrastructure.** `docker-compose.yml` provides Postgres, Redis, NATS, and an OpenTelemetry collector. A `Makefile` provides targets to build the local sqlgen CLI, run migrations, `sqlgen generate`, seed, and test.
- **Relationship naming.** The generated client exposes `Assignees`, `Watchers`, `Subtasks` (renamed for clarity) and the auto-detected self-referential `DependsOnTasks` / `Tasks`; the application uses these names as generated.

## Testing Decisions

- **What makes a good test here:** it exercises externally observable behavior through the public API — GraphQL responses and the resulting persisted/observable state (DB rows, Activity entries, cross-tenant isolation, cache and error behavior) — never generated internals or private wiring. Tests should read like a client of the running service.
- **Single seam — the GraphQL HTTP endpoint.** Tests drive the fully-wired server via `httptest` by POSTing GraphQL operations, against **real Postgres + Redis + NATS** provisioned by testcontainers, with container startup amortized in a shared `TestMain` (per ADR-0005). This one seam covers auth, tenant scoping, resolvers, the generated client, hooks, cache, and events together.
- **Auth is exercised, not bypassed:** a test helper mints JWTs through the same signing code the server uses and sets `X-Workspace-Id`, so authentication and Active-Workspace resolution run for real.
- **Async Activity projector:** asserted by polling the Activity query through the HTTP seam with a bounded timeout (eventual consistency) — no separate projector seam.
- **Coverage focus:** tenant isolation (a User in Workspace A cannot read/mutate Workspace B), the full mutation lifecycle for the core entities, relationship loading in a single query, cursor and offset pagination, filter and/or composition, constraint-error mapping, cache hit/invalidation behavior, and Activity-feed population after mutations.
- **Prior art:** sqlgen's own `cmd/sqlgen/testdata/examples/graphql/tests` (Postgres testcontainer + live gqlgen handler over httptest) is the template to mirror for harness shape and assertions.

## Out of Scope

- REST and gRPC surfaces, and GraphQL subscriptions (GraphQL request/response only; not yet generated by sqlgen).
- A frontend or UI of any kind.
- Real identity-provider integration: SSO Connections store configuration but the SAML/OIDC handshake and real email delivery for Invitations are not implemented (dev flows only).
- Production deployment concerns (Kubernetes, TLS termination, secrets management, autoscaling).
- Billing / subscriptions / metering (time is tracked but not invoiced).
- Enforcement of API Token scopes and rate-limit buckets beyond what the schema records.
- Organization → Workspace hierarchy (tenancy is flat per ADR-0004); lock modes (not available in sqlgen).

## Further Notes

- This application doubles as sqlgen's live integration test bed. During schema/codegen bring-up it already surfaced and drove fixes for four sqlgen generator bugs (self-referential M2M naming, view PK filter comparator, bare-uuid comparator across the GraphQL boundary, and JSONB input dereference); the running application tests are the ongoing guard against regressions of that kind.
- gqlgen is pinned via the module graph (currently v0.17.94); the generated resolver flow depends on gqlgen's preserve-existing-bodies semantics, so a version bump should be validated end to end.
- The generated code is a product: it must remain unedited and regenerated via `sqlgen generate` (which chains the gqlgen subprocess). Hand-written code lives only in `cmd/` and the non-generated `internal/` packages.
