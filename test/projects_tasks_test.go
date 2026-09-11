package test

import (
	"testing"
)

// This file proves the Projects & Tasks core (issue 07) through the one seam the
// harness exercises — the GraphQL HTTP endpoint under an Active Workspace. It
// leans on the register / bootstrapWorkspace / createProject helpers established
// by the identity and tenancy suites and adds task-shaped helpers for the
// blessed createTaskInProject / addTaskDependency mutations.

const createTaskInProjectMutation = `
	mutation ($input: CreateTaskInProjectInput!) {
		createTaskInProject(input: $input) {
			id workspaceID projectID parentTaskID reporterID title status priority
		}
	}`

// joinWorkspace makes the User behind memberToken a Member of wsID at the given
// role, driving the real invitation lifecycle: the owner/admin behind ownerToken
// invites memberEmail, then the member accepts with the issued token. It reuses
// the issue-04 invite / acceptData helpers so membership is established exactly
// as a client would, not by a backdoor insert.
func joinWorkspace(t *testing.T, ownerToken, wsID, memberToken, memberEmail, role string) {
	t.Helper()
	inv := invite(t, ownerToken, wsID, memberEmail, role)
	if inv.Token == "" {
		t.Fatal("invite returned empty token")
	}
	resp := acceptData(t, memberToken, inv.Token)
	if len(resp.Errors) > 0 {
		t.Fatalf("accepting invitation: %+v", resp.Errors)
	}
}

type taskResult struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspaceID"`
	ProjectID    string  `json:"projectID"`
	ParentTaskID *string `json:"parentTaskID"`
	ReporterID   string  `json:"reporterID"`
	Title        string  `json:"title"`
	Status       string  `json:"status"`
	Priority     string  `json:"priority"`
}

// createTaskInProject drives the blessed createTaskInProject mutation in the
// given Active Workspace. status/priority are optional — pass "" to let them
// fall to their DB defaults. parentTaskID is optional — pass "" for a top-level
// Task. The Reporter is never supplied; the server sets it to the caller.
func createTaskInProject(t *testing.T, token, workspaceID, projectID, title, status, priority, parentTaskID string) taskResult {
	t.Helper()
	input := map[string]any{"projectID": projectID, "title": title}
	if status != "" {
		input["status"] = status
	}
	if priority != "" {
		input["priority"] = priority
	}
	if parentTaskID != "" {
		input["parentTaskID"] = parentTaskID
	}
	var out struct {
		CreateTaskInProject taskResult `json:"createTaskInProject"`
	}
	gqlExecData(t, createTaskInProjectMutation, map[string]any{"input": input}, workspaceHeaders(token, workspaceID), &out)
	if out.CreateTaskInProject.ID == "" {
		t.Fatal("createTaskInProject returned empty task id")
	}
	return out.CreateTaskInProject
}

// TestCreateTaskInProjectSetsReporter proves a Workspace member can create a
// Task in a Project with a status and priority (AC 3 / story 21) and that the
// creating User is recorded as the Task's Reporter (AC 4 / story 30). Reporter
// provenance is server-owned: the input carries no reporter field, yet the
// created Task's reporterID is the caller — and when a second member creates a
// Task, its Reporter is that member, not the first.
func TestCreateTaskInProjectSetsReporter(t *testing.T) {
	ownerToken, owner := register(t, "reporter-owner@example.com", "reporter-pw-0001", "Reporter Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Reporters", "reporters-ws")
	projID := createProject(t, ownerToken, wsID, "Reporting")

	task := createTaskInProject(t, ownerToken, wsID, projID, "Wire the seam", "IN_PROGRESS", "HIGH", "")
	if task.Status != "IN_PROGRESS" {
		t.Errorf("task status = %q, want IN_PROGRESS", task.Status)
	}
	if task.Priority != "HIGH" {
		t.Errorf("task priority = %q, want HIGH", task.Priority)
	}
	if task.ProjectID != projID {
		t.Errorf("task projectID = %q, want %q", task.ProjectID, projID)
	}
	if task.WorkspaceID != wsID {
		t.Errorf("task workspaceID = %q, want %q (auto-scoped to Active Workspace)", task.WorkspaceID, wsID)
	}
	if task.ReporterID != owner.ID {
		t.Errorf("task reporterID = %q, want %q (the creating User)", task.ReporterID, owner.ID)
	}

	// A second member creating a Task in the same Project becomes that Task's
	// Reporter — provenance follows the caller, not a client-supplied value.
	memberToken, member := register(t, "reporter-member@example.com", "reporter-pw-0002", "Reporter Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")
	memberTask := createTaskInProject(t, memberToken, wsID, projID, "Second hands", "TODO", "LOW", "")
	if memberTask.ReporterID != member.ID {
		t.Errorf("second task reporterID = %q, want %q (the second creating User)", memberTask.ReporterID, member.ID)
	}
	if memberTask.ReporterID == owner.ID {
		t.Error("second task Reporter is the first User; Reporter must follow the caller")
	}
}

