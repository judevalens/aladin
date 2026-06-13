import type { ApiClient } from "@/shared/api/client";
import type { ShardBuildWire, ShardChannel } from "@/app/state/shard-build-slice";

// Seed endpoint for the live build view: GET /api/shards/{id}/build-state.
// Live updates arrive separately via the "artifact.build-status" realtime event;
// this seeds the slice when a shard tab first opens.
export interface ShardApi {
  getBuildState(shardId: string, channel: ShardChannel): Promise<ShardBuildWire>;
}

export function createShardApi(client: ApiClient): ShardApi {
  return {
    getBuildState: (shardId, channel) =>
      client.fetch<ShardBuildWire>(
        `/api/shards/${shardId}/build-state?channel=${channel}`,
      ),
  };
}
