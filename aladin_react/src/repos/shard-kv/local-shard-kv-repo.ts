import { invoke } from "@tauri-apps/api/core";

import type { DataEventsRepo } from "@/repos/data-events-repo";
import type { ShardKvRow } from "@/repos/shard-kv/local-shard-kv-types";

// Shard local state read from the LOCAL `shard_kv` replica (fed by sync frames).
// Unlike the whole-list watchlist store, the consumer here is the shard BRIDGE
// HOST, which routes PER-KEY changes into subscribed iframes by prefix — so this
// repo exposes a per-shard change stream plus a seed list, not a snapshot store.
// Desktop (Tauri) only: the web host falls back to REST behind the same port.

export type ShardKvChange =
  | { kind: "upsert"; row: ShardKvRow }
  | { kind: "delete"; shardId: string; key: string };

export interface LocalShardKVRepo {
  /** Live keys for one shard under a prefix ("" = all), from the replica. */
  list(shardId: string, prefix?: string): Promise<ShardKvRow[]>;
  /** Per-key changes for one shard, as frames apply. Returns unsubscribe. */
  changes(shardId: string, cb: (change: ShardKvChange) => void): () => void;
}

// A delete event carries only the frame entity id "<shardId>#<key>".
function splitEntityId(id: string): { shardId: string; key: string } {
  const i = id.indexOf("#");
  return i < 0 ? { shardId: id, key: "" } : { shardId: id.slice(0, i), key: id.slice(i + 1) };
}

export function createLocalShardKVRepo(dataEvents: DataEventsRepo): LocalShardKVRepo {
  return {
    list(shardId, prefix = "") {
      return invoke<ShardKvRow[]>("db_list_shard_kv", { shardId, prefix });
    },
    changes(shardId, cb) {
      const sub = dataEvents.events().subscribe((e) => {
        if (e.type === "shardKvUpserted" && e.payload.shardId === shardId) {
          cb({ kind: "upsert", row: e.payload });
        } else if (e.type === "shardKvDeleted") {
          const { shardId: sid, key } = splitEntityId(e.payload.id);
          if (sid === shardId) cb({ kind: "delete", shardId: sid, key });
        }
      });
      return () => sub.unsubscribe();
    },
  };
}
