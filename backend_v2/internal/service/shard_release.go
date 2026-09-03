package service

import (
	"context"
	"errors"

	"aladin/backend_v2/internal/shardresource/release"
	"aladin/backend_v2/internal/shardv2"
)

type ShardRelease = release.Build
type ShardReleaseStore = release.Store
type ResourceStageValidator = release.StageValidator
type ResourceActivationFence = release.ActivationFence

type ShardReleaseService interface {
	Enabled() bool
	Stage(context.Context, string, BuildChannel, BuildResult) error
	Activate(context.Context, string, BuildChannel, string) error
	Active(context.Context, string, BuildChannel) (ShardRelease, error)
}

type shardReleaseService struct{ manager *release.Manager }

type shardReleaseAuthorizer struct{}

func (shardReleaseAuthorizer) PrincipalUserID(ctx context.Context) (string, error) {
	principal, err := RequirePrincipal(ctx)
	return principal.UserID, err
}
func (shardReleaseAuthorizer) RequireRead(ctx context.Context) error {
	return RequireScope(ctx, ScopeArtifactsRead)
}
func (shardReleaseAuthorizer) RequireWrite(ctx context.Context) error {
	return RequireScope(ctx, ScopeArtifactsWrite)
}

func NewShardReleaseService(store ShardReleaseStore, profiles shardv2.Registry, validators ...ResourceStageValidator) ShardReleaseService {
	return &shardReleaseService{manager: release.NewManager(store, profiles, shardReleaseAuthorizer{}, release.ErrorPolicy{
		Failure:    ResourceFailure,
		IsNotFound: func(err error) bool { return errors.Is(err, ErrNotFound) },
	}, validators...)}
}

func (s *shardReleaseService) Enabled() bool { return s.manager.Enabled() }

func ShardBuildIdentity(contract []byte, files map[string][]byte) string {
	return release.BuildIdentity(contract, files)
}

func (s *shardReleaseService) Stage(ctx context.Context, id string, channel BuildChannel, result BuildResult) error {
	return s.manager.Stage(ctx, id, channel, release.BuildOutput{Contract: result.Contract, Files: result.Files, BuildID: result.BuildID})
}

func (s *shardReleaseService) Activate(ctx context.Context, id string, channel BuildChannel, build string) error {
	return s.manager.Activate(ctx, id, channel, build)
}

func (s *shardReleaseService) Active(ctx context.Context, id string, channel BuildChannel) (ShardRelease, error) {
	return s.manager.Active(ctx, id, channel)
}
