package events

import (
	"context"
	"encoding/json"
	"fmt"

	"uuid"

	"github.com/teandresmith/sqlgen/event"
	"github.com/teandresmith/sqlgen/omittable"
	"github.com/teandresmith/sqlgen/types"

	database "github.com/teandresmith/sqlgen-example/internal/database"
)

// Projector consumes mutation events and appends one row per event to the
// activity table — the Activity feed (issue 06, story 44). Each row records the
// actor (from event metadata), the affected entity (table + id), the action, and
// a JSON snapshot of the change. It writes through the same *database.Client the
// rest of the app uses; because the activity table has events disabled
// (sqlgen.yml), these writes publish no further events, so the feed cannot
// recurse (story 45).
//
// A junction table is a relationship, not a feed subject (ADR-0006): its
// mutations are recorded as events on the real entity they belong to — the
// anchor — with the other side of the relationship captured in the payload. So
// activity.entity_id is always a genuine single-uuid entity id.
type Projector struct {
	client *database.Client
}

// NewProjector returns a Projector that writes to the given client.
func NewProjector(client *database.Client) *Projector {
	return &Projector{client: client}
}

// Subscribe registers the projector on the bus for every table and action and
// returns the resulting Subscription (cancelled on shutdown, or by bus.Close).
func (p *Projector) Subscribe(sub event.Subscriber) (event.Subscription, error) {
	return sub.Subscribe(event.SubscribeOptions{}, p.Handle)
}

// anchor names, for a junction (relationship) table, the real entity its
// mutations are recorded against (ADR-0006). fk is the JSON key of the
// anchor-entity id within the composite primary key; tenantOf resolves the
// activity row's workspace from the anchor entity when the event carries no
// tenant stamp of its own.
type anchor struct {
	entityTable string
	fk          string
	tenantOf    func(ctx context.Context, c *database.Client, anchorID uuid.UUID) (uuid.UUID, error)
}

// junctionAnchors maps each composite-key junction to its anchor entity. The
// anchor FK is the side the relationship belongs to, which is also the table the
// junction is tenanted through — so it doubles as the tenant source.
//
// Keys and entity tables are the bare names event.Event.Table carries (the
// schema travels separately in Event.Schema), not the database.Table* hook
// constants, which are schema-qualified ("public.tasks") to keep same-named
// tables in different schemas distinct.
var junctionAnchors = map[string]anchor{
	"task_assignees":    {"tasks", "task_id", tenantOfTask},
	"task_watchers":     {"tasks", "task_id", tenantOfTask},
	"task_labels":       {"tasks", "task_id", tenantOfTask},
	"task_dependencies": {"tasks", "task_id", tenantOfTask},
	"team_members":      {"teams", "team_id", tenantOfTeam},
	"memberships":       {"workspaces", "workspace_id", tenantIsAnchor},
}

// Handle projects one event into an activity row. It runs on a background
// context off the request path, so the write carries its own tenant (set under
// SkipTenancy) rather than relying on a resolved Active Workspace.
func (p *Projector) Handle(ctx context.Context, e event.Event) error {
	entityTable, entityID, workspaceID, ok, err := p.resolveTarget(ctx, e)
	if err != nil {
		return err
	}
	if !ok {
		return nil // not projectable — shared table, or an unmapped composite key
	}

	// Snapshot the change. Event.Input is the mutation input (both sides of a
	// junction, plus any attributes); for a delete it is nil, so fall back to the
	// PK, which still carries the relationship's identity. A nil value marshals to
	// JSON null — a valid non-null jsonb value — satisfying the NOT NULL column.
	snapshot := e.Input
	if snapshot == nil {
		snapshot = e.PK
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("activity projector: snapshot payload for %q: %w", e.Table, err)
	}

	input := &database.CreateActivityInput{
		WorkspaceID: omittable.Set(workspaceID),
		EntityTable: entityTable,
		EntityID:    entityID,
		Action:      string(e.Action),
		Payload:     omittable.Set(types.JSON(payload)),
	}
	if actor, ok := parseActor(e.Metadata[MetadataKeyActor]); ok {
		input.ActorID = omittable.Set(actor)
	}

	// SkipTenancy: the row's workspace_id is supplied explicitly above; no
	// resolver runs off the request path. Activity is not an authz-guarded table,
	// so the write passes the authorization hook unchallenged.
	_, err = p.client.Activities().Create(ctx, input, func(o *database.CallOptions[database.ActivityFieldOptions]) {
		o.SkipTenancy = true
	})
	if err != nil {
		return fmt.Errorf("activity projector: write activity for %q %s: %w", entityTable, e.Action, err)
	}
	return nil
}

