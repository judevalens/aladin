package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresArtifactRepository struct {
	pool   *pgxpool.Pool
	userID string
}

func NewArtifactsPostgres(pool *pgxpool.Pool, userID string) *PostgresArtifactRepository {
	return &PostgresArtifactRepository{pool: pool, userID: userID}
}

func (r *PostgresArtifactRepository) ListArtifacts(ctx context.Context) ([]artifactservice.ArtifactResponse, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, type, title, content, summary, source_url, metadata, created_at, updated_at
		  FROM artifacts
		 WHERE user_id = $1::uuid
		 ORDER BY updated_at DESC, created_at DESC
	`, r.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []artifactservice.ArtifactResponse
	for rows.Next() {
		rec, err := scanArtifactResponse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) GetArtifact(ctx context.Context, id string) (artifactservice.ArtifactResponse, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, type, title, content, summary, source_url, metadata, created_at, updated_at
		  FROM artifacts
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, r.userID)
	return scanArtifactResponse(row)
}

func (r *PostgresArtifactRepository) CreateArtifact(ctx context.Context, rec artifactservice.ArtifactResponse) error {
	createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		return err
	}
	updatedAt, err := time.Parse(time.RFC3339, rec.UpdatedAt)
	if err != nil {
		return err
	}
	metadata, _ := json.Marshal(rec.Metadata)
	_, err = r.pool.Exec(ctx, `
		INSERT INTO artifacts (
		    id, user_id, type, title, content, summary, source_url, metadata, created_at, updated_at
		) VALUES (
		    $1, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10
		)
	`, rec.ID, r.userID, rec.Type, rec.Title, rec.Content, rec.Summary, rec.SourceURL, string(metadata), createdAt, updatedAt)
	return err
}

func (r *PostgresArtifactRepository) UpdateArtifact(ctx context.Context, id string, patch artifactservice.ArtifactPatch) error {
	metadataJSON := any(nil)
	if patch.Metadata != nil {
		raw, _ := json.Marshal(*patch.Metadata)
		metadataJSON = string(raw)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE artifacts
		   SET type = COALESCE($3, type),
		       title = COALESCE($4, title),
		       content = COALESCE($5, content),
		       summary = COALESCE($6, summary),
		       source_url = COALESCE($7, source_url),
		       metadata = COALESCE($8::jsonb, metadata),
		       updated_at = now()
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, r.userID, patch.Type, patch.Title, patch.Content, patch.Summary, patch.SourceURL, metadataJSON)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) DeleteArtifact(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM artifacts
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, r.userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) FindDocument(ctx context.Context, id string) (artifactservice.DocumentRecord, bool, error) {
	var rec artifactservice.DocumentRecord
	var uploadedAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id, filename, title, size, page_count, uploaded_at, ingested
		  FROM documents
		 WHERE id = $1
	`, id).Scan(&rec.ID, &rec.Filename, &rec.Title, &rec.Size, &rec.PageCount, &uploadedAt, &rec.Ingested)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifactservice.DocumentRecord{}, false, nil
	}
	if err != nil {
		return artifactservice.DocumentRecord{}, false, err
	}
	rec.UploadedAt = uploadedAt.Format(time.RFC3339)
	return rec, true, nil
}

func (r *PostgresArtifactRepository) CreateDocument(ctx context.Context, rec artifactservice.DocumentRecord, uploadedAtUnix int64) error {
	uploadedAt := time.Unix(uploadedAtUnix, 0).UTC()
	var size any
	if rec.Size != nil {
		size = *rec.Size
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO documents (id, filename, title, size, page_count, uploaded_at, ingested, uploaded_by)
		VALUES ($1, $2, NULL, $3, NULL, $4, TRUE, $5::uuid)
	`, rec.ID, rec.Filename, size, uploadedAt, r.userID)
	return err
}

func (r *PostgresArtifactRepository) ListDocuments(ctx context.Context) ([]artifactservice.DocumentRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, filename, title, size, page_count, uploaded_at, ingested
		  FROM documents
		 ORDER BY uploaded_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []artifactservice.DocumentRecord
	for rows.Next() {
		var rec artifactservice.DocumentRecord
		var uploadedAt time.Time
		if err := rows.Scan(&rec.ID, &rec.Filename, &rec.Title, &rec.Size, &rec.PageCount, &uploadedAt, &rec.Ingested); err != nil {
			return nil, err
		}
		rec.UploadedAt = uploadedAt.Format(time.RFC3339)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) CreateNote(ctx context.Context, rec artifactservice.NoteRecord) error {
	createdAt, err := time.Parse(time.RFC3339, rec.CreatedAt)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO records (id, type, label, content, status, created_at)
		VALUES ($1, 'note', $2, $3, 'pending', $4)
	`, rec.ID, rec.Label, rec.Content, createdAt)
	return err
}

