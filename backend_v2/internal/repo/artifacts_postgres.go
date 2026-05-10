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
	pool *pgxpool.Pool
}

func NewArtifactsPostgres(pool *pgxpool.Pool) *PostgresArtifactRepository {
	return &PostgresArtifactRepository{pool: pool}
}

func (r *PostgresArtifactRepository) ListArtifacts(ctx context.Context, params artifactservice.ArtifactListParams) ([]artifactservice.ArtifactResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT a.id, n.parent_id, a.type, a.title,
		       CASE WHEN a.type = 'page' THEN COALESCE(p.markdown, a.content) ELSE a.content END AS content,
		       a.summary, a.source_url, a.metadata, a.created_at, a.updated_at,
		       COALESCE(p.revision, 0) AS revision
		  FROM artifacts a
		  JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  LEFT JOIN page_documents p ON p.artifact_id = a.id
		 WHERE a.user_id = $1::uuid
	`
	args := []any{userID}
	if params.FolderID == nil {
		query += ` AND n.parent_id IS NULL`
	} else {
		query += ` AND n.parent_id = $2`
		args = append(args, *params.FolderID)
	}
	query += ` ORDER BY n.position ASC, a.created_at ASC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifactservice.ArtifactResponse, 0)
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
	userID, err := r.userID(ctx)
	if err != nil {
		return artifactservice.ArtifactResponse{}, err
	}
	row := r.pool.QueryRow(ctx, `
		SELECT a.id, n.parent_id, a.type, a.title,
		       CASE WHEN a.type = 'page' THEN COALESCE(p.markdown, a.content) ELSE a.content END AS content,
		       a.summary, a.source_url, a.metadata, a.created_at, a.updated_at,
		       COALESCE(p.revision, 0) AS revision
		  FROM artifacts a
		  LEFT JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  LEFT JOIN page_documents p ON p.artifact_id = a.id
		 WHERE a.id = $1 AND a.user_id = $2::uuid
	`, id, userID)
	return scanArtifactResponse(row)
}

func (r *PostgresArtifactRepository) CreateArtifact(ctx context.Context, rec artifactservice.ArtifactResponse) error {
	userID, err := r.userID(ctx)
	if err != nil {
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
		    id, user_id, type, title, content, summary, source_url, metadata, created_at, updated_at
		) VALUES (
		    $1, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10
		)
	`, rec.ID, userID, rec.Type, rec.Title, rec.Content, rec.Summary, rec.SourceURL, string(metadata), createdAt, updatedAt)
	return err
}

func (r *PostgresArtifactRepository) UpdateArtifact(ctx context.Context, id string, patch artifactservice.ArtifactPatch) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
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
	`, id, userID, patch.Type, patch.Title, patch.Content, patch.Summary, patch.SourceURL, metadataJSON)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) CreatePageDocument(ctx context.Context, artifactID string, markdown string) error {
	if _, err := r.userID(ctx); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO page_documents (artifact_id, markdown, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (artifact_id) DO NOTHING
	`, artifactID, markdown)
	return err
}

func (r *PostgresArtifactRepository) SavePageDocument(ctx context.Context, artifactID string, markdown string) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		INSERT INTO page_documents (artifact_id, markdown, created_at, updated_at)
		VALUES ($1, $2, now(), now())
		ON CONFLICT (artifact_id)
		DO UPDATE SET markdown = EXCLUDED.markdown, updated_at = now()
	`, artifactID, markdown); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE artifacts
		   SET content = $3,
		       updated_at = now()
		 WHERE id = $1 AND user_id = $2::uuid
	`, artifactID, userID, markdown)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}

	return tx.Commit(ctx)
}

func (r *PostgresArtifactRepository) SavePageDocumentRevision(ctx context.Context, artifactID string, markdown string, revision int64) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	tag, err := tx.Exec(ctx, `
		UPDATE page_documents
		   SET markdown = $2,
		       revision = $3,
		       updated_at = now()
		 WHERE artifact_id = $1
		   AND revision < $3
	`, artifactID, markdown, revision)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				  FROM artifacts a
				  JOIN page_documents p ON p.artifact_id = a.id
				 WHERE a.id = $1
				   AND a.user_id = $2::uuid
				   AND a.type = 'page'
			)
		`, artifactID, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return artifactservice.ErrNotFound
		}
		return artifactservice.ErrConflict
	}

	tag, err = tx.Exec(ctx, `
		UPDATE artifacts
		   SET content = $3,
		       updated_at = now()
		 WHERE id = $1
		   AND user_id = $2::uuid
		   AND type = 'page'
	`, artifactID, userID, markdown)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}

	return tx.Commit(ctx)
}

