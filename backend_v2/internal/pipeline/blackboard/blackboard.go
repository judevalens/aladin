package blackboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const TaskIngestArtifact = "pipeline:ingest"

// IngestPayload is the asynq task payload for pipeline:ingest.
type IngestPayload struct {
	KgID     string         `json:"kg_id"`
	SourceID string         `json:"source_id"`
	Artifact *SyncedArtifact `json:"artifact"`
}

const keyPrefix = "pipeline:artifact:"

type Blackboard struct {
	redis *redis.Client
}

func New(redis *redis.Client) *Blackboard {
	return &Blackboard{redis: redis}
}

func (b *Blackboard) Get(ctx context.Context, artifactID string) (*Entry, error) {
	data, err := b.redis.Get(ctx, key(artifactID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("blackboard get: %w", err)
	}
	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("blackboard unmarshal: %w", err)
	}
	return &entry, nil
}

func (b *Blackboard) Set(ctx context.Context, entry *Entry) error {
	entry.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("blackboard marshal: %w", err)
	}
	if err := b.redis.Set(ctx, key(entry.ArtifactID), data, 0).Err(); err != nil {
		slog.Error("blackboard: set failed",
			"component", "blackboard",
			"artifact_id", entry.ArtifactID,
			"kg_id", entry.KgID,
			"state", entry.State,
			"err", err,
		)
		return err
	}
	slog.Debug("blackboard: state set",
		"component", "blackboard",
		"artifact_id", entry.ArtifactID,
		"kg_id", entry.KgID,
		"state", entry.State,
	)
	return nil
}

// Claim atomically transitions an entry from `from` to `to`.
// Returns false (no error) if the entry is no longer in the expected state —
// meaning another goroutine already claimed it.
func (b *Blackboard) Claim(ctx context.Context, artifactID string, from, to State) (bool, error) {
	k := key(artifactID)
	var claimed bool

	err := b.redis.Watch(ctx, func(tx *redis.Tx) error {
		data, err := tx.Get(ctx, k).Bytes()
		if err != nil {
			return err
		}
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}
		if entry.State != from {
			claimed = false
			return nil // already claimed by someone else — not an error
		}
		entry.State = to
		entry.UpdatedAt = time.Now().UTC()
		updated, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			return pipe.Set(ctx, k, updated, 0).Err()
		})
		if err == nil {
			claimed = true
			slog.Debug("blackboard: claimed",
				"component", "blackboard",
				"artifact_id", artifactID,
				"from", from,
				"to", to,
			)
		}
		return err
	}, k)

	return claimed, err
}

func (b *Blackboard) Transition(ctx context.Context, artifactID string, to State) error {
	entry, err := b.Get(ctx, artifactID)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("blackboard: artifact %s not found", artifactID)
	}
	from := entry.State
	entry.State = to
	slog.Debug("blackboard: transition",
		"component", "blackboard",
		"artifact_id", artifactID,
		"kg_id", entry.KgID,
		"from", from,
		"to", to,
	)
	return b.Set(ctx, entry)
}

func (b *Blackboard) Delete(ctx context.Context, artifactID string) error {
	return b.redis.Del(ctx, key(artifactID)).Err()
}

// All returns all in-flight blackboard entries.
func (b *Blackboard) All(ctx context.Context) ([]*Entry, error) {
	keys, err := b.redis.Keys(ctx, keyPrefix+"*").Result()
	if err != nil {
		return nil, fmt.Errorf("blackboard scan: %w", err)
	}
	if len(keys) == 0 {
		return nil, nil
	}

	vals, err := b.redis.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("blackboard mget: %w", err)
	}

	var entries []*Entry
	for _, v := range vals {
		if v == nil {
			continue
		}
		var entry Entry
		if err := json.Unmarshal([]byte(v.(string)), &entry); err != nil {
			continue
		}
		entries = append(entries, &entry)
	}
	return entries, nil
}

// Exists returns true if the artifact is already on the blackboard.
func (b *Blackboard) Exists(ctx context.Context, artifactID string) (bool, error) {
	n, err := b.redis.Exists(ctx, key(artifactID)).Result()
	return n > 0, err
}

// IngestHandler returns an asynq handler for pipeline:ingest tasks.
// Uses a deterministic artifact ID derived from (source_id, external_id) so
// retries are idempotent — same artifact is never processed twice.
func (b *Blackboard) IngestHandler(
	enqueue func(ctx context.Context, artifactID string) error,
) func(ctx context.Context, t interface{ Payload() []byte }) error {
	return func(ctx context.Context, t interface{ Payload() []byte }) error {
		var p IngestPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("ingest: unmarshal payload: %w", err)
		}

		// Deterministic ID — same (source_id, external_id) always maps to the same
		// blackboard key. Retries find the existing entry instead of creating a duplicate.
		artifactID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(p.SourceID+":"+p.Artifact.ExternalID)).String()

		// If already in-flight, skip — pipeline is already handling it
		exists, err := b.Exists(ctx, artifactID)
		if err != nil {
			return fmt.Errorf("ingest: check exists: %w", err)
		}
		if exists {
			slog.Debug("blackboard: artifact already in-flight, skipping",
				"component", "blackboard",
				"artifact_id", artifactID,
				"external_id", p.Artifact.ExternalID,
			)
			return nil
		}

		entry := &Entry{
			ArtifactID:  artifactID,
			KgID:        p.KgID,
			State:       StatePending,
			Attempts:    0,
			MaxAttempts: 3,
			Raw:         p.Artifact,
		}

		if err := b.Set(ctx, entry); err != nil {
			return fmt.Errorf("ingest: set blackboard entry: %w", err)
		}

		slog.Info("blackboard: artifact ingested",
			"component", "blackboard",
			"artifact_id", artifactID,
			"kg_id", p.KgID,
			"source_id", p.SourceID,
			"external_id", p.Artifact.ExternalID,
		)

		return enqueue(ctx, artifactID)
	}
}

func key(artifactID string) string {
	return keyPrefix + artifactID
}
