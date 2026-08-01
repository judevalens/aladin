package repo

import (
	"context"
	"errors"
	"fmt"

	artifactservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// research_postgres.go — persistence for research folders (RESEARCH_SURFACE_PRD §5).
//
// A research folder is two rows that must exist together: the tree_nodes row (kind =
// 'research', folder-shaped) and its 1:1 research_strategies extension. They are written
// in ONE tx alongside the outbox frame, so a client can never observe a research node
// whose extension is missing — which is what lets the sync projection treat the extension
// as always-present.

type PostgresResearchRepository struct{ pool *pgxpool.Pool }

func NewResearchPostgres(pool *pgxpool.Pool) *PostgresResearchRepository {
	return &PostgresResearchRepository{pool: pool}
}

func (r *PostgresResearchRepository) userID(ctx context.Context) (string, error) {
	principal, err := artifactservice.RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	return principal.UserID, nil
}

// ParentIsFolder reports whether the node is a live plain folder owned by the caller.
// Deliberately excludes kind='research' — §5 rules out research nested in research.
func (r *PostgresResearchRepository) ParentIsFolder(ctx context.Context, parentID string) (bool, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return false, err
	}
	var exists bool
	err = r.pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM tree_nodes
		     WHERE id = $1 AND user_id = $2::uuid
		       AND kind = 'folder' AND is_deleted = false
		)
	`, parentID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("research: parent check %s: %w", parentID, err)
	}
	return exists, nil
}

func (r *PostgresResearchRepository) NextResearchPosition(ctx context.Context, parentID *string) (int64, error) {
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

// CreateResearchFolder writes the node + its extension row + the sync frame in one tx.
func (r *PostgresResearchRepository) CreateResearchFolder(
	ctx context.Context, in artifactservice.ResearchCreateInput, position int64,
) (artifactservice.BrowserNodeResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Same per-user advisory lock every tree write takes, so the appended seq is
	// visible in commit order.
	if err := LockUser(ctx, tx, userID); err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tree_nodes (id, user_id, parent_id, kind, title, artifact_id, position, created_at, updated_at)
		VALUES ($1, $2::uuid, $3, 'research', $4, NULL, $5, now(), now())
	`, in.ID, userID, in.ParentID, in.Title, position); err != nil {
		return artifactservice.BrowserNodeResponse{}, fmt.Errorf("research: insert node: %w", err)
	}

	// The sparse extension row (§5): present from creation, empty of strategy facts.
	// Defaults carry source_kind='authored', exec_mode='event' (§8's primitive), and
	// run_state='idle'.
	var hypothesis *string
	if in.Hypothesis != "" {
		hypothesis = &in.Hypothesis
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO research_strategies (node_id, user_id, hypothesis)
		VALUES ($1, $2::uuid, $3)
	`, in.ID, userID, hypothesis); err != nil {
		return artifactservice.BrowserNodeResponse{}, fmt.Errorf("research: insert strategy: %w", err)
	}

	// One frame for the new entity — bumps seq, reads the light projection (which now
	// includes the extension), and appends to the outbox in this same tx.
	if err := emitNodeUpsert(ctx, tx, userID, in.ID); err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}

	var seq int64
	if err := tx.QueryRow(ctx,
		`SELECT seq FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid`, in.ID, userID,
	).Scan(&seq); err != nil {
		return artifactservice.BrowserNodeResponse{}, fmt.Errorf("research: read back seq: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}
	return artifactservice.BrowserNodeResponse{
		ID:       in.ID,
		ParentID: in.ParentID,
		Kind:     "research",
		Title:    in.Title,
		Position: position,
		Seq:      uint64(seq),
	}, nil
}

// UpdateResearchFolder applies the patch across the node (title) and its extension row
// (hypothesis), then emits the node frame — all in one tx. Scoped to kind='research' so
// the research endpoints can't touch a plain folder, mirroring how the folder endpoints
// are scoped to kind='folder'.
func (r *PostgresResearchRepository) UpdateResearchFolder(ctx context.Context, id string, patch artifactservice.ResearchPatch) (artifactservice.BrowserNodeResponse, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := LockUser(ctx, tx, userID); err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}

	// COALESCE keeps an unpatched column untouched, so a hypothesis edit can't blank the
	// title. The kind guard is what makes this endpoint research-only.
	tag, err := tx.Exec(ctx, `
		UPDATE tree_nodes
		   SET title = COALESCE($3, title), updated_at = now()
		 WHERE id = $1 AND user_id = $2::uuid
		   AND kind = 'research' AND is_deleted = false
	`, id, userID, patch.Title)
	if err != nil {
		return artifactservice.BrowserNodeResponse{}, fmt.Errorf("research: update %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return artifactservice.BrowserNodeResponse{}, artifactservice.ErrNotFound
	}

	if patch.Hypothesis != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE research_strategies
			   SET hypothesis = $3, updated_at = now()
			 WHERE node_id = $1 AND user_id = $2::uuid
		`, id, userID, *patch.Hypothesis); err != nil {
			return artifactservice.BrowserNodeResponse{}, fmt.Errorf("research: update hypothesis %s: %w", id, err)
		}
	}

	if err := emitNodeUpsert(ctx, tx, userID, id); err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}

	var (
		parentID *string
		title    string
		position int64
		seq      int64
	)
	if err := tx.QueryRow(ctx,
		`SELECT parent_id, COALESCE(title, ''), position, seq FROM tree_nodes WHERE id = $1 AND user_id = $2::uuid`, id, userID,
	).Scan(&parentID, &title, &position, &seq); err != nil {
		return artifactservice.BrowserNodeResponse{}, fmt.Errorf("research: read back update %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return artifactservice.BrowserNodeResponse{}, err
	}
	return artifactservice.BrowserNodeResponse{
		ID: id, ParentID: parentID, Kind: "research",
		Title: title, Position: position, Seq: uint64(seq),
	}, nil
}

// GetResearchFolder reads the extension row joined to its node — the fields the sync
// frame deliberately leaves off (hypothesis, source, hashes).
func (r *PostgresResearchRepository) GetResearchFolder(ctx context.Context, id string) (artifactservice.ResearchFolder, error) {
	userID, err := r.userID(ctx)
	if err != nil {
		return artifactservice.ResearchFolder{}, err
	}
	var (
		out                                        artifactservice.ResearchFolder
		hypothesis, sourceRef, commitSHA, codeHash *string
	)
	err = r.pool.QueryRow(ctx, `
		SELECT n.id, COALESCE(n.title, ''), n.parent_id,
		       rs.hypothesis, rs.source_kind, rs.source_ref, rs.commit_sha, rs.code_hash,
		       rs.exec_mode, rs.run_state
		  FROM tree_nodes n
		  JOIN research_strategies rs ON rs.node_id = n.id AND rs.user_id = n.user_id
		 WHERE n.id = $1 AND n.user_id = $2::uuid
		   AND n.kind = 'research' AND n.is_deleted = false
	`, id, userID).Scan(
		&out.NodeID, &out.Title, &out.ParentID,
		&hypothesis, &out.SourceKind, &sourceRef, &commitSHA, &codeHash,
		&out.ExecMode, &out.RunState,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return artifactservice.ResearchFolder{}, artifactservice.ErrNotFound
	}
	if err != nil {
		return artifactservice.ResearchFolder{}, fmt.Errorf("research: get %s: %w", id, err)
	}
	out.Hypothesis = derefOr(hypothesis, "")
	out.SourceRef = derefOr(sourceRef, "")
	out.CommitSHA = derefOr(commitSHA, "")
	out.CodeHash = derefOr(codeHash, "")
	return out, nil
}
