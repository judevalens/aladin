import { useState } from "react";
import { useAddSourceDialogState } from "@/modules/sources/hooks/use-add-source-dialog-state";
import { useSourceDetailsDialogState } from "@/modules/sources/hooks/use-source-details-dialog-state";
import { useSourcesScreen } from "@/modules/sources/hooks/use-sources-screen";
import { SourcesRouteView, SourcesScreenView } from "@/modules/sources/ui/sources-views";

export function SourcesRouteScreen() {
  const screen = useSourcesScreen();
  const [integrationsOpen, setIntegrationsOpen] = useState(false);
  const addSourceDialog = useAddSourceDialogState({
    createSource: screen.sourceActions.createSource,
  });
  const sourceDetailsDialog = useSourceDetailsDialogState({
    sources: screen.catalog.sources,
    removeSource: screen.sourceActions.removeSource,
  });

  return (
    <SourcesRouteView>
      <SourcesScreenView
        overview={{
          loading: screen.catalog.loading,
          metrics: screen.catalog.metrics,
          sources: screen.catalog.sources,
          connectedCount: screen.catalog.connectedCount,
          onOpenAddStream: addSourceDialog.openDialog,
          onOpenIntegrations: () => setIntegrationsOpen(true),
          onSelectSource: sourceDetailsDialog.onSelectSource,
        }}
        addSourceDialog={{
          open: addSourceDialog.open,
          streamQuery: addSourceDialog.streamQuery,
          streamTitle: addSourceDialog.streamTitle,
          streamLimit: addSourceDialog.streamLimit,
          streamErrorMessage: addSourceDialog.streamErrorMessage,
          createSourcePending: addSourceDialog.createSourcePending,
          onOpenChange: addSourceDialog.onOpenChange,
          onStreamQueryChange: addSourceDialog.onStreamQueryChange,
          onStreamTitleChange: addSourceDialog.onStreamTitleChange,
          onStreamLimitChange: addSourceDialog.onStreamLimitChange,
          onCreateSource: addSourceDialog.onCreateSource,
        }}
        integrationsDialog={{
          open: integrationsOpen,
          onOpenChange: setIntegrationsOpen,
        }}
        sourceDetailsDialog={{
          selectedSource: sourceDetailsDialog.selectedSource,
          removeSourcePending: sourceDetailsDialog.removeSourcePending,
          onSelectSource: sourceDetailsDialog.onSelectSource,
          onRemoveSelectedSource: sourceDetailsDialog.onRemoveSelectedSource,
        }}
        formatters={screen.formatters}
      />
    </SourcesRouteView>
  );
}
