package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"aladin/backend_v2/internal/artifact"
	"aladin/backend_v2/internal/shardresource/compat"
)

// Shard local state (design/SHARD_LOCAL_STATE.md): a per-shard key/value document
// store. Keys are stable, path-shaped, prefix-queryable; every key has its own
// revision so conflicts are granular; deletes tombstone. The shard iframe never
// talks to storage directly — the host bridge calls the REST routes, which call
// this service. The published channel is the user's real data (and the synced
// channel); the draft channel is the agent's server-side sandbox.

// ShardKVEntry is one key's current state.
type ShardKVEntry struct {
	ShardID   string          `json:"shardId"`
	Channel   BuildChannel    `json:"channel"`
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Revision  int64           `json:"revision"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
	Deleted   bool            `json:"deleted,omitempty"`
}

// ShardKVConflict is returned when a revision-guarded write loses: it carries the
// stored current so the client can re-apply and retry (the shard SDK hook does this
// automatically). For a tombstoned key Current.Deleted is true and Value nil —
// retrying with Current.Revision revives the key.
type ShardKVConflict struct {
	Current ShardKVEntry
}

func (e *ShardKVConflict) Error() string {
	return fmt.Sprintf("shard kv conflict on %q: current revision %d", e.Current.Key, e.Current.Revision)
}

// Quotas — explicit errors, never silent truncation.
const (
	ShardKVMaxValueBytes = 16 << 10 // per key
	ShardKVMaxShardBytes = 1 << 20  // per shard·channel, sum of value bytes
	ShardKVMaxKeyLen     = 256
)

// ShardKVService is the bridge-facing surface. Every method validates that
// shardID names an "app" artifact owned by the ctx principal.
type ShardKVService interface {
	Get(ctx context.Context, shardID string, channel BuildChannel, key string) (ShardKVEntry, bool, error)
	List(ctx context.Context, shardID string, channel BuildChannel, prefix string) ([]ShardKVEntry, error)
	Set(ctx context.Context, shardID string, channel BuildChannel, key string, value json.RawMessage, baseRevision int64) (ShardKVEntry, error)
	Delete(ctx context.Context, shardID string, channel BuildChannel, key string, baseRevision int64) error
}

// ShardKVRepository is the storage port (Postgres impl in internal/repo). Writes
// are revision-guarded, tombstone deletes, and append the sync frame in the same
// tx (published channel only).
type ShardKVRepository interface {
	Get(ctx context.Context, shardID string, channel BuildChannel, key string) (ShardKVEntry, bool, error)
	List(ctx context.Context, shardID string, channel BuildChannel, prefix string) ([]ShardKVEntry, error)
	// UsedBytes sums the stored value sizes for a shard channel (quota check).
	UsedBytes(ctx context.Context, shardID string, channel BuildChannel) (int64, error)
	Set(ctx context.Context, userID, shardID string, channel BuildChannel, key string, value json.RawMessage, baseRevision int64) (ShardKVEntry, error)
	Delete(ctx context.Context, userID, shardID string, channel BuildChannel, key string, baseRevision int64) error
}

type shardKVService struct {
	artifacts artifact.ArtifactService
	repo      ShardKVRepository
	observer  compat.V1Observer
}

func NewShardKVService(artifacts artifact.ArtifactService, repo ShardKVRepository) ShardKVService {
	return NewShardKVServiceWithObserver(artifacts, repo, compat.NewLogV1Observer(slog.Default()))
}

func NewShardKVServiceWithObserver(artifacts artifact.ArtifactService, repo ShardKVRepository, observer compat.V1Observer) ShardKVService {
	return &shardKVService{artifacts: artifacts, repo: repo, observer: observer}
}

func (s *shardKVService) observe(operation string, channel BuildChannel) {
	if s.observer != nil {
		s.observer.Used(operation, channel)
	}
}

// shardKVKeyRe: path segments of word chars/dot/dash, "/"-separated — stable,
// human-readable, prefix-queryable (SHARD_LOCAL_STATE.md "Example Keys").
var shardKVKeyRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]+(/[A-Za-z0-9_.\-]+)*$`)

func validShardKVKey(key string) error {
	if key == "" || len(key) > ShardKVMaxKeyLen {
		return BadRequest(fmt.Sprintf("invalid key: must be 1..%d chars", ShardKVMaxKeyLen))
	}
	if !shardKVKeyRe.MatchString(key) {
		return BadRequest("invalid key: use path segments of [A-Za-z0-9_.-] separated by '/'")
	}
	if strings.Contains(key, "..") {
		return BadRequest("invalid key: '..' is not allowed")
	}
	return nil
}

