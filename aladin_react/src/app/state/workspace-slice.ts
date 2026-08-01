import type { StateCreator } from "zustand";
import type { VoiceCaptureDraft } from "@/shared/api/models";
import {
  initialWorkspaceShellState,
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
  /** Symbol shown in the global ticker modal; null → closed. */
  openTickerSymbol: string | null;
  openTicker: (symbol: string) => void;
  closeTicker: () => void;
  openArtifact: (artifactId: string) => void;
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
      },
    };
  });
}

export const createWorkspaceSlice: StateCreator<WorkspaceSlice, [], [], WorkspaceSlice> = (set) => ({
  workspace: initialWorkspaceShellState,
  commandPaletteOpen: false,
  setCommandPaletteOpen: (open) => set({ commandPaletteOpen: open }),
  openTickerSymbol: null,
  openTicker: (symbol) => set({ openTickerSymbol: symbol }),
  closeTicker: () => set({ openTickerSymbol: null }),
  openArtifact: (artifactId) => openTab({ kind: "artifact", artifactId }, set),
  openResearchTab: (contextId, view) => openTab({ kind: "research", contextId, view }, set),
  activateTab: (key) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeTabKey: key,
      },
    })),
  closeTab: (key) =>
    set((state) => {
      const openTabs = state.workspace.openTabs.filter((tab) => tabKey(tab) !== key);
      return {
        workspace: {
          ...state.workspace,
          openTabs,
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
