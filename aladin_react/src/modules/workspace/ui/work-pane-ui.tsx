import { LineChart, Search, SlidersHorizontal, Star } from "lucide-react";
import type { ReactNode } from "react";
import { ScrollArea } from "@/components/ui/scroll-area";
import { PlaceholderPane } from "@/components/ui/aladin";
import { FileArtifactUI, LinkArtifactUI, VoiceArtifactUI } from "@/modules/artifacts/ui/artifact-ui";
import { PageEditorUI } from "@/modules/pages/ui/page-editor-ui";
import { useWorkPane, type WorkPaneCrumb } from "@/modules/workspace/hooks/use-workspace-state";
import { cn } from "@/shared/lib/utils";

export function WorkPaneUI() {
  const {
    openArtifacts,
    activeArtifact,
    breadcrumbFolders,
    artifactTitle,
    inspectorOpen,
    onActivateArtifact,
    onCloseArtifact,
    onToggleInspector,
    onJumpToFolder,
  } = useWorkPane();

  return (
    <section className="flex min-w-0 flex-1 flex-col overflow-hidden bg-bg">
      <div className="h-10 border-b border-line bg-panel">
        <div className="scrollbar-hidden flex h-full min-w-0 overflow-x-auto overflow-y-hidden">
          {openArtifacts.map((artifact) => {
            const active = artifact.id === activeArtifact?.id;
            return (
              <button
                key={artifact.id}
                className={cn(
                  "group relative flex h-full items-center gap-2 border-r px-4 text-[12.5px] transition-colors",
                  active
                    ? "border-line bg-bg font-medium text-ink"
                    : "border-line bg-panel text-ink-3 hover:bg-raise hover:text-ink",
                )}
                onClick={() => onActivateArtifact(artifact.id)}
                type="button"
              >
                <span className="max-w-[200px] truncate">{artifact.title}</span>
                <span
                  className="ml-1 flex h-4 w-4 items-center justify-center rounded text-ink-4 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                  onClick={(event) => {
                    event.stopPropagation();
                    onCloseArtifact(artifact.id);
                  }}
                >
                  ×
                </span>
              </button>
            );
          })}
        </div>
      </div>
      {activeArtifact ? (
        <WorkPaneStatusBar
          folders={breadcrumbFolders}
          artifactTitle={artifactTitle}
          inspectorOpen={inspectorOpen}
          onToggleInspector={onToggleInspector}
          onJumpToFolder={onJumpToFolder}
        />
      ) : null}
      <div className="min-h-0 flex-1 bg-bg">
        {!activeArtifact ? (
          <PlaceholderPane
            title="Open a page"
            body="Choose a note, link, voice note, or file from the browser to continue."
            className="h-full"
          />
        ) : activeArtifact.kind === "note" ? (
          <PageEditorUI pageId={activeArtifact.id} />
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
        ) : (
          <ScrollArea className="h-full">
            <div className="mx-auto w-full max-w-workspace-max">
              <FileArtifactUI artifact={activeArtifact} />
            </div>
          </ScrollArea>
        )}
      </div>
    </section>
  );
}

function WorkPaneStatusBar({
  folders,
  artifactTitle,
  inspectorOpen,
  onToggleInspector,
  onJumpToFolder,
}: {
  folders: WorkPaneCrumb[];
  artifactTitle: string | null;
  inspectorOpen: boolean;
  onToggleInspector: () => void;
  onJumpToFolder: (folderId: string) => void;
}) {
  return (
    <div className="flex items-center gap-3 border-b border-line bg-panel px-3.5 py-1.5">
      <nav className="flex min-w-0 flex-1 items-center text-[12px] text-ink-3">
        {folders.map((crumb, index) => (
          <div key={crumb.id} className="flex min-w-0 items-center">
            <button
              type="button"
              onClick={() => onJumpToFolder(crumb.id)}
              className="max-w-[14ch] truncate rounded px-1 py-0.5 hover:bg-[rgb(var(--hover))] hover:text-ink"
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
          <Search className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
        <StatusUtilityIcon ariaLabel="Favorite document" onClick={() => undefined}>
          <Star className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
        <StatusUtilityIcon ariaLabel="Open graph context" onClick={() => undefined}>
          <LineChart className="h-[15px] w-[15px]" strokeWidth={1.75} />
        </StatusUtilityIcon>
        <StatusUtilityIcon
          ariaLabel="Toggle inspector"
          isActive={inspectorOpen}
          onClick={onToggleInspector}
        >
          <SlidersHorizontal className="h-[15px] w-[15px]" strokeWidth={1.75} />
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
      className={cn(
        "flex h-6 w-6 items-center justify-center rounded transition-colors",
        isActive
          ? "bg-[rgb(var(--sel))] text-ink"
          : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
      )}
    >
      {children}
    </button>
  );
}
