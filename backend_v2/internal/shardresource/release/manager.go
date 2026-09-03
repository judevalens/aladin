// Package release owns Shard v2 build verification and activation sequencing.
// Authorization and persistence are ports so the lifecycle cannot be bypassed
// by API, MCP, or storage adapters.
package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"aladin/backend_v2/internal/shardresource"
	"aladin/backend_v2/internal/shardv2"
)

type Build struct {
	shardresource.ResourceRelease
	Files map[string][]byte
}

type BuildOutput struct {
	Contract []byte
	Files    map[string][]byte
	BuildID  string
}

type Store interface {
	shardresource.ReleaseReader
	StageResourceBuild(context.Context, string, shardresource.Environment, string, string, *shardv2.Compiled, map[string][]byte) error
	ActivateResourceRelease(context.Context, string, shardresource.Environment, string, shardv2.Registry) error
	ActiveResourceBuild(context.Context, string, shardresource.Environment) (Build, error)
}

type StageValidator interface {
	ValidateResourceStage(context.Context, shardv2.Resource) error
}

// ActivationFence protects owned datastores that cannot share the release
// pointer transaction. Freeze drains old writers and rejects new writers.
type ActivationFence interface {
	FreezeNamespace(context.Context, shardresource.Namespace, bool) error
}

type Authorizer interface {
	PrincipalUserID(context.Context) (string, error)
	RequireRead(context.Context) error
	RequireWrite(context.Context) error
}

type ErrorPolicy struct {
	Failure    func(string, string) error
	IsNotFound func(error) bool
}

type Manager struct {
	store      Store
	profiles   shardv2.Registry
	validators []StageValidator
	fences     []ActivationFence
	auth       Authorizer
	errors     ErrorPolicy
}

func NewManager(store Store, profiles shardv2.Registry, auth Authorizer, errorPolicy ErrorPolicy, validators ...StageValidator) *Manager {
	m := &Manager{store: store, profiles: profiles, validators: validators, auth: auth, errors: errorPolicy}
	for _, validator := range validators {
		if fence, ok := validator.(ActivationFence); ok {
			m.fences = append(m.fences, fence)
		}
	}
	return m
}

func (m *Manager) Enabled() bool { return len(m.profiles) > 0 }

// BuildIdentity covers the contract and every output, including vendored
// import-map and runtime metadata files.
func BuildIdentity(contract []byte, files map[string][]byte) string {
	raw, _ := json.Marshal(struct {
		Contract []byte
		Files    map[string][]byte
	}{contract, files})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func (m *Manager) Stage(ctx context.Context, id string, environment shardresource.Environment, output BuildOutput) error {
	if !m.Enabled() {
		return m.errors.Failure("unsupported-capability", "Shard v2 is disabled")
	}
	compiled, err := shardv2.Compile(output.Contract, m.profiles)
	if err != nil {
		return err
	}
	for _, definition := range compiled.Contract.Resources {
		for _, validator := range m.validators {
			if err := validator.ValidateResourceStage(ctx, definition); err != nil {
				return err
			}
		}
	}
	if len(output.Files["bundle.js"]) == 0 || output.BuildID != BuildIdentity(output.Contract, output.Files) {
		return m.errors.Failure("invalid-schema", "Build output identity does not match")
	}
	size := 0
	for name, data := range output.Files {
		switch name {
		case "bundle.js", "bundle.css", "importmap.json", "anchors.json", "resolver.bundle.mjs", "runtime-manifest.json", "schema.graphql":
		default:
			return m.errors.Failure("bad-request", "Unexpected build output")
		}
		size += len(data)
	}
	if size > 10<<20 {
		return m.errors.Failure("quota", "Build exceeds 10 MiB")
	}
	userID, err := m.auth.PrincipalUserID(ctx)
	if err != nil {
		return err
	}
	generation := "initial"
	previous, err := m.store.ActiveResourceRelease(ctx, userID, id, environment)
	if err == nil {
		generation = previous.Generation
	} else if !m.errors.IsNotFound(err) {
		return err
	}
	return m.store.StageResourceBuild(ctx, id, environment, output.BuildID, generation, compiled, output.Files)
}

func (m *Manager) Activate(ctx context.Context, id string, environment shardresource.Environment, buildID string) (err error) {
	if !m.Enabled() {
		return m.errors.Failure("unsupported-capability", "Shard v2 is disabled")
	}
	userID, err := m.auth.PrincipalUserID(ctx)
	if err != nil {
		return err
	}
	if err := m.auth.RequireWrite(ctx); err != nil {
		return err
	}
	namespace := shardresource.Namespace{UserID: userID, ShardID: id, Environment: environment}
	frozen := 0
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		for index := frozen - 1; index >= 0; index-- {
			if releaseErr := m.fences[index].FreezeNamespace(cleanupCtx, namespace, false); releaseErr != nil {
				err = errors.Join(err, releaseErr)
			}
		}
	}()
	for _, fence := range m.fences {
		if err := fence.FreezeNamespace(ctx, namespace, true); err != nil {
			return err
		}
		frozen++
	}
	return m.store.ActivateResourceRelease(ctx, id, environment, buildID, m.profiles)
}

func (m *Manager) Active(ctx context.Context, id string, environment shardresource.Environment) (Build, error) {
	if err := m.auth.RequireRead(ctx); err != nil {
		return Build{}, err
	}
	release, err := m.store.ActiveResourceBuild(ctx, id, environment)
	if err == nil && !m.Enabled() {
		return Build{}, m.errors.Failure("unsupported-capability", "Shard v2 is disabled; protected release retained")
	}
	return release, err
}
