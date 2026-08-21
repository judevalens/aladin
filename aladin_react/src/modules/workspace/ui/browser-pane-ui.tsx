import { useEffect, useRef, useState } from "react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { ChevronRight, Columns3, FileText, FlaskConical, Folder, Plus, Search, SlidersHorizontal } from "lucide-react";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { AladinPanel } from "@/components/ui/aladin";
import { MillerColumns, type MillerState } from "@/modules/workspace/ui/miller-columns";
import { folderAncestors, isContainerKind } from "@/modules/workspace/domain";
import type { ResearchView } from "@/modules/workspace/domain";

// Structural-slot icons (§5). Muted: the slots are chrome, the captured material is the
// content, and only the research folder itself carries the accent.
import { ARTIFACT_ICONS, RESEARCH_SLOT_ICONS } from "@/modules/workspace/ui/kind-icons";
import { useBrowserPane } from "@/modules/workspace/hooks/use-workspace-state";
import { useAppStore } from "@/app/state/store";
import { PropertyFilterDialogUI } from "@/modules/artifacts/ui/property-filter-dialog-ui";
import { DeleteConfirmDialog, type DeleteTarget } from "@/modules/workspace/ui/delete-confirm-dialog";
import { cn } from "@/lib/utils";

const MAX_INLINE = 2; // depths 0,1 expand inline; depth >= 2 drills into the Miller popup

export function BrowserPaneUI() {
  const [propertyFilterOpen, setPropertyFilterOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const {
    loading,
    errorMessage,
    tree,
    rows,
    activeArtifactId,
    expandedFolderIds,
    browserScrollTop,
    onToggleFolder,
    onOpenResearchView,
    onBrowserScroll,
    onOpenArtifact,
    onStartRenameFolder,
    onStartRenameResearch,
    onStartRenameArtifact,
    onCreateFolderHere,
    onDeleteFolder,
    onDeleteArtifact,
    onCreateResearchHere,
    onCreateNoteHere,
    onCreateBoardHere,
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
        <div className="p-5 text-body text-ink-2">Loading browser tree…</div>
      </section>
    );
  }

  if (errorMessage) {
    return (
      <section className="flex w-[274px] shrink-0 flex-col overflow-hidden border-r border-line bg-explorer p-4">
        <AladinPanel className="rounded-control border border-against/40 bg-against/10 p-4 text-body text-against">{errorMessage}</AladinPanel>
      </section>
    );
  }

  return (
    <section className="flex w-[274px] shrink-0 flex-col overflow-hidden border-r border-line bg-explorer">
      {/* Header */}
      <div className="flex items-center gap-2 px-3 pt-[13px] pb-[11px]">
        <Eyebrow>Workspace</Eyebrow>
        <div className="ml-auto flex items-center gap-0.5">
          <button
            type="button"
            onClick={() => openCommandPalette(true)}
            className="grid h-6 w-6 place-items-center rounded-control text-ink-3 transition-colors hover:bg-hover hover:text-ink"
            aria-label="Add item"
            title="Add item"
          >
            <Icon as={Plus} />
          </button>
          <button
            type="button"
            onClick={(event) => openMiller(null, event.currentTarget.getBoundingClientRect())}
            className="grid h-6 w-6 place-items-center rounded-control text-ink-2 transition-colors hover:bg-hover hover:text-ink"
            aria-label="Browse in columns"
            title="Browse in columns"
          >
            <Icon as={Columns3} />
          </button>
          <button
            type="button"
            onClick={() => setPropertyFilterOpen(true)}
            className="grid h-6 w-6 place-items-center rounded-control text-ink-3 transition-colors hover:bg-hover hover:text-ink"
            aria-label="Filter by property"
            title="Filter by property"
          >
            <Icon as={SlidersHorizontal} />
          </button>
        </div>
      </div>

      {/* Search pill */}
      <div className="px-3 pb-2.5">
        <button
          type="button"
          onClick={() => openCommandPalette(true)}
          className="flex w-full items-center gap-2 rounded-control border border-line bg-field px-2.5 py-1.5 text-left transition-colors hover:border-ink-4"
        >
          <Icon as={Search} size="inline" className="shrink-0 text-ink-3" />
          <span className="min-w-0 flex-1 truncate font-mono text-small text-ink-3">search or ask</span>
          <kbd className="rounded-tap border border-line px-1 font-mono text-meta text-ink-4">⌘K</kbd>
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
            onOpenResearch={onOpenResearchView}
            onOpenArtifact={onOpenArtifact}
            onRequestDelete={setDeleteTarget}
            onStartRenameFolder={onStartRenameFolder}
            onStartRenameResearch={onStartRenameResearch}
            onStartRenameArtifact={onStartRenameArtifact}
            onCreateFolderHere={onCreateFolderHere}
            onCreateResearchHere={onCreateResearchHere}
            onCreateNoteHere={onCreateNoteHere}
            onCreateBoardHere={onCreateBoardHere}
            onOpenMiller={openMiller}
          />
        ))}
        {rows.length === 0 ? (
          <div className="px-3 py-10 text-center text-small text-ink-4">Nothing here yet.</div>
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
      {/* Mounted only while open: the hook reaches for app composition + fetches facets, so
          keeping it unmounted avoids that work (and lets the pane render without providers). */}
      {propertyFilterOpen && <PropertyFilterDialogUI open onOpenChange={setPropertyFilterOpen} />}
      <DeleteConfirmDialog
        target={deleteTarget}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={(target) =>
          target.kind === "artifact" ? onDeleteArtifact(target.id) : onDeleteFolder(target.id)
        }
      />
    </section>
  );
}

