package test

import (
	"testing"
)

// This file proves issue 12 (Teams) through the one seam the harness exercises —
// the GraphQL HTTP endpoint under an Active Workspace. Team creation is the
// generated createTeam mutation, guarded owner/admin-only by the authz hook
// (story 16). Managing a Team's membership drives the blessed addUserToTeam /
// removeUserFromTeam mutations (story 17), also owner/admin-guarded and tenant-
// scoped on the Team. Assigning a Project to a Team drives the generated nested
// updateTeamWithRelated mutation's projects.connect (story 18), which any member
// may call and which resolves both endpoints tenant-scoped; the composite FK
// (migration 0005) backstops the raw updateProject path at the database.

const createTeamMutation = `
	mutation ($input: CreateTeamInput!) {
		createTeam(input: $input) { id workspaceID name slug }
	}`

const addUserToTeamMutation = `
	mutation ($teamID: UUID!, $userID: UUID!) {
		addUserToTeam(teamID: $teamID, userID: $userID) { teamID userID }
	}`

const removeUserFromTeamMutation = `
	mutation ($teamID: UUID!, $userID: UUID!) {
		removeUserFromTeam(teamID: $teamID, userID: $userID)
	}`

// assignProjectToTeamMutation assigns a Project to a Team through the generated
// nested update: projects.connect reads the Project tenant-scoped, then sets its
// team_id (allow_reparent in sqlgen.yml lets it move between Teams).
const assignProjectToTeamMutation = `
	mutation ($projectID: UUID!, $teamID: UUID!) {
		updateTeamWithRelated(id: $teamID, input: { projects: { connect: [$projectID] } }) {
			id
			projects { id teamID }
		}
	}`

// renameTeamNestedMutation sets a Team column through the same nested mutation —
// a write on the Team catalog itself, which stays owner/admin-only.
const renameTeamNestedMutation = `
	mutation ($teamID: UUID!, $name: String!) {
		updateTeamWithRelated(id: $teamID, input: { team: { name: $name } }) { id name }
	}`

// assignedTeam is the updateTeamWithRelated payload assignProjectToTeam selects:
// the Team and the Projects it now owns.
type assignedTeam struct {
	UpdateTeamWithRelated struct {
		ID       string `json:"id"`
		Projects []struct {
			ID     string  `json:"id"`
			TeamID *string `json:"teamID"`
		} `json:"projects"`
	} `json:"updateTeamWithRelated"`
}

// readProjectTeam reads a Project's teamID back through project(id).
func readProjectTeam(t *testing.T, token, wsID, projectID string) *string {
	t.Helper()
	var back struct {
		Project struct {
			TeamID *string `json:"teamID"`
		} `json:"project"`
	}
	gqlExecData(t, projectTeamQuery, map[string]any{"id": projectID}, workspaceHeaders(token, wsID), &back)
	return back.Project.TeamID
}

// updateProjectTeamMutation drives the RAW generated updateProject to set teamID —
// the path the composite FK (migration 0005) has to backstop, since updateProject
// is tenant-scoped only on the Project, not on the referenced Team.
const updateProjectTeamMutation = `
	mutation ($id: UUID!, $teamID: UUID) {
		updateProject(id: $id, input: { teamID: $teamID }) { id teamID }
	}`

const teamUsersQuery = `query ($id: UUID!) { team(id: $id) { users { id } } }`
const projectTeamQuery = `query ($id: UUID!) { project(id: $id) { teamID } }`

type teamResult struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceID"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
}

// teamInput builds a valid createTeam input for the given Active Workspace. name
// and slug are caller-supplied; the rest are the required scalar columns.
func teamInput(wsID, name, slug string) map[string]any {
	return map[string]any{
		"workspaceID": wsID,
		"name":        name,
		"slug":        slug,
		"createdAt":   "2026-09-01T00:00:00Z",
		"updatedAt":   "2026-09-01T00:00:00Z",
	}
}

