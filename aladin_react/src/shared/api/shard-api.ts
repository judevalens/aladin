import { ApiError } from "@/shared/api/client";
import type { ApiClient } from "@/shared/api/client";
import type { ShardBuildWire, ShardChannel } from "@/app/state/shard-build-slice";

// Shard REST surface: the build-state seed (live updates ride the
// "artifact.build-status" realtime event) and the shard-KV routes
// (design/SHARD_LOCAL_STATE.md) the bridge host proxies. All KV calls target the
// PUBLISHED channel — the user's real data; draft is the agent's sandbox and is
// never touched from the app host.

export interface ShardKVEntryWire {
  shardId: string;
  channel: string;
  key: string;
  value: unknown;
  revision: number;
  updatedAt?: string;
  deleted?: boolean;
}

// The 409 body per the doc: latest value + revision so the caller re-applies.
export class ShardKVConflictError extends Error {
  key: string;
  currentRevision: number;
  currentValue: unknown;
  deleted: boolean;

  constructor(key: string, currentRevision: number, currentValue: unknown, deleted: boolean) {
    super(`shard kv conflict on ${key}: current revision ${currentRevision}`);
    this.name = "ShardKVConflictError";
    this.key = key;
    this.currentRevision = currentRevision;
    this.currentValue = currentValue;
    this.deleted = deleted;
  }
}

function rethrowConflict(err: unknown): never {
  if (err instanceof ApiError && err.status === 409 && err.body && typeof err.body === "object") {
    const b = err.body as {
      key?: string;
      currentRevision?: number;
      currentValue?: unknown;
      deleted?: boolean;
    };
    throw new ShardKVConflictError(b.key ?? "", b.currentRevision ?? 0, b.currentValue, !!b.deleted);
  }
  throw err;
}

// One workspace entity as a shard sees it (service.NodeView). `id` echoes the
// REF the shard asked with, so qualified refs (watchlist:<uuid>) line up.
export interface NodeViewWire {
  id: string;
  kind: string;
  title: string;
  data?: unknown;
  seq?: string;
  truncated?: boolean;
}

// The parsed anchors.json (service/docsurface Manifest) — the host reads it for
// per-anchor provenance and staleness.
export interface ShardManifestWire {
  version: number;
  intent?: string;
  anchors: Array<{
    id: string;
    kind?: string;
    route: string;
    source?: string;
    binding?: unknown;
    refs?: string[];
    meaning: string;
  }>;
}

export interface ShardApi {
  getBuildState(shardId: string, channel: ShardChannel): Promise<ShardBuildWire>;
  getManifest(shardId: string): Promise<ShardManifestWire | null>;
  bridgeNodes(shardId: string, ids: string[]): Promise<{ nodes: NodeViewWire[]; missing: string[] }>;
  kvList(shardId: string, prefix?: string): Promise<ShardKVEntryWire[]>;
  kvGet(shardId: string, key: string): Promise<ShardKVEntryWire | null>;
  kvSet(shardId: string, key: string, value: unknown, baseRevision: number): Promise<ShardKVEntryWire>;
  kvDelete(shardId: string, key: string, baseRevision: number): Promise<void>;
}

export function createShardApi(client: ApiClient): ShardApi {
  return {
    getBuildState: (shardId, channel) =>
      client.fetch<ShardBuildWire>(
        `/api/shards/${shardId}/build-state?channel=${channel}`,
      ),
    getManifest: (shardId) =>
      client
        .fetch<ShardManifestWire>(`/api/shards/${shardId}/manifest`)
        .catch((err: unknown) => {
          if (err instanceof ApiError && err.status === 404) return null;
          throw err;
        }),
    bridgeNodes: (shardId, ids) =>
      client.fetch<{ nodes: NodeViewWire[]; missing: string[] }>(
        `/api/shards/${shardId}/bridge/nodes`,
        { method: "POST", body: JSON.stringify({ ids }) },
      ),
    kvList: (shardId, prefix = "") =>
      client
        .fetch<{ entries: ShardKVEntryWire[] }>(
          `/api/shards/${shardId}/kv?prefix=${encodeURIComponent(prefix)}`,
        )
        .then((r) => r.entries),
    kvGet: (shardId, key) =>
      client
        .fetch<ShardKVEntryWire>(`/api/shards/${shardId}/kv/${key}`)
        .catch((err: unknown) => {
          if (err instanceof ApiError && err.status === 404) return null;
          throw err;
        }),
    kvSet: (shardId, key, value, baseRevision) =>
      client
        .fetch<ShardKVEntryWire>(`/api/shards/${shardId}/kv/${key}`, {
          method: "PUT",
          body: JSON.stringify({ value, baseRevision }),
        })
        .catch(rethrowConflict),
    kvDelete: (shardId, key, baseRevision) =>
      client
        .fetch<void>(`/api/shards/${shardId}/kv/${key}?baseRevision=${baseRevision}`, {
          method: "DELETE",
        })
        .catch(rethrowConflict),
  };
}
