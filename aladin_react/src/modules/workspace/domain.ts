import type { ArtifactKind, BrowserTreeNode, VoiceCaptureDraft } from "@/shared/api/models";

export interface RenameDraft {
  kind: "folder" | "artifact";
  rowId: string;
  originalTitle: string;
  draftTitle: string;
}

export interface WorkspaceShellState {
  activeArtifactId: string | null;
  openArtifactIds: string[];
  inspectorOverrides: Record<string, boolean>;
  browserScrollTop: number;
  focusedFolderId: string | null;
  expandedFolderIds: string[];
  activeRename: RenameDraft | null;
  activeVoiceDraft: VoiceCaptureDraft | null;
}

export const initialWorkspaceShellState: WorkspaceShellState = {
  activeArtifactId: null,
  openArtifactIds: [],
  inspectorOverrides: {},
  browserScrollTop: 0,
  focusedFolderId: null,
  expandedFolderIds: [],
  activeRename: null,
  activeVoiceDraft: null,
};

export interface BrowserTreeRow {
  id: string;
  depth: number;
  kind: "folder" | "artifact";
  title: string;
  ancestorFolderIds: string[];
  folderId?: string;
  artifactId?: string;
  artifactKind?: ArtifactKind;
  updatedLabel?: string | null;
}

export function buildBrowserRows(
  tree: BrowserTreeNode[],
  expandedFolderIds: string[],
  rootFolderId: string | null = null,
): BrowserTreeRow[] {
  const rows: BrowserTreeRow[] = [];
  const expandedSet = new Set(expandedFolderIds);
  const scopedNodes = rootFolderId ? findFolderChildren(tree, rootFolderId) ?? [] : tree;
  const rootAncestorIds = rootFolderId ? [rootFolderId] : [];

  const visit = (nodes: BrowserTreeNode[], depth: number, ancestorFolderIds: string[]) => {
    nodes.forEach((node) => {
      if (node.kind === "folder") {
        const folderAncestorIds = [...ancestorFolderIds, node.id];
        rows.push({
          id: node.id,
          depth,
          kind: "folder",
          title: node.title,
          ancestorFolderIds: folderAncestorIds,
          folderId: node.id,
        });
        if (expandedSet.has(node.id)) {
          visit(node.children, depth + 1, folderAncestorIds);
        }
      } else if (node.artifactPreview) {
        rows.push({
          id: node.id,
          depth,
          kind: "artifact",
          title: node.artifactPreview.title,
          ancestorFolderIds,
          artifactId: node.artifactPreview.id,
          artifactKind: node.artifactPreview.kind,
          updatedLabel: node.artifactPreview.updatedLabel,
        });
      }
    });
  };

  visit(scopedNodes, 0, rootAncestorIds);
  return rows;
}

export function findFolderNode(tree: BrowserTreeNode[], folderId: string): BrowserTreeNode | null {
  for (const node of tree) {
    if (node.kind !== "folder") continue;
    if (node.id === folderId) return node;
    const nested = findFolderNode(node.children, folderId);
    if (nested) return nested;
  }
  return null;
}

export function folderParent(tree: BrowserTreeNode[], folderId: string): BrowserTreeNode | null {
  function visit(nodes: BrowserTreeNode[], parent: BrowserTreeNode | null): BrowserTreeNode | null {
    for (const node of nodes) {
      if (node.kind !== "folder") continue;
      if (node.id === folderId) return parent;
      const found = visit(node.children, node);
      if (found !== null) return found;
    }
    return null;
  }
  return visit(tree, null);
}

export function folderAncestors(tree: BrowserTreeNode[], folderId: string): BrowserTreeNode[] {
  const path: BrowserTreeNode[] = [];

  function visit(nodes: BrowserTreeNode[]): boolean {
    for (const node of nodes) {
      if (node.kind !== "folder") continue;
      path.push(node);
      if (node.id === folderId) return true;
      if (visit(node.children)) return true;
      path.pop();
    }
    return false;
  }

  visit(tree);
  return path;
}

export function ancestorFolderIds(tree: BrowserTreeNode[], folderId: string | null): string[] {
  if (!folderId) return [];
  return folderAncestors(tree, folderId).map((node) => node.id);
}

export function findFolderChildren(tree: BrowserTreeNode[], folderId: string): BrowserTreeNode[] | null {
  for (const node of tree) {
    if (node.kind === "folder") {
      if (node.id === folderId) return node.children;
      const nested = findFolderChildren(node.children, folderId);
      if (nested) return nested;
    }
  }
  return null;
}

export function nextFolderTitle(tree: BrowserTreeNode[], folderId: string | null): string {
  const siblings = getFolderScopedNodes(tree, folderId);
  const existingTitles = new Set(
    siblings.filter((node) => node.kind === "folder").map((node) => node.title.toLowerCase()),
  );
  return nextTitled("New Folder", existingTitles);
}

export function nextArtifactTitle(
  tree: BrowserTreeNode[],
  folderId: string | null,
  kind: ArtifactKind = "note",
): string {
  const siblings = getFolderScopedNodes(tree, folderId);
  const existingTitles = new Set(
    siblings
      .filter((node) => node.kind === "artifact")
      .map((node) => node.artifactPreview?.title.toLowerCase() ?? node.title.toLowerCase()),
  );
  const baseTitle =
    kind === "link"
      ? "New Link"
      : kind === "voice"
        ? "Voice Note"
        : kind === "file"
          ? "New File"
          : "New Note";
  return nextTitled(baseTitle, existingTitles);
}

function getFolderScopedNodes(tree: BrowserTreeNode[], folderId: string | null): BrowserTreeNode[] {
  if (folderId === null) return tree;
  return findFolderChildren(tree, folderId) ?? [];
}

function nextTitled(baseTitle: string, existingTitles: Set<string>) {
  if (!existingTitles.has(baseTitle.toLowerCase())) return baseTitle;
  let counter = 1;
  while (existingTitles.has(`${baseTitle} ${counter}`.toLowerCase())) {
    counter += 1;
  }
  return `${baseTitle} ${counter}`;
}
