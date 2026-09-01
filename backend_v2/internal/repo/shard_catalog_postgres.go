package repo

import (
	"aladin/backend_v2/internal/service"
	"context"
)

// The initial catalog is an authoritative projection of published releases,
// joined to live ownership. No asynchronously stale copy can outlive deletion.
func (r *ShardResourcePostgres) FindResourceReleases(ctx context.Context, query string, limit int) ([]service.ShardCatalogRelease, error) {
	p, err := service.RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.RequireScope(ctx, service.ScopeArtifactsRead); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `SELECT a.shard_id,o.title,r.contract_source,r.contract_hash,r.build_id,r.generation
 FROM shard_resource_active a JOIN shard_resource_releases r USING(user_id,shard_id,environment,build_id)
 JOIN artifacts o ON o.id=a.shard_id AND o.user_id=a.user_id AND o.type='app'
 WHERE a.user_id=$1::uuid AND a.environment='published' AND
 (strpos(lower(o.title),lower($2))>0 OR EXISTS(SELECT 1 FROM jsonb_each(convert_from(r.contract_source,'UTF8')::jsonb->'resources') resource WHERE strpos(lower(resource.key||' '||coalesce(resource.value->>'meaning','')),lower($2))>0))
 ORDER BY a.shard_id LIMIT $3`, p.UserID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.ShardCatalogRelease{}
	for rows.Next() {
		var item service.ShardCatalogRelease
		if err := rows.Scan(&item.ShardID, &item.Title, &item.Source, &item.Hash, &item.BuildID, &item.Generation); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
