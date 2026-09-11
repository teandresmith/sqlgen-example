# 09 — Task content & time

Status: ready-for-human
Blocked by: 07-projects-tasks-core

Parent: `.scratch/app-server/PRD.md`

## What to build

The content and effort captured against a Task. A member comments on a Task so discussion stays with the work; attaches a file to a Task or a Comment so supporting material lives with it; and logs Time Entries against a Task in decimal hours so effort is captured.

## Acceptance criteria

- [x] A member can comment on a Task (story 29)
- [x] A member can attach a file to a Task (story 33)
- [x] A member can attach a file to a Comment (story 33)
- [x] A member can log Time Entries against a Task in decimal hours (story 34)
- [x] Integration tests exercise commenting, attachment, and time logging through the HTTP seam

## Blocked by

- 07-projects-tasks-core

## Notes for review

Implemented with the established **blessed-mutation** pattern (issues 07/08): each
target table carries a server-owned provenance column the client must never set, so
the raw generated create/upsert mutations are masked off the **GraphQL API only**
(`sqlgen.yml` `api.operations`, subtractive — the Go client `Create` the resolvers
call is untouched) and a hand-written resolver owns the write.

New GraphQL surface (`internal/graph/task_content.graphqls` +
`task_content.resolvers.go`, with the `createAttachment` helper in the sibling
`task_content_helpers.go` so gqlgen copy-through doesn't strip it):

- `commentOnTask(taskID, body)` — stamps `author_id` from the bearer token;
  resolves the Task tenant-scoped first (comments.task_id has no Workspace
  constraint, so this closes the cross-tenant comment hole); `workspace_id`
  auto-injected by tenant scoping.
- `attachToTask(taskID, …)` / `attachToComment(commentID, …)` — the polymorphic
  `attachments.entity_id` has **no FK**, so a raw create could bind a file to any
  UUID in any Workspace. Both resolvers resolve the target tenant-scoped, then set
  `entity_type` + `entity_id` themselves and stamp `uploaded_by` from the caller.
- `logTime(taskID, hours, spentOn, billable?, notes?)` — stamps `user_id` from the
  caller; `hours` is `Decimal!` (shopspring, `NUMERIC(6,2)`); `billable` falls to
  its DB default (true) when omitted.

Masks added: `comments`, `attachments`, `time_entries` → `{ create, create_many,
upsert }` off the API (mirrors the `memberships` / junction masks).

Tests (`test/task_content_test.go`, through the HTTP seam): commenting (author
provenance), task attachment, comment attachment (with the discriminator-filtered
read-back proving a comment file does **not** surface on the task), decimal time
logging (round-trip + billable default), a cross-Workspace guard on the polymorphic
`attachToTask` binding, and a regression that the six raw create/upsert mutations
are not exposed. `go build ./... && go vet ./... && go test ./...` all green.

No new DDL — the three tables already existed in `migrations/`; only config,
resolvers, and tests changed, then `make generate`.
