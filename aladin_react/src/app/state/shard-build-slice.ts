import type { StateCreator } from "zustand";

// Live build status for Shard ("app") pages, keyed by `${pageId}:${channel}`.
// Seeded by GET /api/shards/{id}/build-state and updated live by the
// "artifact.build-status" realtime event. Ephemeral UI state (not persisted).

export type ShardBuildStatus = "" | "building" | "ok" | "failed";
export type ShardChannel = "draft" | "published";

export interface ShardBuildInfo {
  pageId: string;
  channel: ShardChannel;
  status: ShardBuildStatus;
  errors: string;
  buildId: string;
  builtAt: string;
}

// The wire shape from the backend (service.ShardBuildState json tags).
export interface ShardBuildWire {
  page_id: string;
  channel: string;
  status: string;
  errors?: string;
  build_id?: string;
  built_at?: string;
}

export function shardBuildKey(pageId: string, channel: ShardChannel): string {
  return `${pageId}:${channel}`;
}

export function shardBuildFromWire(w: ShardBuildWire): ShardBuildInfo {
  return {
    pageId: w.page_id,
    channel: w.channel === "published" ? "published" : "draft",
    status: (w.status as ShardBuildStatus) ?? "",
    errors: w.errors ?? "",
    buildId: w.build_id ?? "",
    builtAt: w.built_at ?? "",
  };
}

export interface ShardBuildSlice {
  shardBuilds: Record<string, ShardBuildInfo>;
  setShardBuild: (info: ShardBuildInfo) => void;
}

export const createShardBuildSlice: StateCreator<ShardBuildSlice, [], [], ShardBuildSlice> = (
  set,
) => ({
  shardBuilds: {},
  setShardBuild: (info) =>
    set((state) => ({
      shardBuilds: {
        ...state.shardBuilds,
        [shardBuildKey(info.pageId, info.channel)]: info,
      },
    })),
});