// TestCreateTaskDefaultsStatusAndPriority proves status and priority are
// optional on the blessed input and fall to their schema defaults (backlog,
// medium) when omitted — the DB defaults reach through the generated create.
func TestCreateTaskDefaultsStatusAndPriority(t *testing.T) {
	token, _ := register(t, "defaults@example.com", "defaults-pw-0001", "Defaults")
	wsID := bootstrapWorkspace(t, token, "Defaults", "defaults-ws")
	projID := createProject(t, token, wsID, "Defaulting")

	task := createTaskInProject(t, token, wsID, projID, "Unspecified", "", "", "")
	if task.Status != "BACKLOG" {
		t.Errorf("default status = %q, want BACKLOG", task.Status)
	}
	if task.Priority != "MEDIUM" {
		t.Errorf("default priority = %q, want MEDIUM", task.Priority)
	}
}

// TestCreateSubtask proves a Subtask can be created under a Task via the
// self-referential parent (AC 5 / story 22): the child's parentTaskID points at
// the parent, and the parent's `subtasks` relationship lists the child — the
// same tasks table, self-referenced.
func TestCreateSubtask(t *testing.T) {
	token, _ := register(t, "subtasks@example.com", "subtasks-pw-0001", "Subtasker")
	wsID := bootstrapWorkspace(t, token, "Subtasks", "subtasks-ws")
	projID := createProject(t, token, wsID, "Breakdown")

	parent := createTaskInProject(t, token, wsID, projID, "Epic", "TODO", "MEDIUM", "")
	child := createTaskInProject(t, token, wsID, projID, "Piece of work", "TODO", "MEDIUM", parent.ID)

	if child.ParentTaskID == nil || *child.ParentTaskID != parent.ID {
		t.Fatalf("subtask parentTaskID = %v, want %q", child.ParentTaskID, parent.ID)
	}

	// The parent lists the child through the generated self-referential subtasks
	// relationship — one query, loaded relationally.
	const q = `query ($id: UUID!) { task(id: $id) { id subtasks { id title parentTaskID } } }`
	var out struct {
		Task struct {
			ID       string `json:"id"`
			Subtasks []struct {
				ID           string  `json:"id"`
				Title        string  `json:"title"`
				ParentTaskID *string `json:"parentTaskID"`
			} `json:"subtasks"`
		} `json:"task"`
	}
	gqlExecData(t, q, map[string]any{"id": parent.ID}, workspaceHeaders(token, wsID), &out)
	if len(out.Task.Subtasks) != 1 {
		t.Fatalf("parent has %d subtasks, want 1", len(out.Task.Subtasks))
	}
	if out.Task.Subtasks[0].ID != child.ID {
		t.Errorf("parent subtask id = %q, want %q", out.Task.Subtasks[0].ID, child.ID)
	}
}