// createTeam drives the generated createTeam mutation as the given caller and
// asserts success, returning the new Team. The caller must hold a manager Role
// (owner/admin) — the authz hook rejects a plain member.
func createTeam(t *testing.T, token, wsID, name, slug string) teamResult {
	t.Helper()
	var out struct {
		CreateTeam teamResult `json:"createTeam"`
	}
	gqlExecData(t, createTeamMutation, map[string]any{"input": teamInput(wsID, name, slug)},
		workspaceHeaders(token, wsID), &out)
	if out.CreateTeam.ID == "" {
		t.Fatal("createTeam returned empty team id")
	}
	return out.CreateTeam
}

// teamUserIDs reads back the Users on a Team through the m2m relationship.
func teamUserIDs(t *testing.T, token, wsID, teamID string) []string {
	t.Helper()
	var out struct {
		Team struct {
			Users []struct {
				ID string `json:"id"`
			} `json:"users"`
		} `json:"team"`
	}
	gqlExecData(t, teamUsersQuery, map[string]any{"id": teamID}, workspaceHeaders(token, wsID), &out)
	ids := make([]string, 0, len(out.Team.Users))
	for _, u := range out.Team.Users {
		ids = append(ids, u.ID)
	}
	return ids
}

// TestAdminCreatesTeam proves a Workspace admin can create a Team within a
// Workspace (story 16). The owner (a manager) creates it, workspaceID is
// auto-scoped to the Active Workspace, and name/slug round-trip.
func TestAdminCreatesTeam(t *testing.T) {
	ownerToken, _ := register(t, "team-owner@example.com", "team-pw-00001", "Team Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Teaming", "teaming-ws")

	team := createTeam(t, ownerToken, wsID, "Platform", "platform")
	if team.WorkspaceID != wsID {
		t.Errorf("team workspaceID = %q, want %q (auto-scoped to Active Workspace)", team.WorkspaceID, wsID)
	}
	if team.Name != "Platform" || team.Slug != "platform" {
		t.Errorf("team name/slug = %q/%q, want Platform/platform", team.Name, team.Slug)
	}
}

