package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"aladin/backend_v2/internal/apperror"
	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/auth"
	"aladin/backend_v2/internal/outbox"
	"aladin/backend_v2/internal/page"
	"aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/treesync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type scanner interface {
	Scan(dest ...any) error
}

type PostgresArtifactRepository struct {
	pool *pgxpool.Pool
}

func NewArtifactsPostgres(pool *pgxpool.Pool) *PostgresArtifactRepository {
	return &PostgresArtifactRepository{pool: pool}
}

// withUserTx runs fn in a transaction that holds the per-user advisory lock, so
// the outbox event appended inside gets an xid visible in commit order (the
// canonical write and its frame commit atomically — the transactional outbox).
func (r *PostgresArtifactRepository) withUserTx(ctx context.Context, fn func(tx pgx.Tx, userID string) error) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if err := fn(tx, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresArtifactRepository) ListArtifacts(ctx context.Context, params artifact.ArtifactListParams) ([]artifact.ArtifactResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT a.id, n.parent_id, a.type, a.title,
		       a.content,
		       p.blocks,
		       a.summary, a.source_url, a.metadata, a.created_at, a.updated_at,
		       COALESCE(p.revision, 0) AS revision
		  FROM artifacts a
		  JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  LEFT JOIN page_documents p ON p.artifact_id = a.id
		 WHERE a.user_id = $1::uuid
		   AND n.is_deleted = false
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

	out := make([]artifact.ArtifactResponse, 0)
	for rows.Next() {
		rec, err := scanArtifactResponse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) SearchPageArtifacts(ctx context.Context, params artifact.PageSearchParams) ([]artifact.ArtifactResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	query := "%" + params.Query + "%"
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, n.parent_id, a.type, a.title,
		       a.content,
		       p.blocks,
		       a.summary, a.source_url, a.metadata, a.created_at, a.updated_at,
		       COALESCE(p.revision, 0) AS revision
		  FROM artifacts a
		  LEFT JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  LEFT JOIN page_documents p ON p.artifact_id = a.id
		 WHERE a.user_id = $1::uuid
		   AND COALESCE(n.is_deleted, false) = false
		   AND a.type = 'page'
		   AND (
		       a.title ILIKE $2
		       OR COALESCE(a.summary, '') ILIKE $2
		       OR COALESCE(p.search_text, '') ILIKE $2
		   )
		 ORDER BY a.updated_at DESC, a.created_at DESC
		 LIMIT $3
	`, userID, query, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifact.ArtifactResponse, 0)
	for rows.Next() {
		rec, err := scanArtifactResponse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) GetArtifact(ctx context.Context, id string) (artifact.ArtifactResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifact.ArtifactResponse{}, err
	}
	row := r.pool.QueryRow(ctx, `
		SELECT a.id, n.parent_id, a.type, a.title,
		       a.content,
		       p.blocks,
		       a.summary, a.source_url, a.metadata, a.created_at, a.updated_at,
		       COALESCE(p.revision, 0) AS revision
		  FROM artifacts a
		  LEFT JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  LEFT JOIN page_documents p ON p.artifact_id = a.id
		 WHERE a.id = $1 AND a.user_id = $2::uuid
		   AND COALESCE(n.is_deleted, false) = false
	`, id, userID)
	return scanArtifactResponse(row)
}

// LightNode reads a node's current light representation (joined to its artifact),
// INCLUDING tombstones, with its seq — the model a write returns so the client can
// apply the result under the same seq guard the WS frame uses. No Frame type leaks
// into the REST layer; this is a plain resource representation + its version.
func (r *PostgresArtifactRepository) LightNode(ctx context.Context, id string) (artifact.BrowserNodeResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifact.BrowserNodeResponse{}, err
	}
	var (
		nodeID, kind string
		parentID     *string
		position     int64
		title        *string
		aType        *string
		sourceURL    *string
		summary      *string // projected but unused here; must still be scanned
		metadata     []byte
		runState     *string // research extension columns; unused here but must be scanned
		execMode     *string
		sourceKind   *string
		seq          int64
		isDeleted    bool
	)
	// lightEntitySelect projects 14 columns (…, source_url, summary, metadata,
	// run_state, exec_mode, source_kind, seq, is_deleted); scan them ALL. Dropping any
	// is a pgx "number of field descriptions must equal number of destinations" panic at
	// runtime, not a compile error — TestLightNodeScansAllColumns is the guard.
	err = r.pool.QueryRow(ctx, treesync.LightEntitySelect+` AND n.id = $2`, userID, id).
		Scan(&nodeID, &kind, &parentID, &position, &title, &aType, &sourceURL, &summary, &metadata,
			&runState, &execMode, &sourceKind, &seq, &isDeleted)
	_ = summary
	_ = metadata
	_ = runState
	_ = execMode
	_ = sourceKind
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return artifact.BrowserNodeResponse{}, apperror.ErrNotFound
		}
		return artifact.BrowserNodeResponse{}, fmt.Errorf("light node %s: %w", id, err)
	}
	t := ""
	if title != nil {
		t = *title
	}
	var artifactID *string
	if kind == "artifact" {
		artifactID = &nodeID
	}
	return artifact.BrowserNodeResponse{
		ID:         nodeID,
		ParentID:   parentID,
		Kind:       kind,
		Title:      t,
		ArtifactID: artifactID,
		Position:   position,
		Seq:        uint64(seq),
	}, nil
}

func (r *PostgresArtifactRepository) CreateArtifactGraph(ctx context.Context, rec artifact.ArtifactResponse, node artifact.TreeNodeRecord, pageBlocks json.RawMessage, pageSearchText string) error {
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

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO artifacts (
		    id, user_id, type, title, content, summary, source_url, metadata, created_at, updated_at
		) VALUES (
		    $1, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10
		)
	`, rec.ID, userID, rec.Type, rec.Title, rec.Content, rec.Summary, rec.SourceURL, string(metadata), createdAt, updatedAt); err != nil {
		return err
	}

	if pageBlocks != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO page_documents (artifact_id, blocks, search_text, created_at, updated_at)
			VALUES ($1, $2::jsonb, $3, now(), now())
			ON CONFLICT (artifact_id) DO NOTHING
		`, rec.ID, string(pageBlocks), pageSearchText); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, now(), now())
	`, node.ID, userID, node.ParentID, node.Kind, node.Title, node.ArtifactID, node.Position); err != nil {
		return err
	}

	// Bump the entity version + append one frame describing the new entity, in
	// the same commit as the canonical write (transactional outbox).
	if err := treesync.EmitNodeUpsert(ctx, tx, userID, node.ID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresArtifactRepository) UpdateArtifactGraph(ctx context.Context, id string, patch artifact.ArtifactPatch) error {
	metadataJSON := any(nil)
	if patch.Metadata != nil {
		raw, err := json.Marshal(*patch.Metadata)
		if err != nil {
			return fmt.Errorf("marshal artifact metadata: %w", err)
		}
		metadataJSON = string(raw)
	}
	return r.withUserTx(ctx, func(tx pgx.Tx, userID string) error {
		if patch.FolderID != nil {
			var position int64
			if err := tx.QueryRow(ctx, `
				SELECT COALESCE(MAX(position), 0) + 1
				  FROM tree_nodes
				 WHERE user_id = $1::uuid
				   AND parent_id IS NOT DISTINCT FROM $2
				   AND is_deleted = false
			`, userID, patch.FolderID).Scan(&position); err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `
				UPDATE tree_nodes
				   SET parent_id = $3, position = $4, updated_at = now()
				 WHERE id = $1 AND user_id = $2::uuid AND kind = 'artifact'
			`, id, userID, patch.FolderID, position)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return apperror.ErrNotFound
			}
			if err := treesync.EmitNodeUpsert(ctx, tx, userID, id); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE artifacts
			   SET type = COALESCE($3, type),
			       title = COALESCE($4, title),
			       content = COALESCE($5, content),
			       summary = COALESCE($6, summary),
			       source_url = COALESCE($7, source_url),
			       -- Shallow-merge metadata (top-level keys) so a partial patch — e.g.
			       -- {"properties": …} — preserves existing keys like storageKey/mimeType.
			       metadata = CASE WHEN $8::jsonb IS NULL THEN metadata
			                       ELSE COALESCE(metadata, '{}'::jsonb) || $8::jsonb END,
			       updated_at = now()
			 WHERE id = $1 AND user_id = $2::uuid
		`, id, userID, patch.Type, patch.Title, patch.Content, patch.Summary, patch.SourceURL, metadataJSON)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperror.ErrNotFound
		}
		// One frame for the updated entity (light fields; the seq guard makes the
		// client apply it iff newer).
		return treesync.EmitNodeUpsert(ctx, tx, userID, id)
	})
}

// PageBlockAttribution returns the raw block_attribution JSON for a page (the
// side map {blockId: {by, at}} the collab bridge stamps on agent edits). Returns
// "{}" when the page has no document row or no attribution. User-scoped.
func (r *PostgresArtifactRepository) PageBlockAttribution(ctx context.Context, id string) (json.RawMessage, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = r.pool.QueryRow(ctx, `
		SELECT pd.block_attribution
		  FROM page_documents pd
		  JOIN artifacts a ON a.id = pd.artifact_id
		 WHERE pd.artifact_id = $1 AND a.user_id = $2::uuid AND a.type = 'page'
	`, id, userID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return json.RawMessage("{}"), nil
		}
		return nil, fmt.Errorf("page block attribution %s: %w", id, err)
	}
	return json.RawMessage(raw), nil
}

// PageEditHistory returns a page's coalesced edit sessions (humans + agents),
// newest first. User-scoped via the artifacts join.
func (r *PostgresArtifactRepository) PageEditHistory(ctx context.Context, id string) ([]page.PageEditEntry, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT h.id::text, h.editor_kind, h.editor_name, h.occurred_at, h.ended_at, h.edits
		  FROM page_edit_history h
		  JOIN artifacts a ON a.id = h.page_id
		 WHERE h.page_id = $1 AND a.user_id = $2::uuid AND a.type = 'page'
		 ORDER BY h.occurred_at DESC
		 LIMIT 200
	`, id, userID)
	if err != nil {
		return nil, fmt.Errorf("page edit history %s: %w", id, err)
	}
	defer rows.Close()
	out := make([]page.PageEditEntry, 0)
	for rows.Next() {
		var e page.PageEditEntry
		var occurred, ended time.Time
		if err := rows.Scan(&e.ID, &e.EditorKind, &e.EditorName, &occurred, &ended, &e.Edits); err != nil {
			return nil, fmt.Errorf("scan page edit history: %w", err)
		}
		e.OccurredAt = occurred.UTC().Format(time.RFC3339)
		e.EndedAt = ended.UTC().Format(time.RFC3339)
		out = append(out, e)
	}
	return out, rows.Err()
}

