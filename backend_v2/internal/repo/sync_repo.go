package repo

import (
	"context"

	"aladin/backend_v2/internal/outbox"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Data-layer R1 — server-authoritative sync (the generic outbox).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// SyncRepo is the durable-log access object. It implements service.OutboxReader
// (see outbox.go: PullSince/MinXid/Horizon) over the generic outbox_events
// table. The per-kind cold-start snapshot lives in sync_source.go; the producers
// (artifacts_postgres.go) append frames via appendOutboxEvent in their own write
// transactions. LockUser is shared write infrastructure reused by the producers.

type SyncRepo struct {
	pool *pgxpool.Pool
}

func NewSyncPostgres(pool *pgxpool.Pool) *SyncRepo {
	return &SyncRepo{pool: pool}
}

// LockUser serializes a user's writes so their outbox xids become visible in
// commit order. Postgres assigns an xid at first write, but transactions can
// COMMIT out of assignment order — without this a reader could observe a later
// xid before an earlier one commits and skip it. Call at the start of every
// write transaction for that user, before the canonical write + appendOutboxEvent.
// Released automatically at txn end (advisory_xact_lock).
func LockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	return outbox.LockUser(ctx, tx, userID)
}