func validShardKVChannel(channel BuildChannel) error {
	if channel != ChannelDraft && channel != ChannelPublished {
		return BadRequest("invalid channel: use draft or published")
	}
	return nil
}

// requireOwnedShard resolves + gates the artifact (principal-scoped Get, must be
// an app) and returns the owner's user id for frame emission.
func (s *shardKVService) requireOwnedShard(ctx context.Context, shardID string) (string, error) {
	principal, err := RequirePrincipal(ctx)
	if err != nil {
		return "", err
	}
	rec, err := s.artifacts.Get(ctx, shardID)
	if err != nil {
		return "", err
	}
	if rec.Type != "app" {
		return "", ErrNotFound
	}
	return principal.UserID, nil
}

func (s *shardKVService) Get(ctx context.Context, shardID string, channel BuildChannel, key string) (ShardKVEntry, bool, error) {
	s.observe("get", channel)
	if _, err := s.requireOwnedShard(ctx, shardID); err != nil {
		return ShardKVEntry{}, false, err
	}
	if err := validShardKVChannel(channel); err != nil {
		return ShardKVEntry{}, false, err
	}
	if err := validShardKVKey(key); err != nil {
		return ShardKVEntry{}, false, err
	}
	return s.repo.Get(ctx, shardID, channel, key)
}

func (s *shardKVService) List(ctx context.Context, shardID string, channel BuildChannel, prefix string) ([]ShardKVEntry, error) {
	s.observe("list", channel)
	if _, err := s.requireOwnedShard(ctx, shardID); err != nil {
		return nil, err
	}
	if err := validShardKVChannel(channel); err != nil {
		return nil, err
	}
	// A prefix is a key path or a key path + "/"; empty lists everything.
	if prefix != "" {
		if err := validShardKVKey(strings.TrimSuffix(prefix, "/")); err != nil {
			return nil, err
		}
	}
	return s.repo.List(ctx, shardID, channel, prefix)
}

func (s *shardKVService) Set(ctx context.Context, shardID string, channel BuildChannel, key string, value json.RawMessage, baseRevision int64) (ShardKVEntry, error) {
	s.observe("set", channel)
	userID, err := s.requireOwnedShard(ctx, shardID)
	if err != nil {
		return ShardKVEntry{}, err
	}
	if err := validShardKVChannel(channel); err != nil {
		return ShardKVEntry{}, err
	}
	if err := validShardKVKey(key); err != nil {
		return ShardKVEntry{}, err
	}
	if baseRevision < 0 {
		return ShardKVEntry{}, BadRequest("baseRevision must be >= 0")
	}
	if len(value) == 0 || !json.Valid(value) {
		return ShardKVEntry{}, BadRequest("value must be valid JSON")
	}
	if len(value) > ShardKVMaxValueBytes {
		return ShardKVEntry{}, BadRequest(fmt.Sprintf("value too large: %d bytes (max %d)", len(value), ShardKVMaxValueBytes))
	}
	used, err := s.repo.UsedBytes(ctx, shardID, channel)
	if err != nil {
		return ShardKVEntry{}, err
	}
	// Approximate but safe: the old value (if any) is still counted, so the
	// check is conservative by at most one value's size.
	if used+int64(len(value)) > ShardKVMaxShardBytes {
		return ShardKVEntry{}, BadRequest(fmt.Sprintf("shard state quota exceeded: %d bytes used of %d", used, ShardKVMaxShardBytes))
	}
	return s.repo.Set(ctx, userID, shardID, channel, key, value, baseRevision)
}

func (s *shardKVService) Delete(ctx context.Context, shardID string, channel BuildChannel, key string, baseRevision int64) error {
	s.observe("delete", channel)
	userID, err := s.requireOwnedShard(ctx, shardID)
	if err != nil {
		return err
	}
	if err := validShardKVChannel(channel); err != nil {
		return err
	}
	if err := validShardKVKey(key); err != nil {
		return err
	}
	if baseRevision < 0 {
		return BadRequest("baseRevision must be >= 0")
	}
	return s.repo.Delete(ctx, userID, shardID, channel, key, baseRevision)
}
