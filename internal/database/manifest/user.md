# User

- **Table:** `users` (schema `public`)
- **Kind:** table

Global user identity, spanning workspaces

## Files

- `user_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique user identifier |
| `email` | Email | `string` | `text` |  |  | yes |  | `comparator.String` | Login email address (unique across the system) |
| `display_name` | DisplayName | `string` | `text` |  |  |  |  | `comparator.String` | Name shown in the UI |
| `password_hash` | PasswordHash | `string` | `text` |  |  |  |  | `comparator.String` | Hashed credential for dev login |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `users_email_key` | email | yes | btree |  |
| `users_pkey` | id | yes | btree |  |

## Relationships

| Name | Kind | Target | FK | Filter |
| --- | --- | --- | --- | --- |
| activities | o2m | Activity | public.activity.actor_id |  |
| api_tokens | o2m | APIToken | public.api_tokens.user_id |  |
| attachments | o2m | Attachment | public.attachments.uploaded_by |  |
| comments | o2m | Comment | public.comments.author_id |  |
| invitations | o2m | Invitation | public.invitations.invited_by |  |
| reporter_tasks | o2m | Task | public.tasks.reporter_id |  |
| task_assignees | m2m | Task | public.task_assignees (user_id → task_id) |  |
| task_watchers | m2m | Task | public.task_watchers (user_id → task_id) |  |
| teams | m2m | Team | public.team_members (user_id → team_id) |  |
| time_entries | o2m | TimeEntry | public.time_entries.user_id |  |
| workspaces | m2m | Workspace | public.memberships (user_id → workspace_id) |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*User`, `error`
- Errors: `ErrNotFound`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "display_name", "email", "id", "password_hash", "updated_at" FROM "public"."users" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetUsersInput`
- Returns: `[]*User`, `error`

Generated SQL:

postgres:

```sql
SELECT "created_at", "display_name", "email", "id", "password_hash", "updated_at" FROM "public"."users" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *UserFilter`
- Returns: `int64`, `error`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."users" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."users" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *UserFilter`
- Returns: `bool`, `error`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."users" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[UserFilter]`
- Returns: `*PaginateResult[User]`, `error`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[UserFilter]`
- Returns: `*Connection[User]`, `error`
- Errors: `ErrInvalidCursor`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamUsersInput`
- Returns: `iter.Seq2[*User, error]`, `error`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "display_name", "email", "id", "password_hash", "updated_at" FROM "public"."users" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateUserInput`
- Returns: `*User`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."users" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateUserInput`
- Returns: `[]*User`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."users" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateUserInput`, `target UserConflictTarget`
- Returns: `*User`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: One method over the generated UserConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."users" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateUserInput`, `target UserConflictTarget`
- Returns: `[]*User`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Batched in generation.batch_size chunks over the same UserConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."users" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateUserInput`
- Returns: `*User`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."users" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateUserItem`
- Returns: `[]*User`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."users" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *UserFilter`, `input *UpdateUserInput`
- Returns: `[]*User`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."users" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."users" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."users" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *UserFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."users" WHERE <filter> RETURNING "id"
```

### UpdateWithRelated

- Params: `id uuid.UUID`, `input *UpdateUserWithRelatedInput`
- Returns: `*User`, `error`
- Errors: `ErrNilInput`, `ErrNotFound`, `ErrAlreadyRelated`, `ErrNestedVerbConflict`, `ErrConstraintViolation`
- Notes: Writes the parent and its nested rows in one transaction — a SAVEPOINT when ctx already holds one. Issues no statement of its own: it composes Update and, per eligible relationship (APITokens, Activities, Attachments, Comments, Invitations, ReporterTasks, TimeEntries), the target's and junction's own client methods, so no sql_bodies entry. Each inner call fires its own hooks, events and cache invalidation under its own op; the nested method has none.

## Filter

Type `UserFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| DisplayName | `*comparator.String` |
| Email | `*comparator.String` |
| ID | `*comparator.ID` |
| PasswordHash | `*comparator.String` |
| UpdatedAt | `*comparator.Time` |
| And | `[]*UserFilter` |
| Or | `[]*UserFilter` |

## Sort

Type `UserSort`.

Fields: CreatedAt, DisplayName, Email, ID, PasswordHash, UpdatedAt
