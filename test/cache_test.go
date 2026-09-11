package test

import (
	"context"
	"testing"
	"time"
)

// This file proves the Redis cache (issue 05) through the one seam the harness
// exercises — the GraphQL HTTP endpoint under an Active Workspace — against the
// real Redis testcontainer the shared TestMain boots and the exact WithCache
// wiring the server binary builds. It leans on the register / bootstrapWorkspace
// / createProject / createTaskInProject helpers from the identity, tenancy, and
// projects-&-tasks suites.
//
// The lever that makes "served from cache" observable without reaching into
// generated internals is an out-of-band UPDATE straight to Postgres: it moves
// the row behind the cache's back (bypassing the client, and therefore the
// cache-invalidation mutation hook), so the database and the cached entry
// disagree. A read that returns the OLD value can only have come from the cache;
// a read that returns the NEW value proves a fresh database round-trip.

// oobUpdateTaskTitle rewrites a Task's title directly in Postgres via the shared
// test pool, bypassing the client so the cache is NOT invalidated.
func oobUpdateTaskTitle(t *testing.T, taskID, title string) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(),
		`UPDATE tasks SET title = $1 WHERE id = $2`, title, taskID); err != nil {
		t.Fatalf("out-of-band task title update: %v", err)
	}
}

const taskTitleQuery = `query ($id: UUID!) { task(id: $id) { id title } }`

