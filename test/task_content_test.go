package test

import (
	"testing"

	"github.com/shopspring/decimal"
)

// This file proves issue 09 (Task content & time) through the one seam the
// harness exercises — the GraphQL HTTP endpoint under an Active Workspace. It
// drives the blessed commentOnTask / attachToTask / attachToComment / logTime
// mutations and reads the results back through the Task's (and Comment's)
// relationship fields, plus a cross-workspace guard for the polymorphic
// attachment binding and a regression that the raw create mutations are masked.

const commentOnTaskMutation = `
	mutation ($taskID: UUID!, $body: String!) {
		commentOnTask(taskID: $taskID, body: $body) { id taskID authorID body }
	}`

const attachToTaskMutation = `
	mutation ($taskID: UUID!, $fileName: String!, $contentType: String!, $sizeBytes: Int!, $storageURL: String!) {
		attachToTask(taskID: $taskID, fileName: $fileName, contentType: $contentType, sizeBytes: $sizeBytes, storageURL: $storageURL) {
			id entityType entityID uploadedBy fileName contentType sizeBytes storageURL
		}
	}`

const attachToCommentMutation = `
	mutation ($commentID: UUID!, $fileName: String!, $contentType: String!, $sizeBytes: Int!, $storageURL: String!) {
		attachToComment(commentID: $commentID, fileName: $fileName, contentType: $contentType, sizeBytes: $sizeBytes, storageURL: $storageURL) {
			id entityType entityID uploadedBy fileName
		}
	}`

const logTimeMutation = `
	mutation ($taskID: UUID!, $hours: Decimal!, $spentOn: Time!, $billable: Boolean, $notes: String) {
		logTime(taskID: $taskID, hours: $hours, spentOn: $spentOn, billable: $billable, notes: $notes) {
			id taskID userID hours billable notes
		}
	}`

// taskContentQuery reads a Task's comments and time entries back through the
// generated O2M relationship fields, plus its direct attachments.
const taskContentQuery = `
	query ($id: UUID!) {
		task(id: $id) {
			id
			comments { id body authorID attachments { id fileName entityType } }
			attachments { id fileName entityType uploadedBy }
			timeEntries { id hours billable notes userID }
		}
	}`

// TestMemberCommentsOnTask proves a member can comment on a Task (story 29) and
// that the author is stamped server-side to the authenticated caller, never the
// input. The comment reads back on task.comments.
func TestMemberCommentsOnTask(t *testing.T) {
	ownerToken, _ := register(t, "comment-owner@example.com", "comment-pw-0001", "Comment Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Commenting", "commenting-ws")
	projID := createProject(t, ownerToken, wsID, "Discussed work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Talk about it", "TODO", "MEDIUM", "")

	// A plain member (not the owner) leaves the comment; the author must be them.
	memberToken, member := register(t, "comment-member@example.com", "comment-pw-0002", "Comment Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")

	var out struct {
		CommentOnTask struct {
			ID       string `json:"id"`
			TaskID   string `json:"taskID"`
			AuthorID string `json:"authorID"`
			Body     string `json:"body"`
		} `json:"commentOnTask"`
	}
	gqlExecData(t, commentOnTaskMutation,
		map[string]any{"taskID": task.ID, "body": "Looks good to me"},
		workspaceHeaders(memberToken, wsID), &out)

	if out.CommentOnTask.AuthorID != member.ID {
		t.Errorf("comment authorID = %q, want the caller %q (server-owned provenance)", out.CommentOnTask.AuthorID, member.ID)
	}
	if out.CommentOnTask.TaskID != task.ID {
		t.Errorf("comment taskID = %q, want %q", out.CommentOnTask.TaskID, task.ID)
	}
	if out.CommentOnTask.Body != "Looks good to me" {
		t.Errorf("comment body = %q, want %q", out.CommentOnTask.Body, "Looks good to me")
	}

	content := readTaskContent(t, memberToken, wsID, task.ID)
	if len(content.Comments) != 1 || content.Comments[0].ID != out.CommentOnTask.ID {
		t.Fatalf("task comments = %+v, want the one just created", content.Comments)
	}
	if content.Comments[0].AuthorID != member.ID {
		t.Errorf("read-back comment authorID = %q, want %q", content.Comments[0].AuthorID, member.ID)
	}
}

