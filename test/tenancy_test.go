package test

import (
	"net/http"
	"testing"

	"github.com/teandresmith/sqlgen-example/internal/tenancy"
)

// authHeaders sets only the bearer token — a request with an identity but no
// Active Workspace selected.
func authHeaders(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// workspaceHeaders sets the bearer token and selects an Active Workspace via the
// X-Workspace-Id header the tenancy middleware reads.
func workspaceHeaders(token, workspaceID string) map[string]string {
	return map[string]string{
		"Authorization":         "Bearer " + token,
		tenancy.WorkspaceHeader: workspaceID,
	}
}

const bootstrapWorkspaceMutation = `
	mutation ($input: BootstrapWorkspaceInput!) {
		bootstrapWorkspace(input: $input) { id name slug }
	}`

// bootstrapWorkspace founds a Workspace as the token's User (who becomes owner)
// and returns the new Workspace id.
func bootstrapWorkspace(t *testing.T, token, name, slug string) string {
	t.Helper()
	var out struct {
		BootstrapWorkspace struct {
			ID string `json:"id"`
		} `json:"bootstrapWorkspace"`
	}
	gqlExecData(t, bootstrapWorkspaceMutation, map[string]any{
		"input": map[string]any{"name": name, "slug": slug},
	}, authHeaders(token), &out)
	if out.BootstrapWorkspace.ID == "" {
		t.Fatal("bootstrapWorkspace returned empty workspace id")
	}
	return out.BootstrapWorkspace.ID
}

const createProjectMutation = `
	mutation ($input: CreateProjectInput!) {
		createProject(input: $input) { id name workspaceID }
	}`

// createProject creates a Project in the given Active Workspace and returns its
// id. workspaceID in the input must match the selected tenant (the generated
// create rejects a mismatch).
func createProject(t *testing.T, token, workspaceID, name string) string {
	t.Helper()
	var out struct {
		CreateProject struct {
			ID string `json:"id"`
		} `json:"createProject"`
	}
	gqlExecData(t, createProjectMutation, map[string]any{
		"input": map[string]any{
			"workspaceID": workspaceID,
			"name":        name,
			"createdAt":   "2026-07-14T00:00:00Z",
			"updatedAt":   "2026-07-14T00:00:00Z",
		},
	}, workspaceHeaders(token, workspaceID), &out)
	if out.CreateProject.ID == "" {
		t.Fatal("createProject returned empty project id")
	}
	return out.CreateProject.ID
}

// TestBootstrapWorkspaceRecordsOwner proves a User can found a Workspace and is
// recorded as its `owner` via a Membership (AC 1 / PRD story 4). Ownership is
// asserted behaviorally: the newly-created Workspace is immediately selectable
// (the caller is a Member) and the membership query reports the `owner` role.
func TestBootstrapWorkspaceRecordsOwner(t *testing.T) {
	token, user := register(t, "owner@example.com", "found-a-space-1", "Founder")
	wsID := bootstrapWorkspace(t, token, "Acme", "acme-owner")

	// The owner can select the Workspace (proves the Membership exists) and read
	// their own Membership back with role owner.
	const q = `
		query ($ws: UUID!, $u: UUID!) {
			membership(workspaceID: $ws, userID: $u) { role workspaceID userID }
		}`
	var out struct {
		Membership struct {
			Role        string `json:"role"`
			WorkspaceID string `json:"workspaceID"`
			UserID      string `json:"userID"`
		} `json:"membership"`
	}
	gqlExecData(t, q, map[string]any{"ws": wsID, "u": user.ID}, workspaceHeaders(token, wsID), &out)
	if out.Membership.Role != "OWNER" {
		t.Errorf("membership role = %q, want OWNER", out.Membership.Role)
	}
	if out.Membership.WorkspaceID != wsID || out.Membership.UserID != user.ID {
		t.Errorf("membership pk = (%s, %s), want (%s, %s)", out.Membership.WorkspaceID, out.Membership.UserID, wsID, user.ID)
	}
}

// TestActiveWorkspaceSelectsTenant proves X-Workspace-Id selects the Active
// Workspace per request (AC 2 / story 8): a Project created under a selected
// Workspace is auto-scoped to it and reads back through the same header without
// re-authenticating.
func TestActiveWorkspaceSelectsTenant(t *testing.T) {
	token, _ := register(t, "select@example.com", "select-space-1", "Selector")
	wsID := bootstrapWorkspace(t, token, "Selectable", "selectable-ws")
	projID := createProject(t, token, wsID, "Roadmap")

	const q = `query ($id: UUID!) { project(id: $id) { id name workspaceID } }`
	var out struct {
		Project *struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspaceID"`
		} `json:"project"`
	}
	gqlExecData(t, q, map[string]any{"id": projID}, workspaceHeaders(token, wsID), &out)
	if out.Project == nil {
		t.Fatal("project not readable within its own Active Workspace")
	}
	if out.Project.WorkspaceID != wsID {
		t.Errorf("project workspaceID = %q, want %q (auto-scoped to Active Workspace)", out.Project.WorkspaceID, wsID)
	}
}