// getTaskTitle reads a Task's title through the GraphQL seam, merging any extra
// headers (e.g. the cache-bypass header) onto the Active-Workspace headers.
func getTaskTitle(t *testing.T, token, wsID, taskID string, extraHeaders map[string]string) string {
	t.Helper()
	headers := workspaceHeaders(token, wsID)
	for k, v := range extraHeaders {
		headers[k] = v
	}
	var out struct {
		Task *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"task"`
	}
	gqlExecData(t, taskTitleQuery, map[string]any{"id": taskID}, headers, &out)
	if out.Task == nil {
		t.Fatalf("task %s not found through the GraphQL seam", taskID)
	}
	return out.Task.Title
}

// TestTaskReadServedFromCacheThenInvalidatedByMutation proves both halves of
// story 46 on a real Task: a repeated single-entity Get is served from cache,
// and a mutation invalidates that entry so the next read reflects the change and
// never a stale value.
//
// Creating the Task warms the cache with the full entity via the cache mutation
// hook. An out-of-band DB update then desyncs the row; a read that still returns
// the original title can only have come from the cache. Finally a blessed
// updateTask mutation invalidates the entry, and the following read reflects the
// mutation — proving the cache was evicted rather than serving the old value.
func TestTaskReadServedFromCacheThenInvalidatedByMutation(t *testing.T) {
	token, _ := register(t, "cache-hit@example.com", "cache-pw-000001", "Cache Hit")
	wsID := bootstrapWorkspace(t, token, "CacheHit", "cache-hit-ws")
	projID := createProject(t, token, wsID, "Cached work")

	task := createTaskInProject(t, token, wsID, projID, "original-title", "TODO", "MEDIUM", "")

	// Desync the database behind the cache's back, then read: the original title
	// proves the read was served from cache, not from Postgres.
	oobUpdateTaskTitle(t, task.ID, "stale-desync")
	if got := getTaskTitle(t, token, wsID, task.ID, nil); got != "original-title" {
		t.Fatalf("read after out-of-band update = %q, want %q (should be served from cache)", got, "original-title")
	}

	// A blessed mutation on the Task must invalidate its cache entry.
	const updateMutation = `mutation ($id: UUID!, $input: UpdateTaskInput!) {
		updateTask(id: $id, input: $input) { id title }
	}`
	var updated struct {
		UpdateTask struct {
			Title string `json:"title"`
		} `json:"updateTask"`
	}
	gqlExecData(t, updateMutation, map[string]any{
		"id":    task.ID,
		"input": map[string]any{"title": "updated-via-mutation"},
	}, workspaceHeaders(token, wsID), &updated)
	if updated.UpdateTask.Title != "updated-via-mutation" {
		t.Fatalf("updateTask returned title %q, want %q", updated.UpdateTask.Title, "updated-via-mutation")
	}

	// The next read misses the evicted entry, hits Postgres, and reflects the
	// mutation. Had invalidation not fired, the cache would still hold
	// "original-title" and this read would return it.
	if got := getTaskTitle(t, token, wsID, task.ID, nil); got != "updated-via-mutation" {
		t.Errorf("read after mutation = %q, want %q (mutation must invalidate the cache)", got, "updated-via-mutation")
	}
}

// TestCacheBypassHeaderForcesFreshRead proves story 47: a request can force a
// fresh read past the cache with the Cache-Control: no-cache header (mapped to
// CallOptions.SkipCache by the generated per-request middleware).
//
// After the cache is warmed and the row desynced out of band, a default read is
// served the stale cached value while a no-cache read reveals the fresh database
// row — the two reads diverging on the same Task is the bypass working.
func TestCacheBypassHeaderForcesFreshRead(t *testing.T) {
	token, _ := register(t, "cache-bypass@example.com", "cache-pw-000002", "Cache Bypass")
	wsID := bootstrapWorkspace(t, token, "CacheBypass", "cache-bypass-ws")
	projID := createProject(t, token, wsID, "Bypass work")

	task := createTaskInProject(t, token, wsID, projID, "cached-title", "TODO", "MEDIUM", "")
	oobUpdateTaskTitle(t, task.ID, "fresh-title")

	// Default read is served from the (now stale) cache.
	if got := getTaskTitle(t, token, wsID, task.ID, nil); got != "cached-title" {
		t.Fatalf("default read = %q, want cached %q", got, "cached-title")
	}

	// Cache-Control: no-cache forces a fresh database read, revealing the
	// out-of-band change the cache was hiding.
	bypass := map[string]string{"Cache-Control": "no-cache"}
	if got := getTaskTitle(t, token, wsID, task.ID, bypass); got != "fresh-title" {
		t.Errorf("no-cache read = %q, want fresh %q (bypass must skip the cache)", got, "fresh-title")
	}
}

// TestAcceptInvitationInvalidatesCachedInvitation is the regression guard for
// the stale-cache bug wiring the cache exposed (story 46, "I never read stale
// data"): an Invitation is cached PENDING when it is sent, and accepting it must
// evict that entry so the sender's next read reflects acceptance. The accept
// resolver names the invitation's tenant explicitly (CallOptions.Tenant =
// inv.WorkspaceID) on the status update, so the cache hook invalidates under the
// same per-tenant key the entry was cached with. (The stale read this guards was
// originally caused by the update running under SkipTenancy with an unresolved
// tenant — a class of bug sqlgen now also closes generator-side by deriving the
// invalidation tenant from the mutated row.)
//
// Invalidation is asserted with a bounded poll rather than a single read:
// acceptInvitation writes in a transaction, and the cache hook defers
// invalidation to a post-commit callback that runs asynchronously (the client's
// default CallbackAsync), so eviction is eventually consistent — the same
// eventual-consistency shape the PRD's Activity-projector tests poll for. Before
// the fix the entry is never evicted and the poll exhausts on PENDING.
func TestAcceptInvitationInvalidatesCachedInvitation(t *testing.T) {
	ownerToken, _ := register(t, "cache-inv-owner@example.com", "cache-pw-000003", "Cache Inv Owner")
	wsID := bootstrapWorkspace(t, ownerToken, "CacheInv", "cache-inv-ws")

	inv := invite(t, ownerToken, wsID, "cache-inv-member@example.com", "MEMBER")

	// Warm the cache: the owner reads the Invitation by id, so it is cached
	// PENDING under the workspace tenant (the invite's Create already warms it;
	// this makes the cached entry explicit and independent of that).
	if s := invitationStatus(t, ownerToken, wsID, inv.ID); s != "PENDING" {
		t.Fatalf("invitation status before accept = %q, want PENDING", s)
	}

	// The invited person accepts. The status update runs tenant-resolved, so the
	// cache hook can evict the cached PENDING entry (post-commit, async).
	memberToken, _ := register(t, "cache-inv-member@example.com", "cache-pw-000004", "Cache Inv Member")
	if resp := acceptData(t, memberToken, inv.Token); len(resp.Errors) > 0 {
		t.Fatalf("accept invitation failed: %+v", resp.Errors)
	}

	// The owner's read eventually reflects acceptance — proof the cached PENDING
	// entry was invalidated rather than served stale forever. Without the fix
	// this poll exhausts on PENDING and the test fails.
	deadline := time.Now().Add(2 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		if last = invitationStatus(t, ownerToken, wsID, inv.ID); last == "ACCEPTED" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("invitation status after accept = %q, want ACCEPTED within 2s (accept must invalidate the cached entry)", last)
}
