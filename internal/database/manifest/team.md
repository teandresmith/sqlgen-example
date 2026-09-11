# Team

- **Table:** `teams` (schema `public`)
- **Kind:** table

A group of users within a workspace that owns projects

## Files

- `team_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique team identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `name` | Name | `string` | `text` |  |  |  |  | `comparator.String` | Team display name |
| `slug` | Slug | `string` | `text` |  |  |  |  | `comparator.String` | URL-safe short name (unique per workspace) |
| `description` | Description | `*string` | `text` | yes |  |  |  | `comparator.NullableString` | Optional description |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_teams_workspace` | workspace_id |  | btree |  |
| `teams_pkey` | id | yes | btree |  |
| `teams_workspace_id_slug_idx` | workspace_id, slug | yes | btree |  |
| `teams_ws_id_key` | workspace_id, id | yes | btree |  |

## Relationships

| Name | Kind | Target | FK | Filter |
| --- | --- | --- | --- | --- |
| cycles | o2m | Cycle | public.cycles.team_id |  |
| projects | o2m | Project | public.projects.team_id |  |
| users | m2m | User | public.team_members (team_id → user_id) |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Team`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "description", "id", "name", "slug", "updated_at", "workspace_id" FROM "public"."teams" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetTeamsInput`
- Returns: `[]*Team`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "description", "id", "name", "slug", "updated_at", "workspace_id" FROM "public"."teams" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *TeamFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."teams" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."teams" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *TeamFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."teams" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[TeamFilter]`
- Returns: `*PaginateResult[Team]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TeamFilter]`
- Returns: `*Connection[Team]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTeamsInput`
- Returns: `iter.Seq2[*Team, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "description", "id", "name", "slug", "updated_at", "workspace_id" FROM "public"."teams" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTeamInput`
- Returns: `*Team`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."teams" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateTeamInput`
- Returns: `[]*Team`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."teams" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateTeamInput`, `target TeamConflictTarget`
- Returns: `*Team`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated TeamConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."teams" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateTeamInput`, `target TeamConflictTarget`
- Returns: `[]*Team`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same TeamConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."teams" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateTeamInput`
- Returns: `*Team`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."teams" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateTeamItem`
- Returns: `[]*Team`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."teams" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *TeamFilter`, `input *UpdateTeamInput`
- Returns: `[]*Team`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."teams" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."teams" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."teams" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *TeamFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."teams" WHERE <filter> RETURNING "id"
```

### UpdateWithRelated

- Params: `id uuid.UUID`, `input *UpdateTeamWithRelatedInput`
- Returns: `*Team`, `error`
- Errors: `ErrNilInput`, `ErrNotFound`, `ErrNestedVerbConflict`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Writes the parent and its nested rows in one transaction — a SAVEPOINT when ctx already holds one. Issues no statement of its own: it composes Update and, per eligible relationship (Projects), the target's and junction's own client methods, so no sql_bodies entry. Each inner call fires its own hooks, events and cache invalidation under its own op; the nested method has none.

## Filter

Type `TeamFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| Description | `*comparator.NullableString` |
| ID | `*comparator.ID` |
| Name | `*comparator.String` |
| Slug | `*comparator.String` |
| UpdatedAt | `*comparator.Time` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*TeamFilter` |
| Or | `[]*TeamFilter` |

## Sort

Type `TeamSort`.

Fields: CreatedAt, Description, ID, Name, Slug, UpdatedAt, WorkspaceID
