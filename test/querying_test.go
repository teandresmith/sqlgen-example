package test

import (
	"testing"
)

// This file proves issue 11 (querying, filtering, pagination & Project Stats)
// through the one seam the harness exercises — the GraphQL HTTP endpoint under an
// Active Workspace. The read surface is generated; these tests verify it composes
// correctly under tenant scoping: get-by-id, enum + relationship filters combined
// with and/or, sort, cursor and offset pagination, one-query relationship loading,
// and the Project Stats view.

// --- queries ----------------------------------------------------------------

const taskByIDQuery = `query ($id: UUID!) { task(id: $id) { id title } }`

const projectByIDQuery = `query ($id: UUID!) { project(id: $id) { id name } }`

const taskListQuery = `
	query ($filter: TaskFilter, $sort: [TaskSort!]) {
		taskList(filter: $filter, sort: $sort) {
			totalCount
			items { id title }
		}
	}`

const taskConnQuery = `
	query ($first: Int, $after: String) {
		tasks(first: $first, after: $after) {
			totalCount
			edges { node { id } cursor }
			pageInfo { hasNextPage endCursor }
		}
	}`

const taskOffsetQuery = `
	query ($limit: Int, $offset: Int) {
		taskList(limit: $limit, offset: $offset) {
			totalCount offset limit hasMore
			items { id }
		}
	}`

const taskLoadRelationsQuery = `
	query ($id: UUID!) {
		task(id: $id) {
			id
			comments { id }
			assignees { id }
			labels { id name }
			subtasks { id title }
		}
	}`

const projectStatQuery = `
	query ($id: UUID!) {
		projectStat(projectID: $id) { projectID totalTasks doneTasks openTasks }
	}`

// --- helpers ----------------------------------------------------------------

type taskLite struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// queryTaskList drives taskList with an optional filter and sort and returns the
// server-reported totalCount plus the items. filter/sort are passed as-is (maps /
// slices shaped like the GraphQL inputs); pass nil to omit.
func queryTaskList(t *testing.T, token, wsID string, filter, sort any) (int, []taskLite) {
	t.Helper()
	vars := map[string]any{}
	if filter != nil {
		vars["filter"] = filter
	}
	if sort != nil {
		vars["sort"] = sort
	}
	var out struct {
		TaskList struct {
			TotalCount int        `json:"totalCount"`
			Items      []taskLite `json:"items"`
		} `json:"taskList"`
	}
	gqlExecData(t, taskListQuery, vars, workspaceHeaders(token, wsID), &out)
	return out.TaskList.TotalCount, out.TaskList.Items
}

// titleSet collapses items into a set of titles for order-independent assertions.
func titleSet(items []taskLite) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it.Title] = true
	}
	return s
}

// orderedTitles preserves item order for sort assertions.
func orderedTitles(items []taskLite) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func sameSet(got map[string]bool, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for _, w := range want {
		if !got[w] {
			return false
		}
	}
	return true
}

// --- AC 1: fetch a single entity by id --------------------------------------

