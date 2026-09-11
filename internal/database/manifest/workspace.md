# Workspace

- **Table:** `workspaces` (schema `public`)
- **Kind:** table

Tenant root; the unit of isolation

## Files

- `workspace_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique workspace identifier |
| `name` | Name | `string` | `text` |  |  |  |  | `comparator.String` | Human-readable workspace name |
| `slug` | Slug | `string` | `text` |  |  | yes |  | `comparator.String` | URL-safe unique short name |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `workspaces_pkey` | id | yes | btree |  |
| `workspaces_slug_key` | slug | yes | btree |  |

## Relationships

| Name | Kind | Target | FK | Filter |
| --- | --- | --- | --- | --- |
| activities | o2m | Activity | public.activity.workspace_id |  |
| api_tokens | o2m | APIToken | public.api_tokens.workspace_id |  |
| attachments | o2m | Attachment | public.attachments.workspace_id |  |
| comments | o2m | Comment | public.comments.workspace_id |  |
| cycles | o2m | Cycle | public.cycles.workspace_id |  |
| invitations | o2m | Invitation | public.invitations.workspace_id |  |
| labels | o2m | Label | public.labels.workspace_id |  |
| projects | o2m | Project | public.projects.workspace_id |  |
| sso_connections | o2m | SsoConnection | public.sso_connections.workspace_id |  |
| tasks | o2m | Task | public.tasks.workspace_id |  |
| teams | o2m | Team | public.teams.workspace_id |  |
| time_entries | o2m | TimeEntry | public.time_entries.workspace_id |  |
| users | m2m | User | public.memberships (workspace_id → user_id) |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Workspace`, `error`
- Errors: `ErrNotFound`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "id", "name", "slug", "updated_at" FROM "public"."workspaces" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetWorkspacesInput`
- Returns: `[]*Workspace`, `error`

Generated SQL:

postgres:

```sql
SELECT "created_at", "id", "name", "slug", "updated_at" FROM "public"."workspaces" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *WorkspaceFilter`
- Returns: `int64`, `error`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."workspaces" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."workspaces" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *WorkspaceFilter`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."workspaces" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[WorkspaceFilter]`
- Returns: `*PaginateResult[Workspace]`, `error`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[WorkspaceFilter]`
- Returns: `*Connection[Workspace]`, `error`
- Errors: `ErrInvalidCursor`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamWorkspacesInput`
- Returns: `iter.Seq2[*Workspace, error]`, `error`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "id", "name", "slug", "updated_at" FROM "public"."workspaces" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateWorkspaceInput`
- Returns: `*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspaces" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateWorkspaceInput`
- Returns: `[]*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspaces" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateWorkspaceInput`, `target WorkspaceConflictTarget`
- Returns: `*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: One method over the generated WorkspaceConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspaces" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateWorkspaceInput`, `target WorkspaceConflictTarget`
- Returns: `[]*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks over the same WorkspaceConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."workspaces" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateWorkspaceInput`
- Returns: `*Workspace`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."workspaces" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateWorkspaceItem`
- Returns: `[]*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."workspaces" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *WorkspaceFilter`, `input *UpdateWorkspaceInput`
- Returns: `[]*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."workspaces" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."workspaces" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."workspaces" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *WorkspaceFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."workspaces" WHERE <filter> RETURNING "id"
```

### UpdateWithRelated

- Params: `id uuid.UUID`, `input *UpdateWorkspaceWithRelatedInput`
- Returns: `*Workspace`, `error`
- Errors: `ErrNilInput`, `ErrNotFound`, `ErrConstraintViolation`
- Notes: Writes the parent and its nested rows in one transaction — a SAVEPOINT when ctx already holds one. Issues no statement of its own: it composes Update and, per eligible relationship (APITokens, Activities, Attachments, Comments, Cycles, Invitations, Labels, Projects, SsoConnections, Tasks, Teams, TimeEntries), the target's and junction's own client methods, so no sql_bodies entry. Each inner call fires its own hooks, events and cache invalidation under its own op; the nested method has none.

## Filter

Type `WorkspaceFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| ID | `*comparator.ID` |
| Name | `*comparator.String` |
| Slug | `*comparator.String` |
| UpdatedAt | `*comparator.Time` |
| And | `[]*WorkspaceFilter` |
| Or | `[]*WorkspaceFilter` |

## Sort

Type `WorkspaceSort`.

Fields: CreatedAt, ID, Name, Slug, UpdatedAt
