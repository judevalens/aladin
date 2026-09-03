package postgres

import (
	"context"
	"encoding/json"

	"aladin/backend_v2/internal/relationship"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RelationshipRepo persists the additive `relationships` edge layer. Implements
// service.RelationshipStore. Thin data access: userID is passed explicitly by the
// service. UUID columns are cast to text in SQL so they scan cleanly into strings.
type RelationshipRepo struct {
	pool *pgxpool.Pool
}

func NewRelationshipPostgres(pool *pgxpool.Pool) *RelationshipRepo {
	return &RelationshipRepo{pool: pool}
}

func (r *RelationshipRepo) Create(ctx context.Context, rel relationship.Relationship) (relationship.Relationship, error) {
	if rel.Metadata == nil {
		rel.Metadata = map[string]any{}
	}
	mb, err := json.Marshal(rel.Metadata)
	if err != nil {
		return relationship.Relationship{}, err
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO relationships (user_id, src_kind, src_id, dst_kind, dst_id, rel_type, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		ON CONFLICT (user_id, src_kind, src_id, dst_kind, dst_id, rel_type)
		DO UPDATE SET metadata = EXCLUDED.metadata
		RETURNING id::text, created_at
	`, rel.UserID, rel.SrcKind, rel.SrcID, rel.DstKind, rel.DstID, rel.RelType, string(mb))
	if err := row.Scan(&rel.ID, &rel.CreatedAt); err != nil {
		return relationship.Relationship{}, err
	}
	return rel, nil
}

func (r *RelationshipRepo) ListForNode(ctx context.Context, userID, kind, id string) ([]relationship.Relationship, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, user_id::text, src_kind, src_id, dst_kind, dst_id, rel_type, metadata, created_at
		FROM relationships
		WHERE user_id = $1
		  AND ((src_kind = $2 AND src_id = $3) OR (dst_kind = $2 AND dst_id = $3))
		ORDER BY created_at DESC
	`, userID, kind, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []relationship.Relationship
	for rows.Next() {
		var rel relationship.Relationship
		var meta []byte
		if err := rows.Scan(&rel.ID, &rel.UserID, &rel.SrcKind, &rel.SrcID, &rel.DstKind, &rel.DstID, &rel.RelType, &meta, &rel.CreatedAt); err != nil {
			return nil, err
		}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &rel.Metadata)
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

func (r *RelationshipRepo) Delete(ctx context.Context, userID, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM relationships WHERE user_id = $1 AND id = $2::uuid`, userID, id)
	return err
}