// PageEditDiff returns the before/after markdown around a history entry: the
// entry's own snapshot (after) and the previous entry's snapshot (before).
// User-scoped via the entry → page → artifact join; missing snapshots read as "".
func (r *PostgresArtifactRepository) PageEditDiff(ctx context.Context, entryID string) (page.PageDiff, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return page.PageDiff{}, err
	}
	var pageID string
	var occurred time.Time
	var after *string
	err = r.pool.QueryRow(ctx, `
		SELECT h.page_id, h.occurred_at, h.markdown_snapshot
		  FROM page_edit_history h
		  JOIN artifacts a ON a.id = h.page_id
		 WHERE h.id = $1::uuid AND a.user_id = $2::uuid AND a.type = 'page'
	`, entryID, userID).Scan(&pageID, &occurred, &after)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return page.PageDiff{}, apperror.ErrNotFound
		}
		return page.PageDiff{}, fmt.Errorf("page edit diff %s: %w", entryID, err)
	}
	var before *string
	err = r.pool.QueryRow(ctx, `
		SELECT markdown_snapshot
		  FROM page_edit_history
		 WHERE page_id = $1 AND occurred_at < $2 AND markdown_snapshot IS NOT NULL
		 ORDER BY occurred_at DESC
		 LIMIT 1
	`, pageID, occurred).Scan(&before)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return page.PageDiff{}, fmt.Errorf("page edit diff prev %s: %w", entryID, err)
	}
	deref := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	return page.PageDiff{Before: deref(before), After: deref(after)}, nil
}

