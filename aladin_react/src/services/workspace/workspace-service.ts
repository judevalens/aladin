import type { Artifact, BrowserTreeNode, VoiceCaptureDraft } from "@/shared/api/models";
import {
  ancestorFolderIds,
  buildBreadcrumbView,
  buildBrowserRows,
  initialWorkspaceShellState,
  nextArtifactTitle,
  nextFolderTitle,
  type WorkspaceShellState,
} from "@/modules/workspace/domain";

export function createWorkspaceInitialState(): WorkspaceShellState {
  return initialWorkspaceShellState;
}

export function resolveWorkspaceDestination(pathname: string) {
  if (pathname.startsWith("/folders")) return "folders";
  if (pathname.startsWith("/sources")) return "sources";
  if (pathname.startsWith("/signals")) return "signals";
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
) {
  return {
    type: kind === "note" ? "page" : "link",
    folderId,
    title: nextArtifactTitle(tree, folderId, kind),
    content: kind === "note" ? "New artifact" : "",
    sourceUrl: kind === "link" ? "https://example.com" : undefined,
  };
}

export function createVoiceDraft(tree: BrowserTreeNode[], folderId: string | null): VoiceCaptureDraft {
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
  scopeFolderId: string | null,
  expandedFolderIds: string[],
) {
  return buildBrowserRows(tree, scopeFolderId, expandedFolderIds);
}

export function buildWorkspaceBreadcrumbView(tree: BrowserTreeNode[], folderId: string | null) {
  return buildBreadcrumbView(tree, folderId);
}

export function folderAncestorIds(tree: BrowserTreeNode[], folderId: string | null) {
  return ancestorFolderIds(tree, folderId);
}

export function resolveOpenArtifacts(
  openArtifacts: Artifact[],
  activeArtifactId: string | null,
  activeArtifact: Artifact | null | undefined,
) {
  return activeArtifact ?? openArtifacts.find((artifact) => artifact.id === activeArtifactId) ?? null;
}
