import { createLocalDesktopSessionStore } from "@/shared/runtime/desktop-session-store";
import { createRuntimeConfig } from "@/shared/runtime/runtime-config";
import { createAuthRepo } from "@/repos/auth/auth-repo";
import { createWorkspaceRepo } from "@/repos/workspace/workspace-repo";
import { createArtifactRepo } from "@/repos/artifacts/artifact-repo";
import { createPageRepo } from "@/repos/pages/page-repo";
import { createSourcesRepo } from "@/repos/sources/sources-repo";
import { createIntegrationRepo } from "@/repos/integrations/integration-repo";
import { createApiClient } from "@/shared/api/client";
import * as authService from "@/services/auth/auth-service";
import { AuthSessionService } from "@/services/auth/auth-session-service";
import * as workspaceService from "@/services/workspace/workspace-service";
import { PageSessionsService } from "@/services/pages/page-sessions-service";
import * as sourcesService from "@/services/sources/sources-service";
import { SourcesCatalogService } from "@/services/sources/sources-catalog-service";
import * as integrationService from "@/services/integrations/integration-service";
import { WorkspaceDataService } from "@/services/workspace/workspace-data-service";

export function createAppComposition() {
  const config = createRuntimeConfig();
  const desktopSession = createLocalDesktopSessionStore();
  const apiClient = createApiClient(config, desktopSession);

  const repos = {
    auth: createAuthRepo(apiClient, config),
    workspace: createWorkspaceRepo(apiClient),
    artifacts: createArtifactRepo(apiClient),
    pages: createPageRepo(apiClient),
    sources: createSourcesRepo(apiClient),
    integrations: createIntegrationRepo(apiClient),
  };

  const authSession = new AuthSessionService(repos.auth, desktopSession);
  const workspaceData = new WorkspaceDataService(repos.workspace, repos.artifacts);
  const pageSessions = new PageSessionsService(repos.pages);
  const sourcesCatalog = new SourcesCatalogService(repos.sources, repos.integrations);

  const services = {
    auth: {
      ...authService,
      session: authSession,
    },
    workspace: workspaceService,
    pages: {
      sessions: pageSessions,
    },
    sources: {
      ...sourcesService,
      catalog: sourcesCatalog,
    },
    integrations: integrationService,
    data: {
      workspace: workspaceData,
    },
  };

  return {
    runtime: {
      config,
      desktopSession,
    },
    repos,
    services,
  };
}

export type AppComposition = ReturnType<typeof createAppComposition>;
