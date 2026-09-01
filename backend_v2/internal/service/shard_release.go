package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"aladin/backend_v2/internal/shardv2"
)

// Code and contract share the same protected active pointer. Files contain only
// build outputs, never author-writable paths or storage credentials.
type ShardRelease struct {
	ResourceRelease
	Files map[string][]byte
}
type ShardReleaseStore interface {
	ResourceReleaseReader
	StageResourceBuild(context.Context, string, BuildChannel, string, string, *shardv2.Compiled, map[string][]byte) error
	ActivateResourceRelease(context.Context, string, BuildChannel, string, shardv2.Registry) error
	ActiveResourceBuild(context.Context, string, BuildChannel) (ShardRelease, error)
}
type ShardReleaseService interface {
	Enabled() bool
	Stage(context.Context, string, BuildChannel, BuildResult) error
	Activate(context.Context, string, BuildChannel, string) error
	Active(context.Context, string, BuildChannel) (ShardRelease, error)
}
type shardReleaseService struct {
	store      ShardReleaseStore
	profiles   shardv2.Registry
	validators []ResourceStageValidator
	fences     []ResourceActivationFence
}
type ResourceStageValidator interface {
	ValidateResourceStage(context.Context, shardv2.Resource) error
}

// ResourceActivationFence coordinates the protected Postgres release pointer
// with an owned datastore that cannot participate in the same transaction.
// Freezing must drain older writers and reject newer ones until it is released.
type ResourceActivationFence interface {
	FreezeNamespace(context.Context, ResourceNamespace, bool) error
}

func NewShardReleaseService(store ShardReleaseStore, profiles shardv2.Registry, validators ...ResourceStageValidator) ShardReleaseService {
	service := &shardReleaseService{store: store, profiles: profiles, validators: validators}
	for _, validator := range validators {
		if fence, ok := validator.(ResourceActivationFence); ok {
			service.fences = append(service.fences, fence)
		}
	}
	return service
}

// Enabled reflects the configured execution registry, not merely the presence
// of a release reader (which remains available while execution is disabled).
func (s *shardReleaseService) Enabled() bool { return len(s.profiles) > 0 }

// Build identity includes every output, contract and vendored import-map hash.
func ShardBuildIdentity(contract []byte, files map[string][]byte) string {
	raw, _ := json.Marshal(struct {
		Contract []byte
		Files    map[string][]byte
	}{contract, files})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func (s *shardReleaseService) Stage(ctx context.Context, id string, channel BuildChannel, result BuildResult) error {
	if len(s.profiles) == 0 {
		return ResourceFailure("unsupported-capability", "Shard v2 is disabled")
	}
	compiled, err := shardv2.Compile(result.Contract, s.profiles)
	if err != nil {
		return err
	}
	for _, definition := range compiled.Contract.Resources {
		for _, validator := range s.validators {
			if err := validator.ValidateResourceStage(ctx, definition); err != nil {
				return err
			}
		}
	}
	if len(result.Files["bundle.js"]) == 0 || result.BuildID != ShardBuildIdentity(result.Contract, result.Files) {
		return ResourceFailure("invalid-schema", "Build output identity does not match")
	}
	size := 0
	for name, data := range result.Files {
		if name != "bundle.js" && name != "bundle.css" && name != "importmap.json" && name != "anchors.json" && name != "resolver.bundle.mjs" && name != "runtime-manifest.json" && name != "schema.graphql" {
			return ResourceFailure("bad-request", "Unexpected build output")
		}
		size += len(data)
	}
	if size > 10<<20 {
		return ResourceFailure("quota", "Build exceeds 10 MiB")
	}
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	generation := "initial"
	previous, err := s.store.ActiveResourceRelease(ctx, p.UserID, id, channel)
	if err == nil {
		generation = previous.Generation
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	return s.store.StageResourceBuild(ctx, id, channel, result.BuildID, generation, compiled, result.Files)
}
func (s *shardReleaseService) Activate(ctx context.Context, id string, channel BuildChannel, build string) (err error) {
	if len(s.profiles) == 0 {
		return ResourceFailure("unsupported-capability", "Shard v2 is disabled")
	}
	p, err := RequirePrincipal(ctx)
	if err != nil {
		return err
	}
	if err := RequireScope(ctx, ScopeArtifactsWrite); err != nil {
		return err
	}
	ns := ResourceNamespace{UserID: p.UserID, ShardID: id, Environment: channel}
	frozen := 0
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		for index := frozen - 1; index >= 0; index-- {
			if releaseErr := s.fences[index].FreezeNamespace(cleanupCtx, ns, false); releaseErr != nil {
				err = errors.Join(err, releaseErr)
			}
		}
	}()
	for _, fence := range s.fences {
		if err := fence.FreezeNamespace(ctx, ns, true); err != nil {
			return err
		}
		frozen++
	}
	return s.store.ActivateResourceRelease(ctx, id, channel, build, s.profiles)
}
func (s *shardReleaseService) Active(ctx context.Context, id string, channel BuildChannel) (ShardRelease, error) {
	if err := RequireScope(ctx, ScopeArtifactsRead); err != nil {
		return ShardRelease{}, err
	}
	release, err := s.store.ActiveResourceBuild(ctx, id, channel)
	if err == nil && len(s.profiles) == 0 {
		return ShardRelease{}, ResourceFailure("unsupported-capability", "Shard v2 is disabled; protected release retained")
	}
	return release, err
}