func (r *PostgresArtifactRepository) DeleteArtifact(ctx context.Context, id string) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM artifacts
		 WHERE id = $1 AND user_id = $2::uuid
	`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) ListFolders(ctx context.Context, parentID *string) ([]artifactservice.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE user_id = $1::uuid
		   AND kind = 'folder'
	`
	args := []any{userID}
	if parentID == nil {
		query += ` AND parent_id IS NULL`
	} else {
		query += ` AND parent_id = $2`
		args = append(args, *parentID)
	}
	query += ` ORDER BY position ASC, created_at ASC`
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifactservice.FolderNode, 0)
	for rows.Next() {
		var node artifactservice.FolderNode
		if err := rows.Scan(&node.ID, &node.ParentID, &node.Title); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) ListAllFolders(ctx context.Context) ([]artifactservice.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE user_id = $1::uuid
		   AND kind = 'folder'
		 ORDER BY position ASC, created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifactservice.FolderNode, 0)
	for rows.Next() {
		var node artifactservice.FolderNode
		if err := rows.Scan(&node.ID, &node.ParentID, &node.Title); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) ListAllBrowserNodes(ctx context.Context) ([]artifactservice.BrowserTreeFlatNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			n.id,
			n.parent_id,
			n.kind,
			CASE
				WHEN n.kind = 'folder' THEN n.title
				ELSE COALESCE(NULLIF(a.title, ''), a.metadata->>'originalFilename', a.metadata->>'storageKey', 'Untitled artifact')
			END AS title,
			n.artifact_id,
			a.type,
			CASE WHEN a.updated_at IS NULL THEN NULL ELSE to_char(a.updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') END AS updated_at,
			n.position
		  FROM tree_nodes n
		  LEFT JOIN artifacts a ON a.id = n.artifact_id AND a.user_id = n.user_id
		 WHERE n.user_id = $1::uuid
		 ORDER BY COALESCE(n.parent_id, ''), n.position ASC, n.created_at ASC, n.id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifactservice.BrowserTreeFlatNode, 0)
	for rows.Next() {
		var node artifactservice.BrowserTreeFlatNode
		if err := rows.Scan(
			&node.ID,
			&node.ParentID,
			&node.Kind,
			&node.Title,
			&node.ArtifactID,
			&node.ArtifactType,
			&node.UpdatedAt,
			&node.Position,
		); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) NextNodePosition(ctx context.Context, parentID *string) (int64, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return 0, err
	}
	var next int64
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), 0) + 1
		  FROM tree_nodes
		 WHERE user_id = $1::uuid
		   AND parent_id IS NOT DISTINCT FROM $2
	`, userID, parentID).Scan(&next)
	return next, err
}

func (r *PostgresArtifactRepository) CreateTreeNode(ctx context.Context, node artifactservice.TreeNodeRecord) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, now(), now())
	`, node.ID, userID, node.ParentID, node.Kind, node.Title, node.ArtifactID, node.Position)
	return err
}

func (r *PostgresArtifactRepository) UpdateArtifactNodeParent(ctx context.Context, artifactID string, parentID *string) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	position, err := r.NextNodePosition(ctx, parentID)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE tree_nodes
		   SET parent_id = $3,
		       position = $4,
		       updated_at = now()
		 WHERE id = $1
		   AND user_id = $2::uuid
		   AND kind = 'artifact'
	`, artifactID, userID, parentID, position)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) UpdateFolderTitle(ctx context.Context, id string, title string) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE tree_nodes
		   SET title = $3,
		       updated_at = now()
		 WHERE id = $1
		   AND user_id = $2::uuid
		   AND kind = 'folder'
	`, id, userID, title)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.ErrNotFound
	}
	return nil
}

func (r *PostgresArtifactRepository) GetFolder(ctx context.Context, id string) (artifactservice.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifactservice.FolderNode{}, err
	}
	var node artifactservice.FolderNode
	err = r.pool.QueryRow(ctx, `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE id = $1 AND user_id = $2::uuid
		   AND kind = 'folder'
	`, id, userID).Scan(&node.ID, &node.ParentID, &node.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifactservice.FolderNode{}, artifactservice.ErrNotFound
	}
	if err != nil {
		return artifactservice.FolderNode{}, err
	}
	return node, nil
}

func (r *PostgresArtifactRepository) FolderBreadcrumbs(ctx context.Context, id string) ([]artifactservice.BreadcrumbItem, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE chain AS (
		    SELECT id, parent_id, title, 0 AS depth
		      FROM tree_nodes
		     WHERE id = $1 AND user_id = $2::uuid AND kind = 'folder'
		    UNION ALL
		    SELECT f.id, f.parent_id, f.title, c.depth + 1
		      FROM tree_nodes f
		      JOIN chain c ON c.parent_id = f.id
		     WHERE f.user_id = $2::uuid AND f.kind = 'folder'
		)
		SELECT id, title
		  FROM chain
		 ORDER BY depth DESC
	`, id, userID)
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

func (r *PostgresArtifactRepository) userID(ctx context.Context) (string, error) {
	principal, err := artifactservice.RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return principal.UserID, nil
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
		&rec.Revision,
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
