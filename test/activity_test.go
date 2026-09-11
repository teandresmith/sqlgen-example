package test

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"
	"time"
)

// This file proves the NATS events → Activity projector → feed pipeline (issue
// 06) through the one seam the harness exercises: the GraphQL HTTP endpoint. A
// mutation publishes an event on the real NATS testcontainer; the projector
// consumes it and writes an activity row; the feed query surfaces it. Because
// projection is asynchronous, the assertions poll the feed with a bounded
// timeout — the same eventual-consistency pattern the cache suite uses.

// activityFeedQuery reads the Active Workspace's activity feed for one entity,
// newest first. The feed is tenant-scoped by the generated query, so it only
// ever returns rows in the caller's Active Workspace (story 44).
const activityFeedQuery = `
	query ($entityID: IDComparator) {
		activityList(
			filter: { entityID: $entityID }
			sort: [{ field: CREATED_AT, direction: DESC }]
		) {
			totalCount
			items { entityTable entityID action actorID payload }
		}
	}`

type activityEntry struct {
	EntityTable string          `json:"entityTable"`
	EntityID    string          `json:"entityID"`
	Action      string          `json:"action"`
	ActorID     *string         `json:"actorID"`
	Payload     json.RawMessage `json:"payload"`
}

// activityForEntity returns the current feed entries for one entity id in the
// given Active Workspace. It is a point-in-time read; callers poll it.
func activityForEntity(t *testing.T, token, wsID, entityID string) []activityEntry {
	t.Helper()
	var out struct {
		ActivityList struct {
			Items []activityEntry `json:"items"`
		} `json:"activityList"`
	}
	gqlExecData(t, activityFeedQuery,
		map[string]any{"entityID": map[string]any{"eq": entityID}},
		workspaceHeaders(token, wsID), &out)
	return out.ActivityList.Items
}

const createLabelMutation = `
	mutation ($input: CreateLabelInput!) {
		createLabel(input: $input) { id }
	}`

// createLabel drives the generated createLabel mutation in the given Active
// Workspace and returns the new label id, with any extra request headers merged
// over the Active-Workspace headers. The generated resolver threads the HTTP
// call-option headers (PRD §26.11), so passing X-Skip-Events here exercises the
// suppress path; labels are a tenanted, single-uuid-PK table, so an emitted
// event projects to the activity feed.
func createLabel(t *testing.T, token, wsID, name string, extra map[string]string) string {
	t.Helper()
	headers := workspaceHeaders(token, wsID)
	maps.Copy(headers, extra)
	var out struct {
		CreateLabel struct {
			ID string `json:"id"`
		} `json:"createLabel"`
	}
	gqlExecData(t, createLabelMutation, map[string]any{
		"input": map[string]any{
			"workspaceID": wsID,
			"name":        name,
			"color":       "#3366FF",
			"createdAt":   "2026-07-14T00:00:00Z",
		},
	}, headers, &out)
	if out.CreateLabel.ID == "" {
		t.Fatal("createLabel returned empty label id")
	}
	return out.CreateLabel.ID
}

