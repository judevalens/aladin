import type { StateCreator } from "zustand";
import type { VoiceCaptureDraft } from "@/shared/api/models";

/** One excerpt captured from a reader, en route to a board. */
export interface CapturedExcerpt {
  text: string;
  sourceArtifactId: string;
  sourceTitle: string;
  page: number;
}
import {
  initialWorkspaceShellState,
  promoteMru,
  tabKey,
  type RenameDraft,
  type ResearchView,
  type WorkspaceShellState,
  type WorkTab,
} from "@/modules/workspace/domain";

export interface WorkspaceSlice {
  workspace: WorkspaceShellState;
  commandPaletteOpen: boolean;
  setCommandPaletteOpen: (open: boolean) => void;
  /** The Ctrl+Tab MRU overlay. Shell chrome, like the command palette. */
  tabSwitcherOpen: boolean;
  setTabSwitcherOpen: (open: boolean) => void;
  /** Symbol shown in the global ticker modal; null → closed. */
  openTickerSymbol: string | null;
  openTicker: (symbol: string) => void;
  closeTicker: () => void;
  openArtifact: (artifactId: string) => void;
  /**
   * The wormhole: open (or re-activate) an artifact's tab AND ask its reader to land on a
   * page. The page must not enter the tab identity (a tab per page would break
   * re-activation), so it rides here — a per-artifact location the reader watches. Entries
   * are keyed by a nonce so citing the same page twice re-fires, and they are kept rather
   * than consumed: reopening the tab returns to the last cited page, which doubles as a
   * poor man's "continue".
   */
  openArtifactAt: (artifactId: string, page: number) => void;
  pendingDocLocations: Record<string, { page: number; nonce: number }>;
  /**
   * Capture inbox: excerpts pulled while reading, waiting for a board to catch them. The
   * ACTIVE board tab drains it (via BoardHost.captures) — capture-first, so selecting text
   * works before any board is open; the next board you front receives the backlog.
   * Bounded to the newest 10 so a forgotten session can't dump a wall of cards.
   */
  queuedExcerpts: CapturedExcerpt[];
  queueExcerpt: (excerpt: CapturedExcerpt) => void;
  /** Drains (returns and clears) the inbox. Imperative — called by the active board pane. */
  takeQueuedExcerpts: () => CapturedExcerpt[];
  /** Opens (or re-activates) a research view tab — §11's contextId arm of the union. */
  openResearchTab: (contextId: string, view: ResearchView) => void;
  activateTab: (key: string) => void;
  closeTab: (key: string) => void;
  toggleInspector: (artifactId: string) => void;
  toggleFolder: (folderId: string) => void;
  expandFolders: (folderIds: string[]) => void;
  setBrowserScrollTop: (scrollTop: number) => void;
  setFocusedFolder: (folderId: string | null) => void;
  startRename: (draft: RenameDraft) => void;
  setRenameTitle: (title: string) => void;
  cancelRename: () => void;
  openVoiceDraft: (draft: VoiceCaptureDraft) => void;
  patchVoiceDraft: (patch: Partial<VoiceCaptureDraft>) => void;
  closeVoiceDraft: () => void;
}

/**
 * Appends a tab if it isn't open yet and activates it. §12: order is STABLE — a tab
 * already open keeps its position rather than jumping to the end, because MRU re-sorting
 * moves every other target in the row.
 */
function openTab(tab: WorkTab, set: (fn: (state: any) => any) => void) {
  const key = tabKey(tab);
  set((state: any) => {
    const exists = state.workspace.openTabs.some((t: WorkTab) => tabKey(t) === key);
    return {
      workspace: {
        ...state.workspace,
        activeTabKey: key,
        focusedFolderId: null,
        openTabs: exists ? state.workspace.openTabs : [...state.workspace.openTabs, tab],
        tabMru: promoteMru(state.workspace.tabMru, key),
      },
    };
  });
}

