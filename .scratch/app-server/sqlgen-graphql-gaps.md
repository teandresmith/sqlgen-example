# sqlgen — GraphQL read-surface gaps

Status: findings (for the `../sqlgen` generator owner)
Found while: building issue 11 (querying, filtering, pagination & Project Stats)
Repo context: `sqlgen-example` is the reference consumer; these are generator gaps
observed through its generated output, not app bugs. No app-side fix is possible
without either modifying `../sqlgen` or hand-writing around the generator.

## TL;DR

Verifying the generated read surface through the GraphQL HTTP seam surfaced four
gaps. One is a **cross-tenant read hole**, one is a **silent wrong-results**
correctness bug, one is a **missing feature**, and one is a **design limitation**.

| # | Gap | Class | Severity | App-side workaround today |
| - | --- | ----- | -------- | ------------------------- |
| 1 | Views carrying the tenant column are **not tenant-scoped** | Security (tenant isolation) | **Critical** | Resolver resolves a tenanted parent first, then reads the view |
| 2 | Enum / JSONB / Slice filter fields are **silently dropped** in translation | Correctness | **High** | None — filtering by those columns is impossible via GraphQL |
| 3 | Views get **no GraphQL surface** (type/queries/binding) | Feature gap | Medium | Hand-written type + resolver + `gqlgen.yml` `models:` binding |
| 4 | Cannot filter a root list **by a related entity** (assignee/label) or compose it | Design limitation | Medium | Reverse navigation (`user{taskAssignees}`, `label{tasks}`), no composition |

Recommended shape: tickets **A** (comparator projection), **B** (view tenancy),
**C** (view GraphQL surface, depends on B), and a design spike **D** (relationship
filtering). Details below.

---

## Gap 1 — Views with a tenant column are not tenant-scoped  (Critical)

**Symptom.** The generated view client applies no tenant predicate. Exposing a
view read directly returns rows from *every* Workspace.

**Evidence.**
- `internal/database/project_stat_gen.go` — `projectStatClient` has **no**
  `tenantResolver` field and no `resolveTenant` method.
- Contrast `internal/database/project_gen.go:167` (`tenantResolver …`) and
  `:238` (`resolveTenant`) on the table client, which inject
  `WHERE workspace_id = <tenant>`.
- The `project_stats` view **carries `workspace_id`** (see
  `views/project_stats.sql` and the manifest `project_stat.md`), so it *is*
  tenant-shaped — it just isn't scoped.

**Root cause.** Tenancy auto-detection (`tables carrying workspace_id are
tenanted`) is applied to tables but **not to views**. A view with the tenant
column is treated as untenanted.

**Impact.** Any consumer using the Go view client is unguarded today. The moment
a view is exposed over GraphQL (Gap 3), `projectStats(anyProjectId)` leaks another
Workspace's aggregates. This is the dangerous half hiding behind "expose views":
fixing Gap 3 without Gap 1 ships a vulnerability, on by default.

**Recommended fix.** Treat a view carrying the tenant column as tenanted, exactly
like a table: inject the tenant predicate into `Get`/`GetMany`/`Count`/`Paginate`/
`Connection`, resolved from the same `TenantResolver`, fail-closed. Views are
read-only, so only the read path needs it.

**Current app workaround** (see `internal/graph/project_stats.resolvers.go`): the
resolver resolves the owning Project through the **tenant-scoped table client**
first and uses that as the visibility gate, then reads the view by PK. Correct,
but every view consumer must remember to do this — the generator should own it.

---

## Gap 2 — Enum / JSONB / Slice filter fields are silently dropped  (High)

**Symptom.** `TaskFilter` exposes `status` and `priority` in the GraphQL schema,
but filtering by them is a **no-op** — the server returns the unfiltered set with
no error. The client believes it filtered.

**Evidence.**
- Schema exposes the fields: `internal/graph/task_gen.graphqls` —
  `status: StringComparator`, `priority: StringComparator`, `customFields: StringComparator`.
- Translator drops them: `internal/graph/filter_translate_gen.go:500`
  `translateTaskFilter` handles `createdAt, cycleID, deletedAt, description, dueAt,
  parentTaskID, projectID, reporterID, title, updatedAt, workspaceID` and `and/or`
  — **no `Status`, `Priority`, or `CustomFields`**. Grepping the whole file for
  `Status|Priority|Role|Enum|JSONB|CustomFields` returns nothing.
