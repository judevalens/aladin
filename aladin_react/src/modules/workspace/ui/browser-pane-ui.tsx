import { useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronLeft, ChevronRight, Ellipsis } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AladinPanel } from "@/components/ui/aladin";
import { useBrowserPane } from "@/modules/workspace/hooks/use-workspace-state";
import { cn } from "@/shared/lib/utils";

const DRILL_DEPTH = 4;

export function BrowserPaneUI() {
  const {
    loading,
    errorMessage,
    rows,
    activeArtifactId,
    canGoBack,
    browserPath,
    browserRootTitle,
    expandedFolderIds,
    browserScrollTop,
    onToggleFolder,
    onDrillIntoFolder,
    onPopBrowserFrame,
    onBrowserScroll,
    onOpenArtifact,
    onStartRenameFolder,
    onStartRenameArtifact,
    onCreateFolderHere,
    onCreateNoteHere,
  } = useBrowserPane();
  const scrollRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    if (Math.abs(element.scrollTop - browserScrollTop) < 1) return;
    element.scrollTop = browserScrollTop;
  }, [browserScrollTop, rows]);

  if (loading) {
    return (
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-line bg-explorer sm:w-[368px]">
        <div className="p-5 text-sm text-ink-2">Loading browser tree…</div>
      </section>
    );
  }

  if (errorMessage) {
    return (
      <section className="flex w-[336px] flex-col overflow-hidden border-r border-line bg-explorer p-4 sm:w-[368px]">
        <AladinPanel className="rounded-md border border-against/40 bg-against/10 p-4 text-sm text-against">{errorMessage}</AladinPanel>
      </section>
    );
  }

  return (
    <section className="flex w-[300px] flex-col overflow-hidden border-r border-line bg-explorer sm:w-[332px]">
      <div className="px-2 py-2">
        <div className="flex items-center gap-2 rounded-xl border border-line bg-field px-2 py-1.5">
          {canGoBack ? (
            <button
              type="button"
              onClick={onPopBrowserFrame}
              className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
              aria-label="Back to previous folder view"
            >
              <ChevronLeft className="h-4 w-4" strokeWidth={2} />
            </button>
          ) : (
            <div className="h-7 w-7 shrink-0" aria-hidden />
          )}
          <nav className="flex min-w-0 items-center text-[12px] text-ink-3" aria-label="Browser navigation">
            <span className="truncate rounded px-1 py-0.5 text-ink">Workspace</span>
            {browserPath.map((crumb) => (
              <div key={crumb.id} className="flex min-w-0 items-center">
                <span className="mx-0.5 text-ink-4">/</span>
                <span
                  className={cn(
                    "truncate rounded px-1 py-0.5",
                    crumb.id === browserPath.at(-1)?.id ? "text-ink" : "text-ink-3",
                  )}
                  title={crumb.title}
                >
                  {crumb.title}
                </span>
              </div>
            ))}
            {browserPath.length === 0 ? (
              <span className="mx-0.5 text-ink-4">/</span>
            ) : null}
            {browserPath.length === 0 ? (
              <span className="truncate rounded px-1 py-0.5 text-ink-4">All folders</span>
            ) : null}
            {browserPath.length > 0 && !browserRootTitle ? (
              <span className="mx-0.5 text-ink-4">/</span>
            ) : null}
          </nav>
        </div>
      </div>
      <div
        ref={scrollRef}
        className="min-h-0 flex-1 overflow-y-auto browser-pane-scroll"
        onScroll={(event) => onBrowserScroll(event.currentTarget.scrollTop)}
      >
        <div className="flex flex-col gap-0.5 px-2 py-2">
          {rows.map((row) => (
            <BrowserPaneRow
              key={row.id}
              row={row}
              activeArtifactId={activeArtifactId}
              expandedFolderIds={expandedFolderIds}
              onToggleFolder={onToggleFolder}
              onDrillIntoFolder={onDrillIntoFolder}
              onOpenArtifact={onOpenArtifact}
              onStartRenameFolder={onStartRenameFolder}
              onStartRenameArtifact={onStartRenameArtifact}
              onCreateFolderHere={onCreateFolderHere}
              onCreateNoteHere={onCreateNoteHere}
            />
          ))}
          {rows.length === 0 ? (
            <div className="px-3 py-10 text-center text-[12px] text-ink-4">Nothing here yet.</div>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function BrowserPaneRow({
  row,
  activeArtifactId,
  expandedFolderIds,
  onToggleFolder,
  onDrillIntoFolder,
  onOpenArtifact,
  onStartRenameFolder,
  onStartRenameArtifact,
  onCreateFolderHere,
  onCreateNoteHere,
}: {
  row: (ReturnType<typeof useBrowserPane>)["rows"][number];
  activeArtifactId: string | null;
  expandedFolderIds: string[];
  onToggleFolder: (folderId: string) => void;
  onDrillIntoFolder: (folderId: string) => void;
  onOpenArtifact: (artifactId: string) => void;
  onStartRenameFolder: (folderId: string, title: string) => void;
  onStartRenameArtifact: (artifactId: string, title: string) => void;
  onCreateFolderHere: (folderId: string) => void;
  onCreateNoteHere: (folderId: string) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const isActiveArtifact = row.artifactId === activeArtifactId;
  const isDrillFolder = row.kind === "folder" && row.depth >= DRILL_DEPTH && row.folderId;
  const isExpanded =
    row.folderId && !isDrillFolder ? expandedFolderIds.includes(row.folderId) : false;
  const paddingLeft = `${Math.min(row.depth, DRILL_DEPTH) * 14 + 6}px`;

  return (
    <div className="group relative">
      <div
        className={cn(
          "flex items-center gap-1 rounded-md pr-1",
          isActiveArtifact || menuOpen
            ? "bg-[rgb(var(--sel))] text-ink"
            : "text-ink-2 hover:bg-[rgb(var(--hover))] hover:text-ink",
          isActiveArtifact ? "font-medium" : null,
        )}
      >
        <button
          type="button"
          onClick={() => {
            if (row.kind === "artifact" && row.artifactId) {
              onOpenArtifact(row.artifactId);
              return;
            }
            if (row.kind === "folder" && row.folderId) {
              if (row.depth >= DRILL_DEPTH) {
                onDrillIntoFolder(row.folderId);
                return;
              }
              onToggleFolder(row.folderId);
            }
          }}
          className="flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-1.5 py-1 text-left text-[13px]"
          style={{ paddingLeft }}
        >
          {row.kind === "folder" ? (
            <span className="flex h-5 w-5 items-center justify-center text-ink-3">
              {row.depth >= DRILL_DEPTH ? (
                <ChevronRight className="h-3.5 w-3.5" strokeWidth={2} />
              ) : isExpanded ? (
                <ChevronDown className="h-3.5 w-3.5" strokeWidth={2} />
              ) : (
                <ChevronRight className="h-3.5 w-3.5" strokeWidth={2} />
              )}
            </span>
          ) : (
            <span className="w-5" />
          )}
          <span className="min-w-0 flex-1 truncate">{row.title}</span>
        </button>
        <DropdownMenu onOpenChange={setMenuOpen}>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className={cn(
                "flex h-5 w-5 shrink-0 items-center justify-center rounded text-ink-4 transition-opacity hover:bg-[rgb(var(--hover))] hover:text-ink",
                menuOpen ? "opacity-100" : "opacity-0 group-hover:opacity-100",
              )}
              aria-label={`More actions for ${row.title}`}
            >
              <Ellipsis className="h-3.5 w-3.5" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            {row.kind === "folder" && row.folderId ? (
              <>
                <DropdownMenuItem onClick={() => onStartRenameFolder(row.folderId!, row.title)}>
                  Rename folder
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onCreateFolderHere(row.folderId!)}>
                  New folder here
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onCreateNoteHere(row.folderId!)}>
                  New note here
                </DropdownMenuItem>
              </>
            ) : null}
            {row.kind === "artifact" && row.artifactId ? (
              <>
                <DropdownMenuItem onClick={() => onOpenArtifact(row.artifactId!)}>
                  Open artifact
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => onStartRenameArtifact(row.artifactId!, row.title)}>
                  Rename artifact
                </DropdownMenuItem>
              </>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
