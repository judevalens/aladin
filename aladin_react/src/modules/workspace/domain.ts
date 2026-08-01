import type {
  ArtifactKind,
  BrowserNodeKind,
  BrowserTreeNode,
  ResearchNodeMeta,
  VoiceCaptureDraft,
} from "@/shared/api/models";

/**
 * A CONTAINER is a node that can hold children and therefore expands, drills, accepts
 * drops, and appears as a move target: a plain folder or a research folder
 * (RESEARCH_SURFACE_PRD §5, §21).
 *
 * Every tree traversal must use this instead of `kind === "folder"`. That comparison is
 * what made research nodes render as leaves — and on the backend, the same assumption
 * meant a research folder could contain nothing at all.
 */
export function isContainerKind(kind: BrowserNodeKind | string): boolean {
  return kind === "folder" || kind === "research";
}

export interface RenameDraft {
  // Research folders rename through their own endpoint (the folder API is deliberately
  // folder-only), so the draft carries the kind and the dialog dispatches on it.
  kind: "folder" | "artifact" | "research";
  rowId: string;
  originalTitle: string;
  draftTitle: string;
}

/**
 * The work pane's tab model (RESEARCH_SURFACE_PRD §11).
 *
 * A tab used to be "an open artifact id". That stopped being true with the research
 * bench: Overview, Manifest, Runs, Code and Inspect are views on a research folder's
 * EXTENSION ROW, not rows in the artifact table. So a tab is a discriminated union, and
 * the research arm carries a `contextId` — the research node it belongs to.
 *
 * §12's grouping is derived from that contextId: tabs sharing one are contiguous. There
 * is no add-to-group and no group membership state, which is what deletes an entire
 * surface of UI.
 */
export type ResearchView = "overview" | "manifest" | "runs" | "code" | "inspect";

export type WorkTab =
  | { kind: "artifact"; artifactId: string }
  | { kind: "research"; contextId: string; view: ResearchView };

/** Stable identity for a tab — the key for activation, ordering, and React lists. */
export function tabKey(tab: WorkTab): string {
  return tab.kind === "artifact" ? tab.artifactId : `research:${tab.contextId}:${tab.view}`;
}

/** The research folder a tab belongs to, or null for a loose artifact tab (§12). */
export function tabContextId(tab: WorkTab): string | null {
  return tab.kind === "research" ? tab.contextId : null;
}

/**
 * The structural slots a research folder always has (§5): "manifest, code ref, run log.
 * Always present, empty or not." Plus the Overview, which §15 pins first as the front
 * page. They render as TYPED CHILDREN in the normal tree (§11) — expanding a research
 * folder gives you the notebook sidebar for free, with one tree implementation and no
 * parallel navigation.
 *
 * Inspect is deliberately NOT a slot: it's a view you go to with a question, not a
 * container that's always there. It stays an entry point on the Overview.
 */
export const RESEARCH_SLOT_VIEWS: ResearchView[] = ["overview", "manifest", "runs", "code"];

export const RESEARCH_VIEW_LABEL: Record<ResearchView, string> = {
  overview: "Overview",
  manifest: "Manifest",
  runs: "Runs",
  code: "Code",
  inspect: "Inspect",
};

/** The active tab as an artifact id, or null when the active tab isn't an artifact. */
export function activeArtifactIdOf(state: {
  openTabs: WorkTab[];
  activeTabKey: string | null;
}): string | null {
  const tab = state.openTabs.find((t) => tabKey(t) === state.activeTabKey);
  return tab?.kind === "artifact" ? tab.artifactId : null;
}

export interface WorkspaceShellState {
  /** The active tab's key (see tabKey). Named for the key, not the artifact id. */
  activeTabKey: string | null;
  openTabs: WorkTab[];
  inspectorOverrides: Record<string, boolean>;
  browserScrollTop: number;
  focusedFolderId: string | null;
  expandedFolderIds: string[];
  activeRename: RenameDraft | null;
  activeVoiceDraft: VoiceCaptureDraft | null;
}

