import type { ApiClient } from "@/shared/api/client";
import type {
  CreatedIntegrationToken,
  IntegrationToken,
  IntegrationTokenCreateRequest,
  ProviderConnectSession,
  ProviderConnectionProvider,
  ProviderConnectionProvidersResponse,
} from "@/shared/api/models";

interface IntegrationTokenListResponse {
  tokens: IntegrationToken[];
}

export interface IntegrationRepo {
  getProviders(): Promise<ProviderConnectionProvider[]>;
  startProviderConnect(provider: string): Promise<ProviderConnectSession>;
  syncProviders(): Promise<void>;
  disconnectProvider(connectionId: string): Promise<void>;
  getIntegrationTokens(): Promise<IntegrationToken[]>;
  createIntegrationToken(request: IntegrationTokenCreateRequest): Promise<CreatedIntegrationToken>;
  revokeIntegrationToken(id: string): Promise<void>;
}

export function createIntegrationRepo(client: ApiClient): IntegrationRepo {
  return {
    getProviders: () =>
      client.fetch<ProviderConnectionProvidersResponse>("/api/provider-connections/providers").then(
        (response) => response.providers,
      ),
    startProviderConnect: (provider) =>
      client.fetch<ProviderConnectSession>(`/api/provider-connections/${provider}/connect`, {
        method: "POST",
      }),
    syncProviders: () =>
      client.fetch<void>("/api/provider-connections/sync", {
        method: "POST",
      }),
    disconnectProvider: (connectionId) =>
      client.fetch<void>(`/api/provider-connections/${connectionId}/disconnect`, {
        method: "POST",
      }),
    getIntegrationTokens: () =>
      client.fetch<IntegrationTokenListResponse>("/api/integration-tokens").then(
        (response) => response.tokens,
      ),
    createIntegrationToken: (request) =>
      client.fetch<CreatedIntegrationToken>("/api/integration-tokens", {
        method: "POST",
        body: JSON.stringify(request),
      }),
    revokeIntegrationToken: (id) =>
      client.fetch<void>(`/api/integration-tokens/${id}/revoke`, { method: "POST" }),
  };
}
