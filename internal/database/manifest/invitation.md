# Invitation

- **Table:** `invitations` (schema `public`)
- **Kind:** table

Pending invitations for users to join a workspace

## Files

- `invitation_gen.go`

## Primary key

- Kind: single
- `id` — ID `uuid.UUID`

## Columns

| Name | Go field | Go type | DB type | Null | PK | Unique | Default | Comparator | Comment |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `id` | ID | `uuid.UUID` | `uuid` |  | yes |  | `gen_random_uuid()` | `comparator.ID` | Unique identifier |
| `workspace_id` | WorkspaceID | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | Workspace the invitee is invited to (tenant column) |
| `email` | Email | `string` | `email` |  |  |  |  | `comparator.String` | Invitee email address |
| `role` | Role | `MembershipRole` | `membership_role` |  |  |  | `'member'` | `comparator.Enum[MembershipRole]` | Role the invitee will receive on acceptance |
| `token` | Token | `string` | `text` |  |  | yes |  | `comparator.String` | Opaque acceptance token |
| `status` | Status | `InvitationStatus` | `invitation_status` |  |  |  | `'pending'` | `comparator.Enum[InvitationStatus]` | Current invitation state |
| `invited_by` | InvitedBy | `uuid.UUID` | `uuid` |  |  |  |  | `comparator.ID` | User who issued the invitation |
| `expires_at` | ExpiresAt | `time.Time` | `timestamptz` |  |  |  |  | `comparator.Time` | When the invitation expires |
| `created_at` | CreatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Row creation timestamp |
| `updated_at` | UpdatedAt | `time.Time` | `timestamptz` |  |  |  | `now()` | `comparator.Time` | Last modification timestamp |

## Indexes

| Name | Columns | Unique | Method | Where |
| --- | --- | --- | --- | --- |
| `idx_invitations_workspace` | workspace_id |  | btree |  |
| `invitations_pkey` | id | yes | btree |  |
| `invitations_token_key` | token | yes | btree |  |

## Query methods

### Get

- Params: `id uuid.UUID`
- Returns: `*Invitation`, `error`
- Errors: `ErrNotFound`, `tenancy.ErrMissing`
- Notes: Delegates to GetMany with a primary-key filter; returns ErrNotFound when no row matches.

Generated SQL:

postgres:

```sql
SELECT "created_at", "email", "expires_at", "id", "invited_by", "role", "status", "token", "updated_at", "workspace_id" FROM "public"."invitations" WHERE "id" = $1 LIMIT 1
```

### GetMany

- Params: `input *GetInvitationsInput`
- Returns: `[]*Invitation`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT "created_at", "email", "expires_at", "id", "invited_by", "role", "status", "token", "updated_at", "workspace_id" FROM "public"."invitations" WHERE <filter> ORDER BY <sort> LIMIT <limit>
```

### Count

- Params: `filter *InvitationFilter`
- Returns: `int64`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT COUNT(*) FROM "public"."invitations" WHERE <filter>
```

### Exists

- Params: `id uuid.UUID`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."invitations" WHERE "id" = $1)
```

### ExistsWhere

- Params: `filter *InvitationFilter`
- Returns: `bool`, `error`
- Errors: `tenancy.ErrMissing`

Generated SQL:

postgres:

```sql
SELECT EXISTS(SELECT 1 FROM "public"."invitations" WHERE <filter>)
```

### Paginate

- Params: `input PaginateInput[InvitationFilter]`
- Returns: `*PaginateResult[Invitation]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Offset pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Connection

- Params: `input ConnectionInput[InvitationFilter]`
- Returns: `*Connection[Invitation]`, `error`
- Errors: `ErrInvalidCursor`, `tenancy.ErrMissing`
- Notes: Relay cursor pagination. Issues no statement of its own — composes Count and GetMany, so no sql_bodies entry.

### Stream

- Params: `input *StreamInvitationsInput`
- Returns: `iter.Seq2[*Invitation, error]`, `error`
- Errors: `tenancy.ErrMissing`
- Notes: Scalars only — no relationship loading, cache always bypassed. Errors surface through the iterator's second value.

Generated SQL:

postgres:

```sql
SELECT "created_at", "email", "expires_at", "id", "invited_by", "role", "status", "token", "updated_at", "workspace_id" FROM "public"."invitations" WHERE <filter> ORDER BY <sort>
```

