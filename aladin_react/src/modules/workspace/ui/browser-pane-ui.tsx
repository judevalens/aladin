import { useEffect, useRef, useState } from "react";
import {
  ChevronRight,
  Columns3,
  FileText,
  Folder,
  Link2,
  Mic,
  Paperclip,
  Plus,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import type { ArtifactKind } from "@/shared/api/models";
import type { LucideIcon } from "lucide-react";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { AladinPanel } from "@/components/ui/aladin";
import { MillerColumns, type MillerState } from "@/modules/workspace/ui/miller-columns";
import { folderAncestors } from "@/modules/workspace/domain";
import { useBrowserPane } from "@/modules/workspace/hooks/use-workspace-state";
import { useAppStore } from "@/app/state/store";
import { cn } from "@/shared/lib/utils";

const MAX_INLINE = 2; // depths 0,1 expand inline; depth >= 2 drills into the Miller popup

const ARTIFACT_ICONS: Record<ArtifactKind, LucideIcon> = {
  note: FileText,
  link: Link2,
  voice: Mic,
  file: Paperclip,
};

export function BrowserPaneUI() {
  const {
    loading,
    errorMessage,
    tree,
    rows,
    activeArtifactId,
    expandedFolderIds,
    browserScrollTop,
    onToggleFolder,
    onBrowserScroll,
    onOpenArtifact,
    onStartRenameFolder,
    onStartRenameArtifact,
    onCreateFolderHere,
    onCreateNoteHere,
  } = useBrowserPane();
  const openCommandPalette = useAppStore((state) => state.setCommandPaletteOpen);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [miller, setMiller] = useState<MillerState | null>(null);

  const openMiller = (folderId: string | null, anchor: DOMRect, seedLeaf?: string) => {
    setMiller({
      seed: folderId ? folderAncestors(tree, folderId).map((node) => node.id) : [],
      anchor,
      seedLeaf,
    });
  };

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    if (Math.abs(element.scrollTop - browserScrollTop) < 1) return;
    element.scrollTop = browserScrollTop;
  }, [browserScrollTop, rows]);

  if (loading) {
    return (
      <section className="flex w-[274px] shrink-0 flex-col overflow-hidden border-r border-line bg-explorer">
        <div className="p-5 text-sm text-ink-2">Loading browser tree…</div>
      </section>
    );
  }

  if (errorMessage) {
    return (
      <section className="flex w-[274px] shrink-0 flex-col overflow-hidden border-r border-line bg-explorer p-4">
        <AladinPanel className="rounded-md border border-against/40 bg-against/10 p-4 text-sm text-against">{errorMessage}</AladinPanel>
      </section>
    );
  }

  return (
    <section className="flex w-[274px] shrink-0 flex-col overflow-hidden border-r border-line bg-explorer">
      {/* Header */}
      <div className="flex items-center gap-2 px-3 pt-[13px] pb-[11px]">
        <span className="font-mono text-[11px] font-bold uppercase tracking-[1px] text-ink-3">Workspace</span>
        <div className="ml-auto flex items-center gap-0.5">
          <button
            type="button"
            onClick={() => openCommandPalette(true)}
            className="grid h-6 w-6 place-items-center rounded-md text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
            aria-label="Add item"
            title="Add item"
          >
            <Plus className="h-4 w-4" strokeWidth={1.75} />
          </button>
          <button
            type="button"
            onClick={(event) => openMiller(null, event.currentTarget.getBoundingClientRect())}
            className="grid h-6 w-6 place-items-center rounded-md text-ink-2 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
            aria-label="Browse in columns"
            title="Browse in columns"
          >
            <Columns3 className="h-4 w-4" strokeWidth={1.75} />
          </button>
          <button
            type="button"
            className="grid h-6 w-6 place-items-center rounded-md text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
            aria-label="View options"
            title="View options"
          >
            <SlidersHorizontal className="h-4 w-4" strokeWidth={1.75} />
          </button>
        </div>
      </div>

      {/* Search pill */}
      <div className="px-3 pb-2.5">
        <button
          type="button"
          onClick={() => openCommandPalette(true)}
          className="flex w-full items-center gap-2 rounded-lg border border-line bg-field px-2.5 py-1.5 text-left transition-colors hover:border-ink-4"
        >
          <Search className="h-3.5 w-3.5 shrink-0 text-ink-3" strokeWidth={1.75} />
          <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-3">search or ask</span>
          <kbd className="rounded border border-line px-1 font-mono text-[10px] text-ink-4">⌘K</kbd>
        </button>
      </div>

      {/* Tree */}
      <div
        ref={scrollRef}
        className="min-h-0 flex-1 overflow-y-auto px-2 pt-0.5 pb-3"
        onScroll={(event) => onBrowserScroll(event.currentTarget.scrollTop)}
      >
        {rows.map((row) => (
          <BrowserPaneRow
            key={row.id}
            row={row}
            activeArtifactId={activeArtifactId}
            expandedFolderIds={expandedFolderIds}
            onToggleFolder={onToggleFolder}
            onOpenArtifact={onOpenArtifact}
            onStartRenameFolder={onStartRenameFolder}
            onStartRenameArtifact={onStartRenameArtifact}
            onCreateFolderHere={onCreateFolderHere}
            onCreateNoteHere={onCreateNoteHere}
            onOpenMiller={openMiller}
          />
        ))}
        {rows.length === 0 ? (
          <div className="px-3 py-10 text-center text-[12px] text-ink-4">Nothing here yet.</div>
        ) : null}
      </div>

      {miller ? (
        <MillerColumns
          tree={tree}
          state={miller}
          onOpenArtifact={onOpenArtifact}
          onClose={() => setMiller(null)}
        />
      ) : null}
    </section>
  );
}

