package test

import (
	"testing"
)

// This file proves issue 10 (Cycles & scheduling) through the one seam the
// harness exercises — the GraphQL HTTP endpoint under an Active Workspace. Cycle
// creation is the generated createCycle mutation, guarded owner/admin-only by the
// authz hook (story 32); scheduling a Task into a Cycle drives the generated
// nested updateCycleWithRelated mutation's tasks.connect (story 31), which
// resolves both endpoints tenant-scoped.

const createCycleMutation = `
	mutation ($input: CreateCycleInput!) {
		createCycle(input: $input) { id workspaceID name status startsOn endsOn }
	}`

// scheduleTaskIntoCycleMutation schedules a Task into a Cycle through the
// generated nested update: tasks.connect reads the Task tenant-scoped, then sets
// its cycle_id (allow_reparent in sqlgen.yml lets it move between Cycles).
const scheduleTaskIntoCycleMutation = `
	mutation ($taskID: UUID!, $cycleID: UUID!) {
		updateCycleWithRelated(id: $cycleID, input: { tasks: { connect: [$taskID] } }) {
			id
			tasks { id cycleID }
		}
	}`

// renameCycleNestedMutation sets a Cycle column through the same nested
// mutation — a write on the Cycle catalog itself, which stays owner/admin-only.
const renameCycleNestedMutation = `
	mutation ($cycleID: UUID!, $name: String!) {
		updateCycleWithRelated(id: $cycleID, input: { cycle: { name: $name } }) { id name }
	}`

const taskCycleQuery = `query ($id: UUID!) { task(id: $id) { cycleID } }`

// scheduledCycle is the updateCycleWithRelated payload scheduleTaskIntoCycle
// selects: the Cycle and the Tasks now planned into it.
type scheduledCycle struct {
	UpdateCycleWithRelated struct {
		ID    string `json:"id"`
		Tasks []struct {
			ID      string  `json:"id"`
			CycleID *string `json:"cycleID"`
		} `json:"tasks"`
	} `json:"updateCycleWithRelated"`
}

// readTaskCycle reads a Task's cycleID back through task(id).
func readTaskCycle(t *testing.T, token, wsID, taskID string) *string {
	t.Helper()
	var back struct {
		Task struct {
			CycleID *string `json:"cycleID"`
		} `json:"task"`
	}
	gqlExecData(t, taskCycleQuery, map[string]any{"id": taskID}, workspaceHeaders(token, wsID), &back)
	return back.Task.CycleID
}

// updateTaskCycleMutation drives the RAW generated updateTask to set cycleID —
// the path the composite FK (migration 0004) has to backstop, since updateTask
// is tenant-scoped only on the Task, not on the referenced Cycle.
const updateTaskCycleMutation = `
	mutation ($id: UUID!, $cycleID: UUID) {
		updateTask(id: $id, input: { cycleID: $cycleID }) { id cycleID }
	}`

// cycleInput builds a valid createCycle input for the given Active Workspace with
// a one-week date range. status/name are caller-supplied; the rest are the
// required scalar columns.
func cycleInput(wsID, name, status string) map[string]any {
	return map[string]any{
		"workspaceID": wsID,
		"name":        name,
		"status":      status,
		"startsOn":    "2026-09-01T00:00:00Z",
		"endsOn":      "2026-09-14T00:00:00Z",
		"createdAt":   "2026-08-25T00:00:00Z",
		"updatedAt":   "2026-08-25T00:00:00Z",
	}
}

type cycleResult struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceID"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	StartsOn    string `json:"startsOn"`
	EndsOn      string `json:"endsOn"`
}

// createCycle drives the generated createCycle mutation as the given caller and
// asserts success, returning the new Cycle. The caller must hold a manager Role
// (owner/admin) — the authz hook rejects a plain member.
func createCycle(t *testing.T, token, wsID, name, status string) cycleResult {
	t.Helper()
	var out struct {
		CreateCycle cycleResult `json:"createCycle"`
	}
	gqlExecData(t, createCycleMutation, map[string]any{"input": cycleInput(wsID, name, status)},
		workspaceHeaders(token, wsID), &out)
	if out.CreateCycle.ID == "" {
		t.Fatal("createCycle returned empty cycle id")
	}
	return out.CreateCycle
}

