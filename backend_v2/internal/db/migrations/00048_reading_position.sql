-- Reading position — "you are at page N of this document", synced across devices.
-- One row per (user, artifact); a per-key entity on the R1 sync spine (kind
-- "reading_position", entity id = the artifact id — the outbox is user-scoped so
-- no composite id is needed). Last-write-wins: every PUT bumps seq (the frame's
-- staleness guard) and stamps updated_at; there is no baseRevision guard because
-- "the page I most recently looked at" IS the right merge for two devices.
-- is_deleted is the standard tombstone (row KEPT) for spine symmetry; positions
-- are not user-deletable today.

-- +goose Up
CREATE TABLE reading_positions (
    user_id     uuid   NOT NULL,
    artifact_id text   NOT NULL,
    page        bigint NOT NULL DEFAULT 1,
    seq         bigint NOT NULL DEFAULT 0,
    is_deleted  boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, artifact_id)
);

-- +goose Down
DROP TABLE reading_positions;