- The Go layer fully supports it: `internal/database/task_gen.go:3693,3696` —
  `Priority *comparator.Enum[TaskPriority]`, `Status *comparator.Enum[TaskStatus]`
  (and `CustomFields *comparator.JSONB`).
- No GraphQL comparator input exists for these families:
  `internal/graph/shared_gen.graphqls` defines `String/Numeric/Boolean/Time/ID`
  comparators only. The conventions doc (`_conventions.md`) lists the full Go
  family set as `Bool, Enum, ID, JSON, JSONB, Number, Slice, String, Time` — so
  **Enum, JSON, JSONB, and Slice are unprojected into GraphQL**.

**Root cause — two layered problems.**

1. **No GraphQL representation for generic/complex comparator families.** GraphQL
   has no generics, so `comparator.Enum[T]` can't be one input type. The generator
   didn't emit a specialization, and instead mistyped enum columns as
   `StringComparator` in the schema.
2. **Schema/translator drift.** The schema emitter and the translate emitter
   disagree: the schema advertises `status`, the translator ignores it. A filter
   field present in the schema but absent from the translator is the worst failure
   mode — silent, not an error.

**Recommended fix.**

- **Monomorphize the generic families per concrete type.** For each enum, emit a
  dedicated input reusing the *existing* enum binding (the enum is already bound to
  the Go type — `Task.status` marshals `database.TaskStatus`):

  ```graphql
  # generated, one per enum
  input TaskStatusComparator { eq: TaskStatus  neq: TaskStatus  in: [TaskStatus!]  nin: [TaskStatus!]  isNull: Boolean }

  # TaskFilter.status becomes:
  status: TaskStatusComparator   # was StringComparator
  ```

  Translation is then a one-liner: `comparator.Enum[TaskStatus]{Eq: in.Eq, …}`.
  The GraphQL `DONE` → DB `done` conversion is the existing enum binding — no new
  machinery. This is strictly better than "keep `StringComparator` and parse at
  runtime," which loses schema-level validation (`{eq: "banana"}` becomes a
  runtime error instead of a schema rejection). Respect nullability: only nullable
  enum columns expose `isNull` (mirror the existing `String` vs `NullableString`
  split). Do the same for `JSON`/`JSONB` (Contains/ContainedBy/…) and `Slice`
  (Contains/Overlap/…).

- **Generate schema and translator from one field map.** The invariant: it must be
  impossible to emit a filter field in the schema that the translator can't
  handle. Whatever the comparator work, this guarantee is the durable fix — it
  prevents the next drift.

**Impact / scope.** Affects every enum column across the API — `tasks.status`,
`tasks.priority`, `cycles.status`, `memberships.role`, `invitations.status`,
`sso_connections.provider`, `attachments.entity_type`, `task_dependencies.type` —
plus `tasks.custom_fields` (JSONB). All are advertised and non-functional.

---

## Gap 3 — Views get no GraphQL surface  (Medium; depends on Gap 1)

**Symptom.** The generator emits a Go read client for views but no GraphQL type or
queries, so a view is invisible over the API.

**Evidence.** No `projectStats`/`projectStat` field in any `*_gen.graphqls`; no
`ProjectStat` GraphQL model. To satisfy story 42 the app hand-wrote
`internal/graph/project_stats.graphqls`, `project_stats.resolvers.go`, **and** a
`gqlgen.yml` `models:` binding to `database.ProjectStat` (sqlgen autobinds the
entities it exposes, but not views). That manual `models:` entry is the tell — if
the generator owned views, it would inject the binding like it does for tables.

**Recommended fix.** For each view (honoring `api` config), emit the **read-only**
surface: the object type bound to the generated view struct, `get`-by-PK +
`connection` + `list` queries, and the view's `Filter`/`Sort` inputs (the Go
`ProjectStatFilter` already exists, and `cursor_keys` is already declared for
`project_stats` in `sqlgen.yml`). No mutations. **Blocked on Gap 1** — do not
auto-expose a view read until the view is tenant-scoped, or this becomes the
delivery vehicle for the cross-tenant hole.

---

## Gap 4 — Cannot filter a root list by a related entity  (Design limitation)

