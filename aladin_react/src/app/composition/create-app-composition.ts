import { createLocalDesktopSessionStore } from "@/shared/runtime/desktop-session-store";
import { createRuntimeConfig } from "@/shared/runtime/runtime-config";
import { createAuthRepo } from "@/repos/auth/auth-repo";
import { createArtifactApi } from "@/shared/api/artifact-api";
import { createWorkspaceRepo } from "@/repos/workspace/workspace-repo";
import { createArtifactRepo } from "@/repos/artifacts/artifact-repo";
import { createSourcesRepo } from "@/repos/sources/sources-repo";
import { createGraphPaneRepo } from "@/repos/graph/graph-pane-repo";
import { createPipelineRepo } from "@/repos/pipeline/pipeline-repo";
import { createInsightsRepo } from "@/repos/insights/insights-repo";
import { createInstrumentsRepo } from "@/repos/instruments/instruments-repo";
import { createWatchlistRepo } from "@/repos/watchlist/watchlist-repo";
import { createSearchRepo } from "@/repos/search/search-repo";
import { createLocalSignalsRepo } from "@/repos/signals/local-signals-repo";
import { createBookSignalsRepo } from "@/repos/signals/book-signals-repo";
import { createIntegrationRepo } from "@/repos/integrations/integration-repo";
import { createPageAttributionRepo } from "@/repos/pages/page-attribution-repo";
import { createApiClient } from "@/shared/api/client";
import { createDataEventsRepo } from "@/repos/data-events-repo";
import { createLocalRepos } from "@/repos/local-repos";
import { createLocalSyncRepo } from "@/repos/sync/local-sync-repo";
import { nodeRowToArtifactRow, rowToArtifact } from "@/repos/artifacts/artifact-mappers";
import * as authService from "@/services/auth/auth-service";
import { AuthSessionService } from "@/services/auth/auth-session-service";
import { WorkspaceService } from "@/services/workspace/workspace-service";
import { SourcesCatalogService } from "@/services/sources/sources-catalog-service";
import * as integrationService from "@/services/integrations/integration-service";
import { WorkspaceSyncService } from "@/services/workspace/workspace-sync-service";
import { createRealtimeBoot } from "@/app/composition/realtime-boot";
import { createShardApi } from "@/shared/api/shard-api";
import { createAppEventProcessor } from "@/shared/realtime/app-event-processor";
import { createWebSocketAppEventSource } from "@/shared/realtime/websocket-app-event-source";
import { createShardBuildEventHandler } from "@/shared/realtime/shard-build-event-handler";
import { createQuoteEventHandler } from "@/shared/realtime/quote-event-handler";
import { createCopilotEventHandler } from "@/shared/realtime/copilot-event-handler";
import { createNotificationEventHandler } from "@/shared/realtime/notification-event-handler";
import { createMarketRepo } from "@/repos/market/market-repo";
import { createCopilotRepo } from "@/repos/copilot/copilot-repo";
import { createNotificationsRepo } from "@/repos/notifications/notifications-repo";
import { useAppStore } from "@/app/state/store";
import type { ApiRuntimeConfig } from "@/shared/api/client";