// TestFetchSingleEntityByID proves a member fetches a single entity by id (story
// 35): a Task and a Project resolve by id in the Active Workspace, and a Task in
// another Workspace is invisible (tenant-scoped get returns null).
func TestFetchSingleEntityByID(t *testing.T) {
	token, _ := register(t, "getbyid-owner@example.com", "getbyid-pw-0001", "GetById Owner")
	wsID := bootstrapWorkspace(t, token, "GetById", "getbyid-ws")
	projID := createProject(t, token, wsID, "Findable")
	task := createTaskInProject(t, token, wsID, projID, "Find me", "TODO", "MEDIUM", "")

	var taskOut struct {
		Task *taskLite `json:"task"`
	}
	gqlExecData(t, taskByIDQuery, map[string]any{"id": task.ID}, workspaceHeaders(token, wsID), &taskOut)
	if taskOut.Task == nil || taskOut.Task.ID != task.ID || taskOut.Task.Title != "Find me" {
		t.Fatalf("task(id) = %+v, want the created task %s", taskOut.Task, task.ID)
	}

	var projOut struct {
		Project *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}
	gqlExecData(t, projectByIDQuery, map[string]any{"id": projID}, workspaceHeaders(token, wsID), &projOut)
	if projOut.Project == nil || projOut.Project.ID != projID {
		t.Fatalf("project(id) = %+v, want %s", projOut.Project, projID)
	}

	// A second Workspace cannot see the first's Task by id (tenant-scoped get).
	otherToken, _ := register(t, "getbyid-other@example.com", "getbyid-pw-0002", "GetById Other")
	otherWS := bootstrapWorkspace(t, otherToken, "GetByIdOther", "getbyid-other-ws")
	gqlExecData(t, taskByIDQuery, map[string]any{"id": task.ID}, workspaceHeaders(otherToken, otherWS), &taskOut)
	if taskOut.Task != nil {
		t.Errorf("task(id) from another workspace = %+v, want null (tenant-scoped)", taskOut.Task)
	}
}

// --- AC 2 (columns) + AC 3: enum-column filters combined with and/or ---------

// TestFilterTasksByColumnsAndCompose proves filtering Tasks by status and
// priority (enum columns, story 36) and combining conditions with and/or (story
// 37). This is the path that was broken before the generator wired enum
// comparators into the GraphQL filter translation.
func TestFilterTasksByColumnsAndCompose(t *testing.T) {
	token, _ := register(t, "filter-owner@example.com", "filter-pw-00001", "Filter Owner")
	wsID := bootstrapWorkspace(t, token, "Filtering", "filtering-ws")
	projID := createProject(t, token, wsID, "Filterable")

	createTaskInProject(t, token, wsID, projID, "alpha", "DONE", "HIGH", "")
	createTaskInProject(t, token, wsID, projID, "bravo", "TODO", "HIGH", "")
	createTaskInProject(t, token, wsID, projID, "charlie", "DONE", "LOW", "")

	// status = DONE → alpha, charlie
	_, items := queryTaskList(t, token, wsID, map[string]any{"status": map[string]any{"eq": "DONE"}}, nil)
	if got := titleSet(items); !sameSet(got, "alpha", "charlie") {
		t.Errorf("filter status=DONE → %v, want {alpha, charlie}", orderedTitles(items))
	}

	// priority = HIGH → alpha, bravo
	_, items = queryTaskList(t, token, wsID, map[string]any{"priority": map[string]any{"eq": "HIGH"}}, nil)
	if got := titleSet(items); !sameSet(got, "alpha", "bravo") {
		t.Errorf("filter priority=HIGH → %v, want {alpha, bravo}", orderedTitles(items))
	}

	// status=DONE AND priority=HIGH → alpha only
	_, items = queryTaskList(t, token, wsID, map[string]any{"and": []any{
		map[string]any{"status": map[string]any{"eq": "DONE"}},
		map[string]any{"priority": map[string]any{"eq": "HIGH"}},
	}}, nil)
	if got := titleSet(items); !sameSet(got, "alpha") {
		t.Errorf("filter DONE AND HIGH → %v, want {alpha}", orderedTitles(items))
	}
}

