import type { StateCreator } from "zustand";
import type { VoiceCaptureDraft } from "@/shared/api/models";
import { initialWorkspaceShellState, type RenameDraft, type WorkspaceShellState } from "@/modules/workspace/domain";

export interface WorkspaceSlice {
  workspace: WorkspaceShellState;
  openArtifact: (artifactId: string) => void;
  activateArtifact: (artifactId: string) => void;
  closeArtifact: (artifactId: string) => void;
  toggleInspector: (artifactId: string) => void;
  toggleFolder: (folderId: string) => void;
  expandFolders: (folderIds: string[]) => void;
  drillIntoFolder: (folderId: string) => void;
  popBrowserFrame: () => void;
  setBrowserScrollTop: (scrollTop: number) => void;
  setFocusedFolder: (folderId: string | null) => void;
  startRename: (draft: RenameDraft) => void;
  setRenameTitle: (title: string) => void;
  cancelRename: () => void;
  openVoiceDraft: (draft: VoiceCaptureDraft) => void;
  patchVoiceDraft: (patch: Partial<VoiceCaptureDraft>) => void;
  closeVoiceDraft: () => void;
}

export const createWorkspaceSlice: StateCreator<WorkspaceSlice, [], [], WorkspaceSlice> = (set) => ({
  workspace: initialWorkspaceShellState,
  openArtifact: (artifactId) =>
    set((state) => {
      const openArtifactIds = state.workspace.openArtifactIds.includes(artifactId)
        ? state.workspace.openArtifactIds
        : [...state.workspace.openArtifactIds, artifactId];
      return {
        workspace: {
          ...state.workspace,
          activeArtifactId: artifactId,
          focusedFolderId: null,
          openArtifactIds,
        },
      };
    }),
  activateArtifact: (artifactId) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        activeArtifactId: artifactId,
      },
    })),
  closeArtifact: (artifactId) =>
    set((state) => {
      const openArtifactIds = state.workspace.openArtifactIds.filter((id) => id !== artifactId);
      return {
        workspace: {
          ...state.workspace,
          openArtifactIds,
          activeArtifactId:
            state.workspace.activeArtifactId === artifactId
              ? openArtifactIds.at(-1) ?? null
              : state.workspace.activeArtifactId,
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
  drillIntoFolder: (folderId) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        browserFrameStack: [
          ...state.workspace.browserFrameStack,
          {
            rootFolderId: state.workspace.browserRootFolderId,
            expandedFolderIds: state.workspace.expandedFolderIds,
            scrollTop: state.workspace.browserScrollTop,
          },
        ],
        browserRootFolderId: folderId,
        browserScrollTop: 0,
        focusedFolderId: folderId,
        expandedFolderIds: [],
      },
    })),
  popBrowserFrame: () =>
    set((state) => {
      const previousFrame = state.workspace.browserFrameStack.at(-1);
      if (!previousFrame) return state;
      return {
        workspace: {
          ...state.workspace,
          browserFrameStack: state.workspace.browserFrameStack.slice(0, -1),
          browserRootFolderId: previousFrame.rootFolderId,
          browserScrollTop: previousFrame.scrollTop,
          focusedFolderId: previousFrame.rootFolderId,
          expandedFolderIds: previousFrame.expandedFolderIds,
        },
      };
    }),
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
