import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  ArrowRight,
  ChevronRight,
  Columns3,
  FileText,
  Folder,
  Home,
  Link2,
  Mic,
  Paperclip,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ArtifactKind, BrowserTreeNode } from "@/shared/api/models";
import { findFolderChildren } from "@/modules/workspace/domain";
import { folderTitle } from "@/services/workspace/workspace-helpers";
import { cn } from "@/lib/utils";

const COLUMN_WIDTH = 216;
const PREVIEW_WIDTH = 282;
const PANE_HEIGHT = 460;
const PANE_MAX_WIDTH = COLUMN_WIDTH * 4; // 864 — exactly four columns

const TYPE_META: Record<ArtifactKind, { label: string; icon: LucideIcon }> = {
  note: { label: "Page", icon: FileText },
  link: { label: "Link", icon: Link2 },
  voice: { label: "Voice", icon: Mic },
  file: { label: "File", icon: Paperclip },
};

function findArtifactNode(tree: BrowserTreeNode[], artifactId: string): BrowserTreeNode | null {
  for (const node of tree) {
    if (node.kind === "artifact" && node.artifactPreview?.id === artifactId) return node;
    if (node.children?.length) {
      const found = findArtifactNode(node.children, artifactId);
      if (found) return found;
    }
  }
  return null;
}

export interface MillerState {
  seed: string[]; // folderId path from root to the seeded folder ([] = workspace root)
  anchor: DOMRect;
  seedLeaf?: string;
}

export function MillerColumns({
  tree,
  state,
  onOpenArtifact,
  onClose,
}: {
  tree: BrowserTreeNode[];
  state: MillerState;
  onOpenArtifact: (artifactId: string) => void;
  onClose: () => void;
}) {
  const [path, setPath] = useState<string[]>(state.seed);
  const [leaf, setLeaf] = useState<BrowserTreeNode | null>(
    state.seedLeaf ? findArtifactNode(tree, state.seedLeaf) : null,
  );
  const columnsRef = useRef<HTMLDivElement | null>(null);

  // Esc + outside-click close (the catcher handles the click; Esc handled here).
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  // Auto-scroll to the freshest (rightmost) column on any path/leaf change.
  useLayoutEffect(() => {
    const el = columnsRef.current;
    if (el) el.scrollLeft = el.scrollWidth;
  }, [path, leaf]);

  // Columns: 0 = Workspace (root), then one per path id (children of path[ci-1]).
  const columns: { title: string; icon: LucideIcon; nodes: BrowserTreeNode[] }[] = [
    { title: "Workspace", icon: Home, nodes: tree },
  ];
  for (const folderId of path) {
    columns.push({
      title: folderTitle(tree, folderId) ?? "Folder",
      icon: Folder,
      nodes: findFolderChildren(tree, folderId) ?? [],
    });
  }

  const selectFolder = (columnIndex: number, folderId: string) => {
    setLeaf(null);
    setPath((current) => [...current.slice(0, columnIndex), folderId]);
  };

  // Anchored + clamped position.
  const paneWidth = Math.min(PANE_MAX_WIDTH, window.innerWidth - 24);
  const left = Math.min(Math.max(state.anchor.right - 2, 12), window.innerWidth - paneWidth - 14);
  const top = Math.min(Math.max(state.anchor.top - 4, 12), window.innerHeight - PANE_HEIGHT - 14);

  const breadcrumb = path.map((id) => folderTitle(tree, id) ?? "Folder");

  return createPortal(
    <div className="fixed inset-0 z-40" onMouseDown={onClose}>
      <div
        className="fixed z-40 flex flex-col overflow-hidden rounded-card border border-line bg-explorer animate-pop"
        style={{
          left,
          top,
          width: paneWidth,
          height: PANE_HEIGHT,
          boxShadow: "0 22px 60px rgba(0,0,0,.62), 0 0 0 1px rgba(0,0,0,.4)",
          transformOrigin: "top left",
        }}
        onMouseDown={(event) => event.stopPropagation()}
      >
        {/* Header — live breadcrumb */}
        <div className="flex items-center gap-2.5 border-b border-line2 px-3 py-2">
          <Columns3 className="h-4 w-4 shrink-0 text-ink-2" strokeWidth={1.75} />
          <div className="flex min-w-0 flex-1 items-center gap-1 truncate font-mono text-[10.5px]">
            <span className={breadcrumb.length === 0 && !leaf ? "text-ink" : "text-ink-3"}>workspace</span>
            {breadcrumb.map((name, i) => (
              <span key={i} className={i === breadcrumb.length - 1 && !leaf ? "text-ink" : "text-ink-3"}>
                <span className="px-1 text-ink-4">/</span>
                {name}
              </span>
            ))}
            {leaf?.artifactPreview ? (
              <span className="text-ink">
                <span className="px-1 text-ink-4">/</span>
                {leaf.artifactPreview.title}
              </span>
            ) : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="grid h-6 w-6 shrink-0 place-items-center rounded text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
            aria-label="Close columns"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Columns + optional leaf preview */}
        <div
          ref={columnsRef}
          className="flex min-h-0 flex-1 overflow-x-auto overflow-y-hidden"
          style={{ scrollSnapType: "x proximity", scrollBehavior: "smooth" }}
        >
          {columns.map((column, columnIndex) => {
            const ColIcon = column.icon;
            const selectedId = path[columnIndex];
            return (
              <div
                key={columnIndex}
                className="flex shrink-0 flex-col border-r border-line2"
                style={{ width: COLUMN_WIDTH, scrollSnapAlign: "end" }}
              >
                <div className="flex items-center gap-[7px] border-b border-line2 px-3 py-2">
                  <ColIcon className="h-4 w-4 shrink-0 text-ink-2" strokeWidth={1.75} />
                  <span className="min-w-0 flex-1 truncate text-[12.5px] font-semibold text-ink">{column.title}</span>
                  <span className="font-mono text-[10px] text-ink-4">{column.nodes.length}</span>
                </div>
                <div className="flex min-h-0 flex-1 flex-col gap-px overflow-y-auto px-[7px] pt-[5px] pb-2">
                  {column.nodes.map((node) => (
                    <MillerRow
                      key={node.id}
                      node={node}
                      selected={
                        node.kind === "folder"
                          ? selectedId === node.id
                          : leaf?.id === node.id
                      }
                      onSelectFolder={() => selectFolder(columnIndex, node.id)}
                      onSelectLeaf={() => setLeaf(node)}
                      onOpenLeaf={() => {
                        if (node.artifactPreview) {
                          onOpenArtifact(node.artifactPreview.id);
                          onClose();
                        }
                      }}
                    />
                  ))}
                  {column.nodes.length === 0 ? (
                    <div className="px-2 py-2 text-[12px] text-ink-4">Empty</div>
                  ) : null}
                </div>
              </div>
            );
          })}

          {leaf?.artifactPreview ? (
            <LeafPreview
              leaf={leaf}
              pathNames={breadcrumb}
              onOpen={() => {
                onOpenArtifact(leaf.artifactPreview!.id);
                onClose();
              }}
            />
          ) : null}
        </div>

        {/* Footer hints */}
        <div className="flex h-[30px] items-center gap-3 border-t border-line px-3 font-mono text-[10px] text-ink-4">
          <span>click folder to expand →</span>
          <span>double-click a file to open</span>
          <span className="ml-auto">esc to close</span>
        </div>
      </div>
    </div>,
    document.body,
  );
}