// TestFilterTasksComposedWithOr proves the `or` half of story 37 — combining
// conditions with OR, including that each `or` member AND-s its own fields while
// the members OR together, and that `or` nests inside `and`.
func TestFilterTasksComposedWithOr(t *testing.T) {
	token, _ := register(t, "or-owner@example.com", "or-pw-00000001", "Or Owner")
	wsID := bootstrapWorkspace(t, token, "OrFiltering", "or-filtering-ws")
	projID := createProject(t, token, wsID, "OrFilterable")
	createTaskInProject(t, token, wsID, projID, "alpha", "DONE", "HIGH", "")
	createTaskInProject(t, token, wsID, projID, "bravo", "TODO", "HIGH", "")
	createTaskInProject(t, token, wsID, projID, "charlie", "DONE", "LOW", "")

	// status=TODO OR priority=LOW → bravo (TODO), charlie (LOW)
	_, items := queryTaskList(t, token, wsID, map[string]any{"or": []any{
		map[string]any{"status": map[string]any{"eq": "TODO"}},
		map[string]any{"priority": map[string]any{"eq": "LOW"}},
	}}, nil)
	if got := titleSet(items); !sameSet(got, "bravo", "charlie") {
		t.Errorf("filter TODO OR LOW → %v, want {bravo, charlie}", orderedTitles(items))
	}

	// A single `or` member AND-s its own fields: (status=DONE AND priority=HIGH)
	// → alpha only. Guards the second facet of the old bug (member fields must not
	// be OR-ed with each other).
	_, items = queryTaskList(t, token, wsID, map[string]any{"or": []any{
		map[string]any{"status": map[string]any{"eq": "DONE"}, "priority": map[string]any{"eq": "HIGH"}},
	}}, nil)
	if got := titleSet(items); !sameSet(got, "alpha") {
		t.Errorf("filter or:[(DONE AND HIGH)] → %v, want {alpha}", orderedTitles(items))
	}

	// `or` nested inside `and`: priority=HIGH AND (status=TODO OR status=DONE)
	// → alpha (DONE,HIGH), bravo (TODO,HIGH); excludes charlie (LOW).
	_, items = queryTaskList(t, token, wsID, map[string]any{"and": []any{
		map[string]any{"priority": map[string]any{"eq": "HIGH"}},
		map[string]any{"or": []any{
			map[string]any{"status": map[string]any{"eq": "TODO"}},
			map[string]any{"status": map[string]any{"eq": "DONE"}},
		}},
	}}, nil)
	if got := titleSet(items); !sameSet(got, "alpha", "bravo") {
		t.Errorf("filter HIGH AND (TODO OR DONE) → %v, want {alpha, bravo}", orderedTitles(items))
	}
}

// --- AC 2 (relationships): filter by assignee and label ---------------------

