package test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/teandresmith/sqlgen-example/internal/apitoken"
)

// This file proves issue 13 (API Tokens & SSO Connections) through the one seam
// the harness exercises — the GraphQL HTTP endpoint. API Tokens are minted by the
// blessed createAPIToken / createServiceAPIToken mutations (stories 13–14), which
// return the plaintext secret exactly once; the auth middleware then accepts that
// secret as an alternative to a bearer JWT, self-scoping the request to the
// token's Workspace. revokeAPIToken disables a token (story 15). SSO is configured
// by the blessed, owner/admin-guarded configureSSOConnection (story 12), which
// stores only the provider configuration.

const createAPITokenMutation = `
	mutation ($input: CreateAPITokenInput!) {
		createAPIToken(input: $input) {
			apiToken { id workspaceID userID name scopes expiresAt }
			secret
		}
	}`

const createServiceAPITokenMutation = `
	mutation ($input: CreateAPITokenInput!) {
		createServiceAPIToken(input: $input) {
			apiToken { id workspaceID userID name scopes }
			secret
		}
	}`

const revokeAPITokenMutation = `mutation ($id: UUID!) { revokeAPIToken(id: $id) }`

const configureSSOMutation = `
	mutation ($input: ConfigureSSOConnectionInput!) {
		configureSSOConnection(input: $input) { id workspaceID provider enabled config }
	}`

type apiTokenMeta struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspaceID"`
	UserID      *string  `json:"userID"`
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   *string  `json:"expiresAt"`
}

type createAPITokenResult struct {
	APIToken apiTokenMeta `json:"apiToken"`
	Secret   string       `json:"secret"`
}

// tokenHeaders presents an API Token secret as a bearer credential — and, unlike
// workspaceHeaders, sets no X-Workspace-Id: the token is self-scoping, so the
// middleware establishes the Active Workspace from the token itself.
func tokenHeaders(secret string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + secret}
}

// createPersonalToken mints a personal API Token as the caller in wsID and
// asserts success, returning the metadata and the one-time secret.
func createPersonalToken(t *testing.T, jwt, wsID, name string, scopes []string) createAPITokenResult {
	t.Helper()
	input := map[string]any{"name": name}
	if scopes != nil {
		input["scopes"] = scopes
	}
	var out struct {
		CreateAPIToken createAPITokenResult `json:"createAPIToken"`
	}
	gqlExecData(t, createAPITokenMutation, map[string]any{"input": input}, workspaceHeaders(jwt, wsID), &out)
	if out.CreateAPIToken.APIToken.ID == "" {
		t.Fatal("createAPIToken returned empty token id")
	}
	if !strings.HasPrefix(out.CreateAPIToken.Secret, apitoken.TokenPrefix) {
		t.Fatalf("createAPIToken secret %q lacks the API-token prefix", out.CreateAPIToken.Secret)
	}
	return out.CreateAPIToken
}

// TestUserCreatesPersonalAPIToken proves a User can create a personal API Token
// with scopes (story 13). The secret is returned once and prefixed; the token is
// owned by the caller and auto-scoped to the Active Workspace; scopes round-trip.
func TestUserCreatesPersonalAPIToken(t *testing.T) {
	jwt, owner := register(t, "pat-owner@example.com", "pat-pw-000001", "PAT Owner")
	wsID := bootstrapWorkspace(t, jwt, "PATSpace", "pat-space")

	res := createPersonalToken(t, jwt, wsID, "ci-runner", []string{"tasks:read", "tasks:write"})
	if res.APIToken.WorkspaceID != wsID {
		t.Errorf("token workspaceID = %q, want %q (auto-scoped to Active Workspace)", res.APIToken.WorkspaceID, wsID)
	}
	if res.APIToken.UserID == nil || *res.APIToken.UserID != owner.ID {
		t.Errorf("token userID = %v, want owner %s (a personal token is owned by its creator)", res.APIToken.UserID, owner.ID)
	}
	if res.APIToken.Name != "ci-runner" {
		t.Errorf("token name = %q, want ci-runner", res.APIToken.Name)
	}
	if len(res.APIToken.Scopes) != 2 || res.APIToken.Scopes[0] != "tasks:read" || res.APIToken.Scopes[1] != "tasks:write" {
		t.Errorf("token scopes = %v, want [tasks:read tasks:write]", res.APIToken.Scopes)
	}
}