function BrowserPaneRow({
  row,
  activeArtifactId,
  expandedFolderIds,
  onToggleFolder,
  onOpenArtifact,
  onStartRenameFolder,
  onStartRenameArtifact,
  onCreateFolderHere,
  onCreateNoteHere,
  onOpenMiller,
}: {
  row: (ReturnType<typeof useBrowserPane>)["rows"][number];
  activeArtifactId: string | null;
  expandedFolderIds: string[];
  onToggleFolder: (folderId: string) => void;
  onOpenArtifact: (artifactId: string) => void;
  onStartRenameFolder: (folderId: string, title: string) => void;
  onStartRenameArtifact: (artifactId: string, title: string) => void;
  onCreateFolderHere: (folderId: string) => void;
  onCreateNoteHere: (folderId: string) => void;
  onOpenMiller: (folderId: string | null, anchor: DOMRect, seedLeaf?: string) => void;
}) {
  const rowRef = useRef<HTMLButtonElement | null>(null);
  const isActive = row.artifactId === activeArtifactId;
  const isExpandableFolder = row.kind === "folder" && row.depth < MAX_INLINE;
  const isDrillFolder = row.kind === "folder" && row.depth >= MAX_INLINE;
  const isExpanded = isExpandableFolder && row.folderId ? expandedFolderIds.includes(row.folderId) : false;
  const TypeIcon = row.kind === "folder" ? Folder : row.artifactKind ? ARTIFACT_ICONS[row.artifactKind] : FileText;

  const rect = () => rowRef.current?.getBoundingClientRect() ?? new DOMRect();

  const handleClick = () => {
    if (row.kind === "artifact" && row.artifactId) {
      onOpenArtifact(row.artifactId);
      return;
    }
    if (row.kind === "folder" && row.folderId) {
      if (isDrillFolder) {
        onOpenMiller(row.folderId, rect());
      } else {
        onToggleFolder(row.folderId);
      }
    }
  };

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <button
          ref={rowRef}
          type="button"
          onClick={handleClick}
          style={{ paddingLeft: 10 + row.depth * 15 }}
          className={cn(
            "relative flex h-7 w-full items-center gap-[7px] rounded-md pr-2.5 text-left transition-colors",
            isActive
              ? "bg-[rgb(var(--sel))] text-ink"
              : cn("hover:bg-[rgb(var(--hover))]", row.kind === "folder" ? "text-ink" : "text-ink-2"),
          )}
        >
          {isActive ? <span className="absolute left-0 top-[5px] bottom-[5px] w-0.5 rounded bg-amber" /> : null}
          {/* Vertical guide lines — one per ancestor depth; consecutive rows make them continuous. */}
          {Array.from({ length: row.depth }, (_, ancestorDepth) => (
            <span
              key={ancestorDepth}
              aria-hidden="true"
              className="pointer-events-none absolute top-0 bottom-0 w-px bg-[rgb(var(--line-2))]"
              style={{ left: 10 + ancestorDepth * 15 + 6 }}
            />
          ))}
          {/* Chevron / spacer (14px) */}
          {isExpandableFolder ? (
            <ChevronRight
              className={cn("h-3.5 w-3.5 shrink-0 text-ink-3 transition-transform duration-100", isExpanded && "rotate-90")}
              strokeWidth={2}
            />
          ) : (
            <span className="h-3.5 w-3.5 shrink-0" />
          )}
          {/* Type icon (16px) */}
          <TypeIcon
            className={cn("h-4 w-4 shrink-0", row.kind === "folder" ? "text-ink-2" : "text-ink-3")}
            strokeWidth={1.75}
          />
          <span
            className={cn(
              "min-w-0 flex-1 truncate text-[13px]",
              isActive ? "font-semibold" : row.kind === "folder" ? "font-medium" : "font-normal",
            )}
          >
            {row.title}
          </span>
          {row.kind === "folder" ? (
            <span className="flex shrink-0 items-center gap-1 font-mono text-[10.5px] text-ink-4">
              {row.childCount ?? 0}
              {isDrillFolder ? <ChevronRight className="h-3.5 w-3.5" strokeWidth={2} /> : null}
            </span>
          ) : null}
        </button>
      </ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        {row.kind === "folder" && row.folderId ? (
          <>
            <ContextMenuItem onSelect={() => onOpenMiller(row.folderId!, rect())}>
              <Columns3 className="h-[15px] w-[15px] text-amber" strokeWidth={1.75} />
              <span className="font-medium">Browse in columns</span>
            </ContextMenuItem>
            <ContextMenuSeparator />
            {isExpandableFolder ? (
              <ContextMenuItem onSelect={() => onToggleFolder(row.folderId!)}>
                {isExpanded ? "Collapse" : "Expand"}
              </ContextMenuItem>
            ) : null}
            <ContextMenuItem onSelect={() => onCreateFolderHere(row.folderId!)}>New folder here</ContextMenuItem>
            <ContextMenuItem onSelect={() => onCreateNoteHere(row.folderId!)}>New note here</ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem onSelect={() => onStartRenameFolder(row.folderId!, row.title)}>Rename</ContextMenuItem>
          </>
        ) : null}
        {row.kind === "artifact" && row.artifactId ? (
          <>
            <ContextMenuItem onSelect={() => onOpenArtifact(row.artifactId!)}>
              <span className="font-medium">Open</span>
            </ContextMenuItem>
            <ContextMenuItem
              onSelect={() => onOpenMiller(row.ancestorFolderIds.at(-1) ?? null, rect(), row.artifactId)}
            >
              <Columns3 className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
              Reveal in columns
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem onSelect={() => onStartRenameArtifact(row.artifactId!, row.title)}>Rename</ContextMenuItem>
          </>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}
