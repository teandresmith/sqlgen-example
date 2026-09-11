package test

import (
	"fmt"
	"strings"
	"testing"

	"uuid"
)

// This file proves issue 14 (error mapping & query limits) through the one seam
// the harness exercises — the GraphQL HTTP endpoint under an Active Workspace.
//
// Two guarantees are asserted here as a client observes them:
//
//  1. Predictable, typed errors. A not-found entity, a unique/foreign-key/check
//     constraint violation each come back as a GraphQL error carrying a stable
//     `extensions.code`, so a client can branch on the code rather than parse a
//     message. The mapping is the generated `mapErrorToGQL` for generated
//     resolvers and the server-boundary error presenter for the hand-written
//     blessed ones (inviteToWorkspace here); both agree on the codes.
//  2. Query safety. Over-deep and over-complex queries are rejected before any
//     resolver runs, with the depth/complexity code set.

// errorCode returns the extensions.code of a GraphQL error, or "" if absent.
func errorCode(e gqlError) string {
	if e.Extensions == nil {
		return ""
	}
	code, _ := e.Extensions["code"].(string)
	return code
}

// assertSingleErrorCode asserts the response carries exactly one GraphQL error
// with the wanted extensions.code, and returns it for any further inspection.
func assertSingleErrorCode(t *testing.T, resp gqlResponse, wantCode string) gqlError {
	t.Helper()
	if len(resp.Errors) != 1 {
		t.Fatalf("want exactly one error with code %q, got %d: %+v", wantCode, len(resp.Errors), resp.Errors)
	}
	if got := errorCode(resp.Errors[0]); got != wantCode {
		t.Fatalf("error code = %q, want %q (message: %q)", got, wantCode, resp.Errors[0].Message)
	}
	return resp.Errors[0]
}

// TestNotFoundEntityReturnsTypedError proves a mutation targeting a non-existent
// entity returns a clear, typed NOT_FOUND error rather than a silent success or
// a leaked internal string (AC 1 / PRD story 48). updateTask on a random id runs
// tenant-scoped, matches no row, and surfaces the generated NOT_FOUND mapping.
func TestNotFoundEntityReturnsTypedError(t *testing.T) {
	token, _ := register(t, "notfound@example.com", "not-found-pw-01", "Notfound")
	wsID := bootstrapWorkspace(t, token, "Notfound WS", "notfound-ws")

	const updateTaskMutation = `
		mutation ($id: UUID!, $input: UpdateTaskInput!) {
			updateTask(id: $id, input: $input) { id title }
		}`
	resp := gqlExec(t, updateTaskMutation, map[string]any{
		"id":    uuid.New().String(), // never inserted → not found in this tenant
		"input": map[string]any{"title": "does not matter"},
	}, workspaceHeaders(token, wsID))

	assertSingleErrorCode(t, resp, "NOT_FOUND")
}

// TestUniqueConstraintViolationReturnsTypedError proves a duplicate Label name
// (the schema's UNIQUE (workspace_id, name)) surfaces as a typed CONFLICT via the
// generated mapErrorToGQL (AC 2 / PRD story 49). The first create succeeds; the
// second, same-named create in the same Workspace is rejected.
func TestUniqueConstraintViolationReturnsTypedError(t *testing.T) {
	token, _ := register(t, "unique@example.com", "unique-pw-0001", "Unique")
	wsID := bootstrapWorkspace(t, token, "Unique WS", "unique-ws")

	createLabel(t, token, wsID, "backend", nil) // first one lands

	resp := gqlExec(t, createLabelMutation, map[string]any{
		"input": map[string]any{
			"workspaceID": wsID,
			"name":        "backend", // collides with the first
			"color":       "#3366FF",
			"createdAt":   "2026-07-14T00:00:00Z",
		},
	}, workspaceHeaders(token, wsID))

	assertSingleErrorCode(t, resp, "CONFLICT")
}

// TestForeignKeyViolationReturnsTypedError proves a dangling reference — a Task
// created against a projectID that does not exist — surfaces as a typed
// BAD_REFERENCE (AC 3). It drives the low-level generated createTask so the
// foreign-key check fires on the write (the blessed createTaskInProject would be
// equivalent, but createTask keeps the failure to a single reference).
func TestForeignKeyViolationReturnsTypedError(t *testing.T) {
	token, user := register(t, "fk@example.com", "fk-pw-000001", "FK")
	wsID := bootstrapWorkspace(t, token, "FK WS", "fk-ws")

	const createTaskMutation = `
		mutation ($input: CreateTaskInput!) {
			createTask(input: $input) { id title }
		}`
	resp := gqlExec(t, createTaskMutation, map[string]any{
		"input": map[string]any{
			"workspaceID":  wsID,
			"projectID":    uuid.New().String(), // no such Project → FK violation
			"reporterID":   user.ID,             // a valid User, so only project_id dangles
			"title":        "orphan task",
			"status":       "TODO",
			"priority":     "MEDIUM",
			"customFields": map[string]any{},
			"createdAt":    "2026-07-14T00:00:00Z",
			"updatedAt":    "2026-07-14T00:00:00Z",
		},
	}, workspaceHeaders(token, wsID))

	assertSingleErrorCode(t, resp, "BAD_REFERENCE")
}