**Symptom.** Story 36 asks to "filter Tasks by status, priority, **assignee,
label**, and other fields." Even after Gap 2 is fixed,
`tasks(filter: {status: DONE, assignee: X})` is impossible: assignee and label
live on M2M relationships, not columns, so `TaskFilter` has no such fields.

**Evidence.** The only path is reverse navigation — `user(id){ taskAssignees }`,
`label(id){ tasks }` (`internal/graph/user_gen.graphqls`,
`label_gen.graphqls`) — and those relationship fields take **no arguments**, so
they can neither be filtered nor composed with a root-list column filter in one
query.

**Root cause.** The generated filter has no notion of "filter a parent by its
children." Relationship fields are unparameterized.

**Options (design spike).**
- Filter arguments on relationship fields (`assignees(filter: …)`), and/or
- Junction-backed filter fields on the parent
  (`TaskFilter.assignees: UserFilter` compiling to an `EXISTS`/join), and/or
- Documented guidance that related-entity filtering goes through the child entity
  (accepting no composition).

Whichever way, it's independent of the comparator work and should be a deliberate
API decision, not left as a silent gap.

---

## Verify next (smaller, unconfirmed)

- **Sort on enum / non-scalar fields.** `TaskSortField` includes `STATUS`; sort is
  a separate translator (`sort_translate_gen.go`). Sorting is just `ORDER BY
  status` (enum ordinal), so it likely works — but given filtering drifted, audit
  that every emitted sort field is actually wired.
- **`get`-by-id** on tables is correct (tenant-scoped, null on miss) — that half of
  the read surface is solid. The gaps concentrate in filter translation and views.

## Suggested ticket breakdown for `../sqlgen`

- **A. Comparator projection.** Project every Go comparator family into GraphQL,
  monomorphizing generics (Enum per type; JSON/JSONB; Slice), driven by a single
  field map shared with the translator so schema and translator can't drift.
  Closes Gap 2.
- **B. View tenancy.** Auto-scope views carrying the tenant column, same as tables
  (read path only, fail-closed). Closes Gap 1; valuable even before Gap 3.
- **C. View GraphQL surface.** Emit read-only type + queries + inputs + binding for
  views. Closes Gap 3. **Depends on B.**
- **D. Relationship filtering (spike).** Decide how a root list filters by a
  related entity. Addresses Gap 4.

---

## Re-verification (after the generator fixes landed)

All four gaps above are **fixed** at every layer:

- **Gap 1 (view tenancy):** `projectStatClient` now carries `tenantResolver` +
  `resolveTenant` (`internal/database/project_stat_gen.go`). Fixed.
- **Gap 2 (enum/JSONB filters):** per-enum comparators generated
  (`TaskStatusComparator`, `TaskPriorityComparator`, `CycleStatusComparator`,
  `InvitationStatusComparator`, `MembershipRoleComparator`) plus `JSONBComparator`;
  `TaskFilter.status/priority/customFields` retyped to them; and the translator
  wires them (`filter_translate_gen.go` → `translateTaskStatusComparator`, etc.).
  Schema/translator/Go now agree. Fixed.
- **Gap 3 (view GraphQL surface):** generator emits `project_stat_gen.graphqls`
  (`projectStat` / `projectStats` / `projectStatList` + type/filter/sort) and binds
  it. The hand-written `project_stats.graphqls`/`.resolvers.go` and the `gqlgen.yml`
  `models:` entry were removed as redundant. Fixed.
- **Gap 4 (relationship filtering):** `TaskFilter` gained nested relationship
  members (`assignees: UserFilter`, `labels: LabelFilter`, `comments`, `subtasks`,
  …), composable with column filters and `and`/`or`; the translator wires them to
  the related-table filters. Fixed — exceeds the original spike.

### But: a new regression — shared tables fail closed  (Blocker)

**Symptom.** The entire integration suite fails at setup. `register` returns
`get users: tenancy: tenant missing from context`; every test that calls
`register`/`bootstrapWorkspace` (i.e. all of them) dies before its own assertions.

**Root cause.** The Gap-4 relationship-filter work added a `filterOptions` helper
that scopes relationship subqueries by resolving the tenant, and the tenant
resolver is now wired into **every** client, including the shared `users` and
`workspaces` clients (`internal/database/client_gen.go:181-182`). `filterOptions`
resolves the tenant **unconditionally** on every `GetMany`
(`user_gen.go` `GetMany` → `filterOptions` → `resolveTenant`), regardless of
whether the filter contains any relationship member — and a shared table has no
tenant column to scope on in the first place. Under `tenancy.required: true`,
`resolveTenant` returns an error when no Active Workspace is in context, and
`filterOptions` propagates it. So a plain shared-table read performed before a
Workspace is selected (register, login, me, bootstrapWorkspace) fails closed.