// SavePageBlocks is the single mutation point for a page's block document.
// expectedRev=0 disables the optimistic check (last-write-wins);
// expectedRev>0 enforces that the stored revision is strictly less than
// expectedRev and returns ErrConflict otherwise. Returns the new revision.
func (r *PostgresArtifactRepository) SavePageBlocks(ctx context.Context, artifactID string, blocks json.RawMessage, searchText string, expectedRev int64) (int64, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return 0, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var newRev int64
	if expectedRev > 0 {
		err = tx.QueryRow(ctx, `
			UPDATE page_documents
			   SET blocks = $2::jsonb,
			       search_text = $3,
			       revision = $4,
			       updated_at = now()
			 WHERE artifact_id = $1
			   AND revision < $4
			RETURNING revision
		`, artifactID, string(blocks), searchText, expectedRev).Scan(&newRev)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
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
					return 0, err
				}
				if !exists {
					return 0, apperror.ErrNotFound
				}
				return 0, apperror.ErrConflict
			}
			return 0, err
		}
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE page_documents
			   SET blocks = $2::jsonb,
			       search_text = $3,
			       revision = revision + 1,
			       updated_at = now()
			 WHERE artifact_id = $1
			RETURNING revision
		`, artifactID, string(blocks), searchText).Scan(&newRev)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, apperror.ErrNotFound
			}
			return 0, err
		}
	}

	tag, err := tx.Exec(ctx, `
		UPDATE artifacts
		   SET updated_at = now()
		 WHERE id = $1
		   AND user_id = $2::uuid
		   AND type = 'page'
	`, artifactID, userID)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, apperror.ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return newRev, nil
}

// GetPageBlocks returns the current blocks JSON + revision for a page.
// Implements PageDocumentStore.
func (r *PostgresArtifactRepository) GetPageBlocks(ctx context.Context, artifactID string) (json.RawMessage, int64, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, 0, err
	}
	var blocks []byte
	var revision int64
	err = r.pool.QueryRow(ctx, `
		SELECT p.blocks, p.revision
		  FROM page_documents p
		  JOIN artifacts a ON a.id = p.artifact_id
		 WHERE p.artifact_id = $1
		   AND a.user_id = $2::uuid
		   AND a.type = 'page'
	`, artifactID, userID).Scan(&blocks, &revision)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, apperror.ErrNotFound
		}
		return nil, 0, err
	}
	return json.RawMessage(blocks), revision, nil
}

// --- You-stream ingestion (Y1) ------------------------------------------------------------
// System-scoped page reads for the background ingest worker. Unlike the user-facing page
// methods above, these run with NO authenticated principal (the worker is a system process),
// so they key on artifact_id and return the owner from the row.

// PageIngestCandidate is a page the idle-sweep found due for ingest.
type PageIngestCandidate struct {
	ArtifactID string
	Revision   int64
}

// PageIngestSnapshot is the live page state the ingest worker reads at run time.
type PageIngestSnapshot struct {
	ArtifactID   string
	OwnerID      string
	Revision     int64
	LastIngested int64
	UpdatedAt    time.Time
	Text         string
}

// ListPagesDueForIngest returns pages whose content moved past their last ingest and which
// have been idle (no edits) for at least idleFor — the ambient-sweep candidate set. Ordered
// oldest-edit-first, capped at limit. System-scoped (all users).
func (r *PostgresArtifactRepository) ListPagesDueForIngest(ctx context.Context, idleFor time.Duration, limit int) ([]PageIngestCandidate, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.artifact_id, p.revision
		  FROM page_documents p
		  JOIN artifacts a ON a.id = p.artifact_id AND a.type = 'page'
		 WHERE p.revision > p.last_ingested_revision
		   AND p.updated_at < now() - make_interval(secs => $1)
		 ORDER BY p.updated_at ASC
		 LIMIT $2
	`, idleFor.Seconds(), limit)
	if err != nil {
		return nil, fmt.Errorf("list pages due for ingest: %w", err)
	}
	defer rows.Close()
	out := []PageIngestCandidate{}
	for rows.Next() {
		var c PageIngestCandidate
		if err := rows.Scan(&c.ArtifactID, &c.Revision); err != nil {
			return nil, fmt.Errorf("scan page ingest candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetPageIngestSnapshot reads a page's live state for the ingest worker (system-scoped).
// Returns ErrNotFound if the page/document is missing.
func (r *PostgresArtifactRepository) GetPageIngestSnapshot(ctx context.Context, artifactID string) (PageIngestSnapshot, error) {
	s := PageIngestSnapshot{ArtifactID: artifactID}
	err := r.pool.QueryRow(ctx, `
		SELECT a.user_id::text, p.revision, p.last_ingested_revision, p.updated_at, p.search_text
		  FROM page_documents p
		  JOIN artifacts a ON a.id = p.artifact_id AND a.type = 'page'
		 WHERE p.artifact_id = $1
	`, artifactID).Scan(&s.OwnerID, &s.Revision, &s.LastIngested, &s.UpdatedAt, &s.Text)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PageIngestSnapshot{}, apperror.ErrNotFound
		}
		return PageIngestSnapshot{}, fmt.Errorf("get page ingest snapshot: %w", err)
	}
	return s, nil
}

// MarkPageIngested advances last_ingested_revision (system-scoped, monotonic). It MUST NOT
// touch updated_at — that reflects edit time and drives the idle window; bumping it here
// would re-trigger the sweep forever.
func (r *PostgresArtifactRepository) MarkPageIngested(ctx context.Context, artifactID string, revision int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE page_documents
		   SET last_ingested_revision = $2
		 WHERE artifact_id = $1 AND last_ingested_revision < $2
	`, artifactID, revision)
	if err != nil {
		return fmt.Errorf("mark page ingested: %w", err)
	}
	return nil
}

func (r *PostgresArtifactRepository) DeleteArtifact(ctx context.Context, id string) error {
	return r.withUserTx(ctx, func(tx pgx.Tx, userID string) error {
		// Soft delete: tombstone the artifact's tree node (the entity spine; for
		// an artifact tree_nodes.id == artifact id). The artifacts row is KEPT
		// (body retained); reads hide it via the tree_nodes join. The tombstone +
		// bumped seq block resurrection by a stale upsert.
		ent, err := treesync.SoftDeleteNode(ctx, tx, userID, id, "artifact")
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperror.ErrNotFound
			}
			return err
		}
		return outbox.AppendData(ctx, tx, userID, service.Frame{Entities: []service.FrameEntity{ent}})
	})
}

func (r *PostgresArtifactRepository) ListFolders(ctx context.Context, parentID *string) ([]artifact.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE user_id = $1::uuid
		   AND kind = 'folder'
		   AND is_deleted = false
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

	out := make([]artifact.FolderNode, 0)
	for rows.Next() {
		var node artifact.FolderNode
		if err := rows.Scan(&node.ID, &node.ParentID, &node.Title); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) ListAllFolders(ctx context.Context) ([]artifact.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE user_id = $1::uuid
		   AND kind = 'folder'
		   AND is_deleted = false
		 ORDER BY position ASC, created_at ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifact.FolderNode, 0)
	for rows.Next() {
		var node artifact.FolderNode
		if err := rows.Scan(&node.ID, &node.ParentID, &node.Title); err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	return out, rows.Err()
}

func (r *PostgresArtifactRepository) ListAllBrowserNodes(ctx context.Context) ([]artifact.BrowserTreeFlatNode, error) {
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
		   AND n.is_deleted = false
		 ORDER BY COALESCE(n.parent_id, ''), n.position ASC, n.created_at ASC, n.id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]artifact.BrowserTreeFlatNode, 0)
	for rows.Next() {
		var node artifact.BrowserTreeFlatNode
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
		   AND is_deleted = false
	`, userID, parentID).Scan(&next)
	return next, err
}

