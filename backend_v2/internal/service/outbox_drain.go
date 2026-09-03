package service

import (
	"context"
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
