# sqlgen v0.1.2 — upgrade findings

Status: findings (for the sqlgen owner and the app owner)
Branch: `chore/sqlgen-v0.1.2` (not pushed)
sqlgen: `v0.1.2` runtime + `cmd/sqlgen@v0.1.2` (`./bin/sqlgen --version` → `sqlgen 0.1.2`),
compared against `v0.1.0` where noted. Go 1.27.1, Postgres 16 (testcontainers), macOS.

## TL;DR

The upgrade is clean. Composite-FK edges work: loads, relationship filters and nested
`connect` / `disconnect` / `clear` all join on the full key. The new tests prove that
v0.1.0 leaked across Workspaces on the same data and v0.1.2 does not. No sqlgen bug
blocks the upgrade. None of the manual relationships can be removed yet: the new naming
rules give the same names as v0.1.0, so every rename is still needed.

| # | Finding | Class | Severity |
| - | ------- | ----- | -------- |
| 1 | Prose `@pk` in a view-file comment warns as an "unrecognized annotation directive" | **sqlgen bug** | Low |
| 2 | CHANGELOG for 0.1.1 / 0.1.2 omits most user-visible changes, including the resolver reseed | **docs gap** | Medium |
| 3 | `configuration.md` does not document `relationships` / `exclude_relationships` / naming / `api.graphql` | **docs gap** | Medium |
| 4 | New `internal/graph/server.go` scaffold is unused here, and its limits disagree with the app's | **needs an app change** (+ docs gap) | Low |
| 5 | No way to rename a detected edge; redeclaring it drops the manifest `constraint` | **docs gap** / feature request | Medium |
| 6 | `ErrAlreadyRelated` is returned for a cross-Workspace connect under `allow_reparent`, which its doc says can't happen | **docs gap** | Low |
| 7 | Generated nested-create doc says the FK is set "from the parent's primary key", but for a composite key it isn't | **docs gap** | Low |
| 8 | GraphQL `<T>Filter` has no `id` member (the Go filter does). Not new in v0.1.2 | **docs gap** | Low |

There's an app-side observation that isn't a sqlgen finding at the end.

---

## 1 — Prose `@pk` in a view comment is reported as an unrecognized directive (sqlgen bug, Low)

Command and output (v0.1.2; v0.1.0 printed only the first line):

```
$ make generate
./bin/sqlgen generate
Warning: CREATE VIEW project_stats skipped in DDL file — use introspection or define the view in input.views
Warning: views/project_stats.sql: unrecognized annotation directive @pk       marks the primary key so a Get(ctx, pk) method is generated. ignored
sqlgen: generated 91 files (21 tables, 1 views) in 7.757s
```

The line it trips on is documentation inside `views/project_stats.sql`:
`--   @pk       marks the primary key so a Get(ctx, pk) method is generated.`
The real directive (`-- @pk: project_id`) is still honored: `ProjectStat` keeps its `Get`.
The diagnostic is wrong twice. `@pk` *is* a recognized directive, and the scanner takes
the whole rest of the line as the directive name. Every `make generate` and
`make diff` now prints a warning that nobody can act on.

Minimal reproduction (warns on v0.1.2, silent on v0.1.0, `Get` generated on both):

```yaml
# sqlgen.yml
version: v1
input: { dialect: postgres, source: files, paths: [./migrations], views: [./views] }
output: { driver: pgx, dir: ./out, package: out }
```
```sql
-- migrations/0001.up.sql
CREATE TABLE items (id UUID PRIMARY KEY, name TEXT NOT NULL);
-- views/item_names.sql
-- Directives:
--   @pk marks the primary key.
-- @pk: id
CREATE VIEW item_names AS SELECT i.id AS id, i.name AS name FROM items i;
```
```
$ sqlgen generate   # 0.1.2
Warning: views/item_names.sql: unrecognized annotation directive @pk marks the primary key. ignored
```

Expected: a known directive name without its `:` should either be skipped as prose, or
get a targeted message such as `@pk is missing ':'`. It should never be called
"unrecognized". Not worked around: the app comment is left as is.

## 2 — CHANGELOG under-describes 0.1.1 / 0.1.2 (docs gap, Medium)

