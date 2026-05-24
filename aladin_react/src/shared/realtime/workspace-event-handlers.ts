import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { WorkspaceSyncService } from "@/services/workspace/workspace-sync-service";
import type { PageMetadataRepo } from "@/repos/pages/page-metadata-repo";
import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { isWorkspaceEventType } from "@/shared/realtime/app-event";

export interface WorkspaceEventHandlerDeps {
  workspace: WorkspaceSyncService;
  artifactRepo: ArtifactRepo;
  pages: PageMetadataRepo;
}

export function createWorkspaceEventHandler({
  workspace,
  artifactRepo,
  pages,
}: WorkspaceEventHandlerDeps) {
  return async function handle(event: AppEventEnvelope) {
    if (!isWorkspaceEventType(event.type)) return;

    switch (event.type) {
      case "artifact.created":
      case "folder.created":
      case "folder.updated":
      case "folder.deleted":
        // Tree-affecting events: whole-tree refresh per v1 design doc.
        await workspace.refreshTree();
        return;

      case "artifact.updated": {
        const id = event.subscriptionKey.resourceId;
        if (!id || id === "*") return;
        const artifact = await artifactRepo.getArtifact(id, { policy: "remote" });
        workspace.publishArtifact(artifact);
        return;
      }

      case "artifact.deleted": {
        const id = event.subscriptionKey.resourceId;
        if (!id || id === "*") return;
        await workspace.refreshTree();
        return;
      }

      case "page.updated": {
        // Metadata-only refresh; do not touch in-flight editor drafts.
        const id = event.subscriptionKey.resourceId;
        if (!id || id === "*") return;
        const payload = event.payload as
          | { id?: string; title?: string; revision?: number; updatedAt?: string }
          | null;
        if (payload?.id && typeof payload.revision === "number") {
          await pages.upsertMetadata({
            id: payload.id,
            title: payload.title ?? "",
            revision: payload.revision,
            updatedAt: payload.updatedAt
              ? Date.parse(payload.updatedAt) || Date.now()
              : Date.now(),
            syncStatus: "SYNCED",
            version: 0,
          });
        }
        return;
      }
    }
  };
}