## Mutation methods

### Create

- Params: `input *CreateInvitationInput`
- Returns: `*Invitation`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`

Generated SQL:

postgres:

```sql
INSERT INTO "public"."invitations" (<columns>) VALUES (<values>) RETURNING "id"
```

### CreateMany

- Params: `inputs []*CreateInvitationInput`
- Returns: `[]*Invitation`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."invitations" (<columns>) VALUES <values> RETURNING "id"
```

### Upsert

- Params: `input *CreateInvitationInput`, `target InvitationConflictTarget`
- Returns: `*Invitation`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: One method over the generated InvitationConflictTarget enum — the target argument selects the conflict columns at call time.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."invitations" (<columns>) VALUES (<values>) ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded> RETURNING "id"
```

### UpsertMany

- Params: `inputs []*CreateInvitationInput`, `target InvitationConflictTarget`
- Returns: `[]*Invitation`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Batched in generation.batch_size chunks over the same InvitationConflictTarget enum Upsert takes. Inputs resolving to one target are deduped before the statement is built, last occurrence wins. Earlier chunks are not rolled back on a later failure — wrap in a transaction for atomicity.

Generated SQL:

postgres:

```sql
INSERT INTO "public"."invitations" (<columns>) VALUES <values> ON CONFLICT (<conflict_target>) DO UPDATE SET <excluded>
```

### Update

- Params: `id uuid.UUID`, `input *UpdateInvitationInput`
- Returns: `*Invitation`, `error`
- Errors: `ErrNotFound`, `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Only fields where IsSet() reports true reach the SET clause; an input with none set issues no UPDATE and returns the row unchanged.

Generated SQL:

postgres:

```sql
UPDATE "public"."invitations" SET <set> WHERE "id" = $1
```

### UpdateMany

- Params: `items []UpdateInvitationItem`
- Returns: `[]*Invitation`, `error`
- Errors: `ErrNilInput`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Issues one statement per item. Batched like CreateMany — earlier items are not rolled back on a later failure. A primary key that does not exist is silently skipped, even under strict_updates.

Generated SQL:

postgres:

```sql
UPDATE "public"."invitations" SET <set> WHERE "id" = $1
```

### UpdateWhere

- Params: `filter *InvitationFilter`, `input *UpdateInvitationInput`
- Returns: `[]*Invitation`, `error`
- Errors: `ErrNilInput`, `ErrEmptyFilter`, `ErrConstraintViolation`, `tenancy.ErrMissing`, `tenancy.ErrMismatch`
- Notes: Idempotent — returns an empty slice when no row matches. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
UPDATE "public"."invitations" SET <set> WHERE <filter> RETURNING "id"
```

### HardDelete

- Params: `id uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — a primary key that does not exist is not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."invitations" WHERE "id" = $1
```

### HardDeleteMany

- Params: `ids []uuid.UUID`
- Returns: `error`
- Errors: `tenancy.ErrMissing`
- Notes: Idempotent — primary keys that do not exist are not an error.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."invitations" WHERE "id" IN (<ids>)
```

### HardDeleteWhere

- Params: `filter *InvitationFilter`
- Returns: `error`
- Errors: `ErrEmptyFilter`, `tenancy.ErrMissing`
- Notes: Idempotent — no match is not an error. ErrEmptyFilter when the filter produces no conditions.

Generated SQL:

postgres:

```sql
DELETE FROM "public"."invitations" WHERE <filter> RETURNING "id"
```

## Filter

Type `InvitationFilter`.

| Field | Type |
| --- | --- |
| CreatedAt | `*comparator.Time` |
| Email | `*comparator.String` |
| ExpiresAt | `*comparator.Time` |
| ID | `*comparator.ID` |
| InvitedBy | `*comparator.ID` |
| Role | `*comparator.Enum[MembershipRole]` |
| Status | `*comparator.Enum[InvitationStatus]` |
| Token | `*comparator.String` |
| UpdatedAt | `*comparator.Time` |
| WorkspaceID | `*comparator.ID` |
| And | `[]*InvitationFilter` |
| Or | `[]*InvitationFilter` |

## Sort

Type `InvitationSort`.

Fields: CreatedAt, Email, ExpiresAt, ID, InvitedBy, Role, Status, Token, UpdatedAt, WorkspaceID
