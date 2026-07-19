import type {
  Artifact,
  BrowserTreeNode,
  VoiceCaptureDraft,
} from "@/shared/api/models";
import {
  ancestorFolderIds,
  buildBrowserRows,
  findFolderNode,
  nextArtifactTitle,
  nextFolderTitle,
} from "@/modules/workspace/domain";

export function resolveWorkspaceDestination(pathname: string) {
  if (pathname.startsWith("/markets") || pathname.startsWith("/ticker/")) return "markets";
  if (pathname.startsWith("/folders")) return "folders";
  if (pathname.startsWith("/sources")) return "sources";
  if (pathname.startsWith("/signals")) return "signals";
  if (pathname.startsWith("/insights")) return "insights";
  // Both the index and the entity detail page (/entity/:id — note: NOT a prefix of
  // "/entities") belong to the Entities destination.
  if (pathname.startsWith("/entities") || pathname.startsWith("/entity/")) return "entities";
  if (pathname.startsWith("/graph")) return "graph";
  return "home";
}

export function createFolderCommand(tree: BrowserTreeNode[], folderId: string | null) {
  return {
    parentId: folderId,
    title: nextFolderTitle(tree, folderId),
  };
}

export function createArtifactCommand(
  tree: BrowserTreeNode[],
  folderId: string | null,
  kind: "note" | "link",
  opts?: { title?: string; sourceUrl?: string },
) {
  const title = opts?.title?.trim() || nextArtifactTitle(tree, folderId, kind);
  return {
    type: kind === "note" ? "page" : "link",
    folderId,
    title,
    content: kind === "note" ? "New artifact" : "",
    sourceUrl: kind === "link" ? (opts?.sourceUrl ?? "") : undefined,
  };
}

export function createVoiceDraft(
  tree: BrowserTreeNode[],
  folderId: string | null,
): VoiceCaptureDraft {
  return {
    folderId,
    title: nextArtifactTitle(tree, folderId, "voice"),
    description: "",
    phase: "ready",
    errorMessage: null,
  };
}

export function buildWorkspaceRows(
  tree: BrowserTreeNode[],
  expandedFolderIds: string[],
  rootFolderId: string | null,
) {
  return buildBrowserRows(tree, expandedFolderIds, rootFolderId);
}

export function folderAncestorIds(tree: BrowserTreeNode[], folderId: string | null) {
  return ancestorFolderIds(tree, folderId);
}

export function folderTitle(tree: BrowserTreeNode[], folderId: string | null) {
  if (!folderId) return null;
  return findFolderNode(tree, folderId)?.title ?? null;
}

export function resolveOpenArtifacts(
  openArtifacts: Artifact[],
  activeArtifactId: string | null,
  activeArtifact: Artifact | null | undefined,
) {
  return activeArtifact ?? openArtifacts.find((artifact) => artifact.id === activeArtifactId) ?? null;
}
