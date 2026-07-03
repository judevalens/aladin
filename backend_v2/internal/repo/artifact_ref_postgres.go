package repo

import (
	"context"
	"fmt"
	"strings"

	coreservice "aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresArtifactRefRepository backs the `#` cross-reference picker + projection: search
// across claims and artifacts (pages/shards), and the artifact↔target links in
// artifact_refs.
type PostgresArtifactRefRepository struct{ pool *pgxpool.Pool }

func NewArtifactRefPostgres(pool *pgxpool.Pool) *PostgresArtifactRefRepository {
	return &PostgresArtifactRefRepository{pool: pool}
}

// SearchClaims matches the `#` typeahead against claim propositions: the tenant's own
// theses + shared discovered claims whose canonical_text contains the query or is
// trigram-similar. ownerUserID "" → shared only.
func (r *PostgresArtifactRefRepository) SearchClaims(ctx context.Context, ownerUserID, query string, limit int) ([]coreservice.RefHit, error) {
	like := "%" + strings.TrimSpace(query) + "%"
	var owner any
	if strings.TrimSpace(ownerUserID) != "" {
		owner = ownerUserID
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, canonical_text, polarity,
		       GREATEST(similarity(canonical_text, $1), 0) AS sim
		  FROM claims
		 WHERE (scope = 'shared' OR (scope = 'tenant' AND owner_user_id = $2::uuid))
		   AND (canonical_text ILIKE $3 OR similarity(canonical_text, $1) >= 0.1)
		 ORDER BY (canonical_text ILIKE $3) DESC, sim DESC, canonical_text ASC
		 LIMIT $4
	`, strings.TrimSpace(query), owner, like, limit)
	if err != nil {
		return nil, fmt.Errorf("artifact ref search claims: %w", err)
	}
	defer rows.Close()

	out := []coreservice.RefHit{}
	for rows.Next() {
		var h coreservice.RefHit
		var sim float64
		if err := rows.Scan(&h.ID, &h.Label, &h.Detail, &sim); err != nil {
			return nil, fmt.Errorf("artifact ref search claims scan: %w", err)
		}
		h.Kind = coreservice.RefKindClaim
		out = append(out, h)
	}
	return out, rows.Err()
}

// SearchArtifacts matches the `#` typeahead against the user's pages + shards by title.
// artifacts.type 'page' → kind 'page', 'app' → kind 'shard'. ownerUserID "" → no matches
// (artifacts are user-scoped).
func (r *PostgresArtifactRefRepository) SearchArtifacts(ctx context.Context, ownerUserID, query string, limit int) ([]coreservice.RefHit, error) {
	if strings.TrimSpace(ownerUserID) == "" {
		return []coreservice.RefHit{}, nil
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

	out := []coreservice.RefHit{}
	for rows.Next() {
		var id, typ, title string
		if err := rows.Scan(&id, &typ, &title); err != nil {
			return nil, fmt.Errorf("artifact ref search artifacts scan: %w", err)
		}
		kind := coreservice.RefKindPage
		if typ == "app" {
			kind = coreservice.RefKindShard
		}
		out = append(out, coreservice.RefHit{Kind: kind, ID: id, Label: title})
	}
	return out, rows.Err()
}

// ReplaceRefs reconciles the projected `#` refs for an artifact: in one transaction it drops
// all existing origin='reference' rows for the page and inserts the given set. The set is the
// source of truth, derived from the page's ydoc on save.
func (r *PostgresArtifactRefRepository) ReplaceRefs(ctx context.Context, artifactID string, refs []coreservice.ArtifactRef) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("artifact ref sync begin: %w", err)
	}
	defer tx.Rollback(ctx)

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
	return tx.Commit(ctx)
}

// ListForArtifact returns a page's outgoing refs, resolved to current target labels (claim
// canonical_text or artifact title), falling back to the stored surface if the target is
// gone. Polymorphic join: claims for kind='claim', artifacts for page/shard.
func (r *PostgresArtifactRefRepository) ListForArtifact(ctx context.Context, artifactID string) ([]coreservice.AttachedRef, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ar.target_kind, ar.target_id,
		       COALESCE(c.canonical_text, a.title, ar.surface, '') AS label,
		       COALESCE(ar.block_id, '')
		  FROM artifact_refs ar
		  LEFT JOIN claims    c ON ar.target_kind = 'claim'            AND c.id::text = ar.target_id
		  LEFT JOIN artifacts a ON ar.target_kind IN ('page', 'shard') AND a.id = ar.target_id
		 WHERE ar.artifact_id = $1 AND ar.origin = 'reference'
		 ORDER BY ar.target_kind, label
	`, artifactID)
	if err != nil {
		return nil, fmt.Errorf("artifact ref list: %w", err)
	}
	defer rows.Close()

	out := []coreservice.AttachedRef{}
	for rows.Next() {
		var a coreservice.AttachedRef
		if err := rows.Scan(&a.Kind, &a.TargetID, &a.Label, &a.BlockID); err != nil {
			return nil, fmt.Errorf("artifact ref list scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
