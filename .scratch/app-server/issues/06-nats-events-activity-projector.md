# 06 — NATS events & Activity projector

Status: done
Blocked by: 03-workspaces-membership-tenant-scoping, 07-projects-tasks-core

Parent: `.scratch/app-server/PRD.md`

## What to build

Domain events and the Activity feed they drive. Mutations publish events via a NATS `event.Publisher` attached with `WithEventPublisher`. A subscriber — the Activity projector — consumes them and writes rows to the `activity` table via the same client, recording actor (from event metadata / context), entity table + id, action, and a payload snapshot. The `activity` table has events disabled so the feed does not recurse. A request can suppress events via a header (e.g. for bulk operations). An Activity feed query exposes the entries; because projection is asynchronous, tests poll the feed through the HTTP seam with a bounded timeout.

## Acceptance criteria

- [x] Mutations publish events via a NATS `event.Publisher` attached with `WithEventPublisher` — the bus is built in `internal/events` (`natsbus.Connect`) and wired in `cmd/server` and the test harness
- [x] The Activity projector writes `activity` rows recording actor, entity table + id, action, and a payload snapshot — `internal/events/projector.go`; actor travels via `event.Metadata` (the `MetadataFunc` stamped by `WithEventPublisher`, regenerated in), the tenant via the system stamp
- [x] The `activity` table has events disabled, so writing Activity generates no further Activity (story 45) — `activity` is excluded from the generated `buildEventHooks` (sqlgen.yml `activity.events.enabled: false`)
- [x] An Activity feed query returns mutations in the caller's Active Workspace (story 44) — the generated tenant-scoped `activityList`/`activities` queries
- [x] An event-suppress request header prevents side effects (story 47) — `X-Skip-Events: true` on a generated mutation short-circuits publish; proven by `TestSkipEventsHeaderSuppressesActivity`
- [x] Integration test performs a mutation on a Task, then polls the Activity feed (bounded timeout) against a real NATS (testcontainer) and sees the entry — `TestTaskMutationProjectsActivity` in `test/activity_test.go`

## Notes for review

- **Projector scope (ADR-0006).** `activity.entity_id` is always a real single-uuid entity id. Single-uuid tables record on themselves. Composite-key **junctions are relationships, not feed subjects**, so their mutations are projected onto their *anchor entity* (`task_* → tasks`, `team_members → teams`, `memberships → workspaces`) with the relationship's other side in `payload`; the workspace is resolved from the anchor (the event's own tenant stamp for `memberships`, else a `SkipTenancy` `Get` of the anchor entity). Shared-table mutations (`users`, `workspaces` directly) carry no workspace and are skipped. Covered by `TestTaskDependencyAnchorsToDependentTask` (parent-lookup path) and `TestMembershipAnchorsToWorkspace` (direct-tenant path).
- **Follow-up (issue 07 surface, not fixed here).** The hand-written `createTaskInProject` resolver calls `Tasks().Create(ctx, in)` without threading `callOptionsFromHTTP`, so `X-Skip-Events` / `Cache-Control: no-cache` / `X-Skip-Hooks` are ignored on that path (they work on every generated mutation). The suppress test therefore exercises the generated `createLabel` path. Worth aligning the custom resolver with the generated call-option threading.
- **Regeneration.** `make generate` was run to thread `cfg.MetadataFunc` into the generated event hooks (the app predated that `event.Config` field); only `event_hooks_gen.go` and the manifest hash changed.

## Blocked by

- 03-workspaces-membership-tenant-scoping
- 07-projects-tasks-core
