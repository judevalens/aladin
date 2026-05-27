import { createLocalDesktopSessionStore } from "@/shared/runtime/desktop-session-store";
import { createRuntimeConfig } from "@/shared/runtime/runtime-config";
import { createAuthRepo } from "@/repos/auth/auth-repo";
import { createArtifactApi } from "@/shared/api/artifact-api";
import { createWorkspaceRepo } from "@/repos/workspace/workspace-repo";
import { createArtifactRepo } from "@/repos/artifacts/artifact-repo";
import { createPageRepo } from "@/repos/pages/page-repo";
import { createSourcesRepo } from "@/repos/sources/sources-repo";
import { createIntegrationRepo } from "@/repos/integrations/integration-repo";
import { createApiClient } from "@/shared/api/client";
import { createDataEventsRepo } from "@/repos/data-events-repo";
import { createLocalRepos } from "@/repos/local-repos";
import { createLocalSyncRepo } from "@/repos/sync/local-sync-repo";
import { rowToArtifact } from "@/repos/artifacts/artifact-mappers";
import * as authService from "@/services/auth/auth-service";
import { AuthSessionService } from "@/services/auth/auth-session-service";
import { WorkspaceService } from "@/services/workspace/workspace-service";
import { PageDocumentService } from "@/services/pages/page-document-service";
import { PageSessionsService } from "@/services/pages/page-sessions-service";
import { SourcesCatalogService } from "@/services/sources/sources-catalog-service";
import * as integrationService from "@/services/integrations/integration-service";
import { WorkspaceSyncService } from "@/services/workspace/workspace-sync-service";
import { createRealtimeBoot } from "@/app/composition/realtime-boot";

export function createAppComposition() {
  const config = createRuntimeConfig();
  const desktopSession = createLocalDesktopSessionStore();
  const apiClient = createApiClient(config, desktopSession);
  const dataEvents = createDataEventsRepo();
  const local = createLocalRepos();
  const localSync = createLocalSyncRepo();
  const apis = {
    artifacts: createArtifactApi(apiClient),
  };

  const repos = {
    auth: createAuthRepo(apiClient),
    workspace: createWorkspaceRepo(
      local.repos.browser,
      local.repos.artifacts,
      apiClient,
      localSync,
    ),
    artifacts: createArtifactRepo(apis.artifacts, apiClient, local.repos.artifacts),
    pages: createPageRepo(apiClient),
    sources: createSourcesRepo(apiClient),
    integrations: createIntegrationRepo(apiClient),
  };

  const authSession = new AuthSessionService(repos.auth, desktopSession);
  const workspaceSync = new WorkspaceSyncService(repos.workspace, repos.artifacts);
  const workspace = new WorkspaceService(
    repos.workspace,
    repos.artifacts,
    workspaceSync,
  );
  const pageDocuments = new PageDocumentService(repos.pages);
  const pageSessions = new PageSessionsService(pageDocuments);
  const sourcesCatalog = new SourcesCatalogService(repos.sources, repos.integrations);

  void dataEvents.connect();
  dataEvents.events().subscribe((event) => {
    if (event.type === "browserNodeCreated") {
      workspaceSync.handleBrowserNodeCreated(event.payload);
      return;
    }
    if (event.type === "browserNodeUpdated") {
      workspaceSync.handleBrowserNodeUpdated(event.payload);
      return;
    }
    if (event.type === "browserNodeDeleted") {
      workspaceSync.handleBrowserNodeDeleted(event.payload.id);
      return;
    }
    if (event.type === "artifactChanged") {
      workspaceSync.publishArtifact(rowToArtifact(apiClient, event.payload));
      return;
    }
    if (event.type === "artifactDeleted") {
      workspaceSync.handleArtifactDeleted(event.payload.id);
      return;
    }
  });

  const realtime = createRealtimeBoot({
    authSession,
    admin: local.admin,
    localSync,
    desktopSession,
    config,
  });
  realtime.start();

  const services = {
    auth: {
      ...authService,
      session: authSession,
    },
    workspace,
    pages: {
      documents: pageDocuments,
      sessions: pageSessions,
    },
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