// TestMemberAttachesFileToTask proves a member can attach a file to a Task
// (story 33): the server sets entityType = TASK, binds entityID to the Task, and
// stamps the uploader to the caller. The attachment reads back on
// task.attachments.
func TestMemberAttachesFileToTask(t *testing.T) {
	ownerToken, owner := register(t, "attach-task-owner@example.com", "attach-pw-0001", "Attach Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "AttachingTask", "attaching-task-ws")
	projID := createProject(t, ownerToken, wsID, "Filed work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Has a file", "TODO", "MEDIUM", "")

	var out struct {
		AttachToTask struct {
			ID         string `json:"id"`
			EntityType string `json:"entityType"`
			EntityID   string `json:"entityID"`
			UploadedBy string `json:"uploadedBy"`
			FileName   string `json:"fileName"`
			SizeBytes  int64  `json:"sizeBytes"`
		} `json:"attachToTask"`
	}
	gqlExecData(t, attachToTaskMutation, map[string]any{
		"taskID": task.ID, "fileName": "spec.pdf", "contentType": "application/pdf",
		"sizeBytes": 20480, "storageURL": "s3://bucket/spec.pdf",
	}, workspaceHeaders(ownerToken, wsID), &out)

	if out.AttachToTask.EntityType != "TASK" {
		t.Errorf("attachment entityType = %q, want TASK (server-set discriminator)", out.AttachToTask.EntityType)
	}
	if out.AttachToTask.EntityID != task.ID {
		t.Errorf("attachment entityID = %q, want the task %q", out.AttachToTask.EntityID, task.ID)
	}
	if out.AttachToTask.UploadedBy != owner.ID {
		t.Errorf("attachment uploadedBy = %q, want the caller %q", out.AttachToTask.UploadedBy, owner.ID)
	}
	if out.AttachToTask.SizeBytes != 20480 {
		t.Errorf("attachment sizeBytes = %d, want 20480", out.AttachToTask.SizeBytes)
	}

	content := readTaskContent(t, ownerToken, wsID, task.ID)
	if len(content.Attachments) != 1 || content.Attachments[0].ID != out.AttachToTask.ID {
		t.Fatalf("task attachments = %+v, want the one just created", content.Attachments)
	}
	if content.Attachments[0].EntityType != "TASK" {
		t.Errorf("read-back attachment entityType = %q, want TASK", content.Attachments[0].EntityType)
	}
}

// TestMemberAttachesFileToComment proves a member can attach a file to a Comment
// (story 33): the server sets entityType = COMMENT and binds entityID to the
// Comment. The attachment reads back on the Comment's attachments field (a Task's
// attachments filter to entity_type = 'task', so it must NOT appear there).
func TestMemberAttachesFileToComment(t *testing.T) {
	ownerToken, owner := register(t, "attach-comment-owner@example.com", "attach-pw-0002", "AttachC Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "AttachingComment", "attaching-comment-ws")
	projID := createProject(t, ownerToken, wsID, "Commented work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Has a comment", "TODO", "MEDIUM", "")

	// A comment must exist to attach to.
	var cout struct {
		CommentOnTask struct {
			ID string `json:"id"`
		} `json:"commentOnTask"`
	}
	gqlExecData(t, commentOnTaskMutation,
		map[string]any{"taskID": task.ID, "body": "See the attached mockup"},
		workspaceHeaders(ownerToken, wsID), &cout)
	commentID := cout.CommentOnTask.ID

	var out struct {
		AttachToComment struct {
			ID         string `json:"id"`
			EntityType string `json:"entityType"`
			EntityID   string `json:"entityID"`
			UploadedBy string `json:"uploadedBy"`
			FileName   string `json:"fileName"`
		} `json:"attachToComment"`
	}
	gqlExecData(t, attachToCommentMutation, map[string]any{
		"commentID": commentID, "fileName": "mockup.png", "contentType": "image/png",
		"sizeBytes": 8192, "storageURL": "s3://bucket/mockup.png",
	}, workspaceHeaders(ownerToken, wsID), &out)

	if out.AttachToComment.EntityType != "COMMENT" {
		t.Errorf("attachment entityType = %q, want COMMENT", out.AttachToComment.EntityType)
	}
	if out.AttachToComment.EntityID != commentID {
		t.Errorf("attachment entityID = %q, want the comment %q", out.AttachToComment.EntityID, commentID)
	}
	if out.AttachToComment.UploadedBy != owner.ID {
		t.Errorf("attachment uploadedBy = %q, want the caller %q", out.AttachToComment.UploadedBy, owner.ID)
	}

	// The Comment-bound attachment surfaces on the Comment's attachments (the
	// discriminator filter), and the Task's own attachments stay empty.
	content := readTaskContent(t, ownerToken, wsID, task.ID)
	if len(content.Attachments) != 0 {
		t.Errorf("task.attachments = %+v, want empty (the file is bound to the comment, not the task)", content.Attachments)
	}
	if len(content.Comments) != 1 {
		t.Fatalf("task comments = %d, want 1", len(content.Comments))
	}
	commentAtts := content.Comments[0].Attachments
	if len(commentAtts) != 1 || commentAtts[0].ID != out.AttachToComment.ID {
		t.Fatalf("comment attachments = %+v, want the one just created", commentAtts)
	}
	if commentAtts[0].EntityType != "COMMENT" {
		t.Errorf("read-back comment attachment entityType = %q, want COMMENT", commentAtts[0].EntityType)
	}
}

// TestMemberLogsTimeInDecimalHours proves a member can log a Time Entry against a
// Task in decimal hours (story 34), that the logging User is stamped to the
// caller, and that the decimal quantity round-trips exactly. It reads back on
// task.timeEntries.
func TestMemberLogsTimeInDecimalHours(t *testing.T) {
	ownerToken, owner := register(t, "logtime-owner@example.com", "logtime-pw-0001", "LogTime Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "LoggingTime", "logging-time-ws")
	projID := createProject(t, ownerToken, wsID, "Timed work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Track effort", "IN_PROGRESS", "HIGH", "")

	notes := "pairing session"
	var out struct {
		LogTime struct {
			ID       string `json:"id"`
			TaskID   string `json:"taskID"`
			UserID   string `json:"userID"`
			Hours    string `json:"hours"`
			Billable bool   `json:"billable"`
			Notes    string `json:"notes"`
		} `json:"logTime"`
	}
	gqlExecData(t, logTimeMutation, map[string]any{
		"taskID": task.ID, "hours": "2.50", "spentOn": "2026-08-25T00:00:00Z",
		"billable": false, "notes": notes,
	}, workspaceHeaders(ownerToken, wsID), &out)

	if out.LogTime.UserID != owner.ID {
		t.Errorf("time entry userID = %q, want the caller %q (server-owned actor)", out.LogTime.UserID, owner.ID)
	}
	if !decimal.RequireFromString(out.LogTime.Hours).Equal(decimal.RequireFromString("2.5")) {
		t.Errorf("time entry hours = %q, want 2.5 (decimal round-trip)", out.LogTime.Hours)
	}
	if out.LogTime.Billable {
		t.Error("time entry billable = true, want false (explicit override honored)")
	}
	if out.LogTime.Notes != notes {
		t.Errorf("time entry notes = %q, want %q", out.LogTime.Notes, notes)
	}

	content := readTaskContent(t, ownerToken, wsID, task.ID)
	if len(content.TimeEntries) != 1 || content.TimeEntries[0].ID != out.LogTime.ID {
		t.Fatalf("task timeEntries = %+v, want the one just logged", content.TimeEntries)
	}
	if !decimal.RequireFromString(content.TimeEntries[0].Hours).Equal(decimal.RequireFromString("2.5")) {
		t.Errorf("read-back hours = %q, want 2.5", content.TimeEntries[0].Hours)
	}
}

// TestLogTimeDefaultsBillableTrue proves the billable default (true) is applied
// when the argument is omitted — the blessed resolver leaves it to the DB default
// rather than forcing a value.
func TestLogTimeDefaultsBillableTrue(t *testing.T) {
	ownerToken, _ := register(t, "billable-owner@example.com", "billable-pw-001", "Billable Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "Billing", "billing-ws")
	projID := createProject(t, ownerToken, wsID, "Billed work")
	task := createTaskInProject(t, ownerToken, wsID, projID, "Bill it", "TODO", "MEDIUM", "")

	var out struct {
		LogTime struct {
			Billable bool `json:"billable"`
		} `json:"logTime"`
	}
	gqlExecData(t, logTimeMutation, map[string]any{
		"taskID": task.ID, "hours": "1.25", "spentOn": "2026-08-25T00:00:00Z",
	}, workspaceHeaders(ownerToken, wsID), &out)
	if !out.LogTime.Billable {
		t.Error("time entry billable = false, want true (DB default when omitted)")
	}
}

// TestAttachToTaskRejectsCrossWorkspaceTask proves the blessed attachToTask
// guards the polymorphic binding: because attachments.entity_id has no foreign
// key, a raw create could point a file at a Task in another Workspace. The
// resolver resolves the Task tenant-scoped first, so a Task from another
// Workspace is invisible and the attach is refused before any row is written.
func TestAttachToTaskRejectsCrossWorkspaceTask(t *testing.T) {
	token, _ := register(t, "xws-attach@example.com", "xws-attach-pw01", "XWS Attach")
	wsA := bootstrapWorkspace(t, token, "AttachA", "attach-a-ws")
	wsB := bootstrapWorkspace(t, token, "AttachB", "attach-b-ws")

	projB := createProject(t, token, wsB, "B work")
	taskB := createTaskInProject(t, token, wsB, projB, "In B", "TODO", "MEDIUM", "")

	// Active in A, referencing a Task that lives in B — B's Task is invisible here,
	// so the attach is refused.
	resp := gqlExec(t, attachToTaskMutation, map[string]any{
		"taskID": taskB.ID, "fileName": "leak.pdf", "contentType": "application/pdf",
		"sizeBytes": 1, "storageURL": "s3://bucket/leak.pdf",
	}, workspaceHeaders(token, wsA))
	if len(resp.Errors) == 0 {
		t.Error("attached a file to a task in another workspace; cross-tenant polymorphic binding not guarded")
	}

	// Nothing was persisted against B's task either.
	content := readTaskContent(t, token, wsB, taskB.ID)
	if len(content.Attachments) != 0 {
		t.Errorf("taskB has %d attachments after a rejected cross-workspace attach, want 0", len(content.Attachments))
	}
}

// TestRawContentCreateMutationsAreNotExposed locks in the security fix: the
// generated raw create/upsert mutations for comments, attachments, and
// time_entries are masked out of the GraphQL API (sqlgen.yml api.operations),
// because each lets a client set a server-owned provenance field (authorID /
// uploadedBy / userID) — and createAttachment additionally forges the
// FK-less polymorphic binding. The only write path is the blessed mutations. If
// a mask is ever removed, this test fails.
func TestRawContentCreateMutationsAreNotExposed(t *testing.T) {
	token, _ := register(t, "raw-content@example.com", "raw-content-p01", "Raw Content")
	wsID := bootstrapWorkspace(t, token, "RawContent", "raw-content-ws")
	h := workspaceHeaders(token, wsID)

	raw := map[string]string{
		"createComment":    `mutation { createComment(input: {taskID: "00000000-0000-0000-0000-000000000001", authorID: "00000000-0000-0000-0000-000000000002", body: "x", createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z"}) { id } }`,
		"upsertComment":    `mutation { upsertComment(input: {taskID: "00000000-0000-0000-0000-000000000001", authorID: "00000000-0000-0000-0000-000000000002", body: "x", createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z"}) { id } }`,
		"createAttachment": `mutation { createAttachment(input: {entityType: TASK, entityID: "00000000-0000-0000-0000-000000000001", uploadedBy: "00000000-0000-0000-0000-000000000002", fileName: "x", contentType: "x", sizeBytes: 1, storageURL: "x", createdAt: "2026-01-01T00:00:00Z"}) { id } }`,
		"upsertAttachment": `mutation { upsertAttachment(input: {entityType: TASK, entityID: "00000000-0000-0000-0000-000000000001", uploadedBy: "00000000-0000-0000-0000-000000000002", fileName: "x", contentType: "x", sizeBytes: 1, storageURL: "x", createdAt: "2026-01-01T00:00:00Z"}) { id } }`,
		"createTimeEntry":  `mutation { createTimeEntry(input: {taskID: "00000000-0000-0000-0000-000000000001", userID: "00000000-0000-0000-0000-000000000002", hours: "1.00", spentOn: "2026-01-01T00:00:00Z", createdAt: "2026-01-01T00:00:00Z"}) { id } }`,
		"upsertTimeEntry":  `mutation { upsertTimeEntry(input: {taskID: "00000000-0000-0000-0000-000000000001", userID: "00000000-0000-0000-0000-000000000002", hours: "1.00", spentOn: "2026-01-01T00:00:00Z", createdAt: "2026-01-01T00:00:00Z"}) { id } }`,
	}
	for name, q := range raw {
		if !rawMutationRejected(t, q, h) {
			t.Errorf("raw %s was accepted; the unguarded content create mutation must not be exposed", name)
		}
	}
}

// --- read-back helpers -----------------------------------------------------

type taskContent struct {
	Comments []struct {
		ID          string `json:"id"`
		Body        string `json:"body"`
		AuthorID    string `json:"authorID"`
		Attachments []struct {
			ID         string `json:"id"`
			FileName   string `json:"fileName"`
			EntityType string `json:"entityType"`
		} `json:"attachments"`
	} `json:"comments"`
	Attachments []struct {
		ID         string `json:"id"`
		FileName   string `json:"fileName"`
		EntityType string `json:"entityType"`
		UploadedBy string `json:"uploadedBy"`
	} `json:"attachments"`
	TimeEntries []struct {
		ID       string `json:"id"`
		Hours    string `json:"hours"`
		Billable bool   `json:"billable"`
		Notes    string `json:"notes"`
		UserID   string `json:"userID"`
	} `json:"timeEntries"`
}

// readTaskContent fetches a Task's comments, attachments, and time entries in one
// query through the generated relationship fields.
func readTaskContent(t *testing.T, token, wsID, taskID string) taskContent {
	t.Helper()
	var out struct {
		Task taskContent `json:"task"`
	}
	gqlExecData(t, taskContentQuery, map[string]any{"id": taskID}, workspaceHeaders(token, wsID), &out)
	return out.Task
}
