package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
)

// outbox.go — the generic durable log: append (producer side) + the
// service.OutboxReader reads (consumer side). Entity DATA never lives here; the
// payload is an opaque JSON frame (service.Frame). Ordering is by `xid` (the
// committing txn's xid8): the client's resume cursor and replay key.

// appendOutboxEvent appends ONE 'data_event' row carrying frame, inside the
// caller's write tx (which must already hold LockUser + the canonical write), so
// the event commits atomically with the data (transactional outbox). A frame
// with no entities is a no-op.
func appendOutboxEvent(ctx context.Context, tx pgx.Tx, userID string, frame service.Frame) error {
	if len(frame.Entities) == 0 {
		return nil
	}
	payload, err := json.Marshal(frame)
	if err != nil {
		return fmt.Errorf("sync: marshal frame: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (user_id, type, payload)
		VALUES ($1::uuid, 'data_event', $2::jsonb)
	`, userID, payload); err != nil {
		return fmt.Errorf("sync: append outbox event: %w", err)
	}
	return nil
}

// appendAppEvent appends ONE 'app_event' row carrying a non-data realtime event
// (service.OutboxAppEvent), inside the caller's write tx. The live drain forwards
// it to the realtime websocket under its own eventType; the durable pull path
// (PullSince/MinXid) filters 'app_event' out, so it never reaches a client's
// offline data store. Use for ephemeral, cross-process UI events (build-status).
func appendAppEvent(ctx context.Context, tx pgx.Tx, userID string, ev service.OutboxAppEvent) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("sync: marshal app event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO outbox_events (user_id, type, payload)
		VALUES ($1::uuid, 'app_event', $2::jsonb)
	`, userID, payload); err != nil {
		return fmt.Errorf("sync: append app event: %w", err)
	}
	return nil
}

// marketQuoteUserID is the sentinel owner for broadcast 'market' rows: outbox_events.user_id
// is NOT NULL, but broadcast routing ignores it (the drain publishes untenanted).
const marketQuoteUserID = "00000000-0000-0000-0000-000000000000"

// AppendMarketQuote appends a broadcast 'market' app_event carrying a quote payload, keyed on
// the instrument's distinct id. Own tx — the market-data hub holds no request tx.
func (r *SyncRepo) AppendMarketQuote(ctx context.Context, instrumentID string, payload []byte) error {
	ev := service.OutboxAppEvent{
		Stream:       service.MarketStream,
		ResourceKind: "quote",
		ResourceID:   instrumentID,
		Operation:    "update",
		Payload:      payload,
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("sync: begin market quote: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := appendAppEvent(ctx, tx, marketQuoteUserID, ev); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Horizon returns the current log horizon: pg_snapshot_xmin of the current
// snapshot, i.e. the smallest still-in-progress xid. Every xid below it is
// committed, so reading `xid < horizon` is gap-free regardless of commit order.
// User-independent. Read as text and parsed (xid8 has no direct Go scan type).
func (r *SyncRepo) Horizon(ctx context.Context) (uint64, error) {
	var s string
	if err := r.pool.QueryRow(ctx,
		`SELECT pg_snapshot_xmin(pg_current_snapshot())::text`).Scan(&s); err != nil {
		return 0, fmt.Errorf("sync: horizon: %w", err)
	}
	h, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("sync: parse horizon %q: %w", s, err)
	}
	return h, nil
}

// PullSince returns the frames committed in [cursor, horizon) for userID, in xid
// order, plus the horizon (the new cursor). The half-open window tiles perfectly
// across calls: no gap, no boundary duplication; the client's seq guard absorbs
// any redelivery. Reads only fully-committed events (xid < horizon).
func (r *SyncRepo) PullSince(ctx context.Context, userID string, cursor uint64) ([]service.Frame, uint64, error) {
	horizon, err := r.Horizon(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT payload
		  FROM outbox_events
		 WHERE user_id = $1::uuid
		   AND type = 'data_event'
		   AND xid >= $2::xid8
		   AND xid <  $3::xid8
		 ORDER BY xid
	`, userID, strconv.FormatUint(cursor, 10), strconv.FormatUint(horizon, 10))
	if err != nil {
		return nil, 0, fmt.Errorf("sync: pull since: %w", err)
	}
	defer rows.Close()

	frames := make([]service.Frame, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, 0, fmt.Errorf("sync: pull scan: %w", err)
		}
		var f service.Frame
		if err := json.Unmarshal(payload, &f); err != nil {
			return nil, 0, fmt.Errorf("sync: decode frame: %w", err)
		}
		frames = append(frames, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("sync: pull rows: %w", err)
	}
	return frames, horizon, nil
}

// MinXid returns the oldest retained event's xid for userID (ok=false if the
// user has no events). Used to detect a cursor that has fallen behind retention,
// which routes the client to a cold-start snapshot. Uses ORDER BY ... LIMIT 1
// (the (user_id, xid) index makes it an index scan) rather than MIN().
func (r *SyncRepo) MinXid(ctx context.Context, userID string) (uint64, bool, error) {
	var s string
	err := r.pool.QueryRow(ctx, `
		SELECT xid::text FROM outbox_events
		 WHERE user_id = $1::uuid
		   AND type = 'data_event'
		 ORDER BY xid
		 LIMIT 1
	`, userID).Scan(&s)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("sync: min xid: %w", err)
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("sync: parse min xid %q: %w", s, err)
	}
	return v, true, nil
}

// DrainSince returns the events committed in [afterCursor, horizon) across ALL
// users, in xid order, plus the horizon (the new cursor) — the CDC drain's read
// (service.OutboxDrainReader). Same gap-free half-open window as PullSince; the
// drain publishes each frame to its user and advances its cursor to the horizon.
func (r *SyncRepo) DrainSince(ctx context.Context, afterCursor uint64) ([]service.DrainedEvent, uint64, error) {
	horizon, err := r.Horizon(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT xid::text, user_id::text, type, payload
		  FROM outbox_events
		 WHERE xid >= $1::xid8
		   AND xid <  $2::xid8
		 ORDER BY xid
	`, strconv.FormatUint(afterCursor, 10), strconv.FormatUint(horizon, 10))
	if err != nil {
		return nil, 0, fmt.Errorf("sync: drain since: %w", err)
	}
	defer rows.Close()

	out := make([]service.DrainedEvent, 0)
	for rows.Next() {
		var xidStr, userID, typ string
		var payload []byte
		if err := rows.Scan(&xidStr, &userID, &typ, &payload); err != nil {
			return nil, 0, fmt.Errorf("sync: drain scan: %w", err)
		}
		xid, err := strconv.ParseUint(xidStr, 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("sync: drain parse xid %q: %w", xidStr, err)
		}
		ev := service.DrainedEvent{Xid: xid, UserID: userID}
		if typ == "app_event" {
			var ae service.OutboxAppEvent
			if err := json.Unmarshal(payload, &ae); err != nil {
				return nil, 0, fmt.Errorf("sync: drain decode app event: %w", err)
			}
			ev.AppEvent = &ae
		} else {
			if err := json.Unmarshal(payload, &ev.Frame); err != nil {
				return nil, 0, fmt.Errorf("sync: drain decode frame: %w", err)
			}
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("sync: drain rows: %w", err)
	}
	return out, horizon, nil
}
