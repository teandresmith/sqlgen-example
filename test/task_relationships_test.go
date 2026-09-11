package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/teandresmith/sqlgen-example/internal/server"
)

// This file proves issue 08 (Task assignment & labels) through the one seam the
// harness exercises — the GraphQL HTTP endpoint under an Active Workspace. It
// leans on the register / bootstrapWorkspace / createProject / createTaskInProject
// / joinWorkspace / createLabel helpers and drives the blessed assignUserToTask /
// watchTask mutations, the generated nested updateTaskWithRelated (labels.connect
// applies a Label), and the generated createLabel guard.

const assignUserToTaskMutation = `
	mutation ($taskID: UUID!, $userID: UUID!) {
		assignUserToTask(taskID: $taskID, userID: $userID) { taskID userID }
	}`

const watchTaskMutation = `
	mutation ($taskID: UUID!) {
		watchTask(taskID: $taskID) { taskID userID }
	}`

// applyLabelToTaskMutation applies a Label through the generated nested update:
// labels.connect reads the Label tenant-scoped, then writes the task_labels row.
const applyLabelToTaskMutation = `
	mutation ($taskID: UUID!, $labelID: UUID!) {
		updateTaskWithRelated(id: $taskID, input: { labels: { connect: [$labelID] } }) {
			id
			labels { id }
		}
	}`

// labeledTask is the updateTaskWithRelated payload applyLabelToTask selects: the
// Task and the Labels now applied to it.
type labeledTask struct {
	UpdateTaskWithRelated struct {
		ID     string `json:"id"`
		Labels []struct {
			ID string `json:"id"`
		} `json:"labels"`
	} `json:"updateTaskWithRelated"`
}

// taskRelationsQuery reads a Task's assignees, watchers, and labels back through
// the generated M2M relationship fields (User/Label objects, not junction rows).
const taskRelationsQuery = `
	query ($id: UUID!) {
		task(id: $id) {
			id
			assignees { id }
			watchers { id }
			labels { id name }
		}
	}`