// TestAdminCreatesCycleWithDateRangeAndStatus proves a Workspace admin can define
// a Cycle with a date range and status (story 32). The owner (a manager) creates
// it and the stored range/status round-trip.
func TestAdminCreatesCycleWithDateRangeAndStatus(t *testing.T) {
	ownerToken, _ := register(t, "cycle-owner@example.com", "cycle-pw-00001", "Cycle Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Cycling", "cycling-ws")

	cycle := createCycle(t, ownerToken, wsID, "Sprint 1", "UPCOMING")
	if cycle.WorkspaceID != wsID {
		t.Errorf("cycle workspaceID = %q, want %q (auto-scoped to Active Workspace)", cycle.WorkspaceID, wsID)
	}
	if cycle.Status != "UPCOMING" {
		t.Errorf("cycle status = %q, want UPCOMING", cycle.Status)
	}
	if cycle.StartsOn == "" || cycle.EndsOn == "" {
		t.Errorf("cycle date range not persisted: startsOn=%q endsOn=%q", cycle.StartsOn, cycle.EndsOn)
	}
}

// TestCycleManagementGuardedByRole proves defining a Cycle is owner/admin-only,
// enforced by the authz hook (story 32): a plain member is rejected creating a
// Cycle, while the owner (the control) succeeds. Mirrors the Label-catalog guard.
func TestCycleManagementGuardedByRole(t *testing.T) {
	ownerToken, _ := register(t, "cycle-guard-owner@example.com", "cycle-pw-00002", "Cycle Guard Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "CycleGuard", "cycle-guard-ws")

	memberToken, member := register(t, "cycle-guard-member@example.com", "cycle-pw-00003", "Cycle Guard Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	// A plain member cannot create a Cycle — guarded by the authz hook.
	memberResp := gqlExec(t, createCycleMutation, map[string]any{"input": cycleInput(wsID, "member-made", "UPCOMING")},
		workspaceHeaders(memberToken, wsID))
	if len(memberResp.Errors) == 0 {
		t.Error("a member was allowed to create a cycle; owner/admin-only operation not guarded (story 32)")
	}

	// Control: the owner CAN create a Cycle, so the rejection is about Role.
	if c := createCycle(t, ownerToken, wsID, "owner-made", "UPCOMING"); c.ID == "" {
		t.Error("owner cycle create failed, control broken")
	}
}

// TestMemberSchedulesTaskIntoCycle proves a member can schedule a Task into a
// Cycle (story 31): the owner defines the Cycle, a plain member schedules the
// Task into it via updateCycleWithRelated (the empty Cycle update is not
// admin-guarded), and the Task's cycleID reads back pointing at the Cycle.
func TestMemberSchedulesTaskIntoCycle(t *testing.T) {
	ownerToken, _ := register(t, "sched-owner@example.com", "sched-pw-00001", "Sched Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Scheduling", "scheduling-ws")
	projID := createProject(t, ownerToken, wsID, "Planned work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Plan me", "TODO", "MEDIUM", "")

	// The manager defines the Cycle; a plain member joins and schedules.
	cycle := createCycle(t, ownerToken, wsID, "Sprint 2", "ACTIVE")
	memberToken, member := register(t, "sched-member@example.com", "sched-pw-00002", "Sched Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	var out scheduledCycle
	gqlExecData(t, scheduleTaskIntoCycleMutation,
		map[string]any{"taskID": task.ID, "cycleID": cycle.ID},
		workspaceHeaders(memberToken, wsID), &out)
	got := out.UpdateCycleWithRelated.Tasks
	if len(got) != 1 || got[0].ID != task.ID || got[0].CycleID == nil || *got[0].CycleID != cycle.ID {
		t.Fatalf("scheduled cycle tasks = %+v, want task %s with cycleID %q", got, task.ID, cycle.ID)
	}

	// Read back through task(id) to confirm the schedule persisted, and through the
	// Cycle's tasks relationship to confirm the Task is planned into the iteration.
	if back := readTaskCycle(t, memberToken, wsID, task.ID); back == nil || *back != cycle.ID {
		t.Errorf("read-back task cycleID = %v, want %q", back, cycle.ID)
	}

	const cycleTasksQuery = `query ($id: UUID!) { cycle(id: $id) { tasks { id } } }`
	var cts struct {
		Cycle struct {
			Tasks []struct {
				ID string `json:"id"`
			} `json:"tasks"`
		} `json:"cycle"`
	}
	gqlExecData(t, cycleTasksQuery, map[string]any{"id": cycle.ID}, workspaceHeaders(memberToken, wsID), &cts)
	if len(cts.Cycle.Tasks) != 1 || cts.Cycle.Tasks[0].ID != task.ID {
		t.Errorf("cycle.tasks = %+v, want the scheduled task %s", cts.Cycle.Tasks, task.ID)
	}
}

// TestMemberReschedulesTaskIntoAnotherCycle proves a Task already planned into
// one Cycle moves to another by connecting it there (allow_reparent on
// cycles.Tasks): without it the nested connect adopts only unscheduled Tasks.
func TestMemberReschedulesTaskIntoAnotherCycle(t *testing.T) {
	ownerToken, _ := register(t, "resched-owner@example.com", "resched-pw-0001", "Resched Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Rescheduling", "rescheduling-ws")
	projID := createProject(t, ownerToken, wsID, "Moving work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Move me", "TODO", "MEDIUM", "")
	first := createCycle(t, ownerToken, wsID, "Sprint 1", "ACTIVE")
	second := createCycle(t, ownerToken, wsID, "Sprint 2", "UPCOMING")

	memberToken, member := register(t, "resched-member@example.com", "resched-pw-0002", "Resched Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")
	h := workspaceHeaders(memberToken, wsID)

	gqlExecData(t, scheduleTaskIntoCycleMutation, map[string]any{"taskID": task.ID, "cycleID": first.ID}, h, &scheduledCycle{})
	gqlExecData(t, scheduleTaskIntoCycleMutation, map[string]any{"taskID": task.ID, "cycleID": second.ID}, h, &scheduledCycle{})

	if back := readTaskCycle(t, memberToken, wsID, task.ID); back == nil || *back != second.ID {
		t.Errorf("rescheduled task cycleID = %v, want %q", back, second.ID)
	}
}

// TestMemberCannotEditCycleThroughNestedUpdate proves the nested mutation opens
// scheduling to members without opening the Cycle catalog: setting a Cycle
// column through updateCycleWithRelated is still owner/admin-only (story 32).
func TestMemberCannotEditCycleThroughNestedUpdate(t *testing.T) {
	ownerToken, _ := register(t, "nested-cycle-owner@example.com", "nested-cyc-pw-01", "Nested Cycle Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "NestedCycle", "nested-cycle-ws")
	cycle := createCycle(t, ownerToken, wsID, "Sprint", "UPCOMING")

	memberToken, member := register(t, "nested-cycle-member@example.com", "nested-cyc-pw-02", "Nested Cycle Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	resp := gqlExec(t, renameCycleNestedMutation, map[string]any{"cycleID": cycle.ID, "name": "member-renamed"},
		workspaceHeaders(memberToken, wsID))
	if len(resp.Errors) == 0 {
		t.Error("a member renamed a cycle through updateCycleWithRelated; owner/admin-only write not guarded")
	}

	// Control: the owner CAN, so the rejection is about Role.
	var out struct {
		UpdateCycleWithRelated struct {
			Name string `json:"name"`
		} `json:"updateCycleWithRelated"`
	}
	gqlExecData(t, renameCycleNestedMutation, map[string]any{"cycleID": cycle.ID, "name": "owner-renamed"},
		workspaceHeaders(ownerToken, wsID), &out)
	if out.UpdateCycleWithRelated.Name != "owner-renamed" {
		t.Errorf("owner rename returned name %q, want owner-renamed", out.UpdateCycleWithRelated.Name)
	}
}

// TestScheduleRejectsCrossWorkspaceCycle proves scheduling guards tenant
// isolation in both directions: a Cycle from another Workspace cannot be
// scheduled into (the nested update resolves the Cycle tenant-scoped first), and
// a Task from another Workspace cannot be connected (connect reads it through the
// tenant-scoped Tasks client) — closing the hole that tasks.cycle_id (an FK with
// no Workspace constraint) would otherwise leave open to the generated updateTask.
func TestScheduleRejectsCrossWorkspaceCycle(t *testing.T) {
	token, _ := register(t, "xws-cycle@example.com", "xws-cycle-pw01", "XWS Cycle")
	wsA := bootstrapWorkspace(t, token, "CycleA", "cycle-a-ws")
	wsB := bootstrapWorkspace(t, token, "CycleB", "cycle-b-ws")

	projA := createProject(t, token, wsA, "A work")
	taskA := createTaskInProject(t, token, wsA, projA, "In A", "TODO", "MEDIUM", "")
	cycleA := createCycle(t, token, wsA, "A sprint", "UPCOMING")
	cycleB := createCycle(t, token, wsB, "B sprint", "UPCOMING")
	projB := createProject(t, token, wsB, "B work")
	taskB := createTaskInProject(t, token, wsB, projB, "In B", "TODO", "MEDIUM", "")

	// Active in A, referencing a Cycle that lives in B — B's Cycle is invisible
	// here, so the scheduling is refused before any row is written.
	resp := gqlExec(t, scheduleTaskIntoCycleMutation,
		map[string]any{"taskID": taskA.ID, "cycleID": cycleB.ID},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("a task was scheduled into a cycle in another workspace; cross-tenant scheduling not guarded")
	}
	if back := readTaskCycle(t, token, wsA, taskA.ID); back != nil {
		t.Errorf("taskA cycleID = %v after a rejected cross-workspace schedule, want null", *back)
	}

	// Active in A, connecting B's Task into A's Cycle — B's Task is invisible here.
	resp = gqlExec(t, scheduleTaskIntoCycleMutation,
		map[string]any{"taskID": taskB.ID, "cycleID": cycleA.ID},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("a task from another workspace was scheduled into this cycle; cross-tenant connect not guarded")
	}
	if back := readTaskCycle(t, token, wsB, taskB.ID); back != nil {
		t.Errorf("taskB cycleID = %v after a rejected cross-workspace connect, want null", *back)
	}
}

// TestRawUpdateTaskCannotScheduleCrossWorkspaceCycle proves the composite FK
// (migration 0004, tasks_cycle_same_ws) closes the cross-tenant scheduling hole
// at the DATABASE, not only in the nested mutation. The generated updateTask is
// tenant-scoped on the Task but places no Workspace constraint on the cycleID it
// writes; the composite FK (workspace_id, cycle_id) -> cycles(workspace_id, id)
// makes Postgres reject a Cycle from another Workspace, whatever the caller path.
func TestRawUpdateTaskCannotScheduleCrossWorkspaceCycle(t *testing.T) {
	token, _ := register(t, "rawsched@example.com", "rawsched-pw-01", "Raw Sched")
	wsA := bootstrapWorkspace(t, token, "RawSchedA", "raw-sched-a-ws")
	wsB := bootstrapWorkspace(t, token, "RawSchedB", "raw-sched-b-ws")

	projA := createProject(t, token, wsA, "A work")
	taskA := createTaskInProject(t, token, wsA, projA, "In A", "TODO", "MEDIUM", "")
	cycleB := createCycle(t, token, wsB, "B sprint", "UPCOMING")

	// Raw updateTask, active in A, pointing at B's Cycle — rejected by the FK at
	// the DB even though updateTask never consults the Cycle's tenant itself.
	resp := gqlExec(t, updateTaskCycleMutation,
		map[string]any{"id": taskA.ID, "cycleID": cycleB.ID},
		workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("raw updateTask scheduled a task into a cross-workspace cycle; composite FK not enforced")
	}

	// Nothing persisted.
	if back := readTaskCycle(t, token, wsA, taskA.ID); back != nil {
		t.Errorf("taskA cycleID = %v after a rejected raw cross-workspace update, want null", *back)
	}

	// Control: a SAME-Workspace Cycle via the raw updateTask still succeeds, so the
	// rejection above is specifically the cross-tenant pair, not a broken mutation.
	cycleA := createCycle(t, token, wsA, "A sprint", "UPCOMING")
	var out struct {
		UpdateTask struct {
			CycleID *string `json:"cycleID"`
		} `json:"updateTask"`
	}
	gqlExecData(t, updateTaskCycleMutation,
		map[string]any{"id": taskA.ID, "cycleID": cycleA.ID},
		workspaceHeaders(token, wsA), &out)
	if out.UpdateTask.CycleID == nil || *out.UpdateTask.CycleID != cycleA.ID {
		t.Errorf("same-workspace raw updateTask cycleID = %v, want %q", out.UpdateTask.CycleID, cycleA.ID)
	}
}
