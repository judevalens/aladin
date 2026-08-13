import { FlaskConical, LineChart, Search, SlidersHorizontal, Star } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { PlaceholderPane } from "@/components/ui/aladin";
import { FileArtifactUI, LinkArtifactUI, VoiceArtifactUI } from "@/modules/artifacts/ui/artifact-ui";
import { PageEditorUI } from "@/modules/pages/ui/page-editor-ui";
import { DocSurfaceKeepAlive } from "@/modules/doc-surface/ui/doc-surface-ui";
import { GraphSidePaneUI } from "@/modules/graph/ui/graph-side-pane-ui";
import { ResearchPaneUI } from "@/modules/research/ui/research-pane-ui";
import { FileArtifactPaneUI } from "@/modules/documents/ui/document-viewer-ui";
import { useWorkPane, type WorkPaneCrumb } from "@/modules/workspace/hooks/use-workspace-state";
import { cn } from "@/lib/utils";

export function WorkPaneUI() {
  const {
    tabs,
    openArtifacts,
    activeTab,
    activeArtifact,
    breadcrumbFolders,
    artifactTitle,
    inspectorOpen,
    onActivateTab,
    onCloseTab,
    onToggleInspector,
    onJumpToFolder,
  } = useWorkPane();
  const [graphOpen, setGraphOpen] = useState(false);
  const stripRef = useRef<HTMLDivElement | null>(null);
  const activeTabRef = useRef<HTMLButtonElement | null>(null);

  // If the strip is a status display, it has to display the right status. Activating a tab
  // from the tree, from ⌘K or from the switcher can leave it off screen while the content
  // pane changes — the strip then reports the wrong tab as current.
  //
  // Deliberately NOT scrollIntoView: that scrolls every scrollable ancestor, which here means
  // the whole shell can shift. Setting scrollLeft moves only the strip.
  useEffect(() => {
    const strip = stripRef.current;
    const tab = activeTabRef.current;
    if (!strip || !tab) return;
    const left = tab.offsetLeft;
    const right = left + tab.offsetWidth;
    if (left < strip.scrollLeft) strip.scrollLeft = left;
    else if (right > strip.scrollLeft + strip.clientWidth) {
      strip.scrollLeft = right - strip.clientWidth;
    }
  }, [activeTab?.key, tabs.length]);

  return (
    <section className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
      <div className="h-10 border-b border-line bg-panel">
        <div ref={stripRef} className="scrollbar-hidden flex h-full min-w-0 overflow-x-auto overflow-y-hidden">
          {tabs.map((entry, index) => {
            const active = entry.key === activeTab?.key;
            const current = entry.tab;
            const isResearch = current.kind === "research";
            // §12: ownership reads from a heavier divider at each group's end — no
            // per-group color, since Aladin has one accent.
            const nextTab = tabs[index + 1]?.tab;
            const endsGroup =
              current.kind === "research" &&
              (!nextTab || nextTab.kind !== "research" || nextTab.contextId !== current.contextId);
            return (
              <button
                key={entry.key}
                ref={active ? activeTabRef : undefined}
                className={cn(
                  "group relative flex h-full items-center gap-2 border-r px-4 text-small transition-colors",
                  active
                    ? "border-line bg-bg font-medium text-ink"
                    : "border-line bg-panel text-ink-3 hover:bg-raise hover:text-ink",
                  endsGroup && "border-r-2 border-r-line-2",
                )}
                onClick={() => onActivateTab(entry.key)}
                type="button"
              >
                {isResearch ? (
                  <Icon as={FlaskConical} size="inline" className="shrink-0 text-amber" />
                ) : null}
                <span className="max-w-[200px] truncate">{entry.label}</span>
                <span
                  className="ml-1 flex h-4 w-4 items-center justify-center rounded-tap text-ink-4 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                  onClick={(event) => {
                    event.stopPropagation();
                    onCloseTab(entry.key);
                  }}
                >
                  ×
                </span>
              </button>
            );
          })}
        </div>
      </div>
      {activeArtifact && activeTab?.tab.kind === "artifact" ? (
        <WorkPaneStatusBar
          folders={breadcrumbFolders}
          artifactTitle={artifactTitle}
          inspectorOpen={inspectorOpen}
          graphOpen={graphOpen}
          onToggleGraph={() => setGraphOpen((open) => !open)}
          onToggleInspector={onToggleInspector}
          onJumpToFolder={onJumpToFolder}
        />
      ) : null}
      <div className="flex min-h-0 flex-1 overflow-hidden">
        <div className="min-w-0 flex-1 bg-bg">
        {activeTab?.tab.kind === "research" ? (
          <ResearchPaneUI contextId={activeTab.tab.contextId} view={activeTab.tab.view} />
        ) : !activeArtifact ? (
          <PlaceholderPane
            title="Open a page"
            body="Choose a note, link, voice note, or file from the browser to continue."
            className="h-full"
          />
        ) : activeArtifact.kind === "note" ? (
          <PageEditorUI pageId={activeArtifact.id} />
        ) : activeArtifact.kind === "app" ? (
          <DocSurfaceKeepAlive
            activeId={activeArtifact.id}
            artifacts={openArtifacts.filter((a) => a.kind === "app")}
          />
        ) : activeArtifact.kind === "link" ? (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <LinkArtifactUI artifact={activeArtifact} />
            </div>
          </ScrollArea>
        ) : activeArtifact.kind === "voice" ? (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <VoiceArtifactUI artifact={activeArtifact} />
            </div>
          </ScrollArea>
        ) : activeArtifact.kind === "file" ? (
          // An ingested PDF opens as a document (INGESTION_PRD §6); every other file
          // keeps the download card.
          <FileArtifactPaneUI
            artifact={activeArtifact}
            fallback={
              <ScrollArea className="h-full">
                <div className="mx-auto w-full max-w-workspace-max">
                  <FileArtifactUI artifact={activeArtifact} />
                </div>
              </ScrollArea>
            }
          />
        ) : (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <FileArtifactUI artifact={activeArtifact} />
            </div>
          </ScrollArea>
        )}
        </div>
        {graphOpen && activeArtifact ? (
          <GraphSidePaneUI artifact={activeArtifact} onClose={() => setGraphOpen(false)} />
        ) : null}
      </div>
    </section>
  );
}