func (r *PostgresArtifactRepository) CreateTreeNode(ctx context.Context, node artifact.TreeNodeRecord) error {
	return r.withUserTx(ctx, func(tx pgx.Tx, userID string) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
			VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, now(), now())
		`, node.ID, userID, node.ParentID, node.Kind, node.Title, node.ArtifactID, node.Position); err != nil {
			return err
		}
		// Folder (or bare node) create: one frame for the new entity.
		return treesync.EmitNodeUpsert(ctx, tx, userID, node.ID)
	})
}

func (r *PostgresArtifactRepository) DeleteBrowserNode(ctx context.Context, id string) error {
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

	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}

	// Collect the whole subtree (root + descendants) with kind, so we can emit a
	// delete tombstone per entity (DL Phase A: cascade delete propagates).
	rows, err := tx.Query(ctx, `
		WITH RECURSIVE subtree AS (
		    SELECT id, kind, artifact_id
		      FROM tree_nodes
		     WHERE id = $1 AND user_id = $2::uuid
		    UNION ALL
		    SELECT n.id, n.kind, n.artifact_id
		      FROM tree_nodes n
		      JOIN subtree s ON n.parent_id = s.id
		     WHERE n.user_id = $2::uuid
		)
		SELECT id, kind, artifact_id FROM subtree
	`, id, userID)
	if err != nil {
		return err
	}
	type subnode struct {
		id         string
		kind       string
		artifactID *string
	}
	var nodes []subnode
	for rows.Next() {
		var n subnode
		if err := rows.Scan(&n.id, &n.kind, &n.artifactID); err != nil {
			rows.Close()
			return err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(nodes) == 0 {
		return apperror.ErrNotFound
	}

	// Soft delete the whole subtree: tombstone each node (is_deleted=true, seq
	// bumped, rows KEPT so a stale lower-seq upsert can't resurrect them) and
	// emit ONE frame with a delete entity per node. Artifact rows are kept
	// (bodies retained); reads hide them via the tree_nodes join. The entity is
	// keyed by the node id (== artifact id for artifacts — the spine).
	ents := make([]service.FrameEntity, 0, len(nodes))
	for _, n := range nodes {
		ent, err := treesync.SoftDeleteNode(ctx, tx, userID, n.id, n.kind)
		if err != nil {
			return err
		}
		ents = append(ents, ent)
	}
	if err := outbox.AppendData(ctx, tx, userID, service.Frame{Entities: ents}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresArtifactRepository) UpdateArtifactNodeParent(ctx context.Context, artifactID string, parentID *string) error {
	position, err := r.NextNodePosition(ctx, parentID)
	if err != nil {
		return err
	}
	return r.withUserTx(ctx, func(tx pgx.Tx, userID string) error {
		tag, err := tx.Exec(ctx, `
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
			return apperror.ErrNotFound
		}
		// A move changes placement (parentId + position): one frame for the entity.
		return treesync.EmitNodeUpsert(ctx, tx, userID, artifactID)
	})
}

