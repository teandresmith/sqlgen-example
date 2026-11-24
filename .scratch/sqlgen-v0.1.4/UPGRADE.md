# Upgrade sqlgen v0.1.2 → v0.1.4

Branch `chore/sqlgen-v0.1.4` (off `chore/sqlgen-v0.1.2`). Not pushed.

## Re-check of the v0.1.2 findings

| # | Finding | v0.1.4 |
|---|---|---|
| 1 | prose `@pk` warns "unrecognized annotation directive" | fixed, warning gone |
| 2 | CHANGELOG omits the reseed/relaxed inputs | not fixed: 0.1.3/0.1.4 entries are one line each |
| 3 | configuration.md lacks relationship config | fixed (Relationships section, naming rules, `server_scaffold`) |
| 4 | unused server.go scaffold, limits 6/1000 vs 12/200 | fixed: `api.graphql.server_scaffold: false`, file deleted |
| 5 | no rename of a detected edge | fixed: a declared edge over the same key takes the place of the detected one |
| 6 | `ErrAlreadyRelated` doc | fixed, doc now covers the pinned-member case |
| 7 | nested-create comment wrong for composite keys | fixed |
| 8 | GraphQL filters have no `id` | fixed: `id: IDComparator` on every filter |

## Config changes
- Removed the three `exclude_relationships` entries: regenerating without them
  changes nothing (only `generated_at`). The three declarations stay: removing
  them renames Subtasks/Assignees/Watchers back to the detected names.
- `server_scaffold: false`; `internal/graph/server.go` removed.
- Took the `WorkspaceSetting.workspaces` -> `workspace` rename (approved).

## New behaviour that needs a decision
1. Belongs-to detection (0.1.3): every FK now also gives a to-one field, so
   Task gains `cycle`, `parentTask`, `project`, `reporter`, `workspace`, and so
   on, across the schema. Additive, but it widens the public GraphQL and the
   relationship filters (`TaskFilter.reporter` etc.).
2. It would rename `WorkspaceSetting.workspaces` to `workspace` (Go field and
   GraphQL). The brief forbids public renames without approval, so the old name
   is pinned with a declared o2o. Delete that entry to take the new name.

## Gates
generate 0, `make diff` clean, build, vet, `go test ./...` all ok (including the
18 composite-edge subtests), manifest-validate ok (0.2.0).