// TestUserCreatesServiceAPIToken proves a User can create a service (userless) API
// Token (story 14): the token carries no owning User (userID is null) but is still
// scoped to the Active Workspace, and its secret is returned once.
func TestUserCreatesServiceAPIToken(t *testing.T) {
	jwt, _ := register(t, "svc-owner@example.com", "svc-pw-000001", "Svc Owner")
	wsID := bootstrapWorkspace(t, jwt, "SvcSpace", "svc-space")

	var out struct {
		CreateServiceAPIToken createAPITokenResult `json:"createServiceAPIToken"`
	}
	gqlExecData(t, createServiceAPITokenMutation,
		map[string]any{"input": map[string]any{"name": "nightly-job"}},
		workspaceHeaders(jwt, wsID), &out)

	res := out.CreateServiceAPIToken
	if res.APIToken.ID == "" || res.Secret == "" {
		t.Fatal("createServiceAPIToken returned empty token id or secret")
	}
	if res.APIToken.UserID != nil {
		t.Errorf("service token userID = %v, want null (a service token has no owning User)", *res.APIToken.UserID)
	}
	if res.APIToken.WorkspaceID != wsID {
		t.Errorf("service token workspaceID = %q, want %q", res.APIToken.WorkspaceID, wsID)
	}
}

// TestPersonalAPITokenAuthenticatesRequest proves the auth middleware
// authenticates a request presenting a valid API Token (AC 4). Presenting ONLY
// the token — no bearer JWT, no X-Workspace-Id — the request both resolves the
// caller's identity (me returns the owner) and reads a Workspace-scoped resource
// the token self-scoped to, exactly as a JWT + X-Workspace-Id request would.
func TestPersonalAPITokenAuthenticatesRequest(t *testing.T) {
	jwt, owner := register(t, "pat-auth@example.com", "pat-pw-000002", "PAT Auth")
	wsID := bootstrapWorkspace(t, jwt, "PATAuth", "pat-auth-ws")
	projID := createProject(t, jwt, wsID, "Token-visible project")

	res := createPersonalToken(t, jwt, wsID, "reader", nil)
	h := tokenHeaders(res.Secret)

	// Identity: `me` resolves the token's owner with no JWT present.
	var meOut struct {
		Me struct {
			ID string `json:"id"`
		} `json:"me"`
	}
	gqlExecData(t, `query { me { id } }`, nil, h, &meOut)
	if meOut.Me.ID != owner.ID {
		t.Errorf("me via API token = %q, want owner %s", meOut.Me.ID, owner.ID)
	}

	// Tenant-scoped read: the token self-scopes to its Workspace, so the project is
	// visible without an X-Workspace-Id header.
	var projOut struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, `query ($id: UUID!) { project(id: $id) { id } }`, map[string]any{"id": projID}, h, &projOut)
	if projOut.Project.ID != projID {
		t.Errorf("project via API token = %q, want %s", projOut.Project.ID, projID)
	}
}

