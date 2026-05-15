import { useQueries } from "@tanstack/react-query";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";
import { useArtifactQuery } from "@/features/workspace/queries";
import { PageEditorPanel } from "@/features/pages/PageEditorPanel";
import { LinkArtifactPanel } from "@/features/artifacts/LinkArtifactPanel";
import { VoiceArtifactPanel } from "@/features/artifacts/VoiceArtifactPanel";
import { FileArtifactPanel } from "@/features/artifacts/FileArtifactPanel";
import { api, toArtifact } from "@/shared/api/client";
import { PlaceholderPane } from "@/components/ui/aladin";

export function WorkPane() {
  const { state, dispatch } = useWorkspaceShell();
  const activeArtifactQuery = useArtifactQuery(state.activeArtifactId);
  const openArtifactQueries = useQueries({
    queries: state.openArtifactIds.map((artifactId) => ({
      queryKey: ["artifact", artifactId],
      queryFn: async () => {
        const artifact = await api.getUserArtifact(artifactId);
        return toArtifact(artifact);
      },
    })),
  });

  const openArtifacts = openArtifactQueries.flatMap((query) => (query.data ? [query.data] : []));
  const activeArtifact = activeArtifactQuery.data ?? openArtifacts.find((artifact) => artifact.id === state.activeArtifactId);

  return (
    <section className="flex min-w-0 flex-1 bg-white">
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-2 border-b border-gray-300 px-4 py-3">
          <div className="flex min-w-0 flex-1 gap-2 overflow-auto">
            {openArtifacts.map((artifact) => {
              const active = artifact.id === activeArtifact?.id;
              return (
                <button
                  key={artifact.id}
                  className={`group flex h-[40px] items-center gap-2 border px-3 text-sm ${
                    active
                      ? "border-gray-300 bg-white text-black"
                      : "border-transparent bg-gray-50 text-gray-700 hover:bg-gray-100"
                  }`}
                  onClick={() => dispatch({ type: "activate-artifact", artifactId: artifact.id })}
                  type="button"
                >
                  <span className="max-w-[180px] truncate">{artifact.title}</span>
                  <span
                    className="px-1 text-xs text-gray-500 hover:bg-gray-100"
                    onClick={(event) => {
                      event.stopPropagation();
                      dispatch({ type: "close-artifact", artifactId: artifact.id });
                    }}
                  >
                    ×
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        <div className="min-h-0 flex-1">
          {!activeArtifact ? (
            <PlaceholderPane
              title="Open a page"
              body="Choose a note, link, voice note, or file from the browser to continue."
              className="h-full"
            />
          ) : activeArtifact.kind === "note" ? (
            <PageEditorPanel pageId={activeArtifact.id} />
          ) : activeArtifact.kind === "link" ? (
            <ScrollArea className="h-full px-6 py-5">
              <div className="mx-auto w-full max-w-workspace-max">
                <LinkArtifactPanel artifact={activeArtifact} />
              </div>
            </ScrollArea>
          ) : activeArtifact.kind === "voice" ? (
            <ScrollArea className="h-full px-6 py-5">
              <div className="mx-auto w-full max-w-workspace-max">
                <VoiceArtifactPanel artifact={activeArtifact} />
              </div>
            </ScrollArea>
          ) : (
            <ScrollArea className="h-full px-6 py-5">
              <div className="mx-auto w-full max-w-workspace-max">
                <FileArtifactPanel artifact={activeArtifact} />
              </div>
            </ScrollArea>
          )}
        </div>
      </div>
    </section>
  );
}