// TestArchiveProjectHidesButRetainsHistory proves a Workspace member can archive
// a Project (soft-delete) so it disappears from active views without losing
// history (AC 2 / story 20). After archiving, the Project is unreadable and
// absent from the active list; restoring it brings the same row back with its
// Tasks intact — proof the archive soft-deleted rather than destroyed.
func TestArchiveProjectHidesButRetainsHistory(t *testing.T) {
	token, _ := register(t, "archive@example.com", "archive-pw-00001", "Archiver")
	wsID := bootstrapWorkspace(t, token, "Archives", "archives-ws")
	projID := createProject(t, token, wsID, "Sunset")
	task := createTaskInProject(t, token, wsID, projID, "History to retain", "TODO", "MEDIUM", "")

	// Archive (soft-delete) the Project.
	const archiveMutation = `mutation ($id: UUID!) { softDeleteProject(id: $id) { id deletedAt } }`
	var archived struct {
		SoftDeleteProject struct {
			ID        string  `json:"id"`
			DeletedAt *string `json:"deletedAt"`
		} `json:"softDeleteProject"`
	}
	gqlExecData(t, archiveMutation, map[string]any{"id": projID}, workspaceHeaders(token, wsID), &archived)
	if archived.SoftDeleteProject.DeletedAt == nil {
		t.Fatal("archived project has nil deletedAt; archive should stamp a soft-delete time")
	}

	// It has left active views: a direct read returns nothing.
	const readQ = `query ($id: UUID!) { project(id: $id) { id } }`
	var read struct {
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, readQ, map[string]any{"id": projID}, workspaceHeaders(token, wsID), &read)
	if read.Project != nil {
		t.Error("archived project still readable in active views")
	}

	// And it is absent from the active list.
	const listQ = `query { projectList(limit: 50) { totalCount items { id } } }`
	var list struct {
		ProjectList struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"projectList"`
	}
	gqlExecData(t, listQ, nil, workspaceHeaders(token, wsID), &list)
	for _, p := range list.ProjectList.Items {
		if p.ID == projID {
			t.Error("archived project still appears in the active projectList")
		}
	}

	// History is retained, not destroyed: restoring the Project brings the same
	// row back, and its Task is still attached.
	const restoreMutation = `mutation ($id: UUID!) { restoreProject(id: $id) { id deletedAt tasks { id } } }`
	var restored struct {
		RestoreProject *struct {
			ID        string  `json:"id"`
			DeletedAt *string `json:"deletedAt"`
			Tasks     []struct {
				ID string `json:"id"`
			} `json:"tasks"`
		} `json:"restoreProject"`
	}
	gqlExecData(t, restoreMutation, map[string]any{"id": projID}, workspaceHeaders(token, wsID), &restored)
	if restored.RestoreProject == nil {
		t.Fatal("archived project could not be restored; the row was not retained")
	}
	if restored.RestoreProject.DeletedAt != nil {
		t.Error("restored project still has deletedAt set")
	}
	if len(restored.RestoreProject.Tasks) != 1 || restored.RestoreProject.Tasks[0].ID != task.ID {
		t.Errorf("restored project tasks = %+v, want the original task %q retained", restored.RestoreProject.Tasks, task.ID)
	}
}

// TestWorkspaceMemberCreatesAndArchivesProject proves the Project create and
// archive paths for a plain (non-owner) Member — stories 19 and 20 are written
// "As a Workspace member", and Projects are not an authz-guarded table, so a
// `member` Role suffices. The owner only founds the Workspace and invites; the
// Member does the work.
func TestWorkspaceMemberCreatesAndArchivesProject(t *testing.T) {
	ownerToken, _ := register(t, "ws-owner@example.com", "ws-owner-pw-001", "WS Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "MemberRun", "member-run-ws")

	memberToken, member := register(t, "ws-member@example.com", "ws-member-pw-01", "WS Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// The Member creates a Project (story 19) and a Task in it, then archives the
	// Project (story 20) — all under a `member` Role.
	projID := createProject(t, memberToken, wsID, "Member's board")
	_ = createTaskInProject(t, memberToken, wsID, projID, "Member's task", "TODO", "MEDIUM", "")

	const archiveMutation = `mutation ($id: UUID!) { softDeleteProject(id: $id) { id deletedAt } }`
	var archived struct {
		SoftDeleteProject struct {
			DeletedAt *string `json:"deletedAt"`
		} `json:"softDeleteProject"`
	}
	gqlExecData(t, archiveMutation, map[string]any{"id": projID}, workspaceHeaders(memberToken, wsID), &archived)
	if archived.SoftDeleteProject.DeletedAt == nil {
		t.Fatal("a member could not archive a project; archive should stamp a soft-delete time")
	}

	// It has left active views for the Member.
	const readQ = `query ($id: UUID!) { project(id: $id) { id } }`
	var read struct {
		Project *struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	gqlExecData(t, readQ, map[string]any{"id": projID}, workspaceHeaders(memberToken, wsID), &read)
	if read.Project != nil {
		t.Error("archived project still readable in active views after a member archived it")
	}
}

const addTaskDependencyMutation = `
	mutation ($taskID: UUID!, $dependsOn: UUID!, $type: DependencyType) {
		addTaskDependency(taskID: $taskID, dependsOnTaskID: $dependsOn, type: $type) {
			taskID dependsOnTaskID type
		}
	}`

// TestTaskDependencyVisibleFromBothSides proves a Task can declare a dependency
// on another Task (AC 6 / story 23) and that the link is readable from either
// direction (story 24): the dependent Task lists it under dependsOnTasks and the
// depended-on Task lists the dependent under tasks. An omitted type defaults to
// BLOCKS.
func TestTaskDependencyVisibleFromBothSides(t *testing.T) {
	token, _ := register(t, "deps@example.com", "deps-pw-000001", "Depender")
	wsID := bootstrapWorkspace(t, token, "Deps", "deps-ws")
	projID := createProject(t, token, wsID, "Ordering")

	blocked := createTaskInProject(t, token, wsID, projID, "Ship release", "TODO", "HIGH", "")
	blocker := createTaskInProject(t, token, wsID, projID, "Cut branch", "TODO", "HIGH", "")

	// "blocked" depends on "blocker"; leave type unset to prove the BLOCKS default.
	var dep struct {
		AddTaskDependency struct {
			TaskID          string `json:"taskID"`
			DependsOnTaskID string `json:"dependsOnTaskID"`
			Type            string `json:"type"`
		} `json:"addTaskDependency"`
	}
	gqlExecData(t, addTaskDependencyMutation, map[string]any{
		"taskID": blocked.ID, "dependsOn": blocker.ID, "type": nil,
	}, workspaceHeaders(token, wsID), &dep)
	if dep.AddTaskDependency.Type != "BLOCKS" {
		t.Errorf("dependency type = %q, want BLOCKS (the default)", dep.AddTaskDependency.Type)
	}

	// Dependency side: the dependent Task lists what it depends on.
	const q = `query ($id: UUID!) {
		task(id: $id) {
			dependsOnTasks { id title }
			tasks { id title }
		}
	}`
	var blockedView struct {
		Task struct {
			DependsOnTasks []struct {
				ID string `json:"id"`
			} `json:"dependsOnTasks"`
			Tasks []struct {
				ID string `json:"id"`
			} `json:"tasks"`
		} `json:"task"`
	}
	gqlExecData(t, q, map[string]any{"id": blocked.ID}, workspaceHeaders(token, wsID), &blockedView)
	if len(blockedView.Task.DependsOnTasks) != 1 || blockedView.Task.DependsOnTasks[0].ID != blocker.ID {
		t.Errorf("blocked.dependsOnTasks = %+v, want [%s]", blockedView.Task.DependsOnTasks, blocker.ID)
	}

	// Dependent side: the depended-on Task lists what depends on it.
	var blockerView struct {
		Task struct {
			DependsOnTasks []struct {
				ID string `json:"id"`
			} `json:"dependsOnTasks"`
			Tasks []struct {
				ID string `json:"id"`
			} `json:"tasks"`
		} `json:"task"`
	}
	gqlExecData(t, q, map[string]any{"id": blocker.ID}, workspaceHeaders(token, wsID), &blockerView)
	if len(blockerView.Task.Tasks) != 1 || blockerView.Task.Tasks[0].ID != blocked.ID {
		t.Errorf("blocker.tasks (dependents) = %+v, want [%s]", blockerView.Task.Tasks, blocked.ID)
	}
}

// TestAddTaskDependencyRejectsSelfAndCrossTenant proves the blessed mutation
// guards the two invariants tenant scoping alone cannot: a Task may not depend
// on itself, and a dependency may not span Workspaces. The task_dependencies
// junction has no workspace_id, so the resolver enforces isolation by resolving
// both Tasks through the Active Workspace first.
func TestAddTaskDependencyRejectsSelfAndCrossTenant(t *testing.T) {
	token, _ := register(t, "dep-guard@example.com", "dep-guard-pw-01", "Dep Guard")
	wsA := bootstrapWorkspace(t, token, "GuardA", "guard-a-ws")
	wsB := bootstrapWorkspace(t, token, "GuardB", "guard-b-ws")

	projA := createProject(t, token, wsA, "A work")
	taskA := createTaskInProject(t, token, wsA, projA, "In A", "TODO", "MEDIUM", "")

	projB := createProject(t, token, wsB, "B work")
	taskB := createTaskInProject(t, token, wsB, projB, "In B", "TODO", "MEDIUM", "")

	// Self-dependency is rejected.
	self := gqlExec(t, addTaskDependencyMutation, map[string]any{
		"taskID": taskA.ID, "dependsOn": taskA.ID, "type": nil,
	}, workspaceHeaders(token, wsA))
	if len(self.Errors) == 0 {
		t.Error("addTaskDependency allowed a task to depend on itself")
	}

	// Cross-tenant: active in A, referencing a Task that lives in B — B's Task is
	// invisible here, so the link is refused before any row is written.
	cross := gqlExec(t, addTaskDependencyMutation, map[string]any{
		"taskID": taskA.ID, "dependsOn": taskB.ID, "type": nil,
	}, workspaceHeaders(token, wsA))
	if len(cross.Errors) == 0 {
		t.Error("addTaskDependency linked a task to one in another workspace")
	}

	// Nothing was persisted: A's task has no dependencies.
	const q = `query ($id: UUID!) { task(id: $id) { dependsOnTasks { id } } }`
	var view struct {
		Task struct {
			DependsOnTasks []struct {
				ID string `json:"id"`
			} `json:"dependsOnTasks"`
		} `json:"task"`
	}
	gqlExecData(t, q, map[string]any{"id": taskA.ID}, workspaceHeaders(token, wsA), &view)
	if len(view.Task.DependsOnTasks) != 0 {
		t.Errorf("taskA has %d dependencies after rejected links, want 0", len(view.Task.DependsOnTasks))
	}
}
