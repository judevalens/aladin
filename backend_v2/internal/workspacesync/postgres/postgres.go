package postgres

import "github.com/jackc/pgx/v5/pgxpool"

// Data-layer R1 — server-authoritative sync (the generic outbox).
// Architecture: ~/.claude/plans/data-layer-offline-readable.md.
//
// SyncRepo is the durable-log access object. It implements workspacesync.OutboxReader
// (see outbox.go: PullSince/MinXid/Horizon) over the generic outbox_events
// table. The per-kind cold-start snapshot lives in sync_source.go; the producers
// Domain producers append frames through internal/outbox in their own write
// transactions.

type SyncRepo struct {
	pool *pgxpool.Pool
}

func NewSyncPostgres(pool *pgxpool.Pool) *SyncRepo {
	return &SyncRepo{pool: pool}
}
