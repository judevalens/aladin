import { useSourcesScreen } from "@/modules/sources/hooks/use-sources-screen";
import { SourcesRouteView, SourcesScreenView } from "@/modules/sources/ui/sources-views";

export function SourcesRouteScreen() {
  const screen = useSourcesScreen();

  return (
    <SourcesRouteView>
      <SourcesScreenView
        loading={screen.loading}
        metrics={screen.metrics}
        sources={screen.sources}
        providers={screen.providers}
        tokens={screen.tokens}
        selectedSource={screen.selectedSource}
        selectedProvider={screen.selectedProvider}
        connectedCount={screen.connectedCount}
        addOpen={screen.addOpen}
        integrationsOpen={screen.integrationsOpen}
        streamQuery={screen.streamQuery}
        streamTitle={screen.streamTitle}
        streamLimit={screen.streamLimit}
        streamErrorMessage={screen.streamErrorMessage}
        tokenName={screen.tokenName}
        createdToken={screen.createdToken}
        connectPending={screen.connectPending}
        syncPending={screen.syncPending}
        disconnectPending={screen.disconnectPending}
        createSourcePending={screen.createSourcePending}
        createTokenPending={screen.createTokenPending}
        revokeTokenPending={screen.revokeTokenPending}
        removeSourcePending={screen.removeSourcePending}
        onOpenAddStream={screen.onOpenAddStream}
        onOpenIntegrations={screen.onOpenIntegrations}
        onAddOpenChange={screen.onAddOpenChange}
        onIntegrationsOpenChange={screen.onIntegrationsOpenChange}
        onSelectSource={screen.onSelectSource}
        onSelectProvider={screen.onSelectProvider}
        onStreamQueryChange={screen.onStreamQueryChange}
        onStreamTitleChange={screen.onStreamTitleChange}
        onStreamLimitChange={screen.onStreamLimitChange}
        onTokenNameChange={screen.onTokenNameChange}
        onCreateSource={screen.onCreateSource}
        onConnectProvider={screen.onConnectProvider}
        onSyncProviders={screen.onSyncProviders}
        onDisconnectProvider={screen.onDisconnectProvider}
        onCreateToken={screen.onCreateToken}
        onRevokeToken={screen.onRevokeToken}
        onRemoveSelectedSource={screen.onRemoveSelectedSource}
        providerLabel={screen.providerLabel}
        descriptionLine={screen.descriptionLine}
        healthLabel={screen.healthLabel}
        lastRefreshSummary={screen.lastRefreshSummary}
        formatSourceFacts={screen.formatSourceFacts}
      />
    </SourcesRouteView>
  );
}
