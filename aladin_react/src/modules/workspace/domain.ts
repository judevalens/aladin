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

/** Moves `key` to the head of the MRU list. Idempotent, and safe for a key not yet in it. */
export function promoteMru(mru: string[], key: string): string[] {
  return [key, ...mru.filter((k) => k !== key)];
}

/**
 * MRU order, restricted to tabs that are actually open.
 *
 * The filter is the important half: a key can only leave `openTabs` through closeTab today,
 * but deriving the intersection rather than trusting stored state means a future path that
 * drops a tab some other way (a sync delete, say) can't leave a row in here pointing at
 * nothing. Anything open but missing from the MRU list is appended, so the switcher can
 * never hide a tab.
 */
export function orderByMru<T extends { key: string }>(items: T[], mru: string[]): T[] {
  const byKey = new Map(items.map((item) => [item.key, item]));
  const ordered = mru.flatMap((key) => {
    const item = byKey.get(key);
    if (!item) return [];
    byKey.delete(key);
    return [item];
  });
  return [...ordered, ...byKey.values()];
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
  /**
   * Tab keys in most-recently-used order, head first. The strip is ordered STRUCTURALLY
   * (§12 grouping) which is right for reading structure and useless for "go back to the
   * thing I was just in" — recency needs somewhere to live, and this is it. Index 0 is the
   * active tab, so the switcher opening on index 1 is what makes a repeated Ctrl+Tab a
   * toggle between the last two.
   */
  tabMru: string[];
  inspectorOverrides: Record<string, boolean>;
  browserScrollTop: number;
  focusedFolderId: string | null;
  expandedFolderIds: string[];
  activeRename: RenameDraft | null;
  activeVoiceDraft: VoiceCaptureDraft | null;
  /**
   * Multi-select overlay on the browser tree (row ids, not artifact ids — a row's id is
   * the browser node id, stable across the tree). Empty = no selection, the tree behaves
   * exactly as before. A plain click always clears it; only ⌘/⇧-clicks grow it.
   */
  selectedRowKeys: string[];
  /** The row the next ⇧-click ranges from. Set by the last ⌘-click or retarget. */
  selectionAnchorKey: string | null;
}

export const initialWorkspaceShellState: WorkspaceShellState = {
  activeTabKey: null,
  openTabs: [],
  tabMru: [],
  inspectorOverrides: {},
  browserScrollTop: 0,
  focusedFolderId: null,
  expandedFolderIds: [],
  activeRename: null,
  activeVoiceDraft: null,
  selectedRowKeys: [],
  selectionAnchorKey: null,
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
          : kind === "board"
            ? "New Board"
            : "New Note";
  return nextTitled(baseTitle, existingTitles);
}

function getFolderScopedNodes(tree: BrowserTreeNode[], folderId: string | null): BrowserTreeNode[] {
  if (folderId === null) return tree;
  return findFolderChildren(tree, folderId) ?? [];
}

/**
 * Rows that can enter a multi-selection: real tree nodes only. Research SLOT rows are
 * derived (they exist because the kind guarantees them), so there is nothing to delete —
 * modifier-clicks on them fall through to plain behaviour.
 */
export function isSelectableRow(row: BrowserTreeRow): boolean {
  return row.kind !== "research-view";
}

/**
 * The ⇧-click range: every selectable row between the anchor and the target, inclusive,
 * in RENDERED order — ranges must match what the eye sees, so this walks the row list the
 * pane actually painted, not the tree. A missing anchor (collapsed away, deleted) degrades
 * to just the target.
 */
export function rowRangeKeys(
  rows: BrowserTreeRow[],
  anchorKey: string | null,
  targetKey: string,
): string[] {
  const anchorIndex = rows.findIndex((row) => row.id === anchorKey);
  const targetIndex = rows.findIndex((row) => row.id === targetKey);
  if (targetIndex === -1) return [];
  if (anchorIndex === -1) return [targetKey];
  const [from, to] = anchorIndex < targetIndex ? [anchorIndex, targetIndex] : [targetIndex, anchorIndex];
  return rows.slice(from, to + 1).filter(isSelectableRow).map((row) => row.id);
}

/** What a bulk delete acts on. Structurally identical to the confirm dialog's DeleteTarget. */
export interface SelectionDeleteTarget {
  kind: "folder" | "research" | "artifact";
  id: string;
  title: string;
  childCount?: number;
}

/**
 * Turns a selection into delete targets, COALESCED: a row whose ancestor folder is also
 * selected is folded into that folder — the server's delete is recursive, so deleting the
 * child separately would double-delete, and the count the user confirms must match what
 * actually happens. Keys not in the current rows (collapsed or already gone) are dropped.
 */
export function selectionDeleteTargets(
  rows: BrowserTreeRow[],
  selectedKeys: string[],
): SelectionDeleteTarget[] {
  const selected = new Set(selectedKeys);
  const selectedRows = rows.filter((row) => selected.has(row.id) && isSelectableRow(row));
  const selectedFolderIds = new Set(
    selectedRows.filter((row) => isContainerKind(row.kind)).map((row) => row.id),
  );
  return selectedRows.flatMap((row): SelectionDeleteTarget[] => {
    // A folder row's ancestorFolderIds includes itself — exclude it when asking whether
    // a selected ancestor already covers this row.
    const ancestors = isContainerKind(row.kind)
      ? row.ancestorFolderIds.slice(0, -1)
      : row.ancestorFolderIds;
    if (ancestors.some((id) => selectedFolderIds.has(id))) return [];
    if (isContainerKind(row.kind) && row.folderId) {
      return [{
        kind: row.kind === "research" ? ("research" as const) : ("folder" as const),
        id: row.folderId,
        title: row.title,
        childCount: row.childCount,
      }];
    }
    if (row.kind === "artifact" && row.artifactId) {
      return [{ kind: "artifact" as const, id: row.artifactId, title: row.title }];
    }
    return [];
  });
}

function nextTitled(baseTitle: string, existingTitles: Set<string>) {
  if (!existingTitles.has(baseTitle.toLowerCase())) return baseTitle;
  let counter = 1;
  while (existingTitles.has(`${baseTitle} ${counter}`.toLowerCase())) {
    counter += 1;
  }
  return `${baseTitle} ${counter}`;
}