// TestServiceAPITokenAuthenticatesAndIsFailClosedOnAdmin proves a service token
// authenticates for ordinary member work but is fail-closed on the role-guarded
// administrative surfaces: it carries an Active Workspace but no Role, so it can
// create a Project (unguarded) yet cannot create a Team (owner/admin-guarded).
func TestServiceAPITokenAuthenticatesAndIsFailClosedOnAdmin(t *testing.T) {
	jwt, _ := register(t, "svc-auth@example.com", "svc-pw-000002", "Svc Auth")
	wsID := bootstrapWorkspace(t, jwt, "SvcAuth", "svc-auth-ws")

	var out struct {
		CreateServiceAPIToken createAPITokenResult `json:"createServiceAPIToken"`
	}
	gqlExecData(t, createServiceAPITokenMutation,
		map[string]any{"input": map[string]any{"name": "automation"}},
		workspaceHeaders(jwt, wsID), &out)
	h := tokenHeaders(out.CreateServiceAPIToken.Secret)

	// Member-level write succeeds: the service token acts in the Workspace.
	var projOut struct {
		CreateProject struct {
			ID string `json:"id"`
		} `json:"createProject"`
	}
	gqlExecData(t, createProjectMutation, map[string]any{
		"input": map[string]any{
			"workspaceID": wsID,
			"name":        "svc-made",
			"createdAt":   "2026-09-01T00:00:00Z",
			"updatedAt":   "2026-09-01T00:00:00Z",
		},
	}, h, &projOut)
	if projOut.CreateProject.ID == "" {
		t.Fatal("service token could not create a project; token did not authenticate for member work")
	}

	// Admin-level write is refused: a service token has no Role, so the authz guard
	// on the Team catalog rejects it (fail-closed).
	teamResp := gqlExec(t, createTeamMutation,
		map[string]any{"input": teamInput(wsID, "svc-team", "svc-team")}, h)
	if len(teamResp.Errors) == 0 {
		t.Error("a service token created a Team; role-guarded surface not fail-closed for a userless token")
	}
}

// TestRevokedAPITokenCannotAuthenticate proves a revoked token can no longer
// authenticate (story 15): the token works, the owner revokes it, and a request
// presenting it is then rejected fail-closed at the transport layer (401).
func TestRevokedAPITokenCannotAuthenticate(t *testing.T) {
	jwt, _ := register(t, "revoke@example.com", "rev-pw-000001", "Revoker")
	wsID := bootstrapWorkspace(t, jwt, "RevokeSpace", "revoke-space")
	res := createPersonalToken(t, jwt, wsID, "doomed", nil)
	h := tokenHeaders(res.Secret)

	// It authenticates before revocation.
	var meOut struct {
		Me struct {
			ID string `json:"id"`
		} `json:"me"`
	}
	gqlExecData(t, `query { me { id } }`, nil, h, &meOut)
	if meOut.Me.ID == "" {
		t.Fatal("token did not authenticate before revocation, control broken")
	}

	// Revoke it (as the owner, over a normal JWT + Active Workspace request).
	var revOut struct {
		RevokeAPIToken bool `json:"revokeAPIToken"`
	}
	gqlExecData(t, revokeAPITokenMutation, map[string]any{"id": res.APIToken.ID}, workspaceHeaders(jwt, wsID), &revOut)
	if !revOut.RevokeAPIToken {
		t.Fatal("revokeAPIToken returned false, want true")
	}

	// Presenting the revoked token is now rejected fail-closed with 401.
	if status := postStatus(t, `query { me { id } }`, h); status != http.StatusUnauthorized {
		t.Errorf("request with revoked token → HTTP %d, want 401", status)
	}
}

// TestInvalidAPITokenRejected proves a well-formed-but-unknown API Token is
// rejected fail-closed (401), so a fabricated credential cannot slip through.
func TestInvalidAPITokenRejected(t *testing.T) {
	bogus := apitoken.TokenPrefix + "this-token-was-never-issued"
	if status := postStatus(t, `query { me { id } }`, tokenHeaders(bogus)); status != http.StatusUnauthorized {
		t.Errorf("request with fabricated API token → HTTP %d, want 401", status)
	}
}

// TestAPITokenIsScopedToItsWorkspace proves an API Token is confined to the
// Workspace it was minted in: a token for Workspace A cannot read a resource in
// Workspace B, even though both belong to the same User. The token self-scopes to
// A, so B's project is invisible (tenant isolation), never leaked by the token.
func TestAPITokenIsScopedToItsWorkspace(t *testing.T) {
	jwt, _ := register(t, "pat-iso@example.com", "pat-pw-000003", "PAT Iso")
	wsA := bootstrapWorkspace(t, jwt, "PATIsoA", "pat-iso-a")
	wsB := bootstrapWorkspace(t, jwt, "PATIsoB", "pat-iso-b")
	projB := createProject(t, jwt, wsB, "B-only project")

	tokenA := createPersonalToken(t, jwt, wsA, "a-scoped", nil)

	// The A-scoped token reading B's project sees nothing (tenant-scoped to A).
	var out struct {
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, `query ($id: UUID!) { project(id: $id) { id } }`, map[string]any{"id": projB}, tokenHeaders(tokenA.Secret), &out)
	if out.Project != nil {
		t.Errorf("A-scoped token read B's project %s; API token not confined to its Workspace", projB)
	}
}