func (r *PostgresArtifactRepository) NoteExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM records WHERE id = $1 AND type = 'note')
	`, id).Scan(&exists)
	return exists, err
}

func (r *PostgresArtifactRepository) UpdateNote(ctx context.Context, id string, label *string, content *string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE records
		   SET label = COALESCE(NULLIF(BTRIM($2), ''), label),
		       content = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN BTRIM($3)
		           ELSE content
		       END,
		       status = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN 'pending'
		           ELSE status
		       END,
		       enrichment = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN NULL
		           ELSE enrichment
		       END,
		       embedding = CASE
		           WHEN $3 IS NOT NULL AND BTRIM($3) <> content THEN NULL
		           ELSE embedding
		  END
		 WHERE id = $1 AND type = 'note'
	`, id, derefString(label), derefString(content))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) DeleteNote(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM records WHERE id = $1 AND type = 'note'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) FetchNote(ctx context.Context, id string) (artifactservice.NoteRecord, error) {
	var rec artifactservice.NoteRecord
	var enrichment []byte
	var metadata []byte
	var createdAt time.Time
	rec.Metadata = map[string]any{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, type, label, content, enrichment, metadata, status, created_at
		  FROM records
		 WHERE id = $1 AND type = 'note'
	`, id).Scan(&rec.ID, &rec.Type, &rec.Label, &rec.Content, &enrichment, &metadata, &rec.Status, &createdAt)
	if err != nil {
		return rec, err
	}
	if len(enrichment) > 0 {
		_ = json.Unmarshal(enrichment, &rec.Enrichment)
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &rec.Metadata)
	}
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	return rec, nil
}

func (r *PostgresArtifactRepository) HasNoteEmbedding(ctx context.Context, id string) (bool, error) {
	var hasEmbedding bool
	err := r.pool.QueryRow(ctx, `
		SELECT embedding IS NOT NULL FROM records WHERE id = $1
	`, id).Scan(&hasEmbedding)
	return hasEmbedding, err
}

func (r *PostgresArtifactRepository) RelatedNotes(ctx context.Context, id string, limit int) ([]artifactservice.RelatedRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		    a.id, a.type, a.label, LEFT(COALESCE(a.content, ''), 300) AS content,
		    a.source_url, a.enrichment, a.created_at,
		    1 - (a.embedding <=> (SELECT embedding FROM records WHERE id = $1)) AS similarity
		  FROM records a
		 WHERE a.embedding IS NOT NULL
		   AND a.status != 'superseded'
		   AND a.id != $1
		 ORDER BY a.embedding <=> (SELECT embedding FROM records WHERE id = $1)
		 LIMIT $2
	`, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []artifactservice.RelatedRecord
	for rows.Next() {
		var item artifactservice.RelatedRecord
		var enrichment []byte
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Type, &item.Label, &item.Content, &item.SourceURL, &enrichment, &createdAt, &item.Similarity); err != nil {
			return nil, err
		}
		if len(enrichment) > 0 {
			_ = json.Unmarshal(enrichment, &item.Enrichment)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		results = append(results, item)
	}
	return results, rows.Err()
}

func scanArtifactResponse(row scanner) (artifactservice.ArtifactResponse, error) {
	var rec artifactservice.ArtifactResponse
	var metadata []byte
	var createdAt time.Time
	var updatedAt time.Time
	rec.Metadata = map[string]any{}
	err := row.Scan(
		&rec.ID,
		&rec.Type,
		&rec.Title,
		&rec.Content,
		&rec.Summary,
		&rec.SourceURL,
		&metadata,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rec, artifactservice.ErrNotFound
		}
		return rec, err
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &rec.Metadata)
	}
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	rec.UpdatedAt = updatedAt.Format(time.RFC3339)
	return rec, nil
}

func derefString(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}