Note the outer-query path is still correct: `userClient.GetMany` does **not** apply
a tenant predicate to its own `WHERE` (unlike `projectClient.GetMany`, which guards
with `if apply {…}`). The fault is isolated to `filterOptions` resolving a tenant
for a shared table it should treat as a no-op.

**Recommended fix (in `../sqlgen`).** Gate `filterOptions` so it is a no-op for
shared-table clients (a shared table has no tenant column), and/or only resolve the
tenant when the filter actually carries relationship members that compile to a
scoped `EXISTS`. Do not force tenant resolution on a filter with no relationship
members. This is not consumer-config-fixable short of disabling `tenancy.required`,
which would weaken the fail-closed posture app-wide.

**State left in the repo.** Regenerated against the updated generator (gaps 1–4
verified), hand-written view workaround removed. The app cannot register a user
until the regression above is fixed, so issue 11's tests are blocked. `go build`
and `go vet` pass; `go test ./...` fails wholesale at setup with the error above.

---

## Gap 5 — Top-level filter `or` compiles to `AND`  (High, correctness) — FIXED

Found while writing issue 11's filter tests, after gaps 1–4 and the shared-table
regression were fixed. **Now fixed in the generator** — `ToConditions` groups each
`or` member with `sql.And(...)` and wraps the group in a single `sql.Or(...)`.
Verified end-to-end by `TestFilterTasksComposedWithOr` (union, single-member
field-AND, and `or` nested inside `and`).

**Symptom.** A multi-member top-level `or` returns the wrong rows — it behaves as
`and`. `taskList(filter: { or: [ {status:{eq:TODO}}, {priority:{eq:LOW}} ] })`
returns `[]` (no Task is both TODO and LOW) instead of the union.

**Evidence.** `internal/database/task_gen.go`, `TaskFilter.ToConditions`:

```go
for _, sub := range f.Or {
    subConds := sub.ToConditions(dialect, opts...)
    if len(subConds) > 0 {
        conds = append(conds, sql.Or(subConds...))   // each member wrapped on its own
    }
}
return conds                                          // caller AND-s the whole list
```

Each `or` member is compiled and wrapped in its **own** `sql.Or(...)` — which is a
no-op for a single-field member — and the wrappers are appended to `conds`, which
the caller combines with `AND`. So `or: [A, B]` becomes `A AND B`. The GraphQL
translation is correct (`filter_translate_gen.go` maps `or` → `TaskFilter.Or`); the
fault is purely in `ToConditions`. `and` works only incidentally, because the
top-level list is already AND-ed.

**Root cause.** The `Or` members must be grouped and OR-ed **together**, not each
wrapped and then AND-ed. Correct shape:

```go
if len(f.Or) > 0 {
    var orGroup []sql.Condition
    for _, sub := range f.Or {
        if subConds := sub.ToConditions(dialect, opts...); len(subConds) > 0 {
            orGroup = append(orGroup, sql.And(subConds...)) // each member: AND its own fields
        }
    }
    if len(orGroup) > 0 {
        conds = append(conds, sql.Or(orGroup...))           // the members: OR together
    }
}
```

**Impact.** Every entity's `<Entity>Filter.ToConditions` shares this shape, so
top-level `or` is broken API-wide (and nested `or` inside `and`, recursively).
Half of story 37 (`and`/`or`) is unmet until this lands.

**Suggested ticket — E.** Fix `Or` grouping in `ToConditions` so members OR
together; add a generator test asserting `or:[A,B]` compiles to `A OR B`.

## Impact on issue 11 (this repo)

- Deliverable through the seam **today**: get-by-id, `and`/`or` + column filters on
  scalar columns (title/description/id/time), sort, cursor pagination, offset +
  totalCount, one-query relationship loading, and Project Stats (via the
  hand-written tenant-guarded resolver).
- **Not** deliverable via the generated surface: status/priority (Gap 2) and
  combined assignee/label filtering (Gap 4). Tests for these should ship
  **skipped**, referencing this document, so they flip to green regressions when
  A/D land.