// TestTeamCreationGuardedByRole proves creating a Team is owner/admin-only,
// enforced by the authz hook (story 16): a plain member is rejected creating a
// Team, while the owner (the control) succeeds. Mirrors the Cycle/Label catalog
// guard.
func TestTeamCreationGuardedByRole(t *testing.T) {
	ownerToken, _ := register(t, "team-guard-owner@example.com", "team-pw-00002", "Team Guard Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "TeamGuard", "team-guard-ws")

	memberToken, member := register(t, "team-guard-member@example.com", "team-pw-00003", "Team Guard Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// A plain member cannot create a Team — guarded by the authz hook.
	memberResp := gqlExec(t, createTeamMutation, map[string]any{"input": teamInput(wsID, "member-made", "member-made")},
		workspaceHeaders(memberToken, wsID))
	if len(memberResp.Errors) == 0 {
		t.Error("a member was allowed to create a team; owner/admin-only operation not guarded (story 16)")
	}

	// Control: the owner CAN create a Team, so the rejection is about Role.
	if team := createTeam(t, ownerToken, wsID, "owner-made", "owner-made"); team.ID == "" {
		t.Error("owner team create failed, control broken")
	}
}

// TestAdminAddsAndRemovesUsersOnTeam proves a Workspace admin can add and remove
// Users on a Team (story 17): the owner adds a member User to a Team, the Team's
// users relationship reflects it, then the owner removes them and the
// relationship is empty again. Read-back is through the HTTP seam both times.
func TestAdminAddsAndRemovesUsersOnTeam(t *testing.T) {
	ownerToken, _ := register(t, "team-mgmt-owner@example.com", "team-pw-00004", "Team Mgmt Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "TeamMgmt", "team-mgmt-ws")
	team := createTeam(t, ownerToken, wsID, "Backend", "backend")

	memberToken, member := register(t, "team-mgmt-member@example.com", "team-pw-00005", "Team Mgmt Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// Add the member to the Team.
	var addOut struct {
		AddUserToTeam struct {
			TeamID string `json:"teamID"`
			UserID string `json:"userID"`
		} `json:"addUserToTeam"`
	}
	gqlExecData(t, addUserToTeamMutation, map[string]any{"teamID": team.ID, "userID": member.ID},
		workspaceHeaders(ownerToken, wsID), &addOut)
	if addOut.AddUserToTeam.UserID != member.ID || addOut.AddUserToTeam.TeamID != team.ID {
		t.Fatalf("addUserToTeam = %+v, want team %s / user %s", addOut.AddUserToTeam, team.ID, member.ID)
	}

	// The Team's users relationship now includes the member.
	if ids := teamUserIDs(t, ownerToken, wsID, team.ID); len(ids) != 1 || ids[0] != member.ID {
		t.Fatalf("team.users after add = %v, want [%s]", ids, member.ID)
	}

	// Remove the member from the Team.
	var rmOut struct {
		RemoveUserFromTeam bool `json:"removeUserFromTeam"`
	}
	gqlExecData(t, removeUserFromTeamMutation, map[string]any{"teamID": team.ID, "userID": member.ID},
		workspaceHeaders(ownerToken, wsID), &rmOut)
	if !rmOut.RemoveUserFromTeam {
		t.Error("removeUserFromTeam returned false, want true")
	}

	// The relationship is empty again.
	if ids := teamUserIDs(t, ownerToken, wsID, team.ID); len(ids) != 0 {
		t.Errorf("team.users after remove = %v, want empty", ids)
	}
}

// TestTeamMembershipManagementGuardedByRole proves managing a Team's membership
// is owner/admin-only, enforced by the authz hook on team_members (story 17): a
// plain member can neither add nor remove Users on a Team, while the owner (the
// control) can. This is the guard the blessed mutations inherit by writing to a
// guarded table.
func TestTeamMembershipManagementGuardedByRole(t *testing.T) {
	ownerToken, _ := register(t, "team-mrole-owner@example.com", "team-pw-00006", "Team MRole Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "TeamMRole", "team-mrole-ws")
	team := createTeam(t, ownerToken, wsID, "Design", "design")

	memberToken, member := register(t, "team-mrole-member@example.com", "team-pw-00007", "Team MRole Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// A plain member cannot add a User to a Team — guarded by the authz hook.
	addResp := gqlExec(t, addUserToTeamMutation, map[string]any{"teamID": team.ID, "userID": member.ID},
		workspaceHeaders(memberToken, wsID))
	if len(addResp.Errors) == 0 {
		t.Error("a member was allowed to add a user to a team; owner/admin-only operation not guarded (story 17)")
	}

	// Control: the owner CAN add, so the rejection is about Role.
	var addOut struct {
		AddUserToTeam struct {
			UserID string `json:"userID"`
		} `json:"addUserToTeam"`
	}
	gqlExecData(t, addUserToTeamMutation, map[string]any{"teamID": team.ID, "userID": member.ID},
		workspaceHeaders(ownerToken, wsID), &addOut)
	if addOut.AddUserToTeam.UserID != member.ID {
		t.Fatal("owner addUserToTeam failed, control broken")
	}

	// A plain member cannot remove a User from a Team either.
	rmResp := gqlExec(t, removeUserFromTeamMutation, map[string]any{"teamID": team.ID, "userID": member.ID},
		workspaceHeaders(memberToken, wsID))
	if len(rmResp.Errors) == 0 {
		t.Error("a member was allowed to remove a user from a team; owner/admin-only operation not guarded (story 17)")
	}
}

// TestAddUserToTeamRejectsNonMember proves a Team groups Users within a
// Workspace (CONTEXT.md): a User who holds no Membership in the Active Workspace
// cannot be added to one of its Teams, even by an owner. The resolver checks the
// tenant-scoped Memberships client, so Team membership cannot diverge from the
// Workspace roster.
func TestAddUserToTeamRejectsNonMember(t *testing.T) {
	ownerToken, _ := register(t, "team-nonmember-owner@example.com", "team-pw-00008", "Team Nonmember Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "TeamNonmember", "team-nonmember-ws")
	team := createTeam(t, ownerToken, wsID, "Core", "core")

	// An outsider User exists globally but never joins this Workspace.
	_, outsider := register(t, "team-nonmember-outsider@example.com", "team-pw-00009", "Outsider")

	resp := gqlExec(t, addUserToTeamMutation, map[string]any{"teamID": team.ID, "userID": outsider.ID},
		workspaceHeaders(ownerToken, wsID))
	if len(resp.Errors) == 0 {
		t.Error("a non-member User was added to a Team; membership-within-workspace not enforced (story 17)")
	}

	// Nothing was persisted: the Team has no users.
	if ids := teamUserIDs(t, ownerToken, wsID, team.ID); len(ids) != 0 {
		t.Errorf("team.users after a rejected non-member add = %v, want empty", ids)
	}
}

// TestRemoveUserFromTeamIsIdempotent proves removing a User who is not on the
// Team is not an error (the blessed mutation delegates to the generated
// HardDelete, which is idempotent). This lets a client converge Team membership
// without first checking the current roster.
func TestRemoveUserFromTeamIsIdempotent(t *testing.T) {
	ownerToken, owner := register(t, "team-idem-owner@example.com", "team-pw-00010", "Team Idem Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "TeamIdem", "team-idem-ws")
	team := createTeam(t, ownerToken, wsID, "Ops", "ops")

	// The owner is a Member but is not on the Team; removing them is a no-op.
	var out struct {
		RemoveUserFromTeam bool `json:"removeUserFromTeam"`
	}
	gqlExecData(t, removeUserFromTeamMutation, map[string]any{"teamID": team.ID, "userID": owner.ID},
		workspaceHeaders(ownerToken, wsID), &out)
	if !out.RemoveUserFromTeam {
		t.Error("removeUserFromTeam on an absent membership returned false, want true (idempotent)")
	}
}

// TestMemberAssignsProjectToTeam proves a Workspace member can assign a Project
// to a Team (story 18): the owner creates the Team and Project, a plain member
// assigns the Project to the Team via updateTeamWithRelated (the empty Team
// update is not admin-guarded), and the Project's teamID reads back pointing at
// the Team.
func TestMemberAssignsProjectToTeam(t *testing.T) {
	ownerToken, _ := register(t, "team-assign-owner@example.com", "assign-pw-0001", "Team Assign Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Assigning", "team-assigning-ws")
	team := createTeam(t, ownerToken, wsID, "Owners", "owners")
	projID := createProject(t, ownerToken, wsID, "Owned work")

	memberToken, member := register(t, "team-assign-member@example.com", "assign-pw-0002", "Team Assign Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	var out assignedTeam
	gqlExecData(t, assignProjectToTeamMutation, map[string]any{"projectID": projID, "teamID": team.ID},
		workspaceHeaders(memberToken, wsID), &out)
	got := out.UpdateTeamWithRelated.Projects
	if len(got) != 1 || got[0].ID != projID || got[0].TeamID == nil || *got[0].TeamID != team.ID {
		t.Fatalf("assigned team projects = %+v, want project %s with teamID %q", got, projID, team.ID)
	}

	// Read back through project(id) to confirm the assignment persisted.
	if back := readProjectTeam(t, memberToken, wsID, projID); back == nil || *back != team.ID {
		t.Errorf("read-back project teamID = %v, want %q", back, team.ID)
	}

	// And through the Team's projects relationship to confirm ownership is clear.
	const teamProjectsQuery = `query ($id: UUID!) { team(id: $id) { projects { id } } }`
	var tp struct {
		Team struct {
			Projects []struct {
				ID string `json:"id"`
			} `json:"projects"`
		} `json:"team"`
	}
	gqlExecData(t, teamProjectsQuery, map[string]any{"id": team.ID}, workspaceHeaders(memberToken, wsID), &tp)
	if len(tp.Team.Projects) != 1 || tp.Team.Projects[0].ID != projID {
		t.Errorf("team.projects = %+v, want the assigned project %s", tp.Team.Projects, projID)
	}
}

// TestMemberReassignsProjectToAnotherTeam proves a Project already owned by one
// Team moves to another by connecting it there (allow_reparent on
// teams.Projects): without it the nested connect adopts only unowned Projects.
func TestMemberReassignsProjectToAnotherTeam(t *testing.T) {
	ownerToken, _ := register(t, "reassign-owner@example.com", "reassign-pw-0001", "Reassign Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Reassigning", "reassigning-ws")
	first := createTeam(t, ownerToken, wsID, "First", "first")
	second := createTeam(t, ownerToken, wsID, "Second", "second")
	projID := createProject(t, ownerToken, wsID, "Moving work")

	memberToken, member := register(t, "reassign-member@example.com", "reassign-pw-0002", "Reassign Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")
	h := workspaceHeaders(memberToken, wsID)

	gqlExecData(t, assignProjectToTeamMutation, map[string]any{"projectID": projID, "teamID": first.ID}, h, &assignedTeam{})
	gqlExecData(t, assignProjectToTeamMutation, map[string]any{"projectID": projID, "teamID": second.ID}, h, &assignedTeam{})

	if back := readProjectTeam(t, memberToken, wsID, projID); back == nil || *back != second.ID {
		t.Errorf("reassigned project teamID = %v, want %q", back, second.ID)
	}
}

// TestMemberCannotEditTeamThroughNestedUpdate proves the nested mutation opens
// Project assignment to members without opening the Team catalog: setting a Team
// column through updateTeamWithRelated is still owner/admin-only (story 16).
func TestMemberCannotEditTeamThroughNestedUpdate(t *testing.T) {
	ownerToken, _ := register(t, "nested-team-owner@example.com", "nested-team-pw-01", "Nested Team Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "NestedTeam", "nested-team-ws")
	team := createTeam(t, ownerToken, wsID, "Platform", "platform")

	memberToken, member := register(t, "nested-team-member@example.com", "nested-team-pw-02", "Nested Team Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	resp := gqlExec(t, renameTeamNestedMutation, map[string]any{"teamID": team.ID, "name": "member-renamed"},
		workspaceHeaders(memberToken, wsID))
	if len(resp.Errors) == 0 {
		t.Error("a member renamed a team through updateTeamWithRelated; owner/admin-only write not guarded")
	}

	// Control: the owner CAN, so the rejection is about Role.
	var out struct {
		UpdateTeamWithRelated struct {
			Name string `json:"name"`
		} `json:"updateTeamWithRelated"`
	}
	gqlExecData(t, renameTeamNestedMutation, map[string]any{"teamID": team.ID, "name": "owner-renamed"},
		workspaceHeaders(ownerToken, wsID), &out)
	if out.UpdateTeamWithRelated.Name != "owner-renamed" {
		t.Errorf("owner rename returned name %q, want owner-renamed", out.UpdateTeamWithRelated.Name)
	}
}

// TestAssignRejectsCrossWorkspaceTeam proves Project assignment guards tenant
// isolation in both directions: a Team from another Workspace cannot be assigned
// (the nested update resolves the Team tenant-scoped first), and a Project from
// another Workspace cannot be connected (connect reads it through the
// tenant-scoped Projects client) — closing the hole that projects.team_id (an FK
// with no Workspace constraint) would otherwise leave open to the generated
// updateProject.
func TestAssignRejectsCrossWorkspaceTeam(t *testing.T) {
	token, _ := register(t, "xws-team@example.com", "xws-team-pw-01", "XWS Team")
	wsA := bootstrapWorkspace(t, token, "TeamA", "team-a-ws")
	wsB := bootstrapWorkspace(t, token, "TeamB", "team-b-ws")

	projA := createProject(t, token, wsA, "A work")
	teamA := createTeam(t, token, wsA, "A team", "a-team")
	teamB := createTeam(t, token, wsB, "B team", "b-team")
	projB := createProject(t, token, wsB, "B work")

	// Active in A, referencing a Team that lives in B — B's Team is invisible
	// here, so the assignment is refused before any row is written.
	resp := gqlExec(t, assignProjectToTeamMutation,
		map[string]any{"projectID": projA, "teamID": teamB.ID},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("a project was assigned to a team in another workspace; cross-tenant assignment not guarded")
	}
	if back := readProjectTeam(t, token, wsA, projA); back != nil {
		t.Errorf("projA teamID = %v after a rejected cross-workspace assign, want null", *back)
	}

	// Active in A, connecting B's Project to A's Team — B's Project is invisible here.
	resp = gqlExec(t, assignProjectToTeamMutation,
		map[string]any{"projectID": projB, "teamID": teamA.ID},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("a project from another workspace was assigned to this team; cross-tenant connect not guarded")
	}
	if back := readProjectTeam(t, token, wsB, projB); back != nil {
		t.Errorf("projB teamID = %v after a rejected cross-workspace connect, want null", *back)
	}
}

// TestRawUpdateProjectCannotAssignCrossWorkspaceTeam proves the composite FK
// (migration 0005, projects_team_same_ws) closes the cross-tenant assignment hole
// at the DATABASE, not only in the blessed mutation. The generated updateProject
// is tenant-scoped on the Project but places no Workspace constraint on the teamID
// it writes; the composite FK (workspace_id, team_id) -> teams(workspace_id, id)
// makes Postgres reject a Team from another Workspace, whatever the caller path.
func TestRawUpdateProjectCannotAssignCrossWorkspaceTeam(t *testing.T) {
	token, _ := register(t, "rawassign@example.com", "rawassign-pw-01", "Raw Assign")
	wsA := bootstrapWorkspace(t, token, "RawAssignA", "raw-assign-a-ws")
	wsB := bootstrapWorkspace(t, token, "RawAssignB", "raw-assign-b-ws")

	projA := createProject(t, token, wsA, "A work")
	teamB := createTeam(t, token, wsB, "B team", "b-team")

	// Raw updateProject, active in A, pointing at B's Team — rejected by the FK at
	// the DB even though updateProject never consults the Team's tenant itself.
	resp := gqlExec(t, updateProjectTeamMutation,
		map[string]any{"id": projA, "teamID": teamB.ID},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("raw updateProject assigned a project to a cross-workspace team; composite FK not enforced")
	}

	// Nothing persisted.
	var back struct {
		Project struct {
			TeamID *string `json:"teamID"`
		} `json:"project"`
	}
	gqlExecData(t, projectTeamQuery, map[string]any{"id": projA}, workspaceHeaders(token, wsA), &back)
	if back.Project.TeamID != nil {
		t.Errorf("projA teamID = %v after a rejected raw cross-workspace update, want null", *back.Project.TeamID)
	}

	// Control: a SAME-Workspace Team via the raw updateProject still succeeds, so
	// the rejection above is specifically the cross-tenant pair, not a broken
	// mutation.
	teamA := createTeam(t, token, wsA, "A team", "a-team")
	var out struct {
		UpdateProject struct {
			TeamID *string `json:"teamID"`
		} `json:"updateProject"`
	}
	gqlExecData(t, updateProjectTeamMutation,
		map[string]any{"id": projA, "teamID": teamA.ID},
		workspaceHeaders(token, wsA), &out)
	if out.UpdateProject.TeamID == nil || *out.UpdateProject.TeamID != teamA.ID {
		t.Errorf("same-workspace raw updateProject teamID = %v, want %q", out.UpdateProject.TeamID, teamA.ID)
	}
}
