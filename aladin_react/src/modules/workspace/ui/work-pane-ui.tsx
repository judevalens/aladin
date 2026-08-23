import { FlaskConical, LineChart, Search, SlidersHorizontal, Star } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { PlaceholderPane } from "@/components/ui/aladin";
import { useArtifact } from "@/modules/artifacts/hooks/use-artifact";
import { DesktopBoardPane } from "@/modules/board/ui/desktop-board-pane";
import { FileArtifactUI, LinkArtifactUI, VoiceArtifactUI } from "@/modules/artifacts/ui/artifact-ui";
import { PageEditorUI } from "@/modules/pages/ui/page-editor-ui";
import { DocSurfaceUI } from "@/modules/doc-surface/ui/doc-surface-ui";
import { GraphSidePaneUI } from "@/modules/graph/ui/graph-side-pane-ui";
import { ResearchPaneUI } from "@/modules/research/ui/research-pane-ui";
import { FileArtifactPaneUI } from "@/modules/documents/ui/document-viewer-ui";
import { useWorkPane, type WorkPaneCrumb, type WorkPaneTab } from "@/modules/workspace/hooks/use-workspace-state";
import { capHeavyKeys, LIGHT_KEEP_ALIVE, useKeepAliveKeys } from "@/modules/workspace/hooks/use-keep-alive";
import { cn } from "@/lib/utils";

export function WorkPaneUI() {
  const {
    tabs,
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

  // Tabs used to be a switch on the ACTIVE artifact, so switching away unmounted the pane and
  // threw away its scroll position, editor selection and PDF page. Now every tab in the
  // keep-alive window stays mounted and inactive ones are hidden with CSS.
  //
  // The window is LRU and kind-aware: heavy panes (note editors hold a Yjs doc and a
  // Hocuspocus socket, shards hold a live iframe) are capped tighter than cheap ones, so
  // twenty open tabs never means twenty documents syncing in the background. A tab evicted
  // from the window still works — it just re-mounts, and loses its view state, when revisited.
  const keptKeys = useKeepAliveKeys(activeTab?.key ?? null, LIGHT_KEEP_ALIVE);
  const liveTabs = useMemo(() => {
    const byKey = new Map(tabs.map((t) => [t.key, t]));
    const isHeavy = (key: string) => {
      const kind = byKey.get(key)?.artifact?.kind;
      return kind === "note" || kind === "app" || kind === "board";
    };
    // The active key is included even before the effect lands, or the frame in which it
    // changes would render an empty pane.
    const ordered = activeTab
      ? [activeTab.key, ...keptKeys.filter((k) => k !== activeTab.key)]
      : keptKeys;
    return capHeavyKeys(ordered, isHeavy)
      .flatMap((key) => {
        const tab = byKey.get(key);
        return tab ? [tab] : [];
      })
      // Render in the strip's order, not recency order: a stable DOM order keeps React from
      // reordering the layers on every tab switch.
      .sort((a, b) => tabs.indexOf(a) - tabs.indexOf(b));
  }, [tabs, keptKeys, activeTab]);

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
                  className="ml-1 flex h-4 w-4 items-center justify-center rounded-tap text-ink-4 transition-colors hover:bg-hover hover:text-ink"
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
        {/* Every kept tab stays MOUNTED and is hidden with CSS — see the keep-alive note on
            liveTabs above. `relative` is what the absolute layers position against. */}
        <div className="relative min-w-0 flex-1 bg-bg">
          {liveTabs.length === 0 ? (
            <PlaceholderPane
              title="Open a page"
              body="Choose a note, link, voice note, or file from the browser to continue."
              className="h-full"
            />
          ) : (
            liveTabs.map((entry) => (
              <div
                key={entry.key}
                className={cn("absolute inset-0", entry.key !== activeTab?.key && "hidden")}
                // Hidden panes are still in the tree; keep them out of the a11y tree and out
                // of tab order so focus can't land inside an invisible editor.
                aria-hidden={entry.key !== activeTab?.key}
                inert={entry.key !== activeTab?.key ? true : undefined}
              >
                <TabPane entry={entry} hidden={entry.key !== activeTab?.key} />
              </div>
            ))
          )}
        </div>
        {graphOpen && activeArtifact ? (
          <GraphSidePaneUI artifact={activeArtifact} onClose={() => setGraphOpen(false)} />
        ) : null}
      </div>
    </section>
  );
}

/**
 * One tab's pane. This is the `artifact.kind` switch that used to sit inline in WorkPaneUI,
 * moved down a level so it renders per tab instead of only for the active one — that change is
 * what makes tabs persistent.
 *
 * `hidden` is passed down rather than inferred: a shard iframe needs to know it is off screen
 * so it can stop doing work, and CSS visibility isn't observable from inside the component.
 */
function TabPane({ entry, hidden }: { entry: WorkPaneTab; hidden: boolean }) {
  if (entry.tab.kind === "research") {
    return <ResearchPaneUI contextId={entry.tab.contextId} view={entry.tab.view} />;
  }
  return <ArtifactTabPane artifactId={entry.tab.artifactId} hidden={hidden} />;
}

/**
 * Reads its OWN artifact rather than being handed one from the strip's projection.
 *
 * That projection is rebuilt whenever the set of open tabs changes, which used to mean a pane's
 * data could vanish for a frame because a DIFFERENT tab closed — and a pane that loses its
 * artifact for a frame is a pane that unmounts: pdf.js document destroyed, Yjs socket dropped,
 * shard iframe rebuilt. A pane's data should depend on its own id and nothing else.
 */
function ArtifactTabPane({ artifactId, hidden }: { artifactId: string; hidden: boolean }) {
  const artifact = useArtifact(artifactId);
  if (!artifact) {
    // The tab is open but its artifact hasn't arrived from the replica yet.
    return <PlaceholderPane title="Loading…" body="" className="h-full" />;
  }
  if (artifact.kind === "note") return <PageEditorUI pageId={artifact.id} />;
  if (artifact.kind === "app") return <DocSurfaceUI artifact={artifact} hidden={hidden} />;
  if (artifact.kind === "board") {
    return <DesktopBoardPane boardId={artifact.id} title={artifact.title} />;
  }
  if (artifact.kind === "link") {
    return (
      <ScrollArea className="h-full">
        <div className="mx-auto w-full max-w-workspace-max">
          <LinkArtifactUI artifact={artifact} />
        </div>
      </ScrollArea>
    );
  }
  if (artifact.kind === "voice") {
    return (
      <ScrollArea className="h-full">
        <div className="mx-auto w-full max-w-workspace-max">
          <VoiceArtifactUI artifact={artifact} />
        </div>
      </ScrollArea>
    );
  }
  const downloadCard = (
    <ScrollArea className="h-full">
      <div className="mx-auto w-full max-w-workspace-max">
        <FileArtifactUI artifact={artifact} />
      </div>
    </ScrollArea>
  );
  // An ingested PDF opens as a document (INGESTION_PRD §6); every other file keeps the
  // download card.
  if (artifact.kind === "file") {
    return <FileArtifactPaneUI artifact={artifact} fallback={downloadCard} />;
  }
  return downloadCard;
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
              className="max-w-[14ch] truncate rounded-tap px-1 py-0.5 hover:bg-hover hover:text-ink"
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
          ? "bg-sel text-ink"
          : "text-ink-3 hover:bg-hover hover:text-ink",
      )}
    >
      {children}
    </button>
  );
}
