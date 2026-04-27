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

func (r *PostgresArtifactRepository) ListArtifacts(ctx context.Context, params artifactservice.ArtifactListParams) ([]artifactservice.ArtifactResponse, error) {
	if err := r.ensureDefaultUser(ctx); err != nil {
		return nil, err
	}
	query := `
		SELECT id, folder_id, type, title, content, summary, source_url, metadata, created_at, updated_at
		  FROM artifacts
		 WHERE user_id = $1::uuid
	`
	args := []any{r.userID}
	if params.FolderID == nil {
		query += ` AND folder_id IS NULL`
	} else {
		query += ` AND folder_id = $2`
		args = append(args, *params.FolderID)
	}
	query += ` ORDER BY updated_at DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
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
	if err := r.ensureDefaultUser(ctx); err != nil {
		return artifactservice.ArtifactResponse{}, err
	}
	row := r.pool.QueryRow(ctx, `
		SELECT id, folder_id, type, title, content, summary, source_url, metadata, created_at, updated_at
		  FROM artifacts
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, r.userID)
	return scanArtifactResponse(row)
}

func (r *PostgresArtifactRepository) CreateArtifact(ctx context.Context, rec artifactservice.ArtifactResponse) error {
	if err := r.ensureDefaultUser(ctx); err != nil {
		return err
	}
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
		    id, user_id, folder_id, type, title, content, summary, source_url, metadata, created_at, updated_at
		) VALUES (
		    $1, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11
		)
	`, rec.ID, r.userID, rec.FolderID, rec.Type, rec.Title, rec.Content, rec.Summary, rec.SourceURL, string(metadata), createdAt, updatedAt)
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
		   SET folder_id = COALESCE($3, folder_id),
		       type = COALESCE($4, type),
		       title = COALESCE($5, title),
		       content = COALESCE($6, content),
		       summary = COALESCE($7, summary),
		       source_url = COALESCE($8, source_url),
		       metadata = COALESCE($9::jsonb, metadata),
		       updated_at = now()
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, r.userID, patch.FolderID, patch.Type, patch.Title, patch.Content, patch.Summary, patch.SourceURL, metadataJSON)
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

func (r *PostgresArtifactRepository) ListFolders(ctx context.Context, parentID *string) ([]artifactservice.FolderNode, error) {
	if err := r.ensureDefaultUser(ctx); err != nil {
		return nil, err
	}
	query := `
		SELECT id, parent_id, title
		  FROM folders
		 WHERE user_id = $1::uuid
	`
	args := []any{r.userID}
	if parentID == nil {
		query += ` AND parent_id IS NULL`
	} else {
		query += ` AND parent_id = $2`
		args = append(args, *parentID)
	}
	query += ` ORDER BY title ASC`
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []artifactservice.FolderNode
	for rows.Next() {
		var node artifactservice.FolderNode
		if err := rows.Scan(&node.ID, &node.ParentID, &node.Title); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) CreateFolder(ctx context.Context, folder artifactservice.FolderNode) error {
	if err := r.ensureDefaultUser(ctx); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO folders (id, user_id, parent_id, title, created_at, updated_at)
		VALUES ($1, $2::uuid, $3, $4, now(), now())
	`, folder.ID, r.userID, folder.ParentID, folder.Title)
	return err
}

func (r *PostgresArtifactRepository) GetFolder(ctx context.Context, id string) (artifactservice.FolderNode, error) {
	if err := r.ensureDefaultUser(ctx); err != nil {
		return artifactservice.FolderNode{}, err
	}
	var node artifactservice.FolderNode
	err := r.pool.QueryRow(ctx, `
		SELECT id, parent_id, title
		  FROM folders
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, r.userID).Scan(&node.ID, &node.ParentID, &node.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifactservice.FolderNode{}, artifactservice.ErrNotFound
	}
	if err != nil {
		return artifactservice.FolderNode{}, err
	}
	return node, nil
}

func (r *PostgresArtifactRepository) FolderBreadcrumbs(ctx context.Context, id string) ([]artifactservice.BreadcrumbItem, error) {
	if err := r.ensureDefaultUser(ctx); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE chain AS (
		    SELECT id, parent_id, title, 0 AS depth
		      FROM folders
		     WHERE id = $1 AND user_id = $2::uuid
		    UNION ALL
		    SELECT f.id, f.parent_id, f.title, c.depth + 1
		      FROM folders f
		      JOIN chain c ON c.parent_id = f.id
		     WHERE f.user_id = $2::uuid
		)
		SELECT id, title
		  FROM chain
		 ORDER BY depth DESC
	`, id, r.userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []artifactservice.BreadcrumbItem{{ID: nil, Label: "Folders"}}
	for rows.Next() {
		var itemID string
		var label string
		if err := rows.Scan(&itemID, &label); err != nil {
			return nil, err
		}
		idCopy := itemID
		items = append(items, artifactservice.BreadcrumbItem{ID: &idCopy, Label: label})
	}
	return items, rows.Err()
}

func (r *PostgresArtifactRepository) ensureDefaultUser(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, created_at)
		VALUES ($1::uuid, 'dev@aladin.local', now())
		ON CONFLICT (id) DO NOTHING
	`, r.userID)
	return err
}

func scanArtifactResponse(row scanner) (artifactservice.ArtifactResponse, error) {
	var rec artifactservice.ArtifactResponse
	var metadata []byte
	var createdAt time.Time
	var updatedAt time.Time
	rec.Metadata = map[string]any{}
	err := row.Scan(
		&rec.ID,
		&rec.FolderID,
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