// eventsWebSocketUrl resolves the realtime app-event endpoint. Desktop has an
// absolute websocketBaseUrl (→ :8000); web dev has "" so we use the page origin
// (the vite dev server proxies /api, ws included).
function eventsWebSocketUrl(config: ApiRuntimeConfig): string {
  if (config.websocketBaseUrl) {
    return `${config.websocketBaseUrl}/api/events/ws`;
  }
  if (typeof window !== "undefined") {
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${window.location.host}/api/events/ws`;
  }
  return "/api/events/ws";
}

export function createAppComposition() {
  const config = createRuntimeConfig();
  const desktopSession = createLocalDesktopSessionStore();
  const apiClient = createApiClient(config, desktopSession);
  const dataEvents = createDataEventsRepo();
  const local = createLocalRepos();
  const localSync = createLocalSyncRepo();
  const apis = {
    artifacts: createArtifactApi(apiClient),
    shards: createShardApi(apiClient),
  };

  const repos = {
    auth: createAuthRepo(apiClient),
    workspace: createWorkspaceRepo(
      local.repos.browser,
      local.repos.nodes,
      apiClient,
      localSync,
    ),
    artifacts: createArtifactRepo(apis.artifacts, apiClient, local.repos.artifacts),
    sources: createSourcesRepo(apiClient),
    integrations: createIntegrationRepo(apiClient),
    pages: createPageAttributionRepo(apiClient),
    graphPane: createGraphPaneRepo(apiClient),
    pipeline: createPipelineRepo(apiClient),
    insights: createInsightsRepo(apiClient),
    instruments: createInstrumentsRepo(apiClient),
    watchlist: createWatchlistRepo(apiClient),
    search: createSearchRepo(apiClient),
    market: createMarketRepo(apiClient),
    copilot: createCopilotRepo(apiClient),
    notifications: createNotificationsRepo(apiClient),
    localSignals: createLocalSignalsRepo(dataEvents),
    bookSignals: createBookSignalsRepo(apiClient),
  };

  const authSession = new AuthSessionService(repos.auth, desktopSession);
  const workspaceSync = new WorkspaceSyncService(repos.workspace, repos.artifacts);
  const workspace = new WorkspaceService(
    repos.workspace,
    repos.artifacts,
    workspaceSync,
  );
  const sourcesCatalog = new SourcesCatalogService(repos.sources, repos.integrations);

  void dataEvents.connect();
  dataEvents.events().subscribe((event) => {
    // Data-layer redesign: the unified `nodes` model drives the tree. Any node
    // change reconciles the whole tree from the authoritative local model
    // (coalesced); an artifact node also refreshes its open work-pane view.
    // (Page CONTENT rides Yjs/Hocuspocus — a separate channel.)
    if (event.type === "nodeUpserted") {
      workspaceSync.handleNodeChanged();
      if (event.payload.kind === "artifact") {
        workspaceSync.publishArtifact(
          rowToArtifact(apiClient, nodeRowToArtifactRow(event.payload)),
        );
      }
      return;
    }
    if (event.type === "nodeDeleted") {
      workspaceSync.handleNodeChanged();
      return;
    }
    // A new/updated claim pushed in: light the Signals rail unread dot. If the Signals view is
    // open, useSignals clears it on the live refresh; otherwise it persists until the user looks.
    if (event.type === "signalUpserted") {
      useAppStore.getState().bumpSignals();
      return;
    }
  });

  // Server-push app events (the realtime websocket). Currently drives shard
  // build-status into the store slice; the same pipeline carries M3 live regions.
  // Only the build-status handler is registered — the workspace tree-sync handler
  // stays on the pull/local path, so the verified data layer's behavior is
  // unchanged.
  const appEvents = createAppEventProcessor();
  appEvents.register(createShardBuildEventHandler());
  appEvents.register(createQuoteEventHandler());
  // Copilot streaming rides the existing workspace "*" subscription (the backend publishes an
  // AnyResource key for copilot.* events), so no new subscription is needed — just the handler.
  appEvents.register(createCopilotEventHandler());
  appEvents.register(createNotificationEventHandler());
  let wasDisconnected = false;
  const appEventSource = createWebSocketAppEventSource({
    url: eventsWebSocketUrl(config),
    token: () => desktopSession.getToken(),
    // Explicit list (a non-empty list disables the backend's workspace default), so we keep
    // the workspace wildcard (build-status etc.) AND add the broadcast market stream (quotes).
    subscriptions: [
      { stream: "workspace", resourceKind: "*", resourceId: "*" },
      { stream: "market", resourceKind: "quote", resourceId: "*" },
    ],
    onEvent: (event) => appEvents.dispatch(event),
    // The WS resubscribes on reconnect but has no replay cursor — events in the gap are
    // lost. Bumping the reconnect nonce lets consumers (the copilot dock) reconcile
    // against the server's durable state instead of waiting on events that never come.
    onConnectionChange: (state) => {
      if (state === "open" && wasDisconnected) {
        useAppStore.getState().noteCopilotWsReconnect();
      }
      wasDisconnected = state === "closed";
    },
  });

  const realtime = createRealtimeBoot({
    authSession,
    admin: local.admin,
    localSync,
    desktopSession,
    config,
    appEventSource,
  });
  realtime.start();

  const services = {
    auth: {
      ...authService,
      session: authSession,
    },
    workspace,
    sources: sourcesCatalog,
    integrations: integrationService,
  };

  return {
    runtime: {
      config,
      desktopSession,
      localPersistenceAdmin: local.admin,
      dataEvents,
      localSync,
      apis,
      realtime,
    },
    repos,
    services,
  };
}

export type AppComposition = ReturnType<typeof createAppComposition>;