function BrowserPaneRow({
  row,
  activeArtifactId,
  expandedFolderIds,
  onToggleFolder,
  onOpenResearch,
  onOpenArtifact,
  onRequestDelete,
  onStartRenameFolder,
  onStartRenameResearch,
  onStartRenameArtifact,
  onCreateFolderHere,
  onCreateResearchHere,
  onCreateNoteHere,
  onCreateBoardHere,
  onOpenMiller,
}: {
  row: (ReturnType<typeof useBrowserPane>)["rows"][number];
  activeArtifactId: string | null;
  expandedFolderIds: string[];
  onToggleFolder: (folderId: string) => void;
  onOpenResearch: (contextId: string, view: ResearchView) => void;
  onOpenArtifact: (artifactId: string) => void;
  onRequestDelete: (target: DeleteTarget) => void;
  onStartRenameFolder: (folderId: string, title: string) => void;
  onStartRenameResearch: (nodeId: string, title: string) => void;
  onStartRenameArtifact: (artifactId: string, title: string) => void;
  onCreateFolderHere: (folderId: string) => void;
  onCreateResearchHere: (folderId: string) => void;
  onCreateNoteHere: (folderId: string) => void;
  onCreateBoardHere: (folderId: string) => void;
  onOpenMiller: (folderId: string | null, anchor: DOMRect, seedLeaf?: string) => void;
}) {
  const rowRef = useRef<HTMLButtonElement | null>(null);
  const isActive = row.artifactId === activeArtifactId;
  // A research folder has the anatomy of a folder (§5) — it expands, it drills, it
  // holds children. Everything below keys off isContainerKind, never `=== "folder"`.
  const isContainer = isContainerKind(row.kind);
  const isResearch = row.kind === "research";
  const isSlot = row.kind === "research-view";
  const isExpandableFolder = isContainer && row.depth < MAX_INLINE;
  const isDrillFolder = isContainer && row.depth >= MAX_INLINE;
  const isExpanded = isExpandableFolder && row.folderId ? expandedFolderIds.includes(row.folderId) : false;
  const TypeIcon = isSlot
    ? RESEARCH_SLOT_ICONS[row.researchView ?? "overview"]
    : isResearch
      ? FlaskConical
      : row.kind === "folder"
        ? Folder
        : row.artifactKind
          ? ARTIFACT_ICONS[row.artifactKind]
          : FileText;
  // §5: a research folder has STATE — that's the clearest tell it isn't a plain folder.
  // Anything but idle earns the amber dot; idle stays quiet so the tree doesn't nag.
  const showRunDot = isResearch && !!row.research && row.research.runState !== "idle";

  const rect = () => rowRef.current?.getBoundingClientRect() ?? new DOMRect();

  const handleClick = () => {
    // A structural slot (§5) opens its view. It has no children of its own, so it never
    // expands — it behaves like a leaf that happens to be guaranteed to exist.
    if (isSlot && row.contextId && row.researchView) {
      onOpenResearch(row.contextId, row.researchView);
      return;
    }
    if (row.kind === "artifact" && row.artifactId) {
      onOpenArtifact(row.artifactId);
      return;
    }
    if (isContainer && row.folderId) {
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
            "relative flex h-7 w-full items-center gap-[7px] rounded-control pr-2.5 text-left transition-colors",
            isActive
              ? "bg-sel text-ink"
              : cn("hover:bg-hover", isContainer ? "text-ink" : "text-ink-2"),
          )}
        >
          {isActive ? <span className="absolute left-0 top-[5px] bottom-[5px] w-0.5 rounded-tap bg-amber" /> : null}
          {/* Vertical guide lines — one per ancestor depth; consecutive rows make them continuous. */}
          {Array.from({ length: row.depth }, (_, ancestorDepth) => (
            <span
              key={ancestorDepth}
              aria-hidden="true"
              className="pointer-events-none absolute top-0 bottom-0 w-px bg-line-2"
              style={{ left: 10 + ancestorDepth * 15 + 6 }}
            />
          ))}
          {/* Chevron / spacer (14px) */}
          {isExpandableFolder ? (
            <Icon as={ChevronRight} size="inline" mark className={cn("shrink-0 text-ink-3 transition-transform duration-100", isExpanded && "rotate-90")} />
          ) : (
            <span className="h-3.5 w-3.5 shrink-0" />
          )}
          {/* Type icon (16px) */}
          <Icon as={TypeIcon} className={cn(" shrink-0", isResearch ? "text-amber" : isContainer ? "text-ink-2" : "text-ink-3", isSlot && "text-ink-4", )} />
          <span
            className={cn("min-w-0 flex-1 truncate text-body",
              isActive ? "font-semibold" : isContainer ? "font-medium" : "font-normal",
            )}
          >
            {row.title}
          </span>
          {isContainer ? (
            <span className="flex shrink-0 items-center gap-1 font-mono text-meta text-ink-4">
              {showRunDot ? (
                <span
                  className="size-1.5 rounded-full bg-amber"
                  title={row.research?.runState}
                  aria-label={`run state: ${row.research?.runState}`}
                />
              ) : null}
              {row.childCount ?? 0}
              {isDrillFolder ? <Icon as={ChevronRight} size="inline" mark /> : null}
            </span>
          ) : null}
        </button>
      </ContextMenuTrigger>
      <ContextMenuContent className="w-56">
        {isSlot ? null : isContainer && row.folderId ? (
          <>
            <ContextMenuItem onSelect={() => onOpenMiller(row.folderId!, rect())}>
              <Icon as={Columns3} className="text-amber" />
              <span className="font-medium">Browse in columns</span>
            </ContextMenuItem>
            <ContextMenuSeparator />
            {isExpandableFolder ? (
              <ContextMenuItem onSelect={() => onToggleFolder(row.folderId!)}>
                {isExpanded ? "Collapse" : "Expand"}
              </ContextMenuItem>
            ) : null}
            <ContextMenuItem onSelect={() => onCreateFolderHere(row.folderId!)}>New folder here</ContextMenuItem>
            {/* §5: research nests in plain folders only — never inside another research. */}
            {row.kind === "folder" ? (
              <ContextMenuItem onSelect={() => onCreateResearchHere(row.folderId!)}>
                New research here
              </ContextMenuItem>
            ) : null}
            <ContextMenuItem onSelect={() => onCreateNoteHere(row.folderId!)}>New note here</ContextMenuItem>
            <ContextMenuItem onSelect={() => onCreateBoardHere(row.folderId!)}>New board here</ContextMenuItem>
            {/* Rename dispatches by kind: the folder API is deliberately folder-only, so a
                research folder renames through its own endpoint. */}
            <ContextMenuSeparator />
            <ContextMenuItem
              onSelect={() =>
                isResearch
                  ? onStartRenameResearch(row.folderId!, row.title)
                  : onStartRenameFolder(row.folderId!, row.title)
              }
            >
              Rename
            </ContextMenuItem>
            <ContextMenuItem
              className="text-against data-[highlighted]:text-against"
              onSelect={() =>
                onRequestDelete({
                  kind: isResearch ? "research" : "folder",
                  id: row.folderId!,
                  title: row.title,
                  childCount: row.childCount,
                })
              }
            >
              Delete
            </ContextMenuItem>
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
              <Icon as={Columns3} className="text-ink-3" />
              Reveal in columns
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem onSelect={() => onStartRenameArtifact(row.artifactId!, row.title)}>Rename</ContextMenuItem>
            <ContextMenuItem
              className="text-against data-[highlighted]:text-against"
              onSelect={() => onRequestDelete({ kind: "artifact", id: row.artifactId!, title: row.title })}
            >
              Delete
            </ContextMenuItem>
          </>
        ) : null}
      </ContextMenuContent>
    </ContextMenu>
  );
}
