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
