import { useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useObservableState } from "@/shared/flow/use-observable-state";
import type { IntegrationToken, ProviderConnectionProvider } from "@/shared/api/models";

export interface IntegrationsState {
  providers: ProviderConnectionProvider[];
  tokens: IntegrationToken[];
  selectedProvider: ProviderConnectionProvider;
  tokenName: string;
  createdToken: string | null;
  connectPending: boolean;
  syncPending: boolean;
  disconnectPending: boolean;
  createTokenPending: boolean;
  revokeTokenPending: boolean;
  onSelectProvider: (providerId: string | null) => void;
  onTokenNameChange: (tokenName: string) => void;
  onConnectProvider: (providerId: string) => Promise<void>;
  onSyncProviders: () => Promise<void>;
  onDisconnectProvider: (connectionId: string) => Promise<void>;
  onCreateToken: () => Promise<void>;
  onRevokeToken: (tokenId: string) => Promise<void>;
  onOpenChange: (nextOpen: boolean) => void;
}

export function useIntegrationsDialogState({
  open,
}: {
  open: boolean;
}): IntegrationsState {
  const { services } = useAppComposition();
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(
    null,
  );
  const [tokenName, setTokenName] = useState(
    services.integrations.defaultIntegrationTokenName(),
  );
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [connectPending, setConnectPending] = useState(false);
  const [syncPending, setSyncPending] = useState(false);
  const [disconnectPending, setDisconnectPending] = useState(false);
  const [createTokenPending, setCreateTokenPending] = useState(false);
  const [revokeTokenPending, setRevokeTokenPending] = useState(false);

  const providersLoadable = useObservableState(services.sources.providers());
  const tokensLoadable = useObservableState(services.sources.tokens());
  const providers =
    providersLoadable.status === "data" ? providersLoadable.value : [];
  const tokens = tokensLoadable.status === "data" ? tokensLoadable.value : [];

  const selectedProvider =
    providers.find((provider) => provider.provider === selectedProviderId) ??
    providers.find((provider) => provider.connected) ??
    providers.find((provider) => provider.provider === "google") ??
    providers[0];

  return {
    providers,
    tokens,
    selectedProvider,
    tokenName,
    createdToken,
    connectPending,
    syncPending,
    disconnectPending,
    createTokenPending,
    revokeTokenPending,
    onSelectProvider: setSelectedProviderId,
    onTokenNameChange: setTokenName,
    onConnectProvider: async (providerId: string) => {
      try {
        setConnectPending(true);
        const session = await services.sources.startProviderConnect(providerId);
        window.open(session.connectLink, "_blank", "noopener,noreferrer");
      } finally {
        setConnectPending(false);
      }
    },
    onSyncProviders: async () => {
      try {
        setSyncPending(true);
        await services.sources.syncProviders();
      } finally {
        setSyncPending(false);
      }
    },
    onDisconnectProvider: async (connectionId: string) => {
      try {
        setDisconnectPending(true);
        await services.sources.disconnectProvider(connectionId);
      } finally {
        setDisconnectPending(false);
      }
    },
    onCreateToken: async () => {
      try {
        setCreateTokenPending(true);
        const response = await services.sources.createIntegrationToken({
          name: tokenName.trim(),
          scopes: services.integrations.defaultIntegrationScopes(),
        });
        setCreatedToken(response.token);
      } finally {
        setCreateTokenPending(false);
      }
    },
    onRevokeToken: async (tokenId: string) => {
      try {
        setRevokeTokenPending(true);
        await services.sources.revokeIntegrationToken(tokenId);
      } finally {
        setRevokeTokenPending(false);
      }
    },
    onOpenChange: (nextOpen: boolean) => {
      if (!nextOpen) {
        setCreatedToken(null);
      }
    },
  } satisfies IntegrationsState;
}