// TestMemberRevokesOwnTokenButNotAnothers proves revocation authorization: a plain
// member may revoke a token they own, but not a token owned by someone else (only
// the owner or a Workspace owner/admin may). The owner (a manager) can revoke any
// token — the control that the rejection is about authorization, not a broken path.
func TestMemberRevokesOwnTokenButNotAnothers(t *testing.T) {
	ownerJWT, _ := register(t, "patrev-owner@example.com", "rev-pw-000002", "Rev Owner")
	wsID := bootstrapWorkspace(t, ownerJWT, "RevAuth", "rev-auth-ws")

	memberJWT, member := register(t, "patrev-member@example.com", "rev-pw-000003", "Rev Member")
	joinWorkspace(t, ownerJWT, wsID, memberJWT, member.Email, "MEMBER")

	memberTok := createPersonalToken(t, memberJWT, wsID, "member-own", nil)
	ownerTok := createPersonalToken(t, ownerJWT, wsID, "owner-own", nil)

	// A member cannot revoke a token they do not own.
	resp := gqlExec(t, revokeAPITokenMutation, map[string]any{"id": ownerTok.APIToken.ID}, workspaceHeaders(memberJWT, wsID))
	if len(resp.Errors) == 0 {
		t.Error("a member revoked another user's token; revocation authorization not enforced")
	}

	// A member CAN revoke their own token.
	var ownOut struct {
		RevokeAPIToken bool `json:"revokeAPIToken"`
	}
	gqlExecData(t, revokeAPITokenMutation, map[string]any{"id": memberTok.APIToken.ID}, workspaceHeaders(memberJWT, wsID), &ownOut)
	if !ownOut.RevokeAPIToken {
		t.Error("a member could not revoke their own token")
	}

	// And an owner/admin can revoke anyone's token.
	var mgrOut struct {
		RevokeAPIToken bool `json:"revokeAPIToken"`
	}
	gqlExecData(t, revokeAPITokenMutation, map[string]any{"id": ownerTok.APIToken.ID}, workspaceHeaders(ownerJWT, wsID), &mgrOut)
	if !mgrOut.RevokeAPIToken {
		t.Error("an owner could not revoke a token, control broken")
	}
}

// TestTokenHashNotExposed proves the token secret is off the read surface: the
// APIToken type carries no tokenHash field (write_only access, PRD §32), so a
// leaked read can never recover a credential — the plaintext is only ever the
// one-time createAPIToken result.
func TestTokenHashNotExposed(t *testing.T) {
	const q = `{ __type(name: "APIToken") { fields { name } } }`
	var out struct {
		Type struct {
			Fields []struct {
				Name string `json:"name"`
			} `json:"fields"`
		} `json:"__type"`
	}
	gqlExecData(t, q, nil, nil, &out)
	for _, f := range out.Type.Fields {
		if f.Name == "tokenHash" {
			t.Error("APIToken.tokenHash is exposed on the GraphQL API; the hashed secret must not be readable")
		}
	}
}

type ssoConnResult struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceID"`
	Provider    string         `json:"provider"`
	Enabled     bool           `json:"enabled"`
	Config      map[string]any `json:"config"`
}

