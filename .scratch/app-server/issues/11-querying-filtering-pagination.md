# 11 — Querying, filtering, pagination & Project Stats

Status: ready-for-human
Blocked by: 07-projects-tasks-core

Parent: `.scratch/app-server/PRD.md`

## What to build

The read surface over the domain, proven through the seam. A member fetches a single entity by id; filters Tasks by status, priority, assignee, label, and other fields; combines conditions with and/or; sorts Task lists; pages Task connections by cursor; uses offset pagination with a total count where useful; loads a Task with its Comments, Assignees, Labels, and Subtasks in one query; and reads per-project task statistics from the Project Stats view. This behavior is generated; the ticket verifies it composes correctly under tenant scoping.

## Acceptance criteria

- [x] A member can fetch a single entity by id (story 35)
- [x] A member can filter Tasks by status, priority, assignee, label, and other fields (story 36)
- [x] Filter conditions can be combined with and/or (story 37)
- [x] Task lists can be sorted (story 38)
- [x] Cursor-paginated Task connections page stably (story 39)
- [x] Offset-paginated lists expose a total count where useful (story 40)
- [x] A Task loads with its Comments, Assignees, Labels, and Subtasks in one query (story 41)
- [x] Per-project task statistics are available via the Project Stats view (story 42)
- [x] Integration tests cover each read pattern through the HTTP seam

## Blocked by

- 07-projects-tasks-core

## Notes for review

This is a verification ticket — the read surface is generated. Tests live in
`test/querying_test.go`, one per read pattern, all through the GraphQL HTTP seam:

- **Get by id** — `task(id)` / `project(id)`, incl. cross-Workspace returning null.
- **Column filters (enum)** — `status`/`priority` via the per-enum comparators
  (`{status: {eq: DONE}}`); combined with `and`.
- **Relationship filters** — `assignees: {email: …}` and `labels: {name: …}` on
  `TaskFilter`, composed with a column filter in one query.
- **Sort** — `taskList(sort: [{field: TITLE, direction: ASC|DESC}])`.
- **Cursor pagination** — page `tasks(first, after)`; every Task visited once,
  `hasNextPage` flips on the last page, `totalCount` reported.
- **Offset pagination** — `taskList(limit, offset)` with `totalCount` / `hasMore`.
- **One-query relationship load** — `task(id){ comments assignees labels subtasks }`.
- **Project Stats view** — `projectStat(projectID)` counts (total/done/open), and
  tenant-scoped (another Workspace reads null).

This ticket drove the generator work that unblocked it: the enum/JSONB comparators,
relationship filters, and the view GraphQL surface + view tenancy were all missing
or broken and are now fixed (see `.scratch/app-server/sqlgen-graphql-gaps.md`,
gaps 1–4). The consumer no longer hand-writes the Project Stats surface — the
generator owns it.

All ACs met. Story 37's `or` surfaced a generator bug (top-level `or` compiled to
`AND`); it was fixed in `../sqlgen` (`<Entity>Filter.ToConditions` now groups each
`or` member with `sql.And` and wraps the group in one `sql.Or`) and is covered by
`TestFilterTasksComposedWithOr` — union, single-member field-AND, and `or` nested
inside `and`. History in `.scratch/app-server/sqlgen-graphql-gaps.md` (gap 5).

`go build ./... && go vet ./... && go test ./...` all green (no skips).
