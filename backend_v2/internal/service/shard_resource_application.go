package service

import (
	"context"

	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/shardresource"
)

// shardResourceAccess adapts the service package's established authentication
// and artifact ownership rules to the isolated Shard resource application.
type shardResourceAccess struct{ artifacts artifact.ArtifactService }

func (a shardResourceAccess) Principal(ctx context.Context) (shardresource.Principal, error) {
	principal, err := RequirePrincipal(ctx)
	return shardresource.Principal{
		UserID:    principal.UserID,
		ActorType: principal.ActorType,
		ActorID:   principal.ActorID,
		Scopes:    principal.Scopes,
	}, err
}

func (shardResourceAccess) RequireRead(ctx context.Context) error {
	return RequireScope(ctx, ScopeArtifactsRead)
}

func (shardResourceAccess) CanWrite(ctx context.Context) bool {
	return HasScope(ctx, ScopeArtifactsWrite)
}

func (a shardResourceAccess) RequireApp(ctx context.Context, shardID string) error {
	record, err := a.artifacts.Get(ctx, shardID)
	if err != nil {
		return err
	}
	if record.Type != "app" {
		return ErrNotFound
	}
	return nil
}

func (shardResourceAccess) Forbidden() error           { return ErrForbidden }
func (shardResourceAccess) ErrorCode(err error) string { return ResourceErrorCode(err) }

func NewShardResourceService(artifacts artifact.ArtifactService, releases ResourceReleaseReader, providers map[string]ResourceProvider, options ResourceServiceOptions) ShardResourceService {
	return shardresource.NewService(shardResourceAccess{artifacts: artifacts}, releases, providers, options)
}
