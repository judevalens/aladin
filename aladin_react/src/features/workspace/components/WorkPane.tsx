import { useQueries } from "@tanstack/react-query";
import { FileText, Info } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";
import { useArtifactQuery } from "@/features/workspace/queries";
import { PageEditorPanel } from "@/features/pages/PageEditorPanel";
import { LinkArtifactPanel } from "@/features/artifacts/LinkArtifactPanel";
import { VoiceArtifactPanel } from "@/features/artifacts/VoiceArtifactPanel";
import { FileArtifactPanel } from "@/features/artifacts/FileArtifactPanel";
import { api, toArtifact } from "@/shared/api/client";
import { AladinPanel, PlaceholderPane } from "@/components/ui/aladin";

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
  const inspectorOpen = activeArtifact
    ? (state.inspectorOverrides[activeArtifact.id] ?? (activeArtifact.kind === "link" || activeArtifact.kind === "voice"))
    : false;

  return (
    <section className="flex min-w-0 flex-1 bg-aladin-canvas">
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-2 border-b border-aladin-divider px-4 py-3">
          <div className="flex min-w-0 flex-1 gap-2 overflow-auto">
            {openArtifacts.map((artifact) => {
              const active = artifact.id === activeArtifact?.id;
              return (
                <button
                  key={artifact.id}
                  className={`group flex h-[42px] items-center gap-2 rounded-t-control border px-3 text-sm ${
                    active
                      ? "border-aladin-border bg-aladin-panel text-aladin-ink"
                      : "border-transparent bg-aladin-panel-muted text-aladin-ink-secondary hover:bg-aladin-row-hover"
                  }`}
                  onClick={() => dispatch({ type: "activate-artifact", artifactId: artifact.id })}
                  type="button"
                >
                  <FileText className="h-4 w-4" />
                  <span className="max-w-[180px] truncate">{artifact.title}</span>
                  <span
                    className="rounded-sm px-1 text-xs text-aladin-ink-muted hover:bg-aladin-control-hover"
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
          {activeArtifact ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => dispatch({ type: "toggle-inspector", artifactId: activeArtifact.id })}
            >
              <Info className="h-4 w-4" />
              Inspector
            </Button>
          ) : null}
        </div>

        <div className="flex min-h-0 flex-1">
          <div className="min-w-0 flex-1">
            {!activeArtifact ? (
              <PlaceholderPane
                title="Nothing open"
                body="Open a note, link, voice recording, or file from the browser tree to start working in the React shell."
                className="h-full"
              />
            ) : activeArtifact.kind === "note" ? (
              <PageEditorPanel pageId={activeArtifact.id} />
            ) : activeArtifact.kind === "link" ? (
              <ScrollArea className="h-full px-6 py-5">
                <LinkArtifactPanel artifact={activeArtifact} />
              </ScrollArea>
            ) : activeArtifact.kind === "voice" ? (
              <ScrollArea className="h-full px-6 py-5">
                <VoiceArtifactPanel artifact={activeArtifact} />
              </ScrollArea>
            ) : (
              <ScrollArea className="h-full px-6 py-5">
                <FileArtifactPanel artifact={activeArtifact} />
              </ScrollArea>
            )}
          </div>

          {activeArtifact && inspectorOpen ? (
            <>
              <div className="w-px bg-aladin-divider" />
              <aside className="w-[320px] bg-aladin-panel-muted p-4">
                <AladinPanel className="space-y-4 p-4">
                  <div className="space-y-1">
                    <div className="text-xs font-semibold uppercase tracking-[0.2em] text-aladin-code-text">
                      Insight
                    </div>
                    <h3 className="text-lg font-semibold text-aladin-ink">{activeArtifact.title}</h3>
                  </div>
                  <p className="text-sm leading-6 text-aladin-ink-secondary">
                    {activeArtifact.kind === "note"
                      ? "User-authored markdown remains the source of truth. System context stays outside the editable page body."
                      : activeArtifact.kind === "link"
                        ? "This link is treated as an external source. Future ingestion will extract title, author, key claims, and workspace relevance."
                        : activeArtifact.kind === "voice"
                          ? "Voice enrichment will summarize transcript, key moments, and detected action items after recording support lands."
                          : "File enrichment will expose metadata, preview text, and related workspace context."}
                  </p>
                  <div className="space-y-2 text-sm text-aladin-ink-secondary">
                    <div>
                      <span className="font-medium text-aladin-ink">Type:</span> {activeArtifact.kind}
                    </div>
                    <div>
                      <span className="font-medium text-aladin-ink">Updated:</span> {activeArtifact.updatedLabel}
                    </div>
                  </div>
                </AladinPanel>
              </aside>
            </>
          ) : null}
        </div>
      </div>
    </section>
  );
}