func (r *PostgresArtifactRepository) UpdateFolderTitle(ctx context.Context, id string, title string) error {
	userID, err := r.userID(ctx)
	if err != nil {
		return err
	}
	// DL Phase A: the entity write + the change-feed row land in one txn under
	// the per-user advisory lock (so the appended seq is visible in commit order).
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
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
		return apperror.ErrNotFound
	}
	// One frame for the renamed folder entity.
	if err := treesync.EmitNodeUpsert(ctx, tx, userID, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresArtifactRepository) GetFolder(ctx context.Context, id string) (artifact.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifact.FolderNode{}, err
	}
	var node artifact.FolderNode
	err = r.pool.QueryRow(ctx, `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE id = $1 AND user_id = $2::uuid
		   AND kind = 'folder'
		   AND is_deleted = false
	`, id, userID).Scan(&node.ID, &node.ParentID, &node.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifact.FolderNode{}, apperror.ErrNotFound
	}
	if err != nil {
		return artifact.FolderNode{}, err
	}
	return node, nil
}

// GetContainer resolves a node that can CONTAIN other nodes — a plain folder or a
// research folder (RESEARCH_SURFACE_PRD §21: research artifacts live in the research
// folder via the existing capture surface). Use this for parent/destination validation;
// use GetFolder only where plain-folder semantics are actually required (the folder
// read/rename API), so a research node can't be renamed through the folder endpoints.
func (r *PostgresArtifactRepository) GetContainer(ctx context.Context, id string) (artifact.FolderNode, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifact.FolderNode{}, err
	}
	var node artifact.FolderNode
	err = r.pool.QueryRow(ctx, `
		SELECT id, parent_id, title
		  FROM tree_nodes
		 WHERE id = $1 AND user_id = $2::uuid
		   AND kind IN ('folder', 'research')
		   AND is_deleted = false
	`, id, userID).Scan(&node.ID, &node.ParentID, &node.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifact.FolderNode{}, apperror.ErrNotFound
	}
	if err != nil {
		return artifact.FolderNode{}, err
	}
	return node, nil
}

