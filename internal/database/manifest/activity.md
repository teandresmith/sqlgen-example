# Activity

- **Table:** `activity` (schema `public`)
- **Kind:** table

Append-only activity feed derived from mutation events

## Files

- `activity_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique activity entry identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `actor_id` | ActorID | `*uuid.UUID` | `uuid` | yes |  |  |  | `comparator.NullableID` | User who triggered the activity; NULL for system actions |
| `entity_table` | EntityTable | `string` | `text` |  |  |  |  | `comparator.String` | Table name of the affected entity |
| `entity_id` | EntityID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Primary key of the affected entity |
| `action` | Action | `string` | `text` |  |  |  |  | `comparator.String` | Mutation action (create/update/delete/upsert) |
| `payload` | Payload | `types.JSON` | `jsonb` |  |  |  | `'{}'::jsonb` | `comparator.JSONB` | Snapshot of the change |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | When the activity occurred |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `activity_pkey` | id | yes | btree |  |
| `idx_activity_workspace` | workspace_id, created_at |  | btree |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Activity`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "action", "actor_id", "created_at", "entity_id", "entity_table", "id", "payload", "workspace_id" FROM "public"."activity" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetActivitiesInput`
- Returns: `[]*Activity`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "action", "actor_id", "created_at", "entity_id", "entity_table", "id", "payload", "workspace_id" FROM "public"."activity" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *ActivityFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."activity" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."activity" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *ActivityFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."activity" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[ActivityFilter]`
- Returns: `*PaginateResult[Activity]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[ActivityFilter]`
- Returns: `*Connection[Activity]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamActivitiesInput`
- Returns: `iter.Seq2[*Activity, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "action", "actor_id", "created_at", "entity_id", "entity_table", "id", "payload", "workspace_id" FROM "public"."activity" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateActivityInput`
- Returns: `*Activity`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."activity" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateActivityInput`
- Returns: `[]*Activity`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."activity" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateActivityInput`, `target ActivityConflictTarget`
- Returns: `*Activity`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated ActivityConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."activity" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateActivityInput`, `target ActivityConflictTarget`
- Returns: `[]*Activity`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same ActivityConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."activity" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateActivityInput`
- Returns: `*Activity`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."activity" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateActivityItem`
- Returns: `[]*Activity`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."activity" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *ActivityFilter`, `input *UpdateActivityInput`
- Returns: `[]*Activity`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."activity" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."activity" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."activity" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *ActivityFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."activity" WHERE <filter> RETURNING "id"
```

## Filter

Type `ActivityFilter`.

| Field | Type |
| --- | --- |
| Action | `*comparator.String` |
| ActorID | `*comparator.NullableID` |
| CreatedAt | `*comparator.Time` |
| EntityID | `*comparator.ID` |
| EntityTable | `*comparator.String` |
| ID | `*comparator.ID` |
| Payload | `*comparator.JSONB` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*ActivityFilter` |
| Or | `[]*ActivityFilter` |

## Sort

Type `ActivitySort`.

Fields: Action, ActorID, CreatedAt, EntityID, EntityTable, ID, Payload, WorkspaceID