// TestTaskMutationProjectsActivity proves the end-to-end pipeline: creating a
// Task publishes an event over NATS, the projector writes an activity row, and
// the feed exposes it — recording the affected entity, the action, the actor
// (the mutating caller), and a payload snapshot of the change (AC 1, 2, 6).
func TestTaskMutationProjectsActivity(t *testing.T) {
	token, owner := register(t, "activity-owner@example.com", "activity-pw-0001", "Activity Owner")
	wsID := bootstrapWorkspace(t, token, "Activity", "activity-ws")
	projID := createProject(t, token, wsID, "Feed")

	task := createTaskInProject(t, token, wsID, projID, "Watch me", "TODO", "MEDIUM", "")

	// Projection is asynchronous (mutation → NATS → projector → row), so poll the
	// feed until the create entry appears, bounded so a broken pipeline fails
	// rather than hangs.
	var entry activityEntry
	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		for _, e := range activityForEntity(t, token, wsID, task.ID) {
			if e.EntityTable == "tasks" && e.Action == "create" {
				entry, found = e, true
				break
			}
		}
		if !found {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !found {
		t.Fatalf("no create activity for task %s appeared within 3s", task.ID)
	}

	if entry.EntityID != task.ID {
		t.Errorf("activity entityID = %q, want %q", entry.EntityID, task.ID)
	}
	if entry.ActorID == nil || *entry.ActorID != owner.ID {
		t.Errorf("activity actorID = %v, want %q (the mutating caller)", entry.ActorID, owner.ID)
	}
	// The payload is a snapshot of the change: the created Task's title is in it.
	if !strings.Contains(string(entry.Payload), "Watch me") {
		t.Errorf("activity payload = %s, want a snapshot containing the task title", entry.Payload)
	}
}

// createTaskWithHeaders drives the custom createTaskInProject mutation with extra
// request headers merged over the Active-Workspace headers — used to attach
// X-Skip-Events to the hand-written resolver path.
func createTaskWithHeaders(t *testing.T, token, wsID, projID, title string, extra map[string]string) taskResult {
	t.Helper()
	headers := workspaceHeaders(token, wsID)
	maps.Copy(headers, extra)
	var out struct {
		CreateTaskInProject taskResult `json:"createTaskInProject"`
	}
	gqlExecData(t, createTaskInProjectMutation,
		map[string]any{"input": map[string]any{"projectID": projID, "title": title}},
		headers, &out)
	if out.CreateTaskInProject.ID == "" {
		t.Fatal("createTaskWithHeaders returned empty task id")
	}
	return out.CreateTaskInProject
}

// TestSkipEventsHeaderSuppressesCustomResolver proves the issue-07 fix: the
// hand-written createTaskInProject resolver now threads the per-request HTTP call
// options, so X-Skip-Events suppresses its event — and thus its activity — just
// as it does on generated mutations. Before the fix the resolver called the
// client directly and ignored the header, so the suppressed task still projected.
func TestSkipEventsHeaderSuppressesCustomResolver(t *testing.T) {
	token, _ := register(t, "activity-custom-skip@example.com", "activity-pw-0006", "Custom Skip")
	wsID := bootstrapWorkspace(t, token, "CustomSkip", "custom-skip-ws")
	projID := createProject(t, token, wsID, "Quiet Tasks")

	suppressed := createTaskWithHeaders(t, token, wsID, projID, "Silent task",
		map[string]string{"X-Skip-Events": "true"})
	marker := createTaskInProject(t, token, wsID, projID, "Loud task", "TODO", "MEDIUM", "")

	markerSeen := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !markerSeen {
		if len(activityForEntity(t, token, wsID, marker.ID)) > 0 {
			markerSeen = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !markerSeen {
		t.Fatal("marker task activity never projected within 3s; cannot conclude about suppression")
	}
	if got := activityForEntity(t, token, wsID, suppressed.ID); len(got) != 0 {
		t.Errorf("suppressed task has %d activity entries, want 0 (createTaskInProject must honor X-Skip-Events)", len(got))
	}
}

// TestTaskDependencyAnchorsToDependentTask proves a junction mutation is
// projected onto its anchor entity, not onto the junction row (ADR-0006). A
// task_dependencies row is a composite-key relationship carrying no tenant stamp
// of its own, so the projector must (a) record the event on the dependent task —
// a real entity id — and (b) resolve the workspace by looking the task up. Seeing
// the entry in the workspace-scoped feed proves both.
func TestTaskDependencyAnchorsToDependentTask(t *testing.T) {
	token, _ := register(t, "activity-junction@example.com", "activity-pw-0003", "Activity Junction")
	wsID := bootstrapWorkspace(t, token, "Junctions", "junctions-ws")
	projID := createProject(t, token, wsID, "Anchoring")

	blocked := createTaskInProject(t, token, wsID, projID, "Ship release", "TODO", "HIGH", "")
	blocker := createTaskInProject(t, token, wsID, projID, "Cut branch", "TODO", "HIGH", "")

	var dep struct {
		AddTaskDependency struct {
			TaskID string `json:"taskID"`
		} `json:"addTaskDependency"`
	}
	gqlExecData(t, addTaskDependencyMutation, map[string]any{
		"taskID": blocked.ID, "dependsOn": blocker.ID, "type": nil,
	}, workspaceHeaders(token, wsID), &dep)

	// The dependency surfaces as activity on the dependent task (not a
	// task_dependencies row), with the depended-on task captured in the payload.
	var entry activityEntry
	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		for _, e := range activityForEntity(t, token, wsID, blocked.ID) {
			if e.EntityTable == "tasks" && strings.Contains(string(e.Payload), blocker.ID) {
				entry, found = e, true
				break
			}
		}
		if !found {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !found {
		t.Fatalf("no dependency activity anchored to task %s appeared within 3s", blocked.ID)
	}
	if entry.EntityTable != "tasks" {
		t.Errorf("entityTable = %q, want tasks (anchored to the entity, not the junction)", entry.EntityTable)
	}
	if entry.EntityID != blocked.ID {
		t.Errorf("entityID = %q, want %q (the dependent task)", entry.EntityID, blocked.ID)
	}
}

// TestMembershipAnchorsToWorkspace proves a membership — a composite-key relation
// whose tenant is already on the event — is projected onto its workspace
// (ADR-0006): a member joining surfaces as activity on the workspace, with the
// member's user id in the payload.
func TestMembershipAnchorsToWorkspace(t *testing.T) {
	ownerToken, _ := register(t, "activity-ms-owner@example.com", "activity-pw-0004", "MS Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "MembershipFeed", "membership-feed-ws")

	memberToken, member := register(t, "activity-ms-member@example.com", "activity-pw-0005", "MS Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !found {
		for _, e := range activityForEntity(t, ownerToken, wsID, wsID) {
			if e.EntityTable == "workspaces" && strings.Contains(string(e.Payload), member.ID) {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !found {
		t.Fatalf("no membership activity for member %s anchored to workspace %s within 3s", member.ID, wsID)
	}
}

// TestSkipEventsHeaderSuppressesActivity proves the X-Skip-Events request header
// prevents the side effect (AC 5 / story 47): a suppressed mutation publishes no
// event, so no activity is ever projected for it. A second, un-suppressed label
// created afterward acts as an ordering marker — once its activity appears, the
// suppressed label's (emitted earlier on the same table's subject) would have
// too if it existed, so an empty suppressed feed at that point is conclusive.
func TestSkipEventsHeaderSuppressesActivity(t *testing.T) {
	token, _ := register(t, "activity-skip@example.com", "activity-pw-0002", "Activity Skip")
	wsID := bootstrapWorkspace(t, token, "SkipEvents", "skip-events-ws")

	suppressedID := createLabel(t, token, wsID, "silent", map[string]string{"X-Skip-Events": "true"})
	markerID := createLabel(t, token, wsID, "loud", nil)

	// Wait for the marker's activity — proof the pipeline is flowing.
	markerSeen := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !markerSeen {
		if len(activityForEntity(t, token, wsID, markerID)) > 0 {
			markerSeen = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !markerSeen {
		t.Fatal("marker label activity never projected within 3s; cannot conclude about suppression")
	}

	// The marker projected but the suppressed label must have generated nothing.
	if got := activityForEntity(t, token, wsID, suppressedID); len(got) != 0 {
		t.Errorf("suppressed label has %d activity entries, want 0 (X-Skip-Events must prevent projection)", len(got))
	}
}