// TestFilterTasksByAssigneeAndLabel proves filtering Tasks by assignee and label
// (story 36) via the relationship filter members on TaskFilter, and that a
// relationship filter composes with a column filter in a single query.
func TestFilterTasksByAssigneeAndLabel(t *testing.T) {
	ownerToken, _ := register(t, "relfilter-owner@example.com", "relfilter-pw-01", "RelFilter Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "RelFiltering", "rel-filtering-ws")
	projID := createProject(t, ownerToken, wsID, "Related")

	assigned := createTaskInProject(t, ownerToken, wsID, projID, "assigned", "TODO", "HIGH", "")
	labeled := createTaskInProject(t, ownerToken, wsID, projID, "labeled", "TODO", "LOW", "")

	// A member is assigned to one task.
	memberToken, member := register(t, "relfilter-member@example.com", "relfilter-pw-02", "RelFilter Member")
	joinWorkspace(t, ownerToken, wsID, memberToken, member.Email, "MEMBER")
	var assignOut struct {
		AssignUserToTask struct {
			TaskID string `json:"taskID"`
		} `json:"assignUserToTask"`
	}
	gqlExecData(t, assignUserToTaskMutation,
		map[string]any{"taskID": assigned.ID, "userID": member.ID},
		workspaceHeaders(ownerToken, wsID), &assignOut)

	// A label is applied to the other task.
	labelID := createLabel(t, ownerToken, wsID, "urgent", nil)
	gqlExecData(t, applyLabelToTaskMutation,
		map[string]any{"taskID": labeled.ID, "labelID": labelID},
		workspaceHeaders(ownerToken, wsID), &labeledTask{})

	// Filter by assignee (by the member's email) → only the assigned task.
	_, items := queryTaskList(t, ownerToken, wsID,
		map[string]any{"assignees": map[string]any{"email": map[string]any{"eq": member.Email}}}, nil)
	if got := titleSet(items); !sameSet(got, "assigned") {
		t.Errorf("filter assignees.email=%s → %v, want {assigned}", member.Email, orderedTitles(items))
	}

	// Filter by label name → only the labeled task.
	_, items = queryTaskList(t, ownerToken, wsID,
		map[string]any{"labels": map[string]any{"name": map[string]any{"eq": "urgent"}}}, nil)
	if got := titleSet(items); !sameSet(got, "labeled") {
		t.Errorf("filter labels.name=urgent → %v, want {labeled}", orderedTitles(items))
	}

	// Compose a relationship filter with a column filter: label urgent AND
	// priority LOW → the labeled task (which is LOW); excludes the assigned (HIGH).
	_, items = queryTaskList(t, ownerToken, wsID, map[string]any{"and": []any{
		map[string]any{"labels": map[string]any{"name": map[string]any{"eq": "urgent"}}},
		map[string]any{"priority": map[string]any{"eq": "LOW"}},
	}}, nil)
	if got := titleSet(items); !sameSet(got, "labeled") {
		t.Errorf("filter (label urgent AND priority LOW) → %v, want {labeled}", orderedTitles(items))
	}
}

// --- AC 4: sort -------------------------------------------------------------

// TestSortTaskList proves Task lists can be sorted (story 38): ascending and
// descending by title return the tasks in the requested order.
func TestSortTaskList(t *testing.T) {
	token, _ := register(t, "sort-owner@example.com", "sort-pw-000001", "Sort Owner")
	wsID := bootstrapWorkspace(t, token, "Sorting", "sorting-ws")
	projID := createProject(t, token, wsID, "Sortable")
	createTaskInProject(t, token, wsID, projID, "banana", "TODO", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "apple", "TODO", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "cherry", "TODO", "MEDIUM", "")

	_, items := queryTaskList(t, token, wsID, nil, []any{map[string]any{"field": "TITLE", "direction": "ASC"}})
	if got := orderedTitles(items); !equalSlice(got, []string{"apple", "banana", "cherry"}) {
		t.Errorf("sort TITLE ASC → %v, want [apple banana cherry]", got)
	}

	_, items = queryTaskList(t, token, wsID, nil, []any{map[string]any{"field": "TITLE", "direction": "DESC"}})
	if got := orderedTitles(items); !equalSlice(got, []string{"cherry", "banana", "apple"}) {
		t.Errorf("sort TITLE DESC → %v, want [cherry banana apple]", got)
	}
}

// --- AC 5: cursor pagination ------------------------------------------------

// TestCursorPaginateTasksStably proves cursor-paginated Task connections page
// stably (story 39): paging through in fixed-size windows visits every Task
// exactly once, hasNextPage flips false on the last page, and totalCount is
// reported.
func TestCursorPaginateTasksStably(t *testing.T) {
	token, _ := register(t, "cursor-owner@example.com", "cursor-pw-00001", "Cursor Owner")
	wsID := bootstrapWorkspace(t, token, "Cursoring", "cursoring-ws")
	projID := createProject(t, token, wsID, "Paged")
	const n = 5
	for range n {
		createTaskInProject(t, token, wsID, projID, "task", "TODO", "MEDIUM", "")
	}

	seen := map[string]bool{}
	var after *string
	pages := 0
	for {
		vars := map[string]any{"first": 2}
		if after != nil {
			vars["after"] = *after
		}
		var out struct {
			Tasks struct {
				TotalCount int `json:"totalCount"`
				Edges      []struct {
					Node struct {
						ID string `json:"id"`
					} `json:"node"`
					Cursor string `json:"cursor"`
				} `json:"edges"`
				PageInfo struct {
					HasNextPage bool    `json:"hasNextPage"`
					EndCursor   *string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"tasks"`
		}
		gqlExecData(t, taskConnQuery, vars, workspaceHeaders(token, wsID), &out)

		if out.Tasks.TotalCount != n {
			t.Fatalf("page %d totalCount = %d, want %d", pages, out.Tasks.TotalCount, n)
		}
		for _, e := range out.Tasks.Edges {
			if seen[e.Node.ID] {
				t.Fatalf("task %s returned on more than one page — unstable paging", e.Node.ID)
			}
			seen[e.Node.ID] = true
		}
		pages++
		if !out.Tasks.PageInfo.HasNextPage {
			break
		}
		if out.Tasks.PageInfo.EndCursor == nil {
			t.Fatal("hasNextPage is true but endCursor is null")
		}
		after = out.Tasks.PageInfo.EndCursor
		if pages > n+1 {
			t.Fatal("paging did not terminate")
		}
	}
	if len(seen) != n {
		t.Errorf("paged over %d distinct tasks, want %d", len(seen), n)
	}
}

// --- AC 6: offset pagination with total count -------------------------------

// TestOffsetPaginateExposesTotalCount proves offset-paginated lists expose a
// total count and a hasMore flag (story 40): a window reports the full count and
// hasMore=true, and a window past the end reports hasMore=false.
func TestOffsetPaginateExposesTotalCount(t *testing.T) {
	token, _ := register(t, "offset-owner@example.com", "offset-pw-00001", "Offset Owner")
	wsID := bootstrapWorkspace(t, token, "Offsetting", "offsetting-ws")
	projID := createProject(t, token, wsID, "Counted")
	const n = 5
	for range n {
		createTaskInProject(t, token, wsID, projID, "task", "TODO", "MEDIUM", "")
	}

	var first struct {
		TaskList struct {
			TotalCount int  `json:"totalCount"`
			HasMore    bool `json:"hasMore"`
			Items      []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"taskList"`
	}
	gqlExecData(t, taskOffsetQuery, map[string]any{"limit": 2, "offset": 0}, workspaceHeaders(token, wsID), &first)
	if first.TaskList.TotalCount != n {
		t.Errorf("offset page totalCount = %d, want %d", first.TaskList.TotalCount, n)
	}
	if len(first.TaskList.Items) != 2 {
		t.Errorf("offset page returned %d items, want 2 (the limit)", len(first.TaskList.Items))
	}
	if !first.TaskList.HasMore {
		t.Error("hasMore = false on the first of three pages, want true")
	}

	var last struct {
		TaskList struct {
			HasMore bool `json:"hasMore"`
			Items   []struct {
				ID string `json:"id"`
			} `json:"items"`
		} `json:"taskList"`
	}
	gqlExecData(t, taskOffsetQuery, map[string]any{"limit": 2, "offset": 4}, workspaceHeaders(token, wsID), &last)
	if len(last.TaskList.Items) != 1 {
		t.Errorf("last offset page returned %d items, want 1", len(last.TaskList.Items))
	}
	if last.TaskList.HasMore {
		t.Error("hasMore = true on the last page, want false")
	}
}

// --- AC 7: load a Task with its relationships in one query ------------------

// TestLoadTaskWithRelationsInOneQuery proves a Task loads with its Comments,
// Assignees, Labels, and Subtasks in a single query (story 41).
func TestLoadTaskWithRelationsInOneQuery(t *testing.T) {
	token, owner := register(t, "load-owner@example.com", "load-pw-000001", "Load Owner")
	wsID := bootstrapWorkspace(t, token, "Loading", "loading-ws")
	projID := createProject(t, token, wsID, "Rich")
	task := createTaskInProject(t, token, wsID, projID, "Rich task", "TODO", "MEDIUM", "")

	// A comment, an assignee (the owner), a label, and a subtask.
	gqlExecData(t, commentOnTaskMutation, map[string]any{"taskID": task.ID, "body": "first note"},
		workspaceHeaders(token, wsID), &struct {
			CommentOnTask struct {
				ID string `json:"id"`
			} `json:"commentOnTask"`
		}{})
	gqlExecData(t, assignUserToTaskMutation, map[string]any{"taskID": task.ID, "userID": owner.ID},
		workspaceHeaders(token, wsID), &struct {
			AssignUserToTask struct {
				TaskID string `json:"taskID"`
			} `json:"assignUserToTask"`
		}{})
	labelID := createLabel(t, token, wsID, "loaded", nil)
	gqlExecData(t, applyLabelToTaskMutation, map[string]any{"taskID": task.ID, "labelID": labelID},
		workspaceHeaders(token, wsID), &labeledTask{})
	subtask := createTaskInProject(t, token, wsID, projID, "Rich subtask", "TODO", "MEDIUM", task.ID)

	var out struct {
		Task struct {
			Comments []struct {
				ID string `json:"id"`
			} `json:"comments"`
			Assignees []struct {
				ID string `json:"id"`
			} `json:"assignees"`
			Labels []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"labels"`
			Subtasks []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"subtasks"`
		} `json:"task"`
	}
	gqlExecData(t, taskLoadRelationsQuery, map[string]any{"id": task.ID}, workspaceHeaders(token, wsID), &out)

	if len(out.Task.Comments) != 1 {
		t.Errorf("task.comments = %d, want 1", len(out.Task.Comments))
	}
	if len(out.Task.Assignees) != 1 || out.Task.Assignees[0].ID != owner.ID {
		t.Errorf("task.assignees = %+v, want the owner %s", out.Task.Assignees, owner.ID)
	}
	if len(out.Task.Labels) != 1 || out.Task.Labels[0].ID != labelID {
		t.Errorf("task.labels = %+v, want the applied label %s", out.Task.Labels, labelID)
	}
	if len(out.Task.Subtasks) != 1 || out.Task.Subtasks[0].ID != subtask.ID {
		t.Errorf("task.subtasks = %+v, want the subtask %s", out.Task.Subtasks, subtask.ID)
	}
}

// --- AC 8: Project Stats view -----------------------------------------------

// TestProjectStatsFromView proves per-project task statistics are available via
// the Project Stats view (story 42): the counts reflect the Tasks in the Project,
// and the view is tenant-scoped so a Project in another Workspace is invisible.
func TestProjectStatsFromView(t *testing.T) {
	token, _ := register(t, "stats-owner@example.com", "stats-pw-000001", "Stats Owner")
	wsID := bootstrapWorkspace(t, token, "Statsing", "statsing-ws")
	projID := createProject(t, token, wsID, "Measured")
	createTaskInProject(t, token, wsID, projID, "done-1", "DONE", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "done-2", "DONE", "MEDIUM", "")
	createTaskInProject(t, token, wsID, projID, "open-1", "TODO", "MEDIUM", "")

	var out struct {
		ProjectStat *struct {
			ProjectID  string `json:"projectID"`
			TotalTasks int    `json:"totalTasks"`
			DoneTasks  int    `json:"doneTasks"`
			OpenTasks  int    `json:"openTasks"`
		} `json:"projectStat"`
	}
	gqlExecData(t, projectStatQuery, map[string]any{"id": projID}, workspaceHeaders(token, wsID), &out)
	if out.ProjectStat == nil {
		t.Fatal("projectStat(projectID) = null, want stats for the project")
	}
	if out.ProjectStat.TotalTasks != 3 || out.ProjectStat.DoneTasks != 2 || out.ProjectStat.OpenTasks != 1 {
		t.Errorf("project stats = total %d / done %d / open %d, want 3 / 2 / 1",
			out.ProjectStat.TotalTasks, out.ProjectStat.DoneTasks, out.ProjectStat.OpenTasks)
	}

	// The view is tenant-scoped: another Workspace cannot read this Project's
	// stats — projectStat returns null there.
	otherToken, _ := register(t, "stats-other@example.com", "stats-pw-000002", "Stats Other")
	otherWS := bootstrapWorkspace(t, otherToken, "StatsOther", "stats-other-ws")
	gqlExecData(t, projectStatQuery, map[string]any{"id": projID}, workspaceHeaders(otherToken, otherWS), &out)
	if out.ProjectStat != nil {
		t.Errorf("projectStat for a project in another workspace = %+v, want null (view is tenant-scoped)", out.ProjectStat)
	}
}

// equalSlice reports whether two string slices are equal in order.
func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