function WorkPaneStatusBar({
  folders,
  artifactTitle,
  inspectorOpen,
  graphOpen,
  onToggleGraph,
  onToggleInspector,
  onJumpToFolder,
}: {
  folders: WorkPaneCrumb[];
  artifactTitle: string | null;
  inspectorOpen: boolean;
  graphOpen: boolean;
  onToggleGraph: () => void;
  onToggleInspector: () => void;
  onJumpToFolder: (folderId: string) => void;
}) {
  return (
    <div className="flex items-center gap-3 border-b border-line bg-panel px-3.5 py-1.5">
      <nav className="flex min-w-0 flex-1 items-center text-small text-ink-3">
        {folders.map((crumb, index) => (
          <div key={crumb.id} className="flex min-w-0 items-center">
            <button
              type="button"
              onClick={() => onJumpToFolder(crumb.id)}
              className="max-w-[14ch] truncate rounded-tap px-1 py-0.5 hover:bg-[rgb(var(--hover))] hover:text-ink"
              title={crumb.title}
            >
              {crumb.title}
            </button>
            <span className="mx-0.5 text-ink-4">/</span>
            {index === folders.length - 1 && artifactTitle ? null : null}
          </div>
        ))}
        {artifactTitle ? (
          <span className="max-w-[24ch] truncate px-1 font-medium text-ink" title={artifactTitle}>
            {artifactTitle}
          </span>
        ) : null}
      </nav>
      <div className="flex items-center gap-0.5">
        <StatusUtilityIcon ariaLabel="Search document" onClick={() => undefined}>
          <Icon as={Search} />
        </StatusUtilityIcon>
        <StatusUtilityIcon ariaLabel="Favorite document" onClick={() => undefined}>
          <Icon as={Star} />
        </StatusUtilityIcon>
        <StatusUtilityIcon
          ariaLabel="Toggle graph context"
          isActive={graphOpen}
          onClick={onToggleGraph}
        >
          <Icon as={LineChart} />
        </StatusUtilityIcon>
        <StatusUtilityIcon
          ariaLabel="Toggle inspector"
          isActive={inspectorOpen}
          onClick={onToggleInspector}
        >
          <Icon as={SlidersHorizontal} />
        </StatusUtilityIcon>
      </div>
    </div>
  );
}

function StatusUtilityIcon({
  ariaLabel,
  isActive = false,
  onClick,
  children,
}: {
  ariaLabel: string;
  isActive?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onClick}
      className={cn("flex h-6 w-6 items-center justify-center rounded-tap transition-colors",
        isActive
          ? "bg-[rgb(var(--sel))] text-ink"
          : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
      )}
    >
      {children}
    </button>
  );
}
