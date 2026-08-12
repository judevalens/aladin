package service

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

type DefaultSyncService struct {
	outbox  OutboxReader
	sources []SyncSource
}

func NewSyncService(outbox OutboxReader, sources ...SyncSource) *DefaultSyncService {
	return &DefaultSyncService{outbox: outbox, sources: sources}
}

func (s *DefaultSyncService) Pull(ctx context.Context, userID string, cursor uint64) (PullResult, error) {
	// Cold client → snapshot.
	if cursor == 0 {
		return s.snapshot(ctx, userID)
	}
	// Cursor older than what we still retain → snapshot (else silent data loss).
	minXid, ok, err := s.outbox.MinXid(ctx, userID)
	if err != nil {
		return PullResult{}, err
	}
	if ok && cursor < minXid {
		return s.snapshot(ctx, userID)
	}
	// Cursor NEWER than our horizon → it cannot have come from this database. Cursors are
	// Postgres xids and therefore cluster-local, so a restore from backup, a `prod-down -v`
	// and recreate, or pointing a client at a different backend all leave the client holding a
	// number this server will never reach.
	//
	// Without this the failure is silent and permanent, which is the worst shape available:
	// the too-old guard above catches its case, PullSince simply finds no frames above an
	// impossible cursor, and the client sits reporting "up to date" while receiving nothing,
	// forever. That is what forced a manual client wipe (`make prod-app-clear`) after every
	// prod DB reset. Snapshotting makes the client self-heal on its next pull, the same way
	// the local SQLite replica already recovers from an unusable file (Db::open_or_recover).
	//
	// Note this is not a heuristic: in normal operation the client's cursor came FROM a
	// horizon this server issued, so cursor > horizon is only reachable if the timeline changed.
	current, err := s.outbox.Horizon(ctx)
	if err != nil {
		return PullResult{}, err
	}
	if cursor > current {
		return s.snapshot(ctx, userID)
	}
	frames, horizon, err := s.outbox.PullSince(ctx, userID, cursor)
	if err != nil {
		return PullResult{}, err
	}
	return PullResult{Frames: frames, Cursor: horizon, Mode: PullModeDelta}, nil
}

// snapshot reconstructs the user's current state from every registered source
// and resumes deltas at the current horizon.
func (s *DefaultSyncService) snapshot(ctx context.Context, userID string) (PullResult, error) {
	var entities []FrameEntity
	for _, src := range s.sources {
		es, err := src.Snapshot(ctx, userID)
		if err != nil {
			return PullResult{}, err
		}
		entities = append(entities, es...)
	}
	horizon, err := s.outbox.Horizon(ctx)
	if err != nil {
		return PullResult{}, err
	}
	frames := []Frame{}
	if len(entities) > 0 {
		frames = append(frames, Frame{Entities: entities})
	}
	return PullResult{Frames: frames, Cursor: horizon, Mode: PullModeSnapshot}, nil
}