export const createWorkspaceSlice: StateCreator<WorkspaceSlice, [], [], WorkspaceSlice> = (set) => ({
  workspace: initialWorkspaceShellState,
  commandPaletteOpen: false,
  setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),
  tabSwitcherOpen: false,
  setTabSwitcherOpen: (open) => set({ tabSwitcherOpen: open }),
  openTickerSymbol: null,
  openTicker: (symbol) => set({ openTickerSymbol: symbol }),
  closeTicker: () => set({ openTickerSymbol: null }),
  openArtifact: (artifactId) => openTab({ kind: "artifact", artifactId }, set),
  queuedExcerpts: [],
  queueExcerpt: (excerpt) =>
    set((state) => ({ queuedExcerpts: [...state.queuedExcerpts, excerpt].slice(-10) })),
  takeQueuedExcerpts: () => {
    let taken: CapturedExcerpt[] = [];
    set((state) => {
      taken = state.queuedExcerpts;
      return state.queuedExcerpts.length ? { queuedExcerpts: [] } : {};
    });
    return taken;
  },
  pendingDocLocations: {},
  openArtifactAt: (artifactId, page) => {
    openTab({ kind: "artifact", artifactId }, set);
    set((state) => ({
      pendingDocLocations: {
        ...state.pendingDocLocations,
        [artifactId]: { page, nonce: (state.pendingDocLocations[artifactId]?.nonce ?? 0) + 1 },
      },
    }));
  },
  openResearchTab: (contextId, view) => openTab({ kind: "research", contextId, view }, set),
  activateTab: (key) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeTabKey: key,
        tabMru: promoteMru(state.workspace.tabMru, key),
      },
    })),
  closeTab: (key) =>
    set((state) => {
      const openTabs = state.workspace.openTabs.filter((tab) => tabKey(tab) !== key);
      // The fallback stays "rightmost survivor" deliberately. Now that an MRU list exists,
      // "most recent survivor" would be better, but that is a behaviour change the switcher
      // PRD does not ask for — worth proposing separately, not smuggling in here.
      return {
        workspace: {
          ...state.workspace,
          openTabs,
          tabMru: state.workspace.tabMru.filter((k) => k !== key),
          activeTabKey:
            state.workspace.activeTabKey === key
              ? openTabs.length
                ? tabKey(openTabs[openTabs.length - 1])
                : null
              : state.workspace.activeTabKey,
        },
      };
    }),
  toggleInspector: (artifactId) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        inspectorOverrides: {
          ...state.workspace.inspectorOverrides,
          [artifactId]: !state.workspace.inspectorOverrides[artifactId],
        },
      },
    })),
  toggleFolder: (folderId) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        expandedFolderIds: state.workspace.expandedFolderIds.includes(folderId)
          ? state.workspace.expandedFolderIds.filter((id) => id !== folderId)
          : [...state.workspace.expandedFolderIds, folderId],
      },
    })),
  expandFolders: (folderIds) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        expandedFolderIds: Array.from(
          new Set([...state.workspace.expandedFolderIds, ...folderIds]),
        ),
      },
    })),
  setBrowserScrollTop: (scrollTop) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        browserScrollTop: scrollTop,
      },
    })),
  setFocusedFolder: (folderId) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        focusedFolderId: folderId,
      },
    })),
  startRename: (draft) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeRename: draft,
      },
    })),
  setRenameTitle: (title) =>
    set((state) => ({
      workspace: state.workspace.activeRename
        ? {
            ...state.workspace,
            activeRename: {
              ...state.workspace.activeRename,
              draftTitle: title,
            },
          }
        : state.workspace,
    })),
  cancelRename: () =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeRename: null,
      },
    })),
  openVoiceDraft: (draft) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeVoiceDraft: draft,
      },
    })),
  patchVoiceDraft: (patch) =>
    set((state) => ({
      workspace: state.workspace.activeVoiceDraft
        ? {
            ...state.workspace,
            activeVoiceDraft: {
              ...state.workspace.activeVoiceDraft,
              ...patch,
            },
          }
        : state.workspace,
    })),
  closeVoiceDraft: () =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeVoiceDraft: null,
      },
    })),
});
