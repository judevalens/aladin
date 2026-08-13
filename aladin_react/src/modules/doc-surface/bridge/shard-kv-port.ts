import type { ShardApi, ShardKVEntryWire } from "@/shared/api/shard-api";
import type { LocalShardKVRepo } from "@/repos/shard-kv/local-shard-kv-repo";
import type { ShardKvRow } from "@/repos/shard-kv/local-shard-kv-types";

// The bridge host's storage port for shard local state. One interface, two
// assemblies: desktop reads from the local replica (instant, offline) and gets
// per-key liveness from the sync engine's change stream; web-dev reads over REST
// with no live stream (frames still update the server; a reload re-seeds).
// Writes ALWAYS go over REST — the server is the only writer of record, and the
// mutation returns as a frame that lands in the replica.

export type ShardKVPortChange =
  | { kind: "upsert"; entry: ShardKVEntryWire }
  | { kind: "delete"; key: string };

export interface ShardKVPort {
  list(shardId: string, prefix?: string): Promise<ShardKVEntryWire[]>;
  get(shardId: string, key: string): Promise<ShardKVEntryWire | null>;
  set(shardId: string, key: string, value: unknown, baseRevision: number): Promise<ShardKVEntryWire>;
  remove(shardId: string, key: string, baseRevision: number): Promise<void>;
  /** Per-key liveness (desktop replica). Web returns a no-op unsubscribe. */
  changes(shardId: string, cb: (change: ShardKVPortChange) => void): () => void;
}

function rowToWire(row: ShardKvRow): ShardKVEntryWire {
  return {
    shardId: row.shardId,
    channel: "published",
    key: row.key,
    value: row.value,
    revision: row.revision,
  };
}

export function createShardKVPort(api: ShardApi, replica: LocalShardKVRepo | null): ShardKVPort {
  return {
    async list(shardId, prefix = "") {
      if (replica) return (await replica.list(shardId, prefix)).map(rowToWire);
      return api.kvList(shardId, prefix);
    },
    async get(shardId, key) {
      if (replica) {
        const rows = await replica.list(shardId, key);
        const hit = rows.find((r) => r.key === key);
        return hit ? rowToWire(hit) : null;
      }
      return api.kvGet(shardId, key);
    },
    set: (shardId, key, value, baseRevision) => api.kvSet(shardId, key, value, baseRevision),
    remove: (shardId, key, baseRevision) => api.kvDelete(shardId, key, baseRevision),
    changes(shardId, cb) {
      if (!replica) return () => {};
      return replica.changes(shardId, (change) => {
        if (change.kind === "upsert") cb({ kind: "upsert", entry: rowToWire(change.row) });
        else cb({ kind: "delete", key: change.key });
      });
    },
  };
}
