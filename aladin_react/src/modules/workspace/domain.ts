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
  focusedFolderId: string | null;
  scopeFolderId: string | null;
  scopeBackStack: (string | null)[];
  expandedFolderIds: string[];
  activeRename: RenameDraft | null;
  activeVoiceDraft: VoiceCaptureDraft | null;
}

export const initialWorkspaceShellState: WorkspaceShellState = {
  activeArtifactId: null,
  openArtifactIds: [],
  inspectorOverrides: {},
  focusedFolderId: null,
  scopeFolderId: null,
  scopeBackStack: [],
  expandedFolderIds: [],
  activeRename: null,
  activeVoiceDraft: null,
};

export interface BrowserTreeRow {
  id: string;
  depth: number;
  kind: "folder" | "artifact";
  title: string;
  folderId?: string;
  artifactId?: string;
  artifactKind?: ArtifactKind;
  updatedLabel?: string | null;
  scopeCandidate: boolean;
}

export function buildBrowserRows(
  tree: BrowserTreeNode[],
  currentScopeId: string | null,
  expandedFolderIds: string[],
): BrowserTreeRow[] {
  const rows: BrowserTreeRow[] = [];
  const rootNodes =
    currentScopeId === null ? tree : findFolderChildren(tree, currentScopeId) ?? [];
  const expandedSet = new Set(expandedFolderIds);

  const visit = (nodes: BrowserTreeNode[], depth: number) => {
    nodes.forEach((node) => {
      if (node.kind === "folder") {
        rows.push({
          id: node.id,
          depth,
          kind: "folder",
          title: node.title,
          folderId: node.id,
          scopeCandidate: depth >= 2,
        });
        if (expandedSet.has(node.id)) {
          visit(node.children, depth + 1);
        }
      } else if (node.artifactPreview) {
        rows.push({
          id: node.id,
          depth,
          kind: "artifact",
          title: node.artifactPreview.title,
          artifactId: node.artifactPreview.id,
          artifactKind: node.artifactPreview.kind,
          updatedLabel: node.artifactPreview.updatedLabel,
          scopeCandidate: false,
        });
      }
    });
  };

  visit(rootNodes, 0);
  return rows;
}

export interface BreadcrumbSibling {
  id: string;
  label: string;
}

export interface BreadcrumbCrumb {
  kind: "root" | "folder" | "ellipsis";
  id: string | null;
  label: string;
  siblings: BreadcrumbSibling[];
}

export interface BreadcrumbView {
  crumbs: BreadcrumbCrumb[];
  visible: BreadcrumbCrumb[];
  hidden: BreadcrumbCrumb[];
}

const MAX_VISIBLE_CRUMBS = 3;

export function buildBreadcrumbView(tree: BrowserTreeNode[], folderId: string | null): BreadcrumbView {
  const topLevelFolders = tree.filter((node) => node.kind === "folder");
  const rootCrumb: BreadcrumbCrumb = {
    kind: "root",
    id: null,
    label: "root",
    siblings: topLevelFolders.map((node) => ({ id: node.id, label: node.title })),
  };

  const ancestorChain = folderId ? folderAncestors(tree, folderId) : [];
  const folderCrumbs: BreadcrumbCrumb[] = ancestorChain.map((node) => ({
    kind: "folder",
    id: node.id,
    label: node.title,
    siblings: folderSiblings(tree, node.id).map((sibling) => ({ id: sibling.id, label: sibling.title })),
  }));

  const crumbs: BreadcrumbCrumb[] = [rootCrumb, ...folderCrumbs];

  if (crumbs.length <= MAX_VISIBLE_CRUMBS) {
    return { crumbs, visible: crumbs, hidden: [] };
  }

  const hidden = crumbs.slice(1, -1);
  const ellipsisCrumb: BreadcrumbCrumb = {
    kind: "ellipsis",
    id: null,
    label: "…",
    siblings: hidden
      .filter((crumb) => crumb.id !== null)
      .map((crumb) => ({ id: crumb.id as string, label: crumb.label })),
  };
  const visible: BreadcrumbCrumb[] = [crumbs[0], ellipsisCrumb, crumbs[crumbs.length - 1]];
  return { crumbs, visible, hidden };
}

export function folderSiblings(tree: BrowserTreeNode[], folderId: string): BrowserTreeNode[] {
  const parent = folderParent(tree, folderId);
  const scope = parent ? parent.children : tree;
  return scope.filter((node) => node.kind === "folder" && node.id !== folderId);
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
