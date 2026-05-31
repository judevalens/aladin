package service

import (
	"context"
	"log/slog"
	"time"
)

// Data-layer R1 — the CDC outbox drain (live delivery).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// Producers only append to outbox_events (zero realtime dependency). ONE
// background drain tails the log by xid and publishes each frame over the
// realtime websocket, per user. This is the single live-publish point (clean
// CDC) — pull remains the durable guarantee; the drain is best-effort latency.

// DrainedEvent is one outbox row surfaced to the drain: the committing xid, the
// owning user (so the frame fans out only to that user), and the decoded frame.
type DrainedEvent struct {
	Xid    uint64
	UserID string
	Frame  Frame
}

// OutboxDrainReader is the drain's read port (implemented by the repo). Returns
// events with afterCursor <= xid < horizon across ALL users, in xid order, plus
// the horizon (the new cursor). Same gap-free half-open window as PullSince.
type OutboxDrainReader interface {
	DrainSince(ctx context.Context, afterCursor uint64) (events []DrainedEvent, horizon uint64, err error)
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

// Start launches the drain loop in a goroutine; it stops when ctx is cancelled.
func (d *OutboxDrainer) Start(ctx context.Context) {
	go d.run(ctx)
}

func (d *OutboxDrainer) run(ctx context.Context) {
	// Initialize the cursor to the current horizon: skip everything already
	// committed before boot (clients heal that via pull-on-connect), so the drain
	// only publishes frames that commit from now on. A restart re-publishing a few
	// in-flight frames would be harmless anyway (the client seq guard dedups).
	cursor := uint64(0)
	if _, horizon, err := d.reader.DrainSince(ctx, 0); err == nil {
		cursor = horizon
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

// drainOnce publishes every frame in [cursor, horizon) and returns the horizon
// as the new cursor. Publishing is per-user (TenantID = the event's UserID).
func (d *OutboxDrainer) drainOnce(ctx context.Context, cursor uint64) (uint64, error) {
	events, horizon, err := d.reader.DrainSince(ctx, cursor)
	if err != nil {
		return cursor, err
	}
	for _, e := range events {
		// Server-trusted tenant: the frame fans out only to this user's
		// subscribers. ResolvePublishKeys honors a caller-set TenantID, so no
		// request principal is needed in this background context.
		if err := d.realtime.Publish(ctx, PublishTarget{
			Stream:       WorkspaceStream,
			ResourceKind: AnyResource,
			ResourceID:   AnyResource,
			Operation:    "frame",
			TenantID:     e.UserID,
		}, e.Frame); err != nil {
			slog.Warn("outbox drain publish", "xid", e.Xid, "user", e.UserID, "err", err)
		}
	}
	return horizon, nil
}
