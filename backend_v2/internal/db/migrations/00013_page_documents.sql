-- +goose Up

CREATE TABLE IF NOT EXISTS page_documents (
    artifact_id TEXT PRIMARY KEY REFERENCES artifacts(id) ON DELETE CASCADE,
    markdown    TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO page_documents (artifact_id, markdown, created_at, updated_at)
SELECT id, content, created_at, updated_at
  FROM artifacts
 WHERE type = 'note'
ON CONFLICT (artifact_id) DO NOTHING;

UPDATE artifacts
   SET type = 'page'
 WHERE type = 'note';

-- +goose Down

UPDATE artifacts a
   SET type = 'note',
       content = COALESCE(p.markdown, a.content)
  FROM page_documents p
 WHERE a.id = p.artifact_id
   AND a.type = 'page';

DROP TABLE IF EXISTS page_documents;
