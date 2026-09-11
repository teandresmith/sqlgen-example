# Membership

- **Table:** `memberships` (schema `public`)
- **Kind:** table

Links a user to a workspace with a role (M2M)

## Files

- `membership_gen.go`

## Primary key

- Kind: composite
- Struct: `MembershipPK`
- `workspace_id` — WorkspaceID `uuid.UUID`
- `user_id` — UserID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | Workspace side of the membership (tenant column) |
| `user_id` | UserID | `uuid.UUID` | `uuid` |  | yes |  |  | `comparator.ID` | User side of the membership |
| `role` | Role | `MembershipRole` | `membership_role` |  |  |  | `'member'` | `comparator.Enum[MembershipRole]` | The user's permission level in this workspace |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | When the user joined the workspace |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `memberships_pkey` | workspace_id, user_id | yes | btree |  |

## Query methods

### Get

- Params: `pk MembershipPK`
- Returns: `*Membership`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "role", "updated_at", "user_id", "workspace_id" FROM "public"."memberships" WHERE "workspace_id" = $1 AND "user_id" = $2 LIMIT 1
```

### GetMany

- Params: `input *GetMembershipsInput`
- Returns: `[]*Membership`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "role", "updated_at", "user_id", "workspace_id" FROM "public"."memberships" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *MembershipFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."memberships" WHERE <filter>
```

### Exists

- Params: `pk MembershipPK`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."memberships" WHERE "workspace_id" = $1 AND "user_id" = $2)
```

### ExistsWhere

- Params: `filter *MembershipFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."memberships" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[MembershipFilter]`
- Returns: `*PaginateResult[Membership]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[MembershipFilter]`
- Returns: `*Connection[Membership]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamMembershipsInput`
- Returns: `iter.Seq2[*Membership, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "role", "updated_at", "user_id", "workspace_id" FROM "public"."memberships" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateMembershipInput`
- Returns: `*Membership`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."memberships" (<columns>) VALUES (<values>)
```

### CreateMany

- Params: `inputs []*CreateMembershipInput`
- Returns: `[]*Membership`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."memberships" (<columns>) VALUES <values>
```

### Upsert

- Params: `input *CreateMembershipInput`, `target MembershipConflictTarget`
- Returns: `*Membership`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated MembershipConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."memberships" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### UpsertMany

- Params: `inputs []*CreateMembershipInput`, `target MembershipConflictTarget`
- Returns: `[]*Membership`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same MembershipConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."memberships" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `pk MembershipPK`, `input *UpdateMembershipInput`
- Returns: `*Membership`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."memberships" SET <set> WHERE "workspace_id" = $1 AND "user_id" = $2
```

### UpdateMany

- Params: `items []UpdateMembershipItem`
- Returns: `[]*Membership`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."memberships" SET <set> WHERE "workspace_id" = $1 AND "user_id" = $2
```

### UpdateWhere

- Params: `filter *MembershipFilter`, `input *UpdateMembershipInput`
- Returns: `[]*Membership`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."memberships" SET <set> WHERE <filter> RETURNING "workspace_id", "user_id"
```

### HardDelete

- Params: `pk MembershipPK`
- Returns: `error`
- Errors: `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."memberships" WHERE "workspace_id" = $1 AND "user_id" = $2
```

### HardDeleteMany

- Params: `pks []MembershipPK`
- Returns: `error`
- Errors: `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."memberships" WHERE ("workspace_id", "user_id") IN (<pks>)
```

### HardDeleteWhere

- Params: `filter *MembershipFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."memberships" WHERE <filter> RETURNING "workspace_id", "user_id"
```

## Filter

Type `MembershipFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| Role | `*comparator.Enum[MembershipRole]` |
| UpdatedAt | `*comparator.Time` |
| UserID | `*comparator.ID` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*MembershipFilter` |
| Or | `[]*MembershipFilter` |

## Sort

Type `MembershipSort`.

Fields: CreatedAt, Role, UpdatedAt, UserID, WorkspaceID
