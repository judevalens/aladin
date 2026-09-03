package postgres

import (
	"context"
	"fmt"
	"strings"

	"aladin/backend_v2/internal/artifactref"
	"aladin/backend_v2/internal/outbox"
	coreservice "aladin/backend_v2/internal/service"
	"aladin/backend_v2/internal/treesync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresArtifactRefRepository backs the `#` cross-reference picker + projection: search
// across artifacts (pages/shards), and the artifact↔target links in
// artifact_refs.
type PostgresArtifactRefRepository struct{ pool *pgxpool.Pool }

func NewArtifactRefPostgres(pool *pgxpool.Pool) *PostgresArtifactRefRepository {
	return &PostgresArtifactRefRepository{pool: pool}
}

// SearchArtifacts matches the `#` typeahead against the user's pages + shards by title.
// artifacts.type 'page' → kind 'page', 'app' → kind 'shard'. ownerUserID "" → no matches
// (artifacts are user-scoped).
func (r *PostgresArtifactRefRepository) SearchArtifacts(ctx context.Context, ownerUserID, query string, limit int) ([]artifactref.RefHit, error) {
	if strings.TrimSpace(ownerUserID) == "" {
		return []artifactref.RefHit{}, nil
	}
	like := "%" + strings.TrimSpace(query) + "%"

	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.type, a.title
		  FROM artifacts a
		  LEFT JOIN tree_nodes n ON n.artifact_id = a.id AND n.user_id = a.user_id AND n.kind = 'artifact'
		 WHERE a.user_id = $1::uuid
		   AND COALESCE(n.is_deleted, false) = false
		   AND a.type IN ('page', 'app')
		   AND a.title ILIKE $2
		 ORDER BY a.updated_at DESC, a.created_at DESC
		 LIMIT $3
	`, ownerUserID, like, limit)
	if err != nil {
		return nil, fmt.Errorf("artifact ref search artifacts: %w", err)
	}
	defer rows.Close()

	out := []artifactref.RefHit{}
	for rows.Next() {
		var id, typ, title string
		if err := rows.Scan(&id, &typ, &title); err != nil {
			return nil, fmt.Errorf("artifact ref search artifacts scan: %w", err)
		}
		kind := artifactref.RefKindPage
		if typ == "app" {
			kind = artifactref.RefKindShard
		}
		out = append(out, artifactref.RefHit{Kind: kind, ID: id, Label: title})
	}
	return out, rows.Err()
}

// ReplaceRefs reconciles the projected `#` refs for an artifact: in one transaction it drops
// all existing origin='reference' rows for the page and inserts the given set. The set is the
// source of truth, derived from the page's ydoc on save.
func (r *PostgresArtifactRefRepository) ReplaceRefs(ctx context.Context, artifactID string, refs []artifactref.ArtifactRef) error {
	principal, err := coreservice.RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	userID := principal.UserID

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("artifact ref sync begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := outbox.LockUser(ctx, tx, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM artifact_refs WHERE artifact_id = $1 AND origin = 'reference'
	`, artifactID); err != nil {
		return fmt.Errorf("artifact ref sync clear: %w", err)
	}

	for _, ref := range refs {
		var block any
		if strings.TrimSpace(ref.BlockID) != "" {
			block = ref.BlockID
		}
		var surface any
		if strings.TrimSpace(ref.Surface) != "" {
			surface = ref.Surface
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO artifact_refs (artifact_id, target_kind, target_id, block_id, surface, origin)
			VALUES ($1, $2, $3, $4, $5, 'reference')
			ON CONFLICT (artifact_id, target_kind, target_id, COALESCE(block_id, ''), COALESCE(surface, ''))
			DO NOTHING
		`, artifactID, ref.Kind, ref.TargetID, block, surface); err != nil {
			return fmt.Errorf("artifact ref sync insert: %w", err)
		}
	}
	// The page's # links changed → emit a node frame so reactive views (graph pane) refetch.
	if err := treesync.EmitNodeUpsert(ctx, tx, userID, artifactID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ListForArtifact returns a page's outgoing refs, resolved to current target titles,
// falling back to the stored surface if the target is gone.
func (r *PostgresArtifactRefRepository) ListForArtifact(ctx context.Context, artifactID string) ([]artifactref.AttachedRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ar.target_kind, ar.target_id,
		       COALESCE(a.title, ar.surface, '') AS label,
		       COALESCE(ar.block_id, '')
		  FROM artifact_refs ar
		  LEFT JOIN artifacts a ON ar.target_kind IN ('page', 'shard') AND a.id = ar.target_id
		 WHERE ar.artifact_id = $1 AND ar.origin = 'reference'
		 ORDER BY ar.target_kind, label
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("artifact ref list: %w", err)
	}
	defer rows.Close()

	out := []artifactref.AttachedRef{}
	for rows.Next() {
		var a artifactref.AttachedRef
		if err := rows.Scan(&a.Kind, &a.TargetID, &a.Label, &a.BlockID); err != nil {
			return nil, fmt.Errorf("artifact ref list scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
