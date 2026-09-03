package workspacesync

import "context"

// Data-layer R1 — the sync read service (delta-vs-snapshot policy).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// SyncService owns the recovery policy: a client with a fresh or too-old cursor
// gets a cold-start SNAPSHOT (current state from the canonical tables, including
// tombstones); otherwise it gets a DELTA (the outbox events since its cursor).
// It depends only on small ports (OutboxReader, SyncSource) it defines here, so
// it unit-tests with fakes — no concrete repo dependency.

// SyncService is the read side of the sync engine, consumed by the pull handler.
type SyncService interface {
	// Pull returns the events (or snapshot) for userID since cursor, plus the
	// new cursor (the log horizon). cursor == 0 means a cold client.
	Pull(ctx context.Context, userID string, cursor uint64) (PullResult, error)
}

// OutboxReader is the durable-log read port (implemented by the repo).
type OutboxReader interface {
	// PullSince returns the frames with cursor <= xid < horizon for userID, in
	// xid order, plus the horizon (the new cursor). Half-open [cursor, horizon).
	PullSince(ctx context.Context, userID string, cursor uint64) (frames []Frame, horizon uint64, err error)
	// MinXid is the oldest retained event's xid for userID (ok=false if none).
	// Used to detect a cursor that has fallen behind the retention horizon.
	MinXid(ctx context.Context, userID string) (minXid uint64, ok bool, err error)
	// Horizon is the current log horizon (pg_snapshot_xmin) — the cursor a
	// snapshot resumes from. User-independent.
	Horizon(ctx context.Context) (uint64, error)
}

// SyncSource is a per-kind cold-start snapshot provider (implemented by the
// repo). The engine is entity-agnostic; each kind contributes one source.
type SyncSource interface {
	EntityKind() string
	// Snapshot returns ALL of userID's entities for this kind INCLUDING
	// tombstones (is_deleted → Op delete), each carrying its current seq, so
	// the client's seq guard works on the snapshot and resurrection stays
	// blocked on a fresh client.
	Snapshot(ctx context.Context, userID string) ([]FrameEntity, error)
}