export const initialWorkspaceShellState: WorkspaceShellState = {
  activeTabKey: null,
  openTabs: [],
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
  kind: BrowserNodeKind | "research-view";
  title: string;
  ancestorFolderIds: string[];
  folderId?: string;
  artifactId?: string;
  artifactKind?: ArtifactKind;
  updatedLabel?: string | null;
  childCount?: number;
  /** Set on research rows — drives the run-state dot without a second fetch. */
  research?: ResearchNodeMeta | null;
  /**
   * Set on a research SLOT row (kind === "research-view"): the view it opens and the
   * research folder it belongs to. Slot rows are derived, not database rows — they exist
   * because the kind guarantees the slot exists, empty or not.
   */
  researchView?: ResearchView;
  contextId?: string;
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
      if (isContainerKind(node.kind)) {
        const folderAncestorIds = [...ancestorFolderIds, node.id];
        rows.push({
          id: node.id,
          depth,
          kind: node.kind,
          title: node.title,
          ancestorFolderIds: folderAncestorIds,
          // Research rows carry folderId too: it is the handle expand/drill/create-here
          // all key off, and a research folder is a container in every one of those.
          folderId: node.id,
          childCount: node.children.length,
          research: node.kind === "research" ? node.research ?? null : undefined,
        });
        if (expandedSet.has(node.id)) {
          // §11: the slots come FIRST, above the folder's captured material, so the
          // notebook structure reads before its contents.
          if (node.kind === "research") {
            RESEARCH_SLOT_VIEWS.forEach((view) => {
              rows.push({
                id: `research:${node.id}:${view}`,
                depth: depth + 1,
                kind: "research-view",
                title: RESEARCH_VIEW_LABEL[view],
                ancestorFolderIds: folderAncestorIds,
                researchView: view,
                contextId: node.id,
              });
            });
          }
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
    if (!isContainerKind(node.kind)) continue;
    if (node.id === folderId) return node;
    const nested = findFolderNode(node.children, folderId);
    if (nested) return nested;
  }
  return null;
}

export function folderParent(tree: BrowserTreeNode[], folderId: string): BrowserTreeNode | null {
  function visit(nodes: BrowserTreeNode[], parent: BrowserTreeNode | null): BrowserTreeNode | null {
    for (const node of nodes) {
      if (!isContainerKind(node.kind)) continue;
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
      if (!isContainerKind(node.kind)) continue;
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
    if (isContainerKind(node.kind)) {
      if (node.id === folderId) return node.children;
      const nested = findFolderChildren(node.children, folderId);
      if (nested) return nested;
    }
  }
  return null;
}

export interface FolderOption {
  id: string | null;
  title: string;
  depth: number;
}

/** Flattens the tree into a depth-ordered list of folders for a picker. */
export function flattenFolderOptions(tree: BrowserTreeNode[]): FolderOption[] {
  const out: FolderOption[] = [];
  const visit = (nodes: BrowserTreeNode[], depth: number) => {
    for (const node of nodes) {
      // Research folders ARE valid destinations (§21: research artifacts live in them).
      if (!isContainerKind(node.kind)) continue;
      out.push({ id: node.id, title: node.title, depth });
      if (node.children?.length) visit(node.children, depth + 1);
    }
  };
  visit(tree, 0);
  return out;
}

export function nextFolderTitle(tree: BrowserTreeNode[], folderId: string | null): string {
  const siblings = getFolderScopedNodes(tree, folderId);
  const existingTitles = new Set(
    siblings.filter((node) => node.kind === "folder").map((node) => node.title.toLowerCase()),
  );
  return nextTitled("New Folder", existingTitles);
}

export function nextResearchTitle(tree: BrowserTreeNode[], folderId: string | null): string {
  const siblings = getFolderScopedNodes(tree, folderId);
  const existingTitles = new Set(
    siblings.filter((node) => node.kind === "research").map((node) => node.title.toLowerCase()),
  );
  return nextTitled("New Research", existingTitles);
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