// TestOwnerConfiguresSSOConnection proves a Workspace owner can configure an SSO
// Connection (provider + config), stored only (story 12), and that re-configuring
// the same provider upserts the single connection rather than adding a second.
func TestOwnerConfiguresSSOConnection(t *testing.T) {
	jwt, _ := register(t, "sso-owner@example.com", "sso-pw-000001", "SSO Owner")
	wsID := bootstrapWorkspace(t, jwt, "SSOSpace", "sso-space")

	var out struct {
		ConfigureSSOConnection ssoConnResult `json:"configureSSOConnection"`
	}
	gqlExecData(t, configureSSOMutation, map[string]any{
		"input": map[string]any{
			"provider": "OKTA",
			"config":   map[string]any{"metadataURL": "https://okta.example.com/metadata"},
			"enabled":  true,
		},
	}, workspaceHeaders(jwt, wsID), &out)

	conn := out.ConfigureSSOConnection
	if conn.WorkspaceID != wsID || conn.Provider != "OKTA" || !conn.Enabled {
		t.Errorf("configured connection = %+v, want workspace %s / OKTA / enabled", conn, wsID)
	}
	if conn.Config["metadataURL"] != "https://okta.example.com/metadata" {
		t.Errorf("configured connection config = %v, want the stored metadataURL", conn.Config)
	}

	// Re-configuring the same provider updates the existing connection (upsert on
	// the unique (workspace_id, provider) pair): same id, new config, now disabled.
	var reconf struct {
		ConfigureSSOConnection ssoConnResult `json:"configureSSOConnection"`
	}
	gqlExecData(t, configureSSOMutation, map[string]any{
		"input": map[string]any{
			"provider": "OKTA",
			"config":   map[string]any{"metadataURL": "https://okta.example.com/v2"},
			"enabled":  false,
		},
	}, workspaceHeaders(jwt, wsID), &reconf)
	if reconf.ConfigureSSOConnection.ID != conn.ID {
		t.Errorf("re-configure created a new connection (id %s vs %s); want an upsert on the same row",
			reconf.ConfigureSSOConnection.ID, conn.ID)
	}
	if reconf.ConfigureSSOConnection.Enabled {
		t.Error("re-configure did not update enabled → false")
	}

	// Exactly one connection exists for the Workspace after the two calls.
	var list struct {
		SsoConnectionList struct {
			TotalCount int `json:"totalCount"`
		} `json:"ssoConnectionList"`
	}
	gqlExecData(t, `query { ssoConnectionList(limit: 10) { totalCount } }`, nil, workspaceHeaders(jwt, wsID), &list)
	if list.SsoConnectionList.TotalCount != 1 {
		t.Errorf("workspace has %d SSO connections after two configures of one provider, want 1 (upsert)", list.SsoConnectionList.TotalCount)
	}
}

// TestSSOConfigurationGuardedByRole proves configuring SSO is owner/admin-only,
// enforced by the authz hook on sso_connections (story 12): a plain member is
// rejected, while the owner (the control) succeeds.
func TestSSOConfigurationGuardedByRole(t *testing.T) {
	ownerJWT, _ := register(t, "sso-guard-owner@example.com", "sso-pw-000002", "SSO Guard Owner")
	wsID := bootstrapWorkspace(t, ownerJWT, "SSOGuard", "sso-guard-ws")

	memberJWT, member := register(t, "sso-guard-member@example.com", "sso-pw-000003", "SSO Guard Member")
	joinWorkspace(t, ownerJWT, wsID, memberJWT, member.Email, "MEMBER")

	input := map[string]any{
		"input": map[string]any{
			"provider": "GOOGLE",
			"config":   map[string]any{"clientID": "abc"},
			"enabled":  true,
		},
	}

	// A plain member cannot configure SSO — guarded by the authz hook.
	memberResp := gqlExec(t, configureSSOMutation, input, workspaceHeaders(memberJWT, wsID))
	if len(memberResp.Errors) == 0 {
		t.Error("a member configured SSO; owner/admin-only operation not guarded (story 12)")
	}

	// Control: the owner CAN configure it, so the rejection is about Role.
	var out struct {
		ConfigureSSOConnection ssoConnResult `json:"configureSSOConnection"`
	}
	gqlExecData(t, configureSSOMutation, input, workspaceHeaders(ownerJWT, wsID), &out)
	if out.ConfigureSSOConnection.ID == "" {
		t.Error("owner SSO configuration failed, control broken")
	}
}
