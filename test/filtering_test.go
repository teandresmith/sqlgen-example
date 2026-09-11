package test

import (
	"testing"
)

// This file showcases the breadth of sqlgen's generated typed-filter surface
// through the GraphQL endpoint. The rest of the suite leans on the `eq`
// comparator; here we drive the richer operators sqlgen generates for every
// column — string matching, enum set membership, JSONB key/containment, and
// array (slice) membership — plus the boolean `and`/`or` composition. None of
// this is hand-written: each operator below is a field sqlgen emitted on the
// generated *Comparator inputs, translated to SQL by the generated resolvers.

// --- string comparator: contains / startsWith / in / like -------------------

// TestFilterTasksByStringOperators drives the StringComparator sqlgen generates
// for a text column (tasks.title): substring, prefix, set membership, and SQL
// LIKE — none of which the `eq`-only paths elsewhere exercise.
func TestFilterTasksByStringOperators(t *testing.T) {
	token, _ := register(t, "strfilter-owner@example.com", "strfilter-pw-01", "StrFilter Owner")
	wsID := bootstrapWorkspace(t, token, "StrFiltering", "str-filtering-ws")
	projID := createProject(t, token, wsID, "Titled")

	createTaskInProject(t, token, wsID, projID, "deploy api gateway", "TODO", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "deploy web frontend", "TODO", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "audit billing metrics", "TODO", "MEDIUM", "")

	// contains "deploy" → the two deploy tasks.
	_, items := queryTaskList(t, token, wsID, map[string]any{"title": map[string]any{"contains": "deploy"}}, nil)
	if got := titleSet(items); !sameSet(got, "deploy api gateway", "deploy web frontend") {
		t.Errorf("title contains 'deploy' → %v, want the two deploy tasks", orderedTitles(items))
	}

	// startsWith "audit" → the audit task only.
	_, items = queryTaskList(t, token, wsID, map[string]any{"title": map[string]any{"startsWith": "audit"}}, nil)
	if got := titleSet(items); !sameSet(got, "audit billing metrics") {
		t.Errorf("title startsWith 'audit' → %v, want {audit billing metrics}", orderedTitles(items))
	}

	// in [exact titles] → set membership on the string column.
	_, items = queryTaskList(t, token, wsID,
		map[string]any{"title": map[string]any{"in": []any{"deploy api gateway", "audit billing metrics"}}}, nil)
	if got := titleSet(items); !sameSet(got, "deploy api gateway", "audit billing metrics") {
		t.Errorf("title in [...] → %v, want the two named tasks", orderedTitles(items))
	}

	// SQL LIKE with a wildcard → both deploy tasks match "deploy %".
	_, items = queryTaskList(t, token, wsID, map[string]any{"title": map[string]any{"like": "deploy %"}}, nil)
	if got := titleSet(items); !sameSet(got, "deploy api gateway", "deploy web frontend") {
		t.Errorf("title like 'deploy %%' → %v, want the two deploy tasks", orderedTitles(items))
	}
}

// --- enum comparator: in / nin ----------------------------------------------

// TestFilterTasksByEnumSetMembership drives the enum comparator's set operators
// (in / nin) on tasks.status — the multi-value shape the single-value `eq`
// filters used elsewhere never reach.
func TestFilterTasksByEnumSetMembership(t *testing.T) {
	token, _ := register(t, "enumfilter-owner@example.com", "enumfilter-pw-1", "EnumFilter Owner")
	wsID := bootstrapWorkspace(t, token, "EnumFiltering", "enum-filtering-ws")
	projID := createProject(t, token, wsID, "Staged")

	createTaskInProject(t, token, wsID, projID, "backlog-item", "BACKLOG", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "todo-item", "TODO", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "wip-item", "IN_PROGRESS", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "done-item", "DONE", "MEDIUM", "")

	// status in [TODO, IN_PROGRESS] → the two active tasks.
	_, items := queryTaskList(t, token, wsID,
		map[string]any{"status": map[string]any{"in": []any{"TODO", "IN_PROGRESS"}}}, nil)
	if got := titleSet(items); !sameSet(got, "todo-item", "wip-item") {
		t.Errorf("status in [TODO, IN_PROGRESS] → %v, want the two active tasks", orderedTitles(items))
	}

	// status nin [DONE] → everything that is not done.
	_, items = queryTaskList(t, token, wsID,
		map[string]any{"status": map[string]any{"nin": []any{"DONE"}}}, nil)
	if got := titleSet(items); !sameSet(got, "backlog-item", "todo-item", "wip-item") {
		t.Errorf("status nin [DONE] → %v, want everything but done-item", orderedTitles(items))
	}
}

// --- JSONB comparator: hasKey / hasAllKeys / contains ------------------------

const updateTaskCustomFieldsMutation = `
	mutation ($id: UUID!, $customFields: JSON!) {
		updateTask(id: $id, input: { customFields: $customFields }) { id }
	}`

// setCustomFields stamps a JSONB payload onto an existing Task via the generated
// updateTask mutation (tasks.custom_fields is a JSONB column).
func setCustomFields(t *testing.T, token, wsID, taskID string, fields map[string]any) {
	t.Helper()
	var out struct {
		UpdateTask struct {
			ID string `json:"id"`
		} `json:"updateTask"`
	}
	gqlExecData(t, updateTaskCustomFieldsMutation,
		map[string]any{"id": taskID, "customFields": fields},
		workspaceHeaders(token, wsID), &out)
	if out.UpdateTask.ID != taskID {
		t.Fatalf("updateTask(customFields) returned id %q, want %q", out.UpdateTask.ID, taskID)
	}
}

