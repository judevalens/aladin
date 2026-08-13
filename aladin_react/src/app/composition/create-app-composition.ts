import { createLocalDesktopSessionStore } from "@/shared/runtime/desktop-session-store";
import { createRuntimeConfig } from "@/shared/runtime/runtime-config";
import { createAuthRepo } from "@/repos/auth/auth-repo";
import { createArtifactApi } from "@/shared/api/artifact-api";
import { createWorkspaceRepo } from "@/repos/workspace/workspace-repo";
import { createArtifactRepo } from "@/repos/artifacts/artifact-repo";
import { createSourcesRepo } from "@/repos/sources/sources-repo";
import { createGraphPaneRepo } from "@/repos/graph/graph-pane-repo";
import { createPipelineRepo } from "@/repos/pipeline/pipeline-repo";
import { createResearchRepo } from "@/repos/research/research-repo";
import { createDocumentRepo } from "@/repos/documents/document-repo";
import { createInsightsRepo } from "@/repos/insights/insights-repo";
import { createInstrumentsRepo } from "@/repos/instruments/instruments-repo";
import { createWatchlistRepo } from "@/repos/watchlist/watchlist-repo";
import { createSearchRepo } from "@/repos/search/search-repo";
import { createLocalWatchlistsRepo } from "@/repos/watchlist/local-watchlist-repo";
import { createPropertyQueryRepo } from "@/repos/artifacts/property-query-repo";
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
import { createLocalShardKVRepo } from "@/repos/shard-kv/local-shard-kv-repo";
import { createShardKVPort } from "@/modules/doc-surface/bridge/shard-kv-port";
import { createShardDataHub, createShardFrameHandler } from "@/modules/doc-surface/bridge/shard-data-hub";
import { createContentTokenStore } from "@/shared/runtime/content-token-store";
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
  const shardApi = createShardApi(apiClient);
  // Scoped credential for shard iframe URLs (never the session bearer — shard
  // JS can read its own URL).
  const contentTokens = createContentTokenStore(apiClient);
  const apis = {
    artifacts: createArtifactApi(apiClient),
    shards: shardApi,
    // The bridge host's storage port for shard local state: replica reads +
    // live per-key changes on desktop (the sync engine), REST fallback on web;
    // REST writes everywhere (published channel — the host owns the channel).
    shardKV: createShardKVPort(
      shardApi,
      typeof window !== "undefined" && "__TAURI_INTERNALS__" in window
        ? createLocalShardKVRepo(dataEvents)
        : null,
    ),
    // Workspace-plane liveness: sync frames (already on the ws) → per-shard
    // single-entity refetch → push into the subscribed iframe.
    shardDataHub: createShardDataHub(shardApi),
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
    research: createResearchRepo(apiClient),
    documents: createDocumentRepo(apiClient),
    instruments: createInstrumentsRepo(apiClient),
    watchlist: createWatchlistRepo(apiClient),
    localWatchlists: createLocalWatchlistsRepo(dataEvents),
    search: createSearchRepo(apiClient),
    market: createMarketRepo(apiClient),
    copilot: createCopilotRepo(apiClient),
    notifications: createNotificationsRepo(apiClient),
    propertyQuery: createPropertyQueryRepo(dataEvents),
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
  });

  // Server-push app events (the realtime websocket). Currently drives shard
  // build-status into the store slice; the same pipeline carries M3 live regions.
  // Only the build-status handler is registered — the workspace tree-sync handler
  // stays on the pull/local path, so the verified data layer's behavior is
  // unchanged.
  const appEvents = createAppEventProcessor();
  appEvents.register(createShardBuildEventHandler());
  // Sync frames drive shard live regions: the hub refetches just the entities a
  // shard subscribed to and pushes them into its iframe.
  appEvents.register(createShardFrameHandler(apis.shardDataHub));
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
        // No replay cursor: frames during the gap are gone, so every live shard
        // region reconciles against the server instead of waiting forever.
        apis.shardDataHub.refetchAll();
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
      contentTokens,
      realtime,
    },
    repos,
    services,
  };
}

export type AppComposition = ReturnType<typeof createAppComposition>;