// TestNonMemberWorkspaceRejected proves a request whose X-Workspace-Id the
// caller is not a Member of is rejected at the middleware with 403 (AC 3 /
// story 9). The outsider owns their own Workspace but has no Membership in the
// target one.
func TestNonMemberWorkspaceRejected(t *testing.T) {
	ownerToken, _ := register(t, "insider@example.com", "insider-pw-123", "Insider")
	wsID := bootstrapWorkspace(t, ownerToken, "Private", "private-ws")

	outsiderToken, _ := register(t, "outsider@example.com", "outsider-pw-123", "Outsider")
	// The outsider must be a real, authenticated User (own a Workspace) so the
	// rejection is about Membership, not authentication.
	_ = bootstrapWorkspace(t, outsiderToken, "Outsiders", "outsiders-ws")

	status := postStatus(t, `{ __typename }`, workspaceHeaders(outsiderToken, wsID))
	if status != http.StatusForbidden {
		t.Errorf("HTTP status = %d, want 403 selecting a workspace the caller is not a member of", status)
	}
}

// TestTenantScopedWithoutWorkspaceFailsClosed proves a tenant-scoped operation
// with no valid Active Workspace is rejected — fail-closed (AC 4 / story 10).
// The caller is authenticated but sends no X-Workspace-Id, so the TenantResolver
// returns ErrMissing and the read errors before any row is returned.
func TestTenantScopedWithoutWorkspaceFailsClosed(t *testing.T) {
	token, _ := register(t, "failclosed@example.com", "fail-closed-pw-1", "FailClosed")

	const q = `query { projectList(limit: 10) { totalCount items { id } } }`
	resp := gqlExec(t, q, nil, authHeaders(token))
	if len(resp.Errors) == 0 {
		t.Fatal("tenant-scoped projectList with no Active Workspace succeeded, want a fail-closed error")
	}
}

// TestCrossTenantIsolation proves a User cannot read another tenant's row (AC 7):
// a Project that lives in Workspace A is invisible when the request selects
// Workspace B — both for the Project's own owner (who belongs to both) and for
// an unrelated User who owns only B. Structural workspace_id scoping, driven by
// the TenantResolver, does the filtering; no resolver opts into cross-tenant reads.
func TestCrossTenantIsolation(t *testing.T) {
	ownerToken, _ := register(t, "multi@example.com", "multi-space-pw-1", "MultiOwner")
	wsA := bootstrapWorkspace(t, ownerToken, "Alpha", "alpha-ws")
	wsB := bootstrapWorkspace(t, ownerToken, "Beta", "beta-ws")

	projInA := createProject(t, ownerToken, wsA, "Alpha Secret")

	const q = `query ($id: UUID!) { project(id: $id) { id } }`

	// Same User, but reading A's row while B is the Active Workspace: not found.
	var fromB struct {
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, q, map[string]any{"id": projInA}, workspaceHeaders(ownerToken, wsB), &fromB)
	if fromB.Project != nil {
		t.Errorf("project in workspace A leaked into workspace B context (id=%s)", fromB.Project.ID)
	}

	// Control: the same row IS visible when A is the Active Workspace.
	var fromA struct {
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, q, map[string]any{"id": projInA}, workspaceHeaders(ownerToken, wsA), &fromA)
	if fromA.Project == nil {
		t.Fatal("project not visible within its own workspace A — scoping is over-filtering")
	}

	// An unrelated User who owns only workspace C cannot see A's row from C.
	strangerToken, _ := register(t, "stranger@example.com", "stranger-pw-123", "Stranger")
	wsC := bootstrapWorkspace(t, strangerToken, "Gamma", "gamma-ws")
	var fromC struct {
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, q, map[string]any{"id": projInA}, workspaceHeaders(strangerToken, wsC), &fromC)
	if fromC.Project != nil {
		t.Errorf("project in workspace A leaked to an unrelated tenant (id=%s)", fromC.Project.ID)
	}
}

// TestCrossTenantWriteRejected proves tenant isolation covers writes, not just
// reads (PRD story 43 / Testing Decisions: "cannot read/mutate Workspace B").
// A User who belongs to both A and B, operating with B as the Active Workspace,
// cannot smuggle a write into A by setting the input's workspaceID to A: the
// resolved tenant (B) and the input tenant (A) disagree, so the create is
// rejected before the row is written.
func TestCrossTenantWriteRejected(t *testing.T) {
	token, _ := register(t, "writer@example.com", "write-iso-pw-1", "Writer")
	wsA := bootstrapWorkspace(t, token, "WriteAlpha", "write-alpha-ws")
	wsB := bootstrapWorkspace(t, token, "WriteBeta", "write-beta-ws")

	// Active Workspace is B, but the input claims A — a cross-tenant write.
	resp := gqlExec(t, createProjectMutation, map[string]any{
		"input": map[string]any{
			"workspaceID": wsA,
			"name":        "Smuggled",
			"createdAt":   "2026-07-14T00:00:00Z",
			"updatedAt":   "2026-07-14T00:00:00Z",
		},
	}, workspaceHeaders(token, wsB))
	if len(resp.Errors) == 0 {
		t.Fatal("createProject writing into a non-active workspace succeeded, want a tenant-mismatch error")
	}

	// And the smuggled row must not exist in A.
	const listInA = `query { projectList(limit: 50) { totalCount items { name } } }`
	var out struct {
		ProjectList struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"projectList"`
	}
	gqlExecData(t, listInA, nil, workspaceHeaders(token, wsA), &out)
	for _, p := range out.ProjectList.Items {
		if p.Name == "Smuggled" {
			t.Error("a cross-tenant write persisted a row into workspace A")
		}
	}
}