`CHANGELOG.md` has one line per release ("database introspection",
"relationships through composite foreign keys"). Upgrading 0.1.0 → 0.1.2 *with no config
change* also produced all of the following. Each was found by diffing generated output
(`git diff main -- internal/graph/*.graphqls`), not from release notes:

- **Resolver reseed required.** `UpdateX(ctx, id, input, hasSet)` changed signature. gqlgen
  keeps existing resolver bodies, so `*_gen.resolvers.go` must be deleted and reseeded.
  There is no upgrade note.
- **Create inputs relaxed.** On every `Create<T>Input`, NOT NULL columns with a DB default
  (`status`, `priority`, `customFields`, `createdAt`, `updatedAt`, `color`, `settings`)
  and the tenant column `workspaceID` went from `T!` to optional. The Go `Create<T>Input`
  wraps them in `omittable.Value`.
- **Nested create inputs** (`CycleTasksCreateInput`, `TeamProjectsCreateInput`) dropped
  `workspaceID`. It's now set from the parent as a member of the composite key.
- `ProjectStatFilter.workspaceID`: `StringComparator` → `IDComparator`.
- `BooleanComparator` gained `in: [Boolean!]`.
- New files: `internal/graph/server.go` (written once) and `sqlgenresolver/limits_gen.go`
  (see #4).
- Stricter view-annotation scanning (see #1).
- Manifest `schema_version` 0.1.0 → 0.2.0. Every edge now carries `fk.columns` /
  `references` / `constraint`, replacing the old single `column`.

All of these look intentional and correct. The defaults were verified to be honored on
omission through `omittable`. They are public GraphQL and Go API changes, though, and
adopters need them listed.

## 3 — `configuration.md` omits the relationship and GraphQL config (docs gap, Medium)

```
$ grep -n -i "graphql\|exclude_relationships\|junction_local_fk" docs/configuration.md
115:| `api` | object | disabled | GraphQL API generation. Opt-in. |
429:| `api` | GraphQL schema + gqlgen resolver generation |
```

The parts of `docs/configuration.md` (v0.1.2) that the release notes point adopters at are
missing:
- `tables.<t>.relationships` and `exclude_relationships`
- `fk` / `junction_local_fk` / `junction_reference_fk` as a string *or a list*
- the edge-naming / disambiguation rules
- the replace-by-Go-field-name warning
- the whole `api.graphql` block (`max_depth`, `max_complexity`, `schema_dir`, …)

They exist only in `docs/PRD.md` (§4.8, §5.3 rules 1–6, §13.4). The PRD is a 15k-line
spec, not a reference. Its `exclude_relationships` example (`[Company, AssignedTo]`) also
shows PascalCase, while detected names are snake_case (`task_assignees`).

## 4 — Unused `server.go` scaffold with conflicting limits (needs an app change, Low; docs gap)

```
$ make generate && git status --short | grep '??'
?? internal/graph/server.go
?? internal/graph/sqlgenresolver/limits_gen.go
```

`server.go` is consumer-owned ("written once by sqlgen because it was missing… sqlgen
never rewrites it"). It builds `graph.NewServer` with `ApplyQueryLimits` (depth 6,
complexity 1000, from `sqlgen.yml` `api.graphql`). The app doesn't use it:
`internal/server/server.go` builds its own handler with `FixedDepthLimit{12}` and
`FixedComplexityLimit(200)` (`internal/server/limits.go`). Deleting the scaffold doesn't
work, because the next generate writes it again. Both files are committed as generated.

- **App decision needed:** either adopt `graph.NewServer`/`sqlgenresolver.ApplyQueryLimits`,
  or set `sqlgen.yml` `max_depth` / `max_complexity` to the limits the server actually
  enforces. Today the config advertises limits that aren't the real ones.
- **Docs:** the scaffold and its re-creation aren't documented, and there's no opt-out.

## 5 — No rename for a detected edge; redeclaring one loses its constraint (docs gap / feature request, Medium)

Three of the four manual relationships (Subtasks, Assignees, Watchers) exist only to
rename an edge that detection already finds. The only mechanism is to exclude the edge
and redeclare its join columns by hand. The manifest then drops the `constraint`, because
PRD §13.4 says a manual edge "has no FOREIGN KEY constraint behind it". These three do:

```
$ jq -c '.entities[]|select(.table=="tasks")|.relationships[]|select(.name=="Subtasks")|.fk' manifest_gen.json
{"table":"tasks","schema":"public","columns":["parent_task_id"],"references":["id"]}
# with the manual entry removed (auto-detected parent_task_tasks):
{"table":"tasks","schema":"public","columns":["parent_task_id"],"references":["id"],"constraint":"tasks_parent_task_id_fkey"}
```

Detected names also depend on context: rule 6 renames an edge whenever a second edge
appears between the same two tables. So an unrelated migration can rename a public field.

Suggested shape:
- A rename-only map keyed on something stable: the FK constraint for O2O/O2M, the
  junction table for M2M. For example
  `relationship_names: { tasks_parent_task_id_fkey: Subtasks, task_assignees: Assignees }`.
  Detection keeps the key, kind and constraint, and no exclusion is needed. Optional
  `inverse:` names the belongs-to side.
- Optionally, smart-tag comments in the DDL (`COMMENT ON CONSTRAINT … IS '@name Subtasks'`)
  for schema-first apps like this one (ADR-0002).
- A gentler M2M default: strip the source table's prefix from the junction name
  (`task_assignees` seen from `tasks` → `assignees`).
- A rename-drift guard in `generate` / `diff`. Compare each constraint's edge name with the
  previous manifest and warn or fail when it changes. The 0.2.0 manifest already records
  constraints.

## 6 — `ErrAlreadyRelated` returned with `allow_reparent` set (docs gap, Low)

Under `SkipTenancy`, `Cycles().UpdateWithRelated(cycleA, {Tasks: {Connect: [taskB]}})`, with
`taskB` in Workspace B and `cycle_id IS NULL`, fails with `database.ErrAlreadyRelated`:

```
$ go test ./test -run 'TestCompositeEdgeCycleTasks/nested_writes/SkipTenancy' -v
--- PASS: TestCompositeEdgeCycleTasks/nested_writes/SkipTenancy
```

(The test asserts `errors.Is(err, database.ErrAlreadyRelated)`.) Refusing the connect is
right. `workspace_id` is a key member the edge can't write, and the executor says so:
`// Bound to another key through a member this edge cannot write, so allow_reparent does
not reach it either.` But the sentinel's contract in `errors_gen.go` only covers "already
parented to a different row and the edge does not set allow_reparent". The row has no
parent and the edge sets `allow_reparent`. The doc should cover a composite key whose
non-writable member differs. Under tenant scoping this path can't be reached: the visibility
read returns NOT_FOUND first, which the GraphQL tests confirm.

## 7 — Nested-create doc comment is wrong for composite keys (docs gap, Low)

`internal/database/cycle_gen.go`, `CycleTasksCreateInput`: "CreateTaskInput with the
traversed foreign key removed — that column is always set from the parent's primary key".
Both `workspace_id` and `cycle_id` are removed. `workspace_id` is set from
`parent.WorkspaceID`, which isn't part of the primary key, so the comment should say "from
the columns the key references". `TeamProjectsCreateInput` has the same comment.

## 8 — GraphQL filters have no `id` member (docs gap, Low, pre-existing)

```
query { cycles(filter: { tasks: { id: { eq: "…" } } }) { edges { node { id } } } }
→ 422 Field "id" is not defined by type "TaskFilter".  (GRAPHQL_VALIDATION_FAILED)
```

The Go `TaskFilter` has `ID *comparator.ID`. No `*_gen.graphqls` filter input exposes
the PK, on v0.1.0 or v0.1.2. It may be intentional, but I couldn't find it documented.
The new tests probe by name instead.

---

## Task 1 — manual relationships (one at a time, with diffs)

Method: `/private/tmp/…/scratchpad/exp.sh <variant>`. Each run removes the entry from
`sqlgen.yml`, runs `./bin/sqlgen generate`, diffs `internal/**/*.graphqls`,
`internal/database/*.go` and the manifest, then restores the tree. Every variant exited 0
with only the two baseline warnings. No replacement warning fired for the committed config.

| Relationship | Result | Reason |
| --- | --- | --- |
| `tasks.Subtasks` + exclusion `parent_task_tasks` | **kept** | Removed, the edge is `parent_task_tasks`, so `Task.subtasks` → `parentTaskTasks` and `TaskFilter.subtasks` → `parentTaskTasks`. Rule 6 still applies because the O2M shares `tasks→tasks` with the two `task_dependencies` M2M directions. |
| exclusion `parent_task_tasks` alone | **kept** | Without it, `Subtasks` *and* `parentTaskTasks` are both emitted, which adds a public field. |
| `tasks.Assignees` + exclusion `task_assignees` | **kept** | Removed: `assignees` → `taskAssignees`. Two M2M edges reach `users`, so rule 6 names each after its junction, as v0.1.0 did. |
| exclusion `task_assignees` alone | **kept** | Without it, `taskAssignees` is added next to `assignees`. |
| `tasks.Watchers` + exclusion `task_watchers` | **kept** | Removed: `watchers` → `taskWatchers`, for the same reason. |
| exclusion `task_watchers` alone | **kept** | Without it, `taskWatchers` is added. |
| `tasks.Attachments` | **kept** | Polymorphic, with no FK: removed, `Task.attachments` and `TaskFilter.attachments` disappear. |
| `comments.Attachments` | **kept** | Same: `Comment.attachments` disappears. The shared `AttachmentRelationshipOptions` type just moves to `task_gen.go`. |

Extra check: declaring the edge under the detected name (`name: TaskAssignees`, with no
exclusion) works as documented:
`Warning: tables.public.tasks.relationships: "TaskAssignees" replaces the
foreign-key-inferred relationship "task_assignees" — both resolve to the Go field
"TaskAssignees" (PRD §13.4). Rename the declared relationship to keep both edges.`

No `nullable.import` tightening fired: the `numeric` override's `nullable: decimal.NullDecimal`
shares the parent's import. Net: `sqlgen.yml` is unchanged.

## Task 2 — composite edges

**2.1 Detection**, confirmed in `internal/database/manifest/manifest_gen.json`:

```
cycles.tasks    o2m Task    columns [workspace_id, cycle_id] → [workspace_id, id]  constraint tasks_cycle_same_ws
teams.projects  o2m Project columns [workspace_id, team_id]  → [workspace_id, id]  constraint projects_team_same_ws
```

v0.1.0 had the same edges but keyed on `cycle_id` / `team_id` alone, and its manifest
had `"column":"cycle_id"`. The generated loader (`BuildCompositePKBatchCondition` on both
columns), the relationship filter (`EXISTS … tgt.workspace_id = parent.workspace_id AND
tgt.cycle_id = parent.id`) and the nested executor all use the full key now. The executor's
`clear` and `disconnect` null only `cycle_id` / `team_id` and are scoped on both members.
Its `connect` refuses a row whose `workspace_id` differs.

**2.2 Tests:** `test/composite_edges_test.go`. Both edges run the same matrix.

- **Planted data:** Workspaces A and B are planted with identical names, slugs and titles
  under the same owner. Then one B row is pointed at A's parent with FK triggers off
  (`session_replication_role = replica`), which reverts in `t.Cleanup`. A single-column
  join would attach it to A's parent.
- **Matrix:** `load`, `filter` and `nested writes` (connect, `allow_reparent`,
  cross-Workspace connect, disconnect, clear), each as Workspace A (GraphQL), as
  Workspace B (GraphQL) and with `SkipTenancy` (Go client).
- **What separates the planted row:** under tenant scoping, the child's own tenant filter
  already hides it. Only the `SkipTenancy` cases separate it by the composite key alone.
  The app never uses `SkipTenancy` on cycles, teams, tasks or projects, so those cases
  probe the generated edge, not an app path.

```
$ go test ./test -run TestCompositeEdge -count=1
ok  	github.com/teandresmith/sqlgen-example/test	3.491s        # 18/18 subtests
```

Control: the same file run against the v0.1.0 tree fails only the six `SkipTenancy`
cases, three per edge.

```
--- FAIL: TestCompositeEdgeCycleTasks/load/SkipTenancy
    SkipTenancy: parentA.tasks = [<A1> <B2>], want [<A1>] (B's foreign child … keyed (B, parentA) must not join)
--- FAIL: TestCompositeEdgeCycleTasks/filter/SkipTenancy
    SkipTenancy: parents with B's foreign child = [<parentA>], want none
--- FAIL: TestCompositeEdgeCycleTasks/nested_writes/SkipTenancy
    SkipTenancy cross-Workspace connect error = … foreign_key constraint violation on constraint tasks_cycle_same_ws …, want ErrAlreadyRelated
    SkipTenancy disconnect of foreign child: tasks.cycle_id = <null>, want <parentA>
    SkipTenancy clear (foreign child): tasks.cycle_id = <null>, want <parentA>
(same three for TestCompositeEdgeTeamProjects)
```

v0.1.2 closes a real gap. On v0.1.0, a `SkipTenancy` `disconnect` or `clear` on A's
Cycle or Team unlinked B's rows. Only the composite FK kept a cross-Workspace `connect`
from committing.

**2.3 Hand-written compensation.** The blessed `scheduleTaskIntoCycle` / `assignProjectToTeam`
were already replaced by the generated `updateCycleWithRelated` / `updateTeamWithRelated`
before this upgrade. Only the test constant names remain. There are no manual loaders and
no hand-written tenant guards for these two edges. The generated surface now covers load,
filter and nested writes on the full key. Candidates, none removed:

| Candidate | Where | Recommendation / reasoning |
| --- | --- | --- |
| Composite FKs `tasks_cycle_same_ws` / `projects_team_same_ws` | migrations 0004, 0005 | **Keep. Not a candidate.** They're what sqlgen detects the edges through, and the only guard on the raw `updateTask.cycleID` / `updateProject.teamID` scalar writes, which sqlgen doesn't scope. |
| Raw-update backstop tests | `TestRawUpdateTaskCannotScheduleCrossWorkspaceCycle`, the Team analog | **Keep.** v0.1.2 doesn't change raw scalar writes. |
| Test constant names `scheduleTaskIntoCycleMutation`, `assignProjectToTeamMutation` | `test/cycles_test.go`, `test/teams_test.go` | Cosmetic. They name domain actions and already call the generated mutations. Renaming is optional. |
| Team-membership guards in `AddUserToTeam` (`Teams().Get`, `Memberships().ExistsWhere`) and `RemoveUserFromTeam` (`Teams().Get`) | `internal/graph/team_management.resolvers.go:31,42,73` | **Possible future candidate, if you approve a schema change.** `team_members` has no `workspace_id`. If it got one, with `(workspace_id, team_id) → teams(workspace_id, id)` and `(workspace_id, user_id) → memberships(workspace_id, user_id)`, v0.1.2 rule 4 would detect it as a tenant junction. Tenant isolation and the membership check would become referential. The owner/admin authz guard would still be needed. I haven't tested composite M2M here. |
| `tenantOfTeam` / `tenantOfTask` `SkipTenancy` lookups | `internal/events/projector.go:64-70,172` | Same dependency. They exist because the junctions carry no `workspace_id`. They'd go away only with the migration above, applied to each junction. |
| `AssignUserToTask` / `WatchTask` / `addTaskDependency` task-tenant checks | `internal/graph/task_relationships.resolvers.go` | Same pattern, same prerequisite (a tenant column plus composite FKs on `task_*` junctions). Keep for now. |

## Gate results (final tree)

| Gate | Result |
| --- | --- |
| `make generate` | exit 0. Warnings: `CREATE VIEW project_stats skipped …` (as on v0.1.0) and #1 |
| `make diff` | exit 0, `no changes detected` |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `make test` (`go test ./...`, testcontainers) | exit 0. All packages `ok`, including the 18 new subtests |
| `make manifest-validate` | `manifest valid: schema_version=0.2.0 entities=22 enums=8 extras=1` |

Every `make generate` also rewrites the manifest's `generated_at`, as v0.1.0 did. That
leaves a one-line diff `make diff` ignores.

## App-side observation (not sqlgen, not changed)

`AssignUserToTask` (`internal/graph/task_relationships.resolvers.go:33`) checks only that
the User exists globally, not that they hold a Membership in the Active Workspace. Any
User id can be made an Assignee on a Workspace's Task. That may be intended; worth a
deliberate decision.