// resolveTarget decides which entity an event is recorded against, its id, and
// the owning workspace. A single-uuid event is recorded on itself; a composite
// (junction) event is recorded on its anchor entity (ADR-0006). It reports
// ok=false for events that belong to no workspace feed — shared tables (users,
// workspaces mutated directly) and any composite table without a mapped anchor.
func (p *Projector) resolveTarget(ctx context.Context, e event.Event) (entityTable string, entityID, workspaceID uuid.UUID, ok bool, err error) {
	if id, isSingle := singleUUID(e.PK); isSingle {
		ws, tenanted := parseTenant(e.Metadata["tenant"])
		if !tenanted {
			return "", uuid.UUID{}, uuid.UUID{}, false, nil // shared table — no workspace feed
		}
		return e.Table, id, ws, true, nil
	}

	a, mapped := junctionAnchors[e.Table]
	if !mapped {
		return "", uuid.UUID{}, uuid.UUID{}, false, nil // unmapped composite table — not projectable
	}
	anchorID, found := compositePKValue(e.PK, a.fk)
	if !found {
		return "", uuid.UUID{}, uuid.UUID{}, false, nil
	}
	ws, err := p.tenantFor(ctx, e, a, anchorID)
	if err != nil {
		return "", uuid.UUID{}, uuid.UUID{}, false,
			fmt.Errorf("activity projector: resolve tenant for %q via %s: %w", e.Table, a.entityTable, err)
	}
	return a.entityTable, anchorID, ws, true, nil
}

// tenantFor resolves a junction event's workspace: the event's own tenant stamp
// when present (memberships carries one, its tenant being in the PK), otherwise
// the anchor entity's workspace, fetched by tenantOf.
func (p *Projector) tenantFor(ctx context.Context, e event.Event, a anchor, anchorID uuid.UUID) (uuid.UUID, error) {
	if ws, ok := parseTenant(e.Metadata["tenant"]); ok {
		return ws, nil
	}
	return a.tenantOf(ctx, p.client, anchorID)
}

// tenantOfTask reads a task's workspace under SkipTenancy — the projector runs
// off the request path with no resolved Active Workspace.
func tenantOfTask(ctx context.Context, c *database.Client, id uuid.UUID) (uuid.UUID, error) {
	t, err := c.Tasks().Get(ctx, id, func(o *database.CallOptions[database.TaskFieldOptions]) { o.SkipTenancy = true })
	if err != nil {
		return uuid.UUID{}, err
	}
	return t.WorkspaceID, nil
}

// tenantOfTeam reads a team's workspace under SkipTenancy.
func tenantOfTeam(ctx context.Context, c *database.Client, id uuid.UUID) (uuid.UUID, error) {
	t, err := c.Teams().Get(ctx, id, func(o *database.CallOptions[database.TeamFieldOptions]) { o.SkipTenancy = true })
	if err != nil {
		return uuid.UUID{}, err
	}
	return t.WorkspaceID, nil
}

// tenantIsAnchor is the resolver for memberships, whose anchor id (workspace_id)
// is itself the tenant. It is a fallback only — a membership event carries its
// tenant stamp, so tenantFor returns before reaching here.
func tenantIsAnchor(_ context.Context, _ *database.Client, anchorID uuid.UUID) (uuid.UUID, error) {
	return anchorID, nil
}

// singleUUID reports whether pk is a single-uuid primary key and returns it. A
// composite key decodes to a map and returns ok=false. The key arrives as a
// string after the event's JSON round-trip; the concrete-uuid form is accepted
// too, for robustness against an in-process publisher.
func singleUUID(pk any) (uuid.UUID, bool) {
	switch v := pk.(type) {
	case string:
		id, err := uuid.Parse(v)
		return id, err == nil
	case uuid.UUID:
		return v, true
	default:
		return uuid.UUID{}, false
	}
}

// compositePKValue extracts one uuid component from a composite primary key by
// its JSON key. The composite PK arrives as a map after the event's JSON
// round-trip (e.g. {"task_id":"…","user_id":"…"}).
func compositePKValue(pk any, key string) (uuid.UUID, bool) {
	m, ok := pk.(map[string]any)
	if !ok {
		return uuid.UUID{}, false
	}
	s, ok := m[key].(string)
	if !ok {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}

// parseTenant parses the tenant stamp into a workspace id. An empty or malformed
// value yields ok=false — the event belongs to no workspace feed.
func parseTenant(s string) (uuid.UUID, bool) {
	if s == "" {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(s)
	return id, err == nil
}

// parseActor converts the actor metadata value (set by ActorMetadata) into a
// nullable uuid for the activity row. An empty or unparseable value yields
// ok=false, leaving actor_id NULL — a system (unattributed) action.
func parseActor(s string) (*uuid.UUID, bool) {
	if s == "" {
		return nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, false
	}
	return &id, true
}
