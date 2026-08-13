package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"aladin/backend_v2/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Shard local state (design/SHARD_LOCAL_STATE.md) — revision-guarded KV writes +
// the R1 sync frames for the published channel.
//
// Granularity: one synced entity PER KEY (kind "shard_kv", id "<shard>#<key>"),
// mirroring the doc's "each key has its own revision, so conflicts are granular".
// `revision` doubles as the frame seq (every set/delete bumps it, so the client
// seq guard orders correctly); `deleted_at` is the tombstone. Only the PUBLISHED
// channel emits frames — draft rows are the agent's server-side sandbox and stay
// out of the replica. Writes serialize per user via LockUser (the same advisory
// lock every other producer takes), so guarded read-then-write is race-free.

const shardKVEntityKind = "shard_kv"

func shardKVEntityID(shardID, key string) string { return shardID + "#" + key }

type lightShardKVData struct {
	ShardID  string          `json:"shardId"`
	Channel  string          `json:"channel"`
	Key      string          `json:"key"`
	Value    json.RawMessage `json:"value"`
	Revision int64           `json:"revision"`
}

// lightShardKVSelect projects shard_kv rows for a user's shards (published
// channel — the synced one). Tombstones are NOT filtered: they must reach the
// snapshot so the client seq guard blocks resurrection. $1 = user_id.
const lightShardKVSelect = `
	SELECT k.shard_id, k.channel, k.key, k.value, k.revision, (k.deleted_at IS NOT NULL) AS is_deleted
	  FROM shard_kv k
	  JOIN artifacts a ON a.id = k.shard_id
	 WHERE a.user_id = $1::uuid AND k.channel = 'published'`

func scanLightShardKV(row scanner) (service.FrameEntity, error) {
	var (
		d         lightShardKVData
		isDeleted bool
	)
	if err := row.Scan(&d.ShardID, &d.Channel, &d.Key, &d.Value, &d.Revision, &isDeleted); err != nil {
		return service.FrameEntity{}, err
	}
	ent := service.FrameEntity{
		EntityKind: shardKVEntityKind,
		EntityID:   shardKVEntityID(d.ShardID, d.Key),
		Seq:        uint64(d.Revision),
	}
	if isDeleted {
		ent.Op = service.OpDelete
		return ent, nil
	}
	ent.Op = service.OpUpsert
	data, err := json.Marshal(d)
	if err != nil {
		return service.FrameEntity{}, err
	}
	ent.Data = data
	return ent, nil
}

// emitShardKVFrame reads one key's light projection (revision already final) and
// appends its frame in the caller's tx. Published channel only; draft is a no-op.
func emitShardKVFrame(ctx context.Context, tx pgx.Tx, userID, shardID string, channel service.BuildChannel, key string) error {
	if channel != service.ChannelPublished {
		return nil
	}
	ent, err := scanLightShardKV(tx.QueryRow(ctx, lightShardKVSelect+` AND k.shard_id = $2 AND k.key = $3`, userID, shardID, key))
	if err != nil {
		return fmt.Errorf("sync: light shard_kv %s#%s: %w", shardID, key, err)
	}
	return appendOutboxEvent(ctx, tx, userID, service.Frame{Entities: []service.FrameEntity{ent}})
}

// ShardKVRepo implements service.ShardKVRepository on Postgres.
type ShardKVRepo struct{ pool *pgxpool.Pool }

func NewShardKVPostgres(pool *pgxpool.Pool) *ShardKVRepo { return &ShardKVRepo{pool: pool} }

func scanEntry(row pgx.Row, shardID string, channel service.BuildChannel) (service.ShardKVEntry, error) {
	var (
		e         service.ShardKVEntry
		updatedAt time.Time
		deletedAt *time.Time
	)
	e.ShardID, e.Channel = shardID, channel
	if err := row.Scan(&e.Key, &e.Value, &e.Revision, &updatedAt, &deletedAt); err != nil {
		return service.ShardKVEntry{}, err
	}
	e.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	if deletedAt != nil {
		e.Deleted = true
		e.Value = nil
	}
	return e, nil
}

