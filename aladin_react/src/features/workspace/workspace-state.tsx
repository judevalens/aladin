import type { Dispatch, PropsWithChildren } from "react";
import {
  createContext,
  useContext,
  useMemo,
  useReducer,
} from "react";
import type { VoiceCaptureDraft } from "@/shared/api/models";

export interface RenameDraft {
  kind: "folder" | "artifact";
  rowId: string;
  originalTitle: string;
  draftTitle: string;
}

export interface WorkspaceState {
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

export const initialWorkspaceState: WorkspaceState = {
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

type WorkspaceAction =
  | { type: "open-artifact"; artifactId: string }
  | { type: "activate-artifact"; artifactId: string }
  | { type: "close-artifact"; artifactId: string }
  | { type: "toggle-inspector"; artifactId: string }
  | { type: "toggle-folder"; folderId: string }
  | { type: "set-focused-folder"; folderId: string | null }
  | {
      type: "navigate-scope";
      folderId: string | null;
      pushCurrent?: boolean;
      ancestorFolderIds?: string[];
    }
  | { type: "navigate-scope-back" }
  | { type: "start-rename"; draft: RenameDraft }
  | { type: "set-rename-title"; title: string }
  | { type: "cancel-rename" }
  | { type: "open-voice-draft"; draft: VoiceCaptureDraft }
  | { type: "patch-voice-draft"; patch: Partial<VoiceCaptureDraft> }
  | { type: "close-voice-draft" };

export function workspaceReducer(state: WorkspaceState, action: WorkspaceAction): WorkspaceState {
  switch (action.type) {
    case "open-artifact": {
      const openArtifactIds =
        state.openArtifactIds.includes(action.artifactId)
          ? state.openArtifactIds
          : [...state.openArtifactIds, action.artifactId];
      return {
        ...state,
        activeArtifactId: action.artifactId,
        focusedFolderId: null,
        openArtifactIds,
      };
    }
    case "activate-artifact":
      return {
        ...state,
        activeArtifactId: action.artifactId,
      };
    case "close-artifact": {
      const openArtifactIds = state.openArtifactIds.filter((id) => id !== action.artifactId);
      return {
        ...state,
        openArtifactIds,
        activeArtifactId:
          state.activeArtifactId === action.artifactId
            ? openArtifactIds.at(-1) ?? null
            : state.activeArtifactId,
      };
    }
    case "toggle-inspector":
      return {
        ...state,
        inspectorOverrides: {
          ...state.inspectorOverrides,
          [action.artifactId]: !state.inspectorOverrides[action.artifactId],
        },
      };
    case "toggle-folder":
      return {
        ...state,
        expandedFolderIds: state.expandedFolderIds.includes(action.folderId)
          ? state.expandedFolderIds.filter((id) => id !== action.folderId)
          : [...state.expandedFolderIds, action.folderId],
      };
    case "set-focused-folder":
      return {
        ...state,
        focusedFolderId: action.folderId,
      };
    case "navigate-scope": {
      const nextStack = action.pushCurrent
        ? [...state.scopeBackStack, state.scopeFolderId]
        : state.scopeBackStack;
      const nextExpanded = action.ancestorFolderIds
        ? Array.from(new Set([...state.expandedFolderIds, ...action.ancestorFolderIds]))
        : state.expandedFolderIds;
      return {
        ...state,
        scopeFolderId: action.folderId,
        scopeBackStack: nextStack,
        focusedFolderId: action.folderId,
        expandedFolderIds: nextExpanded,
      };
    }
    case "navigate-scope-back": {
      if (state.scopeBackStack.length === 0) {
        return state;
      }
      const scopeBackStack = [...state.scopeBackStack];
      const folderId = scopeBackStack.pop() ?? null;
      return {
        ...state,
        scopeBackStack,
        scopeFolderId: folderId,
        focusedFolderId: folderId,
      };
    }
    case "start-rename":
      return {
        ...state,
        activeRename: action.draft,
      };
    case "set-rename-title":
      return state.activeRename
        ? {
            ...state,
            activeRename: {
              ...state.activeRename,
              draftTitle: action.title,
            },
          }
        : state;
    case "cancel-rename":
      return {
        ...state,
        activeRename: null,
      };
    case "open-voice-draft":
      return {
        ...state,
        activeVoiceDraft: action.draft,
      };
    case "patch-voice-draft":
      return state.activeVoiceDraft
        ? {
            ...state,
            activeVoiceDraft: {
              ...state.activeVoiceDraft,
              ...action.patch,
            },
          }
        : state;
    case "close-voice-draft":
      return {
        ...state,
        activeVoiceDraft: null,
      };
    default:
      return state;
  }
}

interface WorkspaceContextValue {
  state: WorkspaceState;
  dispatch: Dispatch<WorkspaceAction>;
}

const WorkspaceContext = createContext<WorkspaceContextValue | null>(null);

export function WorkspaceShellProvider({ children }: PropsWithChildren) {
  const [state, dispatch] = useReducer(workspaceReducer, initialWorkspaceState);
  const value = useMemo(() => ({ state, dispatch }), [state]);
  return <WorkspaceContext.Provider value={value}>{children}</WorkspaceContext.Provider>;
}

export function useWorkspaceShell() {
  const context = useContext(WorkspaceContext);
  if (!context) {
    throw new Error("useWorkspaceShell must be used inside WorkspaceShellProvider");
  }
  return context;
}
