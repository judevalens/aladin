-- +goose Up
-- +goose StatementBegin

-- Block-level agent presence (v1): per-block attribution for a page's content.
-- A side map { blockId -> { by, at } } recording which agent last authored or
-- edited each block. Written by the collab bridge on each agent op (the bridge
-- is the only writer that knows "an agent did this"); human edits ride Yjs and
-- are not stamped. The client renders a per-block marker from this map. Kept
-- OUT of the Y.Doc / block schema on purpose (no BlockNote schema surgery, no
-- client/server version-drift gate) — see the Roadmap "Block-level agent
-- presence" note.
ALTER TABLE page_documents
    ADD COLUMN IF NOT EXISTS block_attribution jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE page_documents DROP COLUMN IF EXISTS block_attribution;

-- +goose StatementEnd
