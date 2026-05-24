import type { Subscription } from "rxjs";
import type { ArtifactRepo } from "@/repos/artifacts/artifact-repo";
import type { AuthSessionService } from "@/services/auth/auth-session-service";
import type { WorkspaceSyncService } from "@/services/workspace/workspace-sync-service";
import type { ApiRuntimeConfig } from "@/shared/api/client";
import type { LocalReposAdmin } from "@/repos/local-repos-admin";
import type { LocalSyncRepo } from "@/repos/sync/local-sync-repo";
import type { PageMetadataRepo } from "@/repos/pages/page-metadata-repo";
import type { DesktopSessionStore } from "@/shared/runtime/desktop-session-store";
import {
  createAppEventProcessor,
  type AppEventProcessor,
} from "@/shared/realtime/app-event-processor";
import {
  createWebSocketAppEventSource,
  type WebSocketAppEventSource,
} from "@/shared/realtime/websocket-app-event-source";
import { createWorkspaceEventHandler } from "@/shared/realtime/workspace-event-handlers";

export interface RealtimeBootDeps {
  authSession: AuthSessionService;
  workspace: WorkspaceSyncService;
  artifactRepo: ArtifactRepo;
  pages: PageMetadataRepo;
  admin: LocalReposAdmin;
  localSync: LocalSyncRepo;
  desktopSession: DesktopSessionStore;
  config: ApiRuntimeConfig;
}

export interface RealtimeBoot {
  start(): void;
  stop(): void;
}

export function createRealtimeBoot(deps: RealtimeBootDeps): RealtimeBoot {
  let sessionSub: Subscription | null = null;
  let source: WebSocketAppEventSource | null = null;
  let processor: AppEventProcessor | null = null;
  let unregisterHandler: (() => void) | null = null;
  let currentUserId: string | null = null;

  function startConnection() {
    if (source) return;
    processor = createAppEventProcessor();
    const handle = createWorkspaceEventHandler({
      workspace: deps.workspace,
      artifactRepo: deps.artifactRepo,
      pages: deps.pages,
    });
    unregisterHandler = processor.register(handle);

    const wsUrl = `${deps.config.websocketBaseUrl}/api/events/ws`;
    source = createWebSocketAppEventSource({
      url: wsUrl,
      token: () => deps.desktopSession.getToken(),
      subscriptions: [{ stream: "workspace", resourceKind: "*", resourceId: "*" }],
      onEvent: (event) => processor?.dispatch(event),
    });
    source.start();

    void deps.workspace.refreshTree();
  }

  function stopConnection() {
    source?.stop();
    source = null;
    unregisterHandler?.();
    unregisterHandler = null;
    processor = null;
  }

  return {
    start() {
      if (sessionSub) return;
      sessionSub = deps.authSession.session().subscribe((snapshot) => {
        if (snapshot.status === "authenticated") {
          const userId = snapshot.user?.id ?? null;
          if (userId !== currentUserId) {
            currentUserId = userId;
            stopConnection();
          }
          void deps.localSync.setSession({
            apiBaseUrl: deps.config.apiBaseUrl,
            token: deps.desktopSession.getToken(),
          });
          startConnection();
          void deps.localSync.drainOutbox().catch(() => undefined);
        } else if (snapshot.status === "anonymous") {
          stopConnection();
          currentUserId = null;
          void deps.localSync.setSession(null);
          void deps.admin.clearWorkspace().catch(() => undefined);
        }
      });
    },
    stop() {
      sessionSub?.unsubscribe();
      sessionSub = null;
      stopConnection();
    },
  };
}
