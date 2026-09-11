// This file is hand-written (not generated): it exposes the per-request HTTP
// call-option setters to hand-written resolvers in the parent graph package.
package sqlgenresolver

import (
	"context"

	database "github.com/teandresmith/sqlgen-example/internal/database"
)

// CallOptionsFromHTTP exports the per-request CallOptions setters that
// WithCallOptionsMiddleware extracts from the standard headers (PRD §26.11 —
// Cache-Control: no-cache, X-Skip-Events, X-Skip-Hooks). The generated M/Q
// delegates apply these to every mutation and query; a bespoke resolver that
// calls the client directly (e.g. graph.CreateTaskInProject, which stamps the
// reporter from context) uses this to honor the same headers without also
// adopting field-option projection — so it keeps whatever create-time cache
// behavior a full-entity write has. Returns nil when no override headers were
// set or the middleware did not run.
func CallOptionsFromHTTP[FO any](ctx context.Context) []func(*database.CallOptions[FO]) {
	return callOptionsFromHTTP[FO](ctx)
}