// TestCheckConstraintViolationReturnsTypedError proves a check-constraint
// violation surfaces as a typed INVALID_INPUT (AC 3). The reachable check is the
// `email` DOMAIN on invitations.email; inviting a syntactically invalid address
// trips it on the write. inviteToWorkspace is a hand-written blessed resolver
// that wraps the runtime error with context, so this exercises the server
// boundary error presenter (not the generated per-resolver mapping) — the two
// agree on the code.
func TestCheckConstraintViolationReturnsTypedError(t *testing.T) {
	token, _ := register(t, "check@example.com", "check-pw-0001", "Check")
	wsID := bootstrapWorkspace(t, token, "Check WS", "check-ws")

	resp := gqlExec(t, inviteMutation, map[string]any{
		"input": map[string]any{
			"email": "not-a-valid-email", // violates the email DOMAIN check
			"role":  "MEMBER",
		},
	}, workspaceHeaders(token, wsID))

	assertSingleErrorCode(t, resp, "INVALID_INPUT")
}

// TestQueryDepthLimitRejectsDeepQuery proves an over-deep query is rejected
// before execution (AC 4 / PRD story 51), while a query within the limit is
// accepted. Depth is grown through the self-referential Task.subtasks edge, so
// the query stays schema-valid at any nesting.
func TestQueryDepthLimitRejectsDeepQuery(t *testing.T) {
	token, _ := register(t, "depth@example.com", "depth-pw-0001", "Depth")
	wsID := bootstrapWorkspace(t, token, "Depth WS", "depth-ws")
	headers := workspaceHeaders(token, wsID)

	// A shallow query (task → subtasks → id, depth 3) is well under the limit and
	// runs normally — the random id simply resolves to null, with no error.
	resp := gqlExec(t, nestedSubtaskQuery(2), map[string]any{"id": uuid.New().String()}, headers)
	if len(resp.Errors) != 0 {
		t.Fatalf("shallow query rejected unexpectedly: %+v", resp.Errors)
	}

	// Nesting past the ceiling is refused with the depth code, and no resolver
	// runs (data is absent, not merely null).
	resp = gqlExec(t, nestedSubtaskQuery(15), map[string]any{"id": uuid.New().String()}, headers)
	assertSingleErrorCode(t, resp, "DEPTH_LIMIT_EXCEEDED")
}

// TestQueryComplexityLimitRejectsComplexQuery proves an over-complex query is
// rejected before execution (AC 5 / PRD story 51). The query is broad, not deep
// — many aliased task reads — so it isolates the complexity ceiling from the
// depth one.
func TestQueryComplexityLimitRejectsComplexQuery(t *testing.T) {
	token, _ := register(t, "complexity@example.com", "complexity-pw-01", "Complexity")
	wsID := bootstrapWorkspace(t, token, "Complexity WS", "complexity-ws")
	headers := workspaceHeaders(token, wsID)

	// 150 aliased `task { id }` selections ≈ 300 complexity units, comfortably
	// past the 200 ceiling. Each is depth 2, so depth never trips first.
	resp := gqlExec(t, wideTaskQuery(150), map[string]any{"id": uuid.New().String()}, headers)
	assertSingleErrorCode(t, resp, "COMPLEXITY_LIMIT_EXCEEDED")
}

// nestedSubtaskQuery builds a query that reads a Task and follows the subtasks
// edge `levels` times before selecting a leaf id. The resulting selection-set
// depth is levels+2 (the task field plus the trailing id).
func nestedSubtaskQuery(levels int) string {
	inner := "id"
	for i := 0; i < levels; i++ {
		inner = "subtasks { " + inner + " }"
	}
	return fmt.Sprintf("query ($id: UUID!) { task(id: $id) { %s } }", inner)
}

// wideTaskQuery builds a shallow but broad query with n aliased `task { id }`
// selections, used to exceed the complexity ceiling without adding depth.
func wideTaskQuery(n int) string {
	var b strings.Builder
	b.WriteString("query ($id: UUID!) {\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  a%d: task(id: $id) { id }\n", i)
	}
	b.WriteString("}")
	return b.String()
}