func (r *ShardKVRepo) Get(ctx context.Context, shardID string, channel service.BuildChannel, key string) (service.ShardKVEntry, bool, error) {
	e, err := scanEntry(r.pool.QueryRow(ctx, `
		SELECT key, value, revision, updated_at, deleted_at
		  FROM shard_kv
		 WHERE shard_id = $1 AND channel = $2 AND key = $3
	`, shardID, string(channel), key), shardID, channel)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ShardKVEntry{}, false, nil
	}
	if err != nil {
		return service.ShardKVEntry{}, false, err
	}
	if e.Deleted {
		return service.ShardKVEntry{}, false, nil
	}
	return e, true, nil
}

func (r *ShardKVRepo) List(ctx context.Context, shardID string, channel service.BuildChannel, prefix string) ([]service.ShardKVEntry, error) {
	// text_pattern_ops + LIKE on an escaped prefix — index-friendly. Tombstones
	// filtered: list is the live view (the doc's default).
	rows, err := r.pool.Query(ctx, `
		SELECT key, value, revision, updated_at, deleted_at
		  FROM shard_kv
		 WHERE shard_id = $1 AND channel = $2
		   AND key LIKE $3 ESCAPE '\'
		   AND deleted_at IS NULL
		 ORDER BY key
	`, shardID, string(channel), likePrefix(prefix))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]service.ShardKVEntry, 0)
	for rows.Next() {
		e, err := scanEntry(rows, shardID, channel)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *ShardKVRepo) UsedBytes(ctx context.Context, shardID string, channel service.BuildChannel) (int64, error) {
	var used int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(octet_length(value::text)), 0)
		  FROM shard_kv
		 WHERE shard_id = $1 AND channel = $2 AND deleted_at IS NULL
	`, shardID, string(channel)).Scan(&used)
	return used, err
}

// likePrefix escapes LIKE metacharacters in prefix and appends %.
func likePrefix(prefix string) string {
	esc := ""
	for _, c := range prefix {
		if c == '%' || c == '_' || c == '\\' {
			esc += `\`
		}
		esc += string(c)
	}
	return esc + "%"
}

func (r *ShardKVRepo) Set(ctx context.Context, userID, shardID string, channel service.BuildChannel, key string, value json.RawMessage, baseRevision int64) (service.ShardKVEntry, error) {
	var out service.ShardKVEntry
	err := r.withUserTx(ctx, userID, func(tx pgx.Tx) error {
		current, exists, err := currentRevision(ctx, tx, shardID, channel, key)
		if err != nil {
			return err
		}
		if current != baseRevision {
			return conflictFor(ctx, tx, shardID, channel, key, exists)
		}
		var row pgx.Row
		if exists {
			row = tx.QueryRow(ctx, `
				UPDATE shard_kv
				   SET value = $4::jsonb, revision = revision + 1,
				       updated_by = $5::uuid, updated_at = now(), deleted_at = NULL
				 WHERE shard_id = $1 AND channel = $2 AND key = $3
				RETURNING key, value, revision, updated_at, deleted_at
			`, shardID, string(channel), key, []byte(value), userID)
		} else {
			row = tx.QueryRow(ctx, `
				INSERT INTO shard_kv (shard_id, channel, key, value, revision, created_by, updated_by)
				VALUES ($1, $2, $3, $4::jsonb, 1, $5::uuid, $5::uuid)
				RETURNING key, value, revision, updated_at, deleted_at
			`, shardID, string(channel), key, []byte(value), userID)
		}
		if out, err = scanEntry(row, shardID, channel); err != nil {
			return err
		}
		return emitShardKVFrame(ctx, tx, userID, shardID, channel, key)
	})
	return out, err
}

func (r *ShardKVRepo) Delete(ctx context.Context, userID, shardID string, channel service.BuildChannel, key string, baseRevision int64) error {
	return r.withUserTx(ctx, userID, func(tx pgx.Tx) error {
		current, exists, err := currentRevision(ctx, tx, shardID, channel, key)
		if err != nil {
			return err
		}
		if !exists {
			return nil // nothing to delete — idempotent
		}
		var alreadyDeleted bool
		if err := tx.QueryRow(ctx, `
			SELECT deleted_at IS NOT NULL FROM shard_kv
			 WHERE shard_id = $1 AND channel = $2 AND key = $3
		`, shardID, string(channel), key).Scan(&alreadyDeleted); err != nil {
			return err
		}
		if alreadyDeleted {
			return nil // idempotent
		}
		if current != baseRevision {
			return conflictFor(ctx, tx, shardID, channel, key, true)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE shard_kv
			   SET deleted_at = now(), revision = revision + 1,
			       updated_by = $4::uuid, updated_at = now()
			 WHERE shard_id = $1 AND channel = $2 AND key = $3
		`, shardID, string(channel), key, userID); err != nil {
			return err
		}
		return emitShardKVFrame(ctx, tx, userID, shardID, channel, key)
	})
}

