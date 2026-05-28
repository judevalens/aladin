-- +goose Up
-- +goose StatementBegin

-- M8: Collaborative page editing via Yjs + Hocuspocus.
--
-- page_ydoc holds the canonical Y.Doc state, one row per page. Hocuspocus's
-- @hocuspocus/extension-database store callback upserts the full encoded
-- Y.Doc here (debounced). This is the durable server-side replica; connected
-- clients (+ y-indexeddb) are the others.
--
-- page_documents.blocks stays as a JSON *projection* (search, list/search MCP
-- tools, exports), refreshed from the live Y.Doc by Hocuspocus's onChange
-- hook. last_collab_commit_at records when that projection last ran.
--
-- Test workspace: wipe existing page data rather than migrate. TRUNCATE on
-- artifacts cascades to page_documents, page_ydoc, and tree_nodes via their
-- ON DELETE CASCADE references.

TRUNCATE TABLE artifacts RESTART IDENTITY CASCADE;

CREATE TABLE page_ydoc (
    page_id    TEXT PRIMARY KEY REFERENCES artifacts(id) ON DELETE CASCADE,
    state      BYTEA NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE page_documents
    ADD COLUMN last_collab_commit_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE page_documents DROP COLUMN IF EXISTS last_collab_commit_at;
DROP TABLE IF EXISTS page_ydoc;

-- +goose StatementEnd
