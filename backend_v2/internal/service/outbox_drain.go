package service

import (
	"context"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/safego"
)

// Data-layer R1 — the CDC outbox drain (live delivery).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Producers only append to outbox_events (zero realtime dependency). ONE
// background drain tails the log by xid and publishes each frame over the
// realtime websocket, per user. This is the single live-publish point (clean
// CDC) — pull remains the durable guarantee; the drain is best-effort latency.

// DrainedEvent is one outbox row surfaced to the drain: the committing xid, the
// owning user (so it fans out only to that user), and the decoded payload —
// either a data Frame (type 'data_event') or an AppEvent (type 'app_event').
// Exactly one of Frame/AppEvent is meaningful per row; AppEvent != nil selects
// the app-event path.
type DrainedEvent struct {
	Xid      uint64
	UserID   string
	Frame    Frame
	AppEvent *OutboxAppEvent
}

// OutboxDrainReader is the drain's read port (implemented by the repo). DrainSince returns events
// with afterCursor <= xid < horizon across ALL users, in xid order, plus the next cursor (the
// horizon, or — if the read hit its batch limit — just past the last row, so the caller paginates).
// Horizon is the current log horizon, used to seed the boot cursor.
type OutboxDrainReader interface {
	DrainSince(ctx context.Context, afterCursor uint64) (events []DrainedEvent, nextCursor uint64, err error)
	Horizon(ctx context.Context) (uint64, error)
}

// OutboxDrainer polls the outbox and publishes each new frame to its user's
// realtime subscribers.
type OutboxDrainer struct {
	reader   OutboxDrainReader
	realtime RealtimeEventService
	interval time.Duration
}

func NewOutboxDrainer(reader OutboxDrainReader, realtime RealtimeEventService, interval time.Duration) *OutboxDrainer {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	return &OutboxDrainer{reader: reader, realtime: realtime, interval: interval}
}

// Start launches the drain loop in a goroutine; it stops when ctx is cancelled. Supervised: a
// panic in the drain is recovered and the loop restarted (a wedged drain halts realtime for all
// users, so it must not die silently).
func (d *OutboxDrainer) Start(ctx context.Context) {
	safego.Loop(ctx, "outbox.drain", d.run)
}

func (d *OutboxDrainer) run(ctx context.Context) {
	// Seed the cursor at the current horizon: skip everything committed before boot (clients heal
	// that via pull-on-connect), so the drain only publishes frames that commit from now on. Retry
	// until the horizon read succeeds — a transient DB error at boot must NOT leave the cursor at 0,
	// which would replay the entire outbox to every client on the next tick.
	cursor, ok := d.initCursor(ctx)
	if !ok {
		return // ctx cancelled during init
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			next, err := d.drainOnce(ctx, cursor)
			if err != nil {
				slog.Warn("outbox drain", "err", err)
				continue // keep the cursor; retry next tick
			}
			cursor = next
		}
	}
}

// initCursor reads the current horizon, retrying on error until it succeeds or ctx is cancelled.
// Returns ok=false only if ctx was cancelled before a horizon could be read.
func (d *OutboxDrainer) initCursor(ctx context.Context) (uint64, bool) {
	for {
		if h, err := d.reader.Horizon(ctx); err == nil {
			return h, true
		} else {
			slog.Warn("outbox drain: init horizon failed, retrying", "err", err)
		}
		select {
		case <-ctx.Done():
			return 0, false
		case <-time.After(time.Second):
		}
	}
}

// drainOnce publishes the events in [cursor, nextCursor) in order and advances the cursor.
// Publishing is per-user (TenantID = the event's UserID). On the FIRST publish failure it stops
// and returns that event's xid as the new cursor, so the window [failed, …) is retried next tick
// rather than being burned off the live channel — at-least-once delivery (the client seq guard
// dedups any redelivery). Every event before the failure committed and is not re-published.
func (d *OutboxDrainer) drainOnce(ctx context.Context, cursor uint64) (uint64, error) {
	events, next, err := d.reader.DrainSince(ctx, cursor)
	if err != nil {
		return cursor, err
	}
	for _, e := range events {
		if err := d.publishOne(ctx, e); err != nil {
			slog.Warn("outbox drain publish; will retry window", "xid", e.Xid, "user", e.UserID, "err", err)
			return e.Xid, nil
		}
	}
	return next, nil
}

// publishOne fans one drained event out to its user's realtime subscribers. Server-trusted tenant
// (TenantID = the event's UserID); ResolvePublishKeys honors it, so no request principal is needed
// in this background context.
func (d *OutboxDrainer) publishOne(ctx context.Context, e DrainedEvent) error {
	if e.AppEvent != nil {
		// App-event lane: publish under its own eventType (e.g. "artifact.build-status") — NOT the
		// data-sync "*.frame" stream — so the UI reacts but the offline data engine ignores it. A
		// broadcast-stream app-event (e.g. "market") fans out to all subscribers: its stream is
		// honored and the tenant is left unbound (the row's UserID is a sentinel, irrelevant to
		// broadcast routing).
		stream := e.AppEvent.Stream
		tenantID := e.UserID
		if isBroadcastStream(stream) {
			tenantID = ""
		} else {
			stream = WorkspaceStream
		}
		return d.realtime.Publish(ctx, PublishTarget{
			Stream:       stream,
			ResourceKind: e.AppEvent.ResourceKind,
			ResourceID:   e.AppEvent.ResourceID,
			Operation:    e.AppEvent.Operation,
			TenantID:     tenantID,
		}, e.AppEvent.Payload)
	}
	return d.realtime.Publish(ctx, PublishTarget{
		Stream:       WorkspaceStream,
		ResourceKind: AnyResource,
		ResourceID:   AnyResource,
		Operation:    FrameOperation, // composes eventType WorkspaceFrameKind ("*.frame")
		TenantID:     e.UserID,
	}, e.Frame)
}