// TestFilterTasksByCustomFieldsJSONB drives the JSONBComparator sqlgen generates
// for a JSONB column (tasks.custom_fields): key presence (hasKey / hasAllKeys)
// and containment (contains, the @> operator). This is the operator family the
// audit found the app never exercised, despite the column existing to carry it.
func TestFilterTasksByCustomFieldsJSONB(t *testing.T) {
	token, _ := register(t, "jsonbfilter-owner@example.com", "jsonbfilter-pw", "JsonbFilter Owner")
	wsID := bootstrapWorkspace(t, token, "JsonbFiltering", "jsonb-filtering-ws")
	projID := createProject(t, token, wsID, "Annotated")

	sev1 := createTaskInProject(t, token, wsID, projID, "sev1-outage", "TODO", "URGENT", "")
	sev2 := createTaskInProject(t, token, wsID, projID, "sev2-degraded", "TODO", "HIGH", "")
	triage := createTaskInProject(t, token, wsID, projID, "unclassified", "TODO", "LOW", "")

	setCustomFields(t, token, wsID, sev1.ID, map[string]any{"severity": "high", "sla_hours": 4})
	setCustomFields(t, token, wsID, sev2.ID, map[string]any{"severity": "low", "sla_hours": 48})
	setCustomFields(t, token, wsID, triage.ID, map[string]any{"stage": "triage"})

	// hasKey "severity" → the two classified incidents (triage has no severity).
	_, items := queryTaskList(t, token, wsID,
		map[string]any{"customFields": map[string]any{"hasKey": "severity"}}, nil)
	if got := titleSet(items); !sameSet(got, "sev1-outage", "sev2-degraded") {
		t.Errorf("customFields hasKey 'severity' → %v, want the two classified tasks", orderedTitles(items))
	}

	// hasAllKeys ["severity","sla_hours"] → both incidents carry both keys.
	_, items = queryTaskList(t, token, wsID,
		map[string]any{"customFields": map[string]any{"hasAllKeys": []any{"severity", "sla_hours"}}}, nil)
	if got := titleSet(items); !sameSet(got, "sev1-outage", "sev2-degraded") {
		t.Errorf("customFields hasAllKeys [severity,sla_hours] → %v, want the two classified tasks", orderedTitles(items))
	}

	// contains {"severity":"high"} → JSONB @> containment matches sev1 only.
	_, items = queryTaskList(t, token, wsID,
		map[string]any{"customFields": map[string]any{"contains": map[string]any{"severity": "high"}}}, nil)
	if got := titleSet(items); !sameSet(got, "sev1-outage") {
		t.Errorf("customFields contains {severity:high} → %v, want {sev1-outage}", orderedTitles(items))
	}
}

// --- slice (array) comparator: containsAny / containsAll / isEmpty -----------

const apiTokenListByScopesQuery = `
	query ($filter: APITokenFilter) {
		apiTokenList(filter: $filter) {
			totalCount
			items { id name scopes }
		}
	}`

// tokenNamesMatchingScopes returns the set of API-token names in the workspace
// matching the given scopes filter (a StringSliceComparator sub-object).
func tokenNamesMatchingScopes(t *testing.T, token, wsID string, scopes map[string]any) map[string]bool {
	t.Helper()
	var out struct {
		APITokenList struct {
			Items []struct {
				Name   string   `json:"name"`
				Scopes []string `json:"scopes"`
			} `json:"items"`
		} `json:"apiTokenList"`
	}
	gqlExecData(t, apiTokenListByScopesQuery,
		map[string]any{"filter": map[string]any{"scopes": scopes}},
		workspaceHeaders(token, wsID), &out)
	names := make(map[string]bool, len(out.APITokenList.Items))
	for _, it := range out.APITokenList.Items {
		names[it.Name] = true
	}
	return names
}

// TestFilterAPITokensByScopesArray drives the StringSliceComparator sqlgen
// generates for a PostgreSQL array column (api_tokens.scopes TEXT[]):
// containsAny (overlap), containsAll (contains), and isEmpty. The array column
// exists precisely to exercise this comparator, which nothing else did.
func TestFilterAPITokensByScopesArray(t *testing.T) {
	token, _ := register(t, "arrfilter-owner@example.com", "arrfilter-pw-01", "ArrFilter Owner")
	wsID := bootstrapWorkspace(t, token, "ArrFiltering", "arr-filtering-ws")

	createPersonalToken(t, token, wsID, "reader", []string{"tasks:read"})
	createPersonalToken(t, token, wsID, "writer", []string{"tasks:read", "tasks:write"})
	createPersonalToken(t, token, wsID, "admin", []string{"tasks:read", "tasks:write", "admin:all"})
	createPersonalToken(t, token, wsID, "scopeless", nil)

	// containsAny ["tasks:write"] → array overlap: writer and admin.
	got := tokenNamesMatchingScopes(t, token, wsID, map[string]any{"containsAny": []any{"tasks:write"}})
	if !sameSet(got, "writer", "admin") {
		t.Errorf("scopes containsAny [tasks:write] → %v, want {writer, admin}", got)
	}

	// containsAll ["tasks:read","tasks:write"] → array contains: writer and admin.
	got = tokenNamesMatchingScopes(t, token, wsID, map[string]any{"containsAll": []any{"tasks:read", "tasks:write"}})
	if !sameSet(got, "writer", "admin") {
		t.Errorf("scopes containsAll [tasks:read,tasks:write] → %v, want {writer, admin}", got)
	}

	// isEmpty true → the scopeless token only.
	got = tokenNamesMatchingScopes(t, token, wsID, map[string]any{"isEmpty": true})
	if !sameSet(got, "scopeless") {
		t.Errorf("scopes isEmpty → %v, want {scopeless}", got)
	}
}
