import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { useAppStore } from "@/app/state/store";
import { shardBuildFromWire } from "@/app/state/shard-build-slice";
import type { ShardBuildWire } from "@/app/state/shard-build-slice";

// Routes "artifact.build-status" realtime events into the shard-build store
// slice. Isolated from the workspace tree-sync handler: build status is
// ephemeral UI state, not a data-layer entity.
export function createShardBuildEventHandler() {
  return function handle(event: AppEventEnvelope) {
    if (event.type === "artifact.published") {
      const wire = event.payload as { page_id?: unknown; protocol?: unknown; buildId?: unknown; contractHash?: unknown } | null;
      if (wire?.page_id !== event.subscriptionKey.resourceId || typeof wire.page_id !== "string" || !wire.page_id ||
        wire.protocol !== "bridge/2" || typeof wire.buildId !== "string" || !wire.buildId ||
        typeof wire.contractHash !== "string" || !wire.contractHash) return;
      useAppStore.getState().setShardPublication({ pageId: wire.page_id, eventId: event.eventId, protocol: wire.protocol, buildId: wire.buildId, contractHash: wire.contractHash });
      return;
    }
    if (event.type !== "artifact.build-status") return;
    const wire = event.payload as ShardBuildWire | null;
    if (!wire?.page_id) return;
    useAppStore.getState().setShardBuild(shardBuildFromWire(wire));
  };
}
