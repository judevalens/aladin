import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire } from "@/app/state/shard-build-slice";
import type { ShardBuildWire } from "@/app/state/shard-build-slice";

// Routes "artifact.build-status" realtime events into the shard-build store
// slice. Isolated from the workspace tree-sync handler: build status is
// ephemeral UI state, not a data-layer entity.
export function createShardBuildEventHandler() {
  return function handle(event: AppEventEnvelope) {
    if (event.type !== "artifact.build-status") return;
    const wire = event.payload as ShardBuildWire | null;
    if (!wire?.page_id) return;
    useAppStore.getState().setShardBuild(shardBuildFromWire(wire));
  };
}
