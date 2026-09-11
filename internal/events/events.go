// Package events wires the app's domain-event transport (issue 06). Generated
// mutation hooks publish an event.Event after every successful write; this
// package builds the NATS bus they publish to (attached via
// database.WithEventPublisher) and hosts the Activity projector that consumes
// those events into the activity feed.
//
// The bus is a single *natsbus.Bus: an event.Publisher on the produce side and
// an event.Subscriber on the consume side, so one connection serves both roles.
// ActorMetadata is the event.Config.MetadataFunc that carries the mutating
// caller across the async publish boundary so the projector can record it.
package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/teandresmith/sqlgen/event"
	"github.com/teandresmith/sqlgen/event/natsbus"

	"github.com/teandresmith/sqlgen-example/internal/auth"
)

// MetadataKeyActor is the Event.Metadata key under which the authenticated
// caller's User id travels from the mutation hook to the Activity projector.
// The generated hook stamps the system "tenant" key (workspace id) alongside
// it; the projector reads both back when writing an activity row.
const MetadataKeyActor = "actor"

// Connect dials the NATS event bus with natsbus's production resilience
// defaults (infinite reconnect, a reconnect buffer, drain-on-close) and returns
// it. The returned *natsbus.Bus is both the event.Publisher passed to
// database.WithEventPublisher and the event.Subscriber the projector subscribes
// on; its Close drains and tears down the owned connection.
//
// A subscribe-error handler is registered so a projector Handler failure is
// logged rather than silently discarded — core NATS is at-most-once and cannot
// redeliver, so surfacing the failure is the best it can do (the durable
// JetStream path exists in natsbus but the feed does not need it).
func Connect(url string, logger *slog.Logger) (*natsbus.Bus, error) {
	bus, err := natsbus.Connect(url,
		natsbus.WithSubscribeErrorHandler(func(ctx context.Context, e event.Event, err error) {
			logger.ErrorContext(ctx, "activity projector handler failed",
				"table", e.Table, "action", string(e.Action), "pk", e.PK, "err", err)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting event bus: %w", err)
	}
	return bus, nil
}

// ActorMetadata is the event.Config.MetadataFunc that stamps the authenticated
// caller onto every published event's Metadata. The generated mutation hook
// invokes it once at hook entry — where the request context is still live — so
// the actor survives the async Tx.OnCommit publish, which runs on a background
// context with no request identity. An unauthenticated write (no User in
// context) contributes no actor key, and the resulting activity row records a
// NULL (system) actor.
func ActorMetadata(ctx context.Context) map[string]string {
	id, ok := auth.UserID(ctx)
	if !ok {
		return nil
	}
	return map[string]string{MetadataKeyActor: id.String()}
}