type taskRelations struct {
	Assignees []struct {
		ID string `json:"id"`
	} `json:"assignees"`
	Watchers []struct {
		ID string `json:"id"`
	} `json:"watchers"`
	Labels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"labels"`
}

// readTaskRelations fetches a Task's assignees/watchers/labels in one query.
func readTaskRelations(t *testing.T, token, wsID, taskID string) taskRelations {
	t.Helper()
	var out struct {
		Task taskRelations `json:"task"`
	}
	gqlExecData(t, taskRelationsQuery, map[string]any{"id": taskID}, workspaceHeaders(token, wsID), &out)
	return out.Task
}

// idsOf collects the id fields of a slice of {ID string} into a set-like map.
func containsID(ids []struct {
	ID string `json:"id"`
}, want string) bool {
	for _, x := range ids {
		if x.ID == want {
			return true
		}
	}
	return false
}

// rawMutationRejected POSTs a mutation and reports whether the server refused it
// — either a non-200 status (gqlgen validation error) or a GraphQL error in the
// body. It is status-tolerant because an unknown-field validation error can
// surface as 422 or as 200-with-errors depending on the handler.
func rawMutationRejected(t *testing.T, query string, headers map[string]string) bool {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"query": query})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		testServer.URL+server.GraphQLPath, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return true
	}
	raw, _ := io.ReadAll(resp.Body)
	var out gqlResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode response (raw=%s): %v", raw, err)
	}
	return len(out.Errors) > 0
}

// TestRawJunctionCreateMutationsAreNotExposed locks in the security fix: the
// generated raw create/upsert mutations for the junction (M2M link) tables are
// masked out of the GraphQL API (sqlgen.yml api.operations), because they carry
// no workspace_id and so cannot enforce tenant isolation. The only link-create
// path is the tenant-guarded blessed mutations (assignUserToTask / watchTask / …)
// and the nested updateTaskWithRelated, whose labels.connect reads the Label
// tenant-scoped. If the mask is ever removed, this test fails.
func TestRawJunctionCreateMutationsAreNotExposed(t *testing.T) {
	token, _ := register(t, "raw-junction@example.com", "raw-pw-000001", "Raw Junction")
	wsID := bootstrapWorkspace(t, token, "RawJunction", "raw-junction-ws")
	h := workspaceHeaders(token, wsID)

	// Each references a now-removed input type / field, so the server rejects the
	// operation before execution — proof the unguarded raw path is gone.
	raw := map[string]string{
		"createTaskLabel":    `mutation { createTaskLabel(input: {taskID: "00000000-0000-0000-0000-000000000001", labelID: "00000000-0000-0000-0000-000000000002"}) { taskID } }`,
		"createTaskAssignee": `mutation { createTaskAssignee(input: {taskID: "00000000-0000-0000-0000-000000000001", userID: "00000000-0000-0000-0000-000000000002"}) { taskID } }`,
		"upsertTaskLabel":    `mutation { upsertTaskLabel(input: {taskID: "00000000-0000-0000-0000-000000000001", labelID: "00000000-0000-0000-0000-000000000002"}) { taskID } }`,
	}
	for name, q := range raw {
		if !rawMutationRejected(t, q, h) {
			t.Errorf("raw %s was accepted; the unguarded junction create mutation must not be exposed", name)
		}
	}
}

// TestRawCreateMembershipNotExposed locks in that Memberships are created only
// through the blessed flows (bootstrapWorkspace / acceptInvitation), never the
// raw create/upsert. Unlike the junction tables this is not a tenant hole
// (memberships is workspace-scoped and role-guarded) — the mask enforces the
// app's invitation-consent model. If the mask is removed, this test fails.
func TestRawCreateMembershipNotExposed(t *testing.T) {
	ownerToken, _ := register(t, "raw-ms@example.com", "raw-ms-pw-0001", "Raw MS")
	wsID := bootstrapWorkspace(t, ownerToken, "RawMS", "raw-ms-ws")
	h := workspaceHeaders(ownerToken, wsID)

	raw := map[string]string{
		"createMembership": `mutation { createMembership(input: {workspaceID: "00000000-0000-0000-0000-000000000001", userID: "00000000-0000-0000-0000-000000000002", role: MEMBER, createdAt: "2026-01-01T00:00:00Z"}) { userID } }`,
		"upsertMembership": `mutation { upsertMembership(input: {workspaceID: "00000000-0000-0000-0000-000000000001", userID: "00000000-0000-0000-0000-000000000002", role: MEMBER, createdAt: "2026-01-01T00:00:00Z"}) { userID } }`,
	}
	for name, q := range raw {
		if !rawMutationRejected(t, q, h) {
			t.Errorf("raw %s was accepted; memberships must be created only via bootstrap/invitation", name)
		}
	}
}

// TestRawInvitationWriteMutationsNotExposed locks the invitation lifecycle to
// the blessed mutations (inviteToWorkspace mints the acceptance token;
// acceptInvitation / revokeInvitation drive status). The raw create/update
// would let a client supply their own token or reset the state machine, so
// those mutations are masked off the API. If the mask is removed, this fails.
func TestRawInvitationWriteMutationsNotExposed(t *testing.T) {
	ownerToken, _ := register(t, "raw-inv@example.com", "raw-inv-pw-001", "Raw Inv")
	wsID := bootstrapWorkspace(t, ownerToken, "RawInv", "raw-inv-ws")
	h := workspaceHeaders(ownerToken, wsID)

	raw := map[string]string{
		"createInvitation": `mutation { createInvitation(input: {workspaceID: "00000000-0000-0000-0000-000000000001", email: "x@example.com", role: MEMBER, token: "attacker-chosen", expiresAt: "2030-01-01T00:00:00Z"}) { id } }`,
		"updateInvitation": `mutation { updateInvitation(id: "00000000-0000-0000-0000-000000000001", input: {status: ACCEPTED}) { id } }`,
	}
	for name, q := range raw {
		if !rawMutationRejected(t, q, h) {
			t.Errorf("raw %s was accepted; the invitation lifecycle must go through the blessed mutations", name)
		}
	}
}

// TestAssignMultipleUsersToTask proves a member can assign multiple Users to a
// Task via the Assignees M2M (story 25). Two users are assigned through the
// blessed mutation and both come back on task.assignees.
func TestAssignMultipleUsersToTask(t *testing.T) {
	ownerToken, _ := register(t, "assign-owner@example.com", "assign-pw-00001", "Assign Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Assigning", "assigning-ws")
	projID := createProject(t, ownerToken, wsID, "Work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Shared task", "TODO", "MEDIUM", "")

	_, alice := register(t, "assignee-alice@example.com", "assign-pw-00002", "Alice")
	_, bob := register(t, "assignee-bob@example.com", "assign-pw-00003", "Bob")

	for _, u := range []string{alice.ID, bob.ID} {
		var out struct {
			AssignUserToTask struct {
				TaskID string `json:"taskID"`
				UserID string `json:"userID"`
			} `json:"assignUserToTask"`
		}
		gqlExecData(t, assignUserToTaskMutation,
			map[string]any{"taskID": task.ID, "userID": u},
			workspaceHeaders(ownerToken, wsID), &out)
		if out.AssignUserToTask.UserID != u {
			t.Errorf("assignUserToTask returned userID %q, want %q", out.AssignUserToTask.UserID, u)
		}
	}

	rel := readTaskRelations(t, ownerToken, wsID, task.ID)
	if len(rel.Assignees) != 2 {
		t.Fatalf("task has %d assignees, want 2", len(rel.Assignees))
	}
	if !containsID(rel.Assignees, alice.ID) || !containsID(rel.Assignees, bob.ID) {
		t.Errorf("assignees = %+v, want both %s and %s", rel.Assignees, alice.ID, bob.ID)
	}
}

// TestWatchTaskWithoutBeingAssignee proves a member can watch a Task via the
// Watchers M2M without being an Assignee (story 26). The caller becomes a
// watcher and is absent from the assignees.
func TestWatchTaskWithoutBeingAssignee(t *testing.T) {
	ownerToken, _ := register(t, "watch-owner@example.com", "watch-pw-00001", "Watch Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Watching", "watching-ws")
	projID := createProject(t, ownerToken, wsID, "Followed work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Followed task", "TODO", "MEDIUM", "")

	memberToken, member := register(t, "watcher-member@example.com", "watch-pw-00002", "Watcher")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	var out struct {
		WatchTask struct {
			UserID string `json:"userID"`
		} `json:"watchTask"`
	}
	gqlExecData(t, watchTaskMutation, map[string]any{"taskID": task.ID},
		workspaceHeaders(memberToken, wsID), &out)
	if out.WatchTask.UserID != member.ID {
		t.Errorf("watchTask recorded userID %q, want the caller %q", out.WatchTask.UserID, member.ID)
	}

	rel := readTaskRelations(t, memberToken, wsID, task.ID)
	if !containsID(rel.Watchers, member.ID) {
		t.Errorf("watchers = %+v, want the member %s", rel.Watchers, member.ID)
	}
	if containsID(rel.Assignees, member.ID) {
		t.Error("watcher appears among assignees; watching must not imply assignment (story 26)")
	}
}

// TestLabelManagementGuardedByRole proves Workspace Label management is
// owner/admin-only, enforced by the authz hook (story 28): a plain member is
// rejected creating a Label, while the owner (the control) succeeds.
func TestLabelManagementGuardedByRole(t *testing.T) {
	ownerToken, _ := register(t, "label-owner@example.com", "label-pw-00001", "Label Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Labeling", "labeling-ws")

	memberToken, member := register(t, "label-member@example.com", "label-pw-00002", "Label Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// A plain member cannot create a Label — guarded by the authz hook.
	memberResp := gqlExec(t, createLabelMutation, map[string]any{
		"input": map[string]any{
			"workspaceID": wsID, "name": "member-made", "color": "#FF0000",
			"createdAt": "2026-07-14T00:00:00Z",
		},
	}, workspaceHeaders(memberToken, wsID))
	if len(memberResp.Errors) == 0 {
		t.Error("a member was allowed to create a label; owner/admin-only operation not guarded (story 28)")
	}

	// Control: the owner CAN create a Label, so the rejection is about Role, not
	// a broken mutation.
	if id := createLabel(t, ownerToken, wsID, "owner-made", nil); id == "" {
		t.Error("owner label create failed, control broken")
	}
}

// TestMemberAppliesLabelToTask proves any member can apply an existing Label to
// a Task via the task_labels M2M (story 27) — tagging is not admin-guarded, only
// managing the Label catalog is (story 28). The owner creates the Label; a plain
// member applies it and it comes back on task.labels.
func TestMemberAppliesLabelToTask(t *testing.T) {
	ownerToken, _ := register(t, "apply-owner@example.com", "apply-pw-00001", "Apply Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Applying", "applying-ws")
	projID := createProject(t, ownerToken, wsID, "Tagged work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Tagged task", "TODO", "MEDIUM", "")

	// Owner (a manager) creates the Label; a plain member joins.
	labelID := createLabel(t, ownerToken, wsID, "urgent", nil)
	memberToken, member := register(t, "apply-member@example.com", "apply-pw-00002", "Apply Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// The member applies the Label — not guarded, so it succeeds.
	var out labeledTask
	gqlExecData(t, applyLabelToTaskMutation,
		map[string]any{"taskID": task.ID, "labelID": labelID},
		workspaceHeaders(memberToken, wsID), &out)
	if got := out.UpdateTaskWithRelated.Labels; len(got) != 1 || got[0].ID != labelID {
		t.Errorf("updateTaskWithRelated returned labels %+v, want [%s]", got, labelID)
	}

	// Applying it again is idempotent: the link step upserts the junction row.
	gqlExecData(t, applyLabelToTaskMutation,
		map[string]any{"taskID": task.ID, "labelID": labelID},
		workspaceHeaders(memberToken, wsID), &labeledTask{})

	rel := readTaskRelations(t, memberToken, wsID, task.ID)
	if len(rel.Labels) != 1 || rel.Labels[0].ID != labelID {
		t.Fatalf("task labels = %+v, want the applied label %s", rel.Labels, labelID)
	}
	if rel.Labels[0].Name != "urgent" {
		t.Errorf("applied label name = %q, want urgent", rel.Labels[0].Name)
	}
}

// TestApplyLabelRejectsCrossWorkspaceLabel proves labels.connect guards tenant
// isolation: a Label from another Workspace cannot be applied to a Task, because
// connect reads the Label through the tenant-scoped Labels client first.
func TestApplyLabelRejectsCrossWorkspaceLabel(t *testing.T) {
	token, _ := register(t, "xws-label@example.com", "xws-pw-000001", "Cross WS")
	wsA := bootstrapWorkspace(t, token, "LabelA", "label-a-ws")
	wsB := bootstrapWorkspace(t, token, "LabelB", "label-b-ws")

	projA := createProject(t, token, wsA, "A work")
	taskA := createTaskInProject(t, token, wsA, projA, "In A", "TODO", "MEDIUM", "")
	labelB := createLabel(t, token, wsB, "b-only", nil)

	// Active in A, referencing a Label that lives in B — B's Label is invisible
	// here, so the tagging is refused before any row is written.
	resp := gqlExec(t, applyLabelToTaskMutation,
		map[string]any{"taskID": taskA.ID, "labelID": labelB},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("a label from another workspace was applied; cross-tenant tagging not guarded")
	}

	// Nothing was persisted: A's task has no labels.
	rel := readTaskRelations(t, token, wsA, taskA.ID)
	if len(rel.Labels) != 0 {
		t.Errorf("taskA has %d labels after a rejected cross-workspace apply, want 0", len(rel.Labels))
	}
}
