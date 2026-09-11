# TimeEntry

- **Table:** `time_entries` (schema `public`)
- **Kind:** table

Logged time against a task

## Files

- `time_entry_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `task_id` | TaskID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Task the time was logged against |
| `user_id` | UserID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | User who logged the time |
| `hours` | Hours | `decimal.Decimal` | `numeric(6, 2)` |  |  |  |  | `comparator.String` | Hours worked (decimal) |
| `billable` | Billable | `bool` | `boolean` |  |  |  | `true` | `comparator.Bool` | Whether the time is billable |
| `notes` | Notes | `*string` | `text` | yes |  |  |  | `comparator.NullableString` | Optional note |
| `spent_on` | SpentOn | `time.Time` | `date` |  |  |  |  | `comparator.Time` | The day the work was performed |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_time_entries_task` | task_id |  | btree |  |
| `idx_time_entries_workspace` | workspace_id, spent_on |  | btree |  |
| `time_entries_pkey` | id | yes | btree |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*TimeEntry`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "billable", "created_at", "hours", "id", "notes", "spent_on", "task_id", "user_id", "workspace_id" FROM "public"."time_entries" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetTimeEntriesInput`
- Returns: `[]*TimeEntry`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "billable", "created_at", "hours", "id", "notes", "spent_on", "task_id", "user_id", "workspace_id" FROM "public"."time_entries" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *TimeEntryFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."time_entries" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."time_entries" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *TimeEntryFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."time_entries" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[TimeEntryFilter]`
- Returns: `*PaginateResult[TimeEntry]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TimeEntryFilter]`
- Returns: `*Connection[TimeEntry]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTimeEntriesInput`
- Returns: `iter.Seq2[*TimeEntry, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "billable", "created_at", "hours", "id", "notes", "spent_on", "task_id", "user_id", "workspace_id" FROM "public"."time_entries" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTimeEntryInput`
- Returns: `*TimeEntry`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."time_entries" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateTimeEntryInput`
- Returns: `[]*TimeEntry`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."time_entries" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateTimeEntryInput`, `target TimeEntryConflictTarget`
- Returns: `*TimeEntry`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated TimeEntryConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."time_entries" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateTimeEntryInput`, `target TimeEntryConflictTarget`
- Returns: `[]*TimeEntry`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same TimeEntryConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."time_entries" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateTimeEntryInput`
- Returns: `*TimeEntry`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."time_entries" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateTimeEntryItem`
- Returns: `[]*TimeEntry`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."time_entries" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *TimeEntryFilter`, `input *UpdateTimeEntryInput`
- Returns: `[]*TimeEntry`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."time_entries" SET <set> WHERE <filter> RETURNING "id"
```

### Increment

- Params: `id uuid.UUID`, `input IncrementInput[TimeEntryIncrementColumn]`
- Returns: `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Atomic single-column arithmetic. The column is restricted to the generated TimeEntryIncrementColumn enum — numeric, and identifying nothing: neither PK, FK, nor the tenant column. A negative amount decrements.

Generated SQL:

postgres:

```sql
UPDATE "public"."time_entries" SET <column> = <column> + $1 WHERE "id" = $2
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."time_entries" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."time_entries" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *TimeEntryFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."time_entries" WHERE <filter> RETURNING "id"
```

## Filter

Type `TimeEntryFilter`.

| Field | Type |
| --- | --- |
| Billable | `*comparator.Bool` |
| CreatedAt | `*comparator.Time` |
| Hours | `*comparator.String` |
| ID | `*comparator.ID` |
| Notes | `*comparator.NullableString` |
| SpentOn | `*comparator.Time` |
| TaskID | `*comparator.ID` |
| UserID | `*comparator.ID` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*TimeEntryFilter` |
| Or | `[]*TimeEntryFilter` |

## Sort

Type `TimeEntrySort`.

Fields: Billable, CreatedAt, Hours, ID, Notes, SpentOn, TaskID, UserID, WorkspaceID
