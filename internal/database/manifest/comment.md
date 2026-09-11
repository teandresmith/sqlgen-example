# Comment

- **Table:** `comments` (schema `public`)
- **Kind:** table

A note left by a user on a task

## Files

- `comment_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique comment identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Owning workspace (tenant column) |
| `task_id` | TaskID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Task the comment belongs to |
| `author_id` | AuthorID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | User who wrote the comment |
| `body` | Body | `string` | `text` |  |  |  |  | `comparator.String` | Comment text |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `comments_pkey` | id | yes | btree |  |
| `idx_comments_task` | task_id |  | btree |  |

## Relationships

| Name | Kind | Target | FK | Filter |
| --- | --- | --- | --- | --- |
| Attachments | o2m | Attachment | public.attachments.entity_id | `entity_type = 'comment'` |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Comment`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "author_id", "body", "created_at", "id", "task_id", "updated_at", "workspace_id" FROM "public"."comments" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetCommentsInput`
- Returns: `[]*Comment`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "author_id", "body", "created_at", "id", "task_id", "updated_at", "workspace_id" FROM "public"."comments" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *CommentFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."comments" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."comments" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *CommentFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."comments" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[CommentFilter]`
- Returns: `*PaginateResult[Comment]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[CommentFilter]`
- Returns: `*Connection[Comment]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamCommentsInput`
- Returns: `iter.Seq2[*Comment, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "author_id", "body", "created_at", "id", "task_id", "updated_at", "workspace_id" FROM "public"."comments" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateCommentInput`
- Returns: `*Comment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."comments" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateCommentInput`
- Returns: `[]*Comment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."comments" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateCommentInput`, `target CommentConflictTarget`
- Returns: `*Comment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated CommentConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."comments" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateCommentInput`, `target CommentConflictTarget`
- Returns: `[]*Comment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same CommentConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."comments" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateCommentInput`
- Returns: `*Comment`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."comments" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateCommentItem`
- Returns: `[]*Comment`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."comments" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *CommentFilter`, `input *UpdateCommentInput`
- Returns: `[]*Comment`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."comments" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."comments" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."comments" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *CommentFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."comments" WHERE <filter> RETURNING "id"
```

## Filter

Type `CommentFilter`.

| Field | Type |
| --- | --- |
| AuthorID | `*comparator.ID` |
| Body | `*comparator.String` |
| CreatedAt | `*comparator.Time` |
| ID | `*comparator.ID` |
| TaskID | `*comparator.ID` |
| UpdatedAt | `*comparator.Time` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*CommentFilter` |
| Or | `[]*CommentFilter` |

## Sort

Type `CommentSort`.

Fields: AuthorID, Body, CreatedAt, ID, TaskID, UpdatedAt, WorkspaceID
