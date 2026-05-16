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
  setFocusedFolder: (folderId: string | null) => void;
  navigateScope: (folderId: string | null, pushCurrent?: boolean, ancestorFolderIds?: string[]) => void;
  navigateScopeBack: () => void;
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
  setFocusedFolder: (folderId) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        focusedFolderId: folderId,
      },
    })),
  navigateScope: (folderId, pushCurrent = false, ancestorFolderIds = []) =>
    set((state) => ({
      workspace: {
        ...state.workspace,
        scopeFolderId: folderId,
        scopeBackStack: pushCurrent
          ? [...state.workspace.scopeBackStack, state.workspace.scopeFolderId]
          : state.workspace.scopeBackStack,
        focusedFolderId: folderId,
        expandedFolderIds:
          ancestorFolderIds.length > 0
            ? Array.from(new Set([...state.workspace.expandedFolderIds, ...ancestorFolderIds]))
            : state.workspace.expandedFolderIds,
      },
    })),
  navigateScopeBack: () =>
    set((state) => {
      if (state.workspace.scopeBackStack.length === 0) {
        return state;
      }
      const scopeBackStack = [...state.workspace.scopeBackStack];
      const folderId = scopeBackStack.pop() ?? null;
      return {
        workspace: {
          ...state.workspace,
          scopeBackStack,
          scopeFolderId: folderId,
          focusedFolderId: folderId,
        },
      };
    }),
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