function MillerRow({
  node,
  selected,
  onSelectFolder,
  onSelectLeaf,
  onOpenLeaf,
}: {
  node: BrowserTreeNode;
  selected: boolean;
  onSelectFolder: () => void;
  onSelectLeaf: () => void;
  onOpenLeaf: () => void;
}) {
  const isFolder = node.kind === "folder";
  const Icon = isFolder
    ? Folder
    : node.artifactPreview
      ? TYPE_META[node.artifactPreview.kind].icon
      : FileText;
  const name = isFolder ? node.title : node.artifactPreview?.title ?? "Untitled";

  return (
    <button
      type="button"
      onClick={() => (isFolder ? onSelectFolder() : onSelectLeaf())}
      onDoubleClick={() => !isFolder && onOpenLeaf()}
      className={cn(
        "relative flex h-[30px] items-center gap-2.5 rounded-chip px-[9px] pl-[11px] text-left transition-colors",
        selected ? "bg-amber-soft text-ink" : "text-ink-2 hover:bg-[rgb(var(--hover))] hover:text-ink",
      )}
    >
      {selected ? <span className="absolute left-0 top-[5px] bottom-[5px] w-0.5 rounded bg-amber" /> : null}
      <Icon
        className={cn("h-4 w-4 shrink-0", isFolder && selected ? "text-amber" : isFolder ? "text-ink-2" : "text-ink-3")}
        strokeWidth={1.75}
      />
      <span className={cn("min-w-0 flex-1 truncate text-[12.5px]", isFolder ? "font-medium" : "font-normal", selected && "font-semibold")}>
        {name}
      </span>
      {isFolder ? (
        <span className="flex shrink-0 items-center gap-1 font-mono text-[10px] text-ink-4">
          {node.children?.length ?? 0}
          <ChevronRight className="h-3.5 w-3.5" strokeWidth={2} />
        </span>
      ) : null}
    </button>
  );
}

function LeafPreview({
  leaf,
  pathNames,
  onOpen,
}: {
  leaf: BrowserTreeNode;
  pathNames: string[];
  onOpen: () => void;
}) {
  const preview = leaf.artifactPreview!;
  const meta = TYPE_META[preview.kind];
  const Icon = meta.icon;
  return (
    <div className="flex shrink-0 flex-col bg-card" style={{ width: PREVIEW_WIDTH }}>
      <div className="flex flex-col gap-3 p-[18px]">
        <div
          className="grid h-[122px] place-items-center rounded-[10px] border border-line"
          style={{
            background:
              "repeating-linear-gradient(135deg, var(--field), var(--field) 10px, var(--card) 10px, var(--card) 20px)",
          }}
        >
          <Icon className="h-[26px] w-[26px] text-ink-3" strokeWidth={1.5} />
        </div>
        <div className="space-y-1">
          <div className="font-mono text-[10px] uppercase text-amber">
            {meta.label}
            {preview.updatedLabel ? ` · ${preview.updatedLabel}` : ""}
          </div>
          <div className="text-[14.5px] font-semibold text-ink">{preview.title}</div>
          <div className="font-mono text-[10.5px] text-ink-3">
            workspace{pathNames.length ? ` / ${pathNames.join(" / ")}` : ""}
          </div>
        </div>
      </div>
      <div className="mt-auto p-[18px] pt-0">
        <button
          type="button"
          onClick={onOpen}
          className="flex w-full items-center justify-center gap-1.5 rounded-[9px] bg-amber py-[9px] text-[13px] font-semibold text-[#0f0f12] transition-opacity hover:opacity-90"
        >
          Open in editor
          <ArrowRight className="h-4 w-4" strokeWidth={2} />
        </button>
      </div>
    </div>
  );
}
