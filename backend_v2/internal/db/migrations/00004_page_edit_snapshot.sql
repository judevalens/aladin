-- +goose Up
-- +goose StatementBegin

-- Page edit diffs (option 2): store the page's markdown at each history session's
-- close. A diff for an entry = its snapshot vs the previous entry's snapshot,
-- computed at view time on the client. Markdown is small + prunable (no Yjs
-- gc:false bloat). NULL for in-progress sessions / pre-feature rows.
ALTER TABLE page_edit_history
    ADD COLUMN IF NOT EXISTS markdown_snapshot text;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE page_edit_history DROP COLUMN IF EXISTS markdown_snapshot;

-- +goose StatementEnd
