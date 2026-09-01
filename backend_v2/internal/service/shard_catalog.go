package service

import (
	"aladin/backend_v2/internal/shardv2"
	"context"
	"encoding/json"
	"sort"
	"strings"
)

type ShardCatalogRelease struct {
	ShardID string
	Title   string
	ResourceRelease
}
type ShardCatalogReader interface {
	FindResourceReleases(context.Context, string, int) ([]ShardCatalogRelease, error)
}
type ShardCatalogEntry struct {
	URI          string             `json:"uri"`
	ShardID      string             `json:"shardId"`
	ResourceID   string             `json:"resourceId"`
	Title        string             `json:"title"`
	Meaning      string             `json:"meaning"`
	Provider     string             `json:"provider"`
	ContractHash string             `json:"contractHash"`
	BuildID      string             `json:"buildId"`
	Descriptor   ResourceDescriptor `json:"descriptor"`
}
type ShardCatalogService interface {
	Find(context.Context, string, int) ([]ShardCatalogEntry, error)
}
type shardCatalogService struct {
	reader    ShardCatalogReader
	resources ShardResourceService
}

func NewShardCatalogService(reader ShardCatalogReader, resources ShardResourceService) ShardCatalogService {
	return &shardCatalogService{reader, resources}
}
func (s *shardCatalogService) Find(ctx context.Context, query string, limit int) ([]ShardCatalogEntry, error) {
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return nil, err
	}
	if p.ActorType == ActorTypeContentToken {
		return nil, ErrForbidden
	}
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if len(query) > 256 {
		return nil, ResourceFailure("bad-request", "Search is too long")
	}
	releases, err := s.reader.FindResourceReleases(ctx, query, 100)
	if err != nil {
		return nil, err
	}
	result := []ShardCatalogEntry{}
	for _, release := range releases {
		var contract shardv2.Contract
		if json.Unmarshal(release.Source, &contract) != nil {
			continue
		}
		ids := []string{}
		for id := range contract.Resources {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			definition := contract.Resources[id]
			if !strings.Contains(strings.ToLower(release.Title+" "+id+" "+definition.Meaning), strings.ToLower(query)) {
				continue
			}
			// Catalog candidates never confer access. Re-resolve the current protected
			// release and caller's agent capabilities on every result.
			descriptor, err := s.resources.Describe(ctx, ResourceTarget{ShardID: release.ShardID, Environment: ChannelPublished, Audience: "agent", ContractHash: release.Hash}, ResourceRequest{Resource: id})
			if err != nil {
				switch ResourceErrorCode(err) {
				case "forbidden", "not-found", "contract-changed":
					continue
				default:
					return nil, err
				}
			}
			result = append(result, ShardCatalogEntry{URI: "shard://" + release.ShardID + "/resources/" + id, ShardID: release.ShardID, ResourceID: id, Title: release.Title, Meaning: definition.Meaning, Provider: definition.Source.Provider, ContractHash: release.Hash, BuildID: release.BuildID, Descriptor: descriptor})
			if len(result) == limit {
				return result, nil
			}
		}
	}
	return result, nil
}