// withUserTx wraps fn in a tx holding the per-user advisory lock — the same
// serialization every outbox producer uses, making guarded read-then-write safe.
func (r *ShardKVRepo) withUserTx(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := LockUser(ctx, tx, userID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// currentRevision reads a key's stored revision (0, false when the row is
// absent). Tombstoned rows still carry their revision — a guarded write against
// the tombstone's revision revives the key.
func currentRevision(ctx context.Context, tx pgx.Tx, shardID string, channel service.BuildChannel, key string) (int64, bool, error) {
	var rev int64
	err := tx.QueryRow(ctx, `
		SELECT revision FROM shard_kv
		 WHERE shard_id = $1 AND channel = $2 AND key = $3
	`, shardID, string(channel), key).Scan(&rev)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return rev, true, nil
}

// conflictFor builds the ShardKVConflict carrying the stored current (per the
// doc: 409 returns the latest value + revision so the client re-applies).
func conflictFor(ctx context.Context, tx pgx.Tx, shardID string, channel service.BuildChannel, key string, exists bool) error {
	cur := service.ShardKVEntry{ShardID: shardID, Channel: channel, Key: key}
	if exists {
		e, err := scanEntry(tx.QueryRow(ctx, `
			SELECT key, value, revision, updated_at, deleted_at
			  FROM shard_kv
			 WHERE shard_id = $1 AND channel = $2 AND key = $3
		`, shardID, string(channel), key), shardID, channel)
		if err != nil {
			return err
		}
		cur = e
	}
	return &service.ShardKVConflict{Current: cur}
}

// ShardKVSyncSource is the cold-start snapshot provider (published channel,
// tombstones included). Implements service.SyncSource.
type ShardKVSyncSource struct{ pool *pgxpool.Pool }

func NewShardKVSyncSource(pool *pgxpool.Pool) *ShardKVSyncSource {
	return &ShardKVSyncSource{pool: pool}
}

func (s *ShardKVSyncSource) EntityKind() string { return shardKVEntityKind }

func (s *ShardKVSyncSource) Snapshot(ctx context.Context, userID string) ([]service.FrameEntity, error) {
	rows, err := s.pool.Query(ctx, lightShardKVSelect+` ORDER BY k.shard_id, k.key`, userID)
	if err != nil {
		return nil, fmt.Errorf("sync: shard_kv snapshot query: %w", err)
	}
	defer rows.Close()
	out := make([]service.FrameEntity, 0)
	for rows.Next() {
		ent, err := scanLightShardKV(rows)
		if err != nil {
			return nil, fmt.Errorf("sync: shard_kv snapshot scan: %w", err)
		}
		out = append(out, ent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sync: shard_kv snapshot rows: %w", err)
	}
	return out, nil
}