func (r *PostgresArtifactRepository) FolderBreadcrumbs(ctx context.Context, id string) ([]artifact.BreadcrumbItem, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE chain AS (
		    SELECT id, parent_id, title, 0 AS depth
		      FROM tree_nodes
		     WHERE id = $1 AND user_id = $2::uuid AND kind = 'folder' AND is_deleted = false
		    UNION ALL
		    SELECT f.id, f.parent_id, f.title, c.depth + 1
		      FROM tree_nodes f
		      JOIN chain c ON c.parent_id = f.id
		     WHERE f.user_id = $2::uuid AND f.kind = 'folder' AND f.is_deleted = false
		)
		SELECT id, title
		  FROM chain
		 ORDER BY depth DESC
	`, id, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []artifact.BreadcrumbItem{{ID: nil, Label: "Folders"}}
	for rows.Next() {
		var itemID string
		var label string
		if err := rows.Scan(&itemID, &label); err != nil {
			return nil, err
		}
		idCopy := itemID
		items = append(items, artifact.BreadcrumbItem{ID: &idCopy, Label: label})
	}
	return items, rows.Err()
}

func (r *PostgresArtifactRepository) userID(ctx context.Context) (string, error) {
	principal, err := auth.RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return principal.UserID, nil
}

func scanArtifactResponse(row scanner) (artifact.ArtifactResponse, error) {
	var rec artifact.ArtifactResponse
	var metadata []byte
	var blocks []byte
	var createdAt time.Time
	var updatedAt time.Time
	rec.Metadata = map[string]any{}
	err := row.Scan(
		&rec.ID,
		&rec.FolderID,
		&rec.Type,
		&rec.Title,
		&rec.Content,
		&blocks,
		&rec.Summary,
		&rec.SourceURL,
		&metadata,
		&createdAt,
		&updatedAt,
		&rec.Revision,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rec, apperror.ErrNotFound
		}
		return rec, err
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &rec.Metadata)
	}
	if rec.Type == "page" {
		// Pages carry their body in Blocks; Content is unused.
		rec.Content = ""
		if len(blocks) > 0 {
			rec.Blocks = json.RawMessage(blocks)
		} else {
			rec.Blocks = json.RawMessage(`[]`)
		}
	}
	rec.CreatedAt = createdAt.Format(time.RFC3339)
	rec.UpdatedAt = updatedAt.Format(time.RFC3339)
	return rec, nil
}

// QueryArtifactsByProperty returns the caller's artifacts carrying a typed property (H1c).
//
// Properties are a JSON ARRAY at metadata->'properties' of {key,type,value,values?,options?}, so
// the match is jsonb CONTAINMENT — an array contains a probe array when every probe element is
// contained in some stored element, which matches on key(+value) regardless of position or the
// other fields. Containment is exactly what the jsonb_path_ops GIN index (00035) serves.
//
// An empty Value matches any value for the key ("has a Status"). That case can't use containment
// (there is no value to probe), so it falls back to an existence scan over the array.
func (r *PostgresArtifactRepository) QueryArtifactsByProperty(ctx context.Context, params artifact.PropertyQuery) ([]artifact.ArtifactResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	query := `
		SELECT a.id, n.parent_id, a.type, a.title,
		       a.content,
		       p.blocks,
		       a.summary, a.source_url, a.metadata, a.created_at, a.updated_at,
		       COALESCE(p.revision, 0) AS revision
		  FROM artifacts a
		  JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  LEFT JOIN page_documents p ON p.artifact_id = a.id
		 WHERE a.user_id = $1::uuid
		   AND n.is_deleted = false
	`
	args := []any{userID}
	if params.Value == "" {
		// key-only: any artifact whose properties array holds an element with this key.
		query += ` AND EXISTS (
		       SELECT 1 FROM jsonb_array_elements(
		           CASE WHEN jsonb_typeof(a.metadata->'properties') = 'array'
		                THEN a.metadata->'properties' ELSE '[]'::jsonb END) e
		        WHERE e->>'key' = $2)`
		args = append(args, params.Key)
	} else {
		probe, err := json.Marshal(map[string]any{
			"properties": []map[string]string{{"key": params.Key, "value": params.Value}},
		})
		if err != nil {
			return nil, fmt.Errorf("artifact property probe: %w", err)
		}
		query += ` AND a.metadata @> $2::jsonb`
		args = append(args, string(probe))
	}
	query += ` ORDER BY a.updated_at DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, params.Limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("artifact property query: %w", err)
	}
	defer rows.Close()
	out := make([]artifact.ArtifactResponse, 0)
	for rows.Next() {
		rec, err := scanArtifactResponse(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// PropertyFacets lists the caller's property keys and the distinct values seen for each, so a
// filter UI can offer real choices. Scans the properties arrays (the GIN index serves containment,
// not this aggregation) — bounded by the user's own artifacts.
func (r *PostgresArtifactRepository) PropertyFacets(ctx context.Context) ([]artifact.PropertyFacet, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT e->>'key'  AS key,
		       COALESCE(MIN(e->>'type'), '') AS type,
		       ARRAY_AGG(DISTINCT e->>'value') FILTER (WHERE COALESCE(e->>'value', '') <> '') AS values
		  FROM artifacts a
		  JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		  CROSS JOIN LATERAL jsonb_array_elements(
		      CASE WHEN jsonb_typeof(a.metadata->'properties') = 'array'
		           THEN a.metadata->'properties' ELSE '[]'::jsonb END) e
		 WHERE a.user_id = $1::uuid AND n.is_deleted = false AND COALESCE(e->>'key', '') <> ''
		 GROUP BY e->>'key'
		 ORDER BY key
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("artifact property facets: %w", err)
	}
	defer rows.Close()
	out := make([]artifact.PropertyFacet, 0)
	for rows.Next() {
		var f artifact.PropertyFacet
		if err := rows.Scan(&f.Key, &f.Type, &f.Values); err != nil {
			return nil, fmt.Errorf("artifact property facets scan: %w", err)
		}
		if f.Values == nil {
			f.Values = []string{}
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
