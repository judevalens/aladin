import { useEffect, useMemo, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useObservableState } from "@/shared/flow/use-observable-state";

export function useSourcesScreen() {
  const { services } = useAppComposition();
  const addOpen = useAppStore((state) => state.sourcesUi.addOpen);
  const integrationsOpen = useAppStore((state) => state.sourcesUi.integrationsOpen);
  const selectedSourceId = useAppStore((state) => state.sourcesUi.selectedSourceId);
  const setAddSourceOpen = useAppStore((state) => state.setAddSourceOpen);
  const setIntegrationsOpen = useAppStore((state) => state.setIntegrationsOpen);
  const setSelectedSourceId = useAppStore((state) => state.setSelectedSourceId);

  const sourcesLoadable = useObservableState(
    () => services.sources.catalog.observeSources(),
    () => services.sources.catalog.getSourcesSnapshot(),
    [services.sources.catalog],
  );
  const providersLoadable = useObservableState(
    () => services.sources.catalog.observeProviders(),
    () => services.sources.catalog.getProvidersSnapshot(),
    [services.sources.catalog],
  );
  const tokensLoadable = useObservableState(
    () => services.sources.catalog.observeTokens(),
    () => services.sources.catalog.getTokensSnapshot(),
    [services.sources.catalog],
  );

  useEffect(() => {
    void services.sources.catalog.ensureSourcesLoaded();
    void services.sources.catalog.ensureProvidersLoaded();
  }, [services.sources.catalog]);

  useEffect(() => {
    if (integrationsOpen) {
      void services.sources.catalog.ensureTokensLoaded();
    }
  }, [integrationsOpen, services.sources.catalog]);

  const [streamQuery, setStreamQuery] = useState("");
  const [streamTitle, setStreamTitle] = useState("");
  const [streamLimit, setStreamLimit] = useState("25");
  const [streamErrorMessage, setStreamErrorMessage] = useState<string | null>(null);
  const [selectedProviderId, setSelectedProviderId] = useState<string | null>(null);
  const [tokenName, setTokenName] = useState(services.integrations.defaultIntegrationTokenName());
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [connectPending, setConnectPending] = useState(false);
  const [syncPending, setSyncPending] = useState(false);
  const [disconnectPending, setDisconnectPending] = useState(false);
  const [createSourcePending, setCreateSourcePending] = useState(false);
  const [createTokenPending, setCreateTokenPending] = useState(false);
  const [revokeTokenPending, setRevokeTokenPending] = useState(false);
  const [removeSourcePending, setRemoveSourcePending] = useState(false);

  const sources = sourcesLoadable.data;
  const providers = providersLoadable.data;
  const tokens = tokensLoadable.data;
  const selectedSource = sources.find((source) => source.id === selectedSourceId) ?? null;
  const connectedCount = providers.filter((provider) => provider.connected).length;
  const activeCount = sources.filter((source) => services.sources.isOperational(source.syncState)).length;
  const pendingCount = sources.filter((source) => !source.lastSyncedAt).length;
  const providerCount = new Set(sources.map((source) => source.type)).size;

  const selectedProvider =
    providers.find((provider) => provider.provider === selectedProviderId) ??
    providers.find((provider) => provider.connected) ??
    providers.find((provider) => provider.provider === "google") ??
    providers[0];

  const metrics = useMemo(
    () => [
      {
        label: "Subscribed",
        value: String(sources.length),
        description: "sources currently feeding this workspace",
      },
      {
        label: "Healthy",
        value: String(activeCount),
        description: "sources reporting a live or active state",
      },
      {
        label: "Pending",
        value: String(pendingCount),
        description: "sources still waiting on a first refresh",
      },
      {
        label: "Providers",
        value: String(providerCount),
        description: "upstream systems represented here",
      },
    ],
    [activeCount, pendingCount, providerCount, sources.length],
  );

  return {
    loading:
      sourcesLoadable.status === "loading" ||
      sourcesLoadable.status === "idle" ||
      providersLoadable.status === "loading" ||
      providersLoadable.status === "idle",
    sources,
    providers,
    tokens,
    selectedSource,
    selectedProvider,
    connectedCount,
    metrics,
    addOpen,
    integrationsOpen,
    streamQuery,
    streamTitle,
    streamLimit,
    streamErrorMessage,
    tokenName,
    createdToken,
    connectPending,
    syncPending,
    disconnectPending,
    createSourcePending,
    createTokenPending,
    revokeTokenPending,
    removeSourcePending,
    onOpenAddStream: () => setAddSourceOpen(true),
    onOpenIntegrations: () => setIntegrationsOpen(true),
    onAddOpenChange: setAddSourceOpen,
    onIntegrationsOpenChange: (open: boolean) => {
      setIntegrationsOpen(open);
      if (!open) {
        setCreatedToken(null);
      }
    },
    onSelectSource: (sourceId: string | null) => setSelectedSourceId(sourceId),
    onSelectProvider: setSelectedProviderId,
    onStreamQueryChange: setStreamQuery,
    onStreamTitleChange: setStreamTitle,
    onStreamLimitChange: setStreamLimit,
    onTokenNameChange: setTokenName,
    onCreateSource: async () => {
      try {
        setCreateSourcePending(true);
        setStreamErrorMessage(null);
        await services.sources.catalog.createSource({
          kind: "bluesky.search",
          name: streamTitle.trim() || undefined,
          query: streamQuery.trim(),
          limit: Number.parseInt(streamLimit, 10) || 25,
        });
        setStreamQuery("");
        setStreamTitle("");
        setStreamLimit("25");
        setAddSourceOpen(false);
      } catch (error) {
        setStreamErrorMessage(error instanceof Error ? error.message : "Failed to create stream.");
      } finally {
        setCreateSourcePending(false);
      }
    },
    onConnectProvider: async (provider: string) => {
      try {
        setConnectPending(true);
        const session = await services.sources.catalog.startProviderConnect(provider);
        window.open(session.connectLink, "_blank", "noopener,noreferrer");
      } finally {
        setConnectPending(false);
      }
    },
    onSyncProviders: async () => {
      try {
        setSyncPending(true);
        await services.sources.catalog.syncProviders();
      } finally {
        setSyncPending(false);
      }
    },
    onDisconnectProvider: async (connectionId: string) => {
      try {
        setDisconnectPending(true);
        await services.sources.catalog.disconnectProvider(connectionId);
      } finally {
        setDisconnectPending(false);
      }
    },
    onCreateToken: async () => {
      try {
        setCreateTokenPending(true);
        const response = await services.sources.catalog.createIntegrationToken({
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
        await services.sources.catalog.revokeIntegrationToken(tokenId);
      } finally {
        setRevokeTokenPending(false);
      }
    },
    onRemoveSelectedSource: async () => {
      if (!selectedSource) return;
      try {
        setRemoveSourcePending(true);
        await services.sources.catalog.deleteSource(selectedSource.id);
        setSelectedSourceId(null);
      } finally {
        setRemoveSourcePending(false);
      }
    },
    providerLabel: services.sources.providerLabel,
    descriptionLine: services.sources.descriptionLine,
    healthLabel: services.sources.healthLabel,
    lastRefreshSummary: services.sources.lastRefreshSummary,
    formatSourceFacts: services.sources.formatSourceFacts,
  };
}
