# The Activity feed projects junction mutations onto their anchor entity

The `activity` table records mutations as events on an entity: `entity_table` +
`entity_id uuid` name *what changed*, and `payload` snapshots the change. That
model wants `entity_id` to be a real, globally-unique entity id — the same value
you would join back to `tasks.id`, `workspaces.id`, and so on. Most tables fit it
directly: a single-uuid primary key is exactly such an id, unique regardless of
tenant.

Junction tables do not fit it, and the tempting fixes all distort the model. A
junction like `task_assignees (task_id, user_id)` has no single id — its identity
*is* the pair. Serializing the pair into a text `entity_key` column turns the
identity into an opaque, non-joinable string that the database cannot validate;
adding a surrogate `id` to the junction promotes a *relationship* to an *entity*
it isn't (and, in this codebase, would flip sqlgen's M2M auto-detection). Both
are wrong answers to a wrong question.

## Decision

**A junction is only a relationship association, not a feed subject.** Its current
state is already queryable from the entity it belongs to — `task.assignees`,
`task.watchers`, `team.members`, the GraphQL relationships that exist. So the feed
does not record the junction row; it records the *event* as activity on the real
entity the relationship belongs to — its **anchor** — with the relationship's
other side captured in `payload`:

| Junction | Anchored on | tenant from | payload carries |
|---|---|---|---|
| `task_assignees`, `task_watchers`, `task_labels` | `tasks` (`task_id`) | looked up from the task | the counterpart (`user_id` / `label_id`) |
| `task_dependencies` | `tasks` (`task_id`, the dependent) | looked up from the task | `depends_on_task_id`, `type` |
| `team_members` | `teams` (`team_id`) | looked up from the team | `user_id`, `role` |
| `memberships` | `workspaces` (`workspace_id`) | already on the event (tenant is in the PK) | `user_id`, `role` |

The **anchor is the FK to the entity the relationship belongs to**, which is also
the table the junction is tenanted through. So the tenant is resolved from the
anchor: for a junction whose event carries no tenant stamp (all but
`memberships`), the projector fetches the anchor entity (`Tasks().Get`,
`Teams().Get`, under `SkipTenancy` since it runs off the request path) and reads
its `workspace_id`. This keeps the tenant **row-derived** — correct even for the
admin/`SkipTenancy` bulk-write path — rather than taken from the caller's context,
which would assert a tenant the write never validated.

`entity_id` therefore stays `uuid` for every row: no `entity_key text` column, no
migration, no surrogate ids on link tables. A role change is still captured — as
an event on the workspace (`entity_table: "workspaces"`, `payload: {user_id,
role}`) — so membership/role audit is not lost.

## Consequences

- **`entity_id` remains a real entity id.** The feed's identity column is always a
  genuine single-uuid PK you can join back to `entity_table`, unique regardless of
  tenant. This is the property the decision exists to preserve.
- **The anchor policy is app-owned.** A small explicit map in the projector names
  each junction's anchor FK. The anchor is a domain choice (a membership belongs to
  its workspace; an assignment to its task), not something the generator should
  guess — so it lives in the consumer.
- **One extra read per junction mutation** to resolve the tenant, off the request
  path (asynchronous projection). Junction writes are comparatively infrequent and
  this never touches the request hot path.
- **The pair is not addressable in the feed.** There is no "history of this exact
  `(task, user)` assignment" and no per-pair dedup — by design. Current
  relationship *state* is queried from the junction/relationship; the feed records
  the *events*. The two concerns stay separate.
- **No sqlgen change is required.** This is entirely a consumer-side projection
  policy; the event contract is used as-is.
