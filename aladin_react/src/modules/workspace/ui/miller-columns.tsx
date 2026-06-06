import { useState } from "react";
import { ChevronRight, FileText, Link2, Mic, Paperclip } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ArtifactKind, BrowserTreeNode } from "@/shared/api/models";
import { findFolderChildren } from "@/modules/workspace/domain";
import { folderTitle } from "@/services/workspace/workspace-helpers";
import { cn } from "@/lib/utils";

const COLUMNS = 4;
const COLUMN_WIDTH = 216; // 4 × 216 = 864px (the spec's fixed Miller width)

const ARTIFACT_ICONS: Record<ArtifactKind, LucideIcon> = {
  note: FileText,
  link: Link2,
  voice: Mic,
  file: Paperclip,
};

/**
 * Fixed four-column Miller navigator over the browser tree. Column 0 lists the
 * children of `startFolderId` (null = root); selecting a folder cascades its
 * children into the next column. Only the last four levels are shown.
 */
export function MillerColumns({
  tree,
  startFolderId,
  onOpenArtifact,
  onClose,
}: {
  tree: BrowserTreeNode[];
  startFolderId: string | null;
  onOpenArtifact: (artifactId: string) => void;
  onClose: () => void;
}) {
  const [path, setPath] = useState<string[]>([]);

  // Build every level, then show a sliding window of the last COLUMNS.
  const levels: BrowserTreeNode[][] = [];
  levels.push(startFolderId ? findFolderChildren(tree, startFolderId) ?? [] : tree);
  for (const folderId of path) {
    levels.push(findFolderChildren(tree, folderId) ?? []);
  }
  const baseIndex = Math.max(0, levels.length - COLUMNS);
  const visible = levels.slice(baseIndex);

  const selectFolder = (levelIndex: number, folderId: string) => {
    setPath([...path.slice(0, levelIndex), folderId]);
  };

  const heading = startFolderId ? folderTitle(tree, startFolderId) ?? "Folder" : "Workspace";

  return (
    <div className="flex flex-col" style={{ width: COLUMNS * COLUMN_WIDTH }}>
      <div className="flex items-center justify-between border-b border-line px-3 py-2">
        <span className="truncate font-mono text-[10.5px] font-semibold uppercase tracking-[0.08em] text-ink-3">
          {heading} · columns
        </span>
      </div>
      <div className="flex h-[420px]">
        {visible.map((nodes, vi) => {
          const levelIndex = baseIndex + vi;
          const selectedHere = path[levelIndex];
          return (
            <div
              key={levelIndex}
              className="flex h-full shrink-0 flex-col overflow-y-auto border-r border-line py-1 last:border-r-0"
              style={{ width: COLUMN_WIDTH }}
            >
              {nodes.length === 0 ? (
                <div className="px-3 py-3 text-[12px] text-ink-4">Empty</div>
              ) : (
                nodes.map((node) => {
                  if (node.kind === "folder") {
                    const active = selectedHere === node.id;
                    return (
                      <button
                        key={node.id}
                        type="button"
                        onClick={() => selectFolder(levelIndex, node.id)}
                        className={cn(
                          "mx-1 flex items-center justify-between gap-2 rounded px-2 py-1.5 text-left text-[13px] transition-colors",
                          active
                            ? "bg-[rgb(var(--sel))] text-ink"
                            : "text-ink-2 hover:bg-[rgb(var(--hover))] hover:text-ink",
                        )}
                      >
                        <span className="truncate">{node.title}</span>
                        <ChevronRight className="h-3.5 w-3.5 shrink-0 text-ink-4" strokeWidth={2} />
                      </button>
                    );
                  }
                  if (node.artifactPreview) {
                    const preview = node.artifactPreview;
                    const Icon = ARTIFACT_ICONS[preview.kind] ?? FileText;
                    return (
                      <button
                        key={node.id}
                        type="button"
                        onClick={() => {
                          onOpenArtifact(preview.id);
                          onClose();
                        }}
                        className="mx-1 flex items-center gap-2 rounded px-2 py-1.5 text-left text-[13px] text-ink-2 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                      >
                        <Icon className="h-3.5 w-3.5 shrink-0 text-ink-4" strokeWidth={1.75} />
                        <span className="truncate">{preview.title}</span>
                      </button>
                    );
                  }
                  return null;
                })
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
