import type { ArtifactKind, BrowserTreeNode, BreadcrumbItem } from "@/shared/api/models";

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

export function buildFolderBreadcrumbs(tree: BrowserTreeNode[], folderId: string | null): BreadcrumbItem[] {
  if (folderId === null) {
    return [{ id: null, label: "Folders" }];
  }
  const folderPath = folderAncestors(tree, folderId);
  return [{ id: null, label: "Folders" }, ...folderPath.map((node) => ({ id: node.id, label: node.title }))];
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

export function ancestorFolderIdsForArtifact(tree: BrowserTreeNode[], artifactId: string): string[] {
  const path: string[] = [];

  function visit(nodes: BrowserTreeNode[]): boolean {
    for (const node of nodes) {
      if (node.kind === "folder") {
        path.push(node.id);
        if (visit(node.children)) {
          return true;
        }
        path.pop();
      } else if (node.artifactPreview?.id === artifactId) {
        return true;
      }
    }
    return false;
  }

  visit(tree);
  return path;
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
