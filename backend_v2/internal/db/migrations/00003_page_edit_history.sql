-- +goose Up
-- +goose StatementBegin

-- Page edit history (humans + agents). The collab server is the one place that
-- sees every edit — humans via their provider connection, agents via the bridge
-- — so it writes here. Edits are COALESCED per (page, editor) into a session
-- row: occurred_at = session start, ended_at = last edit, edits = change count.
-- A page "History" view lists these newest-first. See docs/page-edit-history.md.
CREATE TABLE page_edit_history (
    id          uuid        NOT NULL DEFAULT gen_random_uuid(),
    page_id     text        NOT NULL,
    editor_kind text        NOT NULL,                 -- 'human' | 'agent'
    editor_name text        NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    ended_at    timestamptz NOT NULL DEFAULT now(),
    edits       integer     NOT NULL DEFAULT 1,
    PRIMARY KEY (id)
);

CREATE INDEX idx_page_edit_history_page ON page_edit_history (page_id, occurred_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS page_edit_history;

-- +goose StatementEnd
