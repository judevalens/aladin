-- +goose Up
-- +goose StatementBegin

-- Data-layer redesign, Phase 0 — server backbone for the metadata sync engine.
-- Plan: ~/.claude/plans/data-layer-sync-model.md (Offline LWW + Intent Log).
--
-- This is ADDITIVE: it does not touch artifacts/tree_nodes. The workspace wipe
-- ("test workspace, no historical backfill") is a separate cutover step run
-- when Phase A is ready to repopulate — NOT here, so existing Yjs collab pages
-- survive while the new path is built.
--
-- Model recap (so this schema reads in context):
--  * Server-authoritative, unconditional-accept, per-field LWW. Arrival-at-
--    server order is the conflict policy.
--  * workspace_changes is the per-user append-only change feed. Its BIGSERIAL
--    `seq` is BOTH the reconnect cursor (global, filtered per user) and the
--    per-(entity,field) newest-wins comparator source the client stores.
--  * A change is field-granular for updates (carries field + value), and
--    entity-level for create/delete. Realtime ships these rows verbatim (events
--    carry data); pull returns them since the client's cursor.
--  * sync_clients is the idempotency ledger: per device-install client_id, the
--    high-water of that client's monotonic mutation counter. Dedup = "apply iff
--    counter > last_applied" (so a retried write — same clientWriteId — is a
--    no-op; review F1).

CREATE TABLE workspace_changes (
    seq         BIGSERIAL PRIMARY KEY,        -- global monotonic; cursor + comparator
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_kind TEXT NOT NULL,                -- 'node' | 'artifact' | 'edge' | ...
    entity_id   TEXT NOT NULL,
    op          TEXT NOT NULL CHECK (op IN ('create', 'update', 'delete')),
    field       TEXT,                         -- set iff op='update'
    value       JSONB,                        -- the field's new value iff op='update'
    -- "client_id:counter" of the originating mutation; NULL if server-originated.
    -- Carried for trace/echo-attribution; dedup high-water lives in sync_clients.
    mutation_id TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (op = 'update' AND field IS NOT NULL)
        OR (op IN ('create', 'delete') AND field IS NULL)
    )
);

-- NOTE: deliberately NO foreign key from workspace_changes to artifacts/
-- tree_nodes. The feed is an independent append-only log — a delete event must
-- survive the entity row's removal, so it can never cascade-delete its own
-- history.

-- Pull / reconnect resume read pattern: WHERE user_id = $1 AND seq > $cursor
-- ORDER BY seq.
CREATE INDEX idx_workspace_changes_user_seq ON workspace_changes (user_id, seq);

-- Per-client idempotency ledger (review P0-2: multi-device identity).
CREATE TABLE sync_clients (
    client_id    TEXT PRIMARY KEY,            -- stable per device install
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_applied BIGINT NOT NULL DEFAULT 0,   -- high-water of this client's mutation counter
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sync_clients_user ON sync_clients (user_id);

-- WRITE-PATH REQUIREMENT (enforced in Go, not DDL — recorded here so it isn't
-- lost): BIGSERIAL assigns `seq` at INSERT, but transactions can COMMIT out of
-- assignment order, so a reader could see seq=N+1 before seq=N commits and then
-- skip N forever (silent data loss). Every write for a user MUST run under
-- `pg_advisory_xact_lock(hashtext(user_id::text))` so that user's writes
-- serialize and their seqs become visible in order. The entity-table write and
-- the workspace_changes insert MUST share one transaction (atomic seq; review
-- F3). Snapshots (pull fallback) must be read at a consistent seq S so the
-- snapshot value equals the max-seq value.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS sync_clients;
DROP TABLE IF EXISTS workspace_changes;

-- +goose StatementEnd
