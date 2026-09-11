# 13 — API tokens & SSO connections

Status: ready-for-human
Blocked by: 04-authorization-membership-admin, 02-identity-authentication

Parent: `.scratch/app-server/PRD.md`

## What to build

Non-interactive authentication and SSO configuration. A User creates an API Token with scopes so a service or script can call the API on their behalf, and creates a service (userless) API Token so automation can authenticate without a personal account; a User revokes an API Token so a leaked credential can be disabled. The auth middleware accepts a valid API Token as an alternative to a bearer JWT (extending ticket 02). A Workspace owner configures an SSO Connection (provider + config); only the configuration is stored — the real SAML/OIDC handshake is out of scope.

## Acceptance criteria

- [x] A User can create an API Token with scopes (story 13)
- [x] A User can create a service (userless) API Token (story 14)
- [x] A User can revoke an API Token so it can no longer authenticate (story 15)
- [x] The auth middleware authenticates a request presenting a valid API Token
- [x] A Workspace owner can configure an SSO Connection (provider + config), stored only (story 12)
- [x] Integration tests exercise token creation, token-authenticated requests, revocation, and SSO configuration

## Blocked by

- 04-authorization-membership-admin
- 02-identity-authentication

## Implementation notes

Non-interactive auth and SSO configuration, layered on issue 02 (auth) and issue
04 (authz), all six acceptance criteria met and exercised end to end.

### API Tokens (stories 13–15)

- **Minting is server-owned.** `token_hash` is already `write_only` (off every
  read surface); on top of that the whole generated write surface for `api_tokens`
  is subtracted in `sqlgen.yml` (create/update families + the single delete). A
  client could otherwise supply its own `token_hash` (a known plaintext), any
  `user_id` (impersonation), or arbitrary expiry. Writes route through the blessed
  `createAPIToken` (personal, story 13), `createServiceAPIToken` (userless, story
  14), and `revokeAPIToken` (story 15) in `internal/graph/api_token.*`.
- **The secret is shown once.** `internal/apitoken` mints a `tskr_`-prefixed,
  256-bit secret and stores only its SHA-256 hash. SHA-256, not bcrypt: the secret
  is high-entropy (so a slow hash buys nothing) and lookup is by hash equality on
  the unique `token_hash` column (which a per-row bcrypt salt would make
  impossible). `createAPIToken` returns the plaintext in a `CreateAPITokenResult`
  exactly once; it is never readable again.
- **Creation is member self-service, not admin-guarded** — a User mints a token to
  act on their own behalf — so `api_tokens` is deliberately absent from the authz
  guarded tables. Revocation authorization is resolver-owned: the token's owner, or
  a Workspace owner/admin, may revoke; the tenant-scoped `Get` makes a token in
  another Workspace invisible.

### Token authentication (AC 4)

`apitoken.Middleware` sits outermost. A bearer credential carrying the `tskr_`
prefix is an API Token: it is looked up by hash (cross-tenant, like the pre-tenant
Membership lookup), checked for expiry, and — because the row already pins the
Workspace and (for a personal token) the User — dispatched straight to the inner
handler with the Active Workspace, and for a personal token the identity + the
owner's *current* Role, established in context. So a token-authenticated request
needs no `X-Workspace-Id` header (the token self-scopes), and a personal token
whose owner has lost their Membership is rejected fail-closed. Everything without
the prefix falls through unchanged to the JWT + header-tenancy chain. A service
token carries the Workspace but no Role, so it does ordinary member work yet is
fail-closed on the role-guarded admin surfaces. `last_used_at` is stamped
best-effort (SkipHooks, so no Activity spam / cache churn).

### SSO Connections (story 12)

`configureSSOConnection` (`internal/graph/sso_connection.*`) upserts on the unique
`(workspace_id, provider)` pair — "configure" is insert-or-update-that-provider —
with the tenant auto-injected. `sso_connections` is added to the authz guarded
tables (+ matching `selfAuthorized` case), so configure and the retained generated
delete are owner/admin-only; the raw create/update families are subtracted. Only
the configuration is stored — no SAML/OIDC handshake (out of scope). The optional
`enabled` flag exposes the schema's `enabled` column so a connection is not stuck
disabled-forever; omitting it satisfies the literal "provider + config".

### Scope notes

`expires_at` is honored in the middleware (an expired credential is not "valid") —
this enforces token *validity*, distinct from the token *scope* enforcement the PRD
puts out of scope. Token scopes are recorded but never checked, per the PRD.

Only the `apitoken` package, the authz guard, the `sqlgen.yml` masks, the blessed
mutations, and tests were hand-written; then `make generate`. `go build ./... &&
go vet ./... && go test ./...` all green. Reviewed on both the Standards and Spec
axes via `/code-review` (no hard findings on either).
