# TaskDependency

- **Table:** `task_dependencies` (schema `public`)
- **Kind:** table

Directed dependencies between tasks (self-referential M2M)

## Files

- `task_dependency_gen.go`

## Primary key

- Kind: composite
- Struct: `TaskDependencyPK`
- `task_id` — TaskID `uuid.UUID`
- `depends_on_task_id` — DependsOnTaskID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `task_id` | TaskID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | The dependent task |
| `depends_on_task_id` | DependsOnTaskID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | The task depended upon |
| `type` | Type | `DependencyType` | `dependency_type` |  |  |  | `'blocks'` | `comparator.Enum[DependencyType]` | Nature of the dependency |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_task_deps_dependson` | depends_on_task_id |  | btree |  |
| `task_dependencies_pkey` | task_id, depends_on_task_id | yes | btree |  |

## Check constraints

- `task_id`: `task_id <> depends_on_task_id`
- `depends_on_task_id`: `task_id <> depends_on_task_id`

## Query methods

### Get

- Params: `pk TaskDependencyPK`
- Returns: `*TaskDependency`, `error`
- Errors: `ErrNotFound`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "depends_on_task_id", "task_id", "type" FROM "public"."task_dependencies" WHERE "task_id" = $1 AND "depends_on_task_id" = $2 LIMIT 1
```

### GetMany

- Params: `input *GetTaskDependenciesInput`
- Returns: `[]*TaskDependency`, `error`

Generated SQL:

postgres:

```sql
SELECT "created_at", "depends_on_task_id", "task_id", "type" FROM "public"."task_dependencies" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *TaskDependencyFilter`
- Returns: `int64`, `error`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."task_dependencies" WHERE <filter>
```

### Exists

- Params: `pk TaskDependencyPK`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_dependencies" WHERE "task_id" = $1 AND "depends_on_task_id" = $2)
```

### ExistsWhere

- Params: `filter *TaskDependencyFilter`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."task_dependencies" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[TaskDependencyFilter]`
- Returns: `*PaginateResult[TaskDependency]`, `error`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[TaskDependencyFilter]`
- Returns: `*Connection[TaskDependency]`, `error`
- Errors: `ErrInvalidCursor`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamTaskDependenciesInput`
- Returns: `iter.Seq2[*TaskDependency, error]`, `error`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "depends_on_task_id", "task_id", "type" FROM "public"."task_dependencies" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateTaskDependencyInput`
- Returns: `*TaskDependency`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_dependencies" (<columns>) VALUES (<values>)
```

### CreateMany

- Params: `inputs []*CreateTaskDependencyInput`
- Returns: `[]*TaskDependency`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_dependencies" (<columns>) VALUES <values>
```

### Upsert

- Params: `input *CreateTaskDependencyInput`, `target TaskDependencyConflictTarget`
- Returns: `*TaskDependency`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: One method over the generated TaskDependencyConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_dependencies" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### UpsertMany

- Params: `inputs []*CreateTaskDependencyInput`, `target TaskDependencyConflictTarget`
- Returns: `[]*TaskDependency`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks over the same TaskDependencyConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."task_dependencies" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `pk TaskDependencyPK`, `input *UpdateTaskDependencyInput`
- Returns: `*TaskDependency`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_dependencies" SET <set> WHERE "task_id" = $1 AND "depends_on_task_id" = $2
```

### UpdateMany

- Params: `items []UpdateTaskDependencyItem`
- Returns: `[]*TaskDependency`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_dependencies" SET <set> WHERE "task_id" = $1 AND "depends_on_task_id" = $2
```

### UpdateWhere

- Params: `filter *TaskDependencyFilter`, `input *UpdateTaskDependencyInput`
- Returns: `[]*TaskDependency`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."task_dependencies" SET <set> WHERE <filter> RETURNING "task_id", "depends_on_task_id"
```

### HardDelete

- Params: `pk TaskDependencyPK`
- Returns: `error`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_dependencies" WHERE "task_id" = $1 AND "depends_on_task_id" = $2
```

### HardDeleteMany

- Params: `pks []TaskDependencyPK`
- Returns: `error`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_dependencies" WHERE ("task_id", "depends_on_task_id") IN (<pks>)
```

### HardDeleteWhere

- Params: `filter *TaskDependencyFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."task_dependencies" WHERE <filter> RETURNING "task_id", "depends_on_task_id"
```

## Filter

Type `TaskDependencyFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| DependsOnTaskID | `*comparator.ID` |
| TaskID | `*comparator.ID` |
| Type | `*comparator.Enum[DependencyType]` |
| And | `[]*TaskDependencyFilter` |
| Or | `[]*TaskDependencyFilter` |

## Sort

Type `TaskDependencySort`.

Fields: CreatedAt, DependsOnTaskID, TaskID, Type
