-- +goose Up

ALTER TABLE page_documents
    ADD COLUMN IF NOT EXISTS revision BIGINT NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE page_documents
    DROP COLUMN IF EXISTS revision;
