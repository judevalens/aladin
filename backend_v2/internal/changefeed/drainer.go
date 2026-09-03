// Package changefeed owns durable outbox ordering and best-effort live publication.
package changefeed

import (
	"context"
	"log/slog"
	"time"

	"aladin/backend_v2/internal/safego"
	coreservice "aladin/backend_v2/internal/service"
)

const DefaultDrainInterval = 50 * time.Millisecond

type Drainer struct {
	reader   coreservice.OutboxDrainReader
	realtime coreservice.RealtimeEventService
	interval time.Duration
}

func NewDrainer(reader coreservice.OutboxDrainReader, realtime coreservice.RealtimeEventService, interval time.Duration) *Drainer {
	if interval <= 0 {
		interval = DefaultDrainInterval
	}
	return &Drainer{reader: reader, realtime: realtime, interval: interval}
}

func (d *Drainer) Start(ctx context.Context) { safego.Loop(ctx, "outbox.drain", d.run) }

func (d *Drainer) run(ctx context.Context) {
	cursor, ok := d.initCursor(ctx)
	if !ok {
		return
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
				continue
			}
			cursor = next
		}
	}
}

func (d *Drainer) initCursor(ctx context.Context) (uint64, bool) {
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

func (d *Drainer) drainOnce(ctx context.Context, cursor uint64) (uint64, error) {
	events, next, err := d.reader.DrainSince(ctx, cursor)
	if err != nil {
		return cursor, err
	}
	for _, event := range events {
		if err := d.publishOne(ctx, event); err != nil {
			slog.Warn("outbox drain publish; will retry window", "xid", event.Xid, "user", event.UserID, "err", err)
			return event.Xid, nil
		}
	}
	return next, nil
}

func (d *Drainer) publishOne(ctx context.Context, event coreservice.DrainedEvent) error {
	if event.AppEvent != nil {
		stream := event.AppEvent.Stream
		tenantID := event.UserID
		if stream == coreservice.MarketStream {
			tenantID = ""
		} else {
			stream = coreservice.WorkspaceStream
		}
		return d.realtime.Publish(ctx, coreservice.PublishTarget{
			Stream: stream, ResourceKind: event.AppEvent.ResourceKind, ResourceID: event.AppEvent.ResourceID,
			Operation: event.AppEvent.Operation, TenantID: tenantID,
		}, event.AppEvent.Payload)
	}
	return d.realtime.Publish(ctx, coreservice.PublishTarget{
		Stream: coreservice.WorkspaceStream, ResourceKind: coreservice.AnyResource,
		ResourceID: coreservice.AnyResource, Operation: coreservice.FrameOperation, TenantID: event.UserID,
	}, event.Frame)
}
