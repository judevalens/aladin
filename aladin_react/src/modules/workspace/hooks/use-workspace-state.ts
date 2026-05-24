import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { combineLatest, map, of, startWith, type Observable } from "rxjs";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useShellSession } from "@/modules/auth/hooks/use-auth-state";
import type { BrowserTreeRow, RenameDraft } from "@/modules/workspace/domain";
import {
  buildWorkspaceRows,
  createArtifactCommand,
  createFolderCommand,
  createVoiceDraft,
  folderAncestorIds,
  folderTitle,
  resolveOpenArtifacts,
  resolveWorkspaceDestination,
} from "@/services/workspace/workspace-helpers";
import type { WorkspaceService } from "@/services/workspace/workspace-service";
import type {
  Artifact,
  BrowserTreeNode,
  VoiceCaptureDraft,
} from "@/shared/api/models";
import { useObservableState } from "@/shared/flow/use-observable-state";

export type WorkspaceDestination = "home" | "folders" | "sources" | "signals" | "graph";

export interface WorkspaceShellState {
  selectedDestination: WorkspaceDestination;
  userEmail: string;
  logoutPending: boolean;
  createPending: boolean;
  onNavigate: (path: string) => void;
  onLogout: () => Promise<void>;
  onCreateFolder: () => Promise<void>;
  onCreateNote: () => Promise<void>;
  onCreateLink: () => Promise<void>;
  onCreateVoice: () => void;
}

export interface BrowserPaneState {
  loading: boolean;
  errorMessage: string | null;
  rows: BrowserTreeRow[];
  activeArtifactId: string | null;
  canGoBack: boolean;
  browserPath: WorkPaneCrumb[];
  browserRootTitle: string | null;
  expandedFolderIds: string[];
  browserScrollTop: number;
  onToggleFolder: (folderId: string) => void;
  onDrillIntoFolder: (folderId: string) => void;
  onPopBrowserFrame: () => void;
  onBrowserScroll: (scrollTop: number) => void;
  onOpenArtifact: (artifactId: string) => void;
  onStartRenameFolder: (folderId: string, title: string) => void;
  onStartRenameArtifact: (artifactId: string, title: string) => void;
  onCreateFolderHere: (folderId: string) => void;
  onCreateNoteHere: (folderId: string) => void;
}

export interface WorkPaneCrumb {
  id: string;
  title: string;
}

export interface WorkPaneState {
  openArtifacts: Artifact[];
  activeArtifact: Artifact | null;
  breadcrumbFolders: WorkPaneCrumb[];
  artifactTitle: string | null;
  inspectorOpen: boolean;
  onActivateArtifact: (artifactId: string) => void;
  onCloseArtifact: (artifactId: string) => void;
  onToggleInspector: () => void;
  onJumpToFolder: (folderId: string) => void;
}

export interface RenameDialogState {
  rename: RenameDraft | null;
  pending: boolean;
  onDraftTitleChange: (title: string) => void;
  onCancel: () => void;
  onSave: () => Promise<void>;
}

export interface VoiceDraftState {
  draft: VoiceCaptureDraft | null;
  permissionError: string | null;
  pending: boolean;
  onStartRecording: () => Promise<void>;
  onStopRecording: () => void;
  onClose: () => void;
  onPatchDraft: (patch: Partial<VoiceCaptureDraft>) => void;
  onSave: () => Promise<void>;
}

export function useWorkspaceShell(): WorkspaceShellState {
  const { services } = useAppComposition();
  const location = useLocation();
  const navigate = useNavigate();
  const shellSession = useShellSession(navigate);
  const browserRootFolderId = useAppStore((state) => state.workspace.browserRootFolderId);
  const openVoiceDraft = useAppStore((state) => state.openVoiceDraft);
  const [createFolderPending, setCreateFolderPending] = useState(false);
  const [createArtifactPending, setCreateArtifactPending] = useState(false);

  const treeLoadable = useObservableState(services.workspace.tree());

  const tree: BrowserTreeNode[] =
    treeLoadable.status === "data" ? treeLoadable.value : [];

  return {
    selectedDestination: resolveWorkspaceDestination(location.pathname),
    userEmail: shellSession.userEmail,
    logoutPending: shellSession.logoutPending,
    createPending: createFolderPending || createArtifactPending,
    onNavigate: (path: string) => navigate(path),
    onLogout: shellSession.onLogout,
    onCreateFolder: async () => {
      try {
        setCreateFolderPending(true);
        const folder = await services.workspace.createFolder(
          createFolderCommand(tree, browserRootFolderId),
        );
        useAppStore.getState().setFocusedFolder(folder.id);
      } finally {
        setCreateFolderPending(false);
      }
    },
    onCreateNote: async () => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.workspace.createArtifact(
          createArtifactCommand(tree, browserRootFolderId, "note"),
        );
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
    onCreateLink: async () => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.workspace.createArtifact(
          createArtifactCommand(tree, browserRootFolderId, "link"),
        );
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
    onCreateVoice: () => {
      openVoiceDraft(createVoiceDraft(tree, browserRootFolderId));
    },
  };
}

export function useBrowserPane(): BrowserPaneState {
  const { services } = useAppComposition();
  const workspace = useAppStore((state) => state.workspace);
  const openArtifact = useAppStore((state) => state.openArtifact);
  const toggleFolder = useAppStore((state) => state.toggleFolder);
  const expandFolders = useAppStore((state) => state.expandFolders);
  const drillIntoFolder = useAppStore((state) => state.drillIntoFolder);
  const popBrowserFrame = useAppStore((state) => state.popBrowserFrame);
  const setBrowserScrollTop = useAppStore((state) => state.setBrowserScrollTop);
  const startRename = useAppStore((state) => state.startRename);

  const treeLoadable = useObservableState(services.workspace.tree());

  const tree: BrowserTreeNode[] =
    treeLoadable.status === "data" ? treeLoadable.value : [];
  const rows = useMemo(
    () =>
      buildWorkspaceRows(tree, workspace.expandedFolderIds, workspace.browserRootFolderId),
    [tree, workspace.browserRootFolderId, workspace.expandedFolderIds],
  );
  const browserRootTitle = useMemo(
    () => folderTitle(tree, workspace.browserRootFolderId),
    [tree, workspace.browserRootFolderId],
  );
  const browserPath = useMemo<WorkPaneCrumb[]>(
    () =>
      [...workspace.browserFrameStack.map((frame) => frame.rootFolderId), workspace.browserRootFolderId]
        .filter((folderId): folderId is string => Boolean(folderId))
        .map((folderId) => {
          const title = folderTitle(tree, folderId) ?? "Folder";
          return { id: folderId, title };
        }),
    [tree, workspace.browserFrameStack, workspace.browserRootFolderId],
  );

  return {
    loading: treeLoadable.status !== "data",
    errorMessage:
      treeLoadable.status === "error" ? treeLoadable.error.message : null,
    rows,
    activeArtifactId: workspace.activeArtifactId,
    canGoBack: workspace.browserFrameStack.length > 0,
    browserPath,
    browserRootTitle,
    expandedFolderIds: workspace.expandedFolderIds,
    browserScrollTop: workspace.browserScrollTop,
    onToggleFolder: (folderId: string) => {
      toggleFolder(folderId);
    },
    onDrillIntoFolder: (folderId: string) => {
      drillIntoFolder(folderId);
    },
    onPopBrowserFrame: () => {
      popBrowserFrame();
    },
    onBrowserScroll: (scrollTop: number) => {
      setBrowserScrollTop(scrollTop);
    },
    onOpenArtifact: (artifactId: string) => {
      openArtifact(artifactId);
    },
    onStartRenameFolder: (folderId: string, title: string) =>
      startRename({
        kind: "folder",
        rowId: folderId,
        originalTitle: title,
        draftTitle: title,
      }),
    onStartRenameArtifact: (artifactId: string, title: string) =>
      startRename({
        kind: "artifact",
        rowId: artifactId,
        originalTitle: title,
        draftTitle: title,
      }),
    onCreateFolderHere: (folderId: string) => {
      expandFolders([folderId]);
      void services.workspace.createFolder(createFolderCommand(tree, folderId));
    },
    onCreateNoteHere: (folderId: string) => {
      expandFolders([folderId]);
      void services.workspace
        .createArtifact(createArtifactCommand(tree, folderId, "note"))
        .then((artifact) => {
          openArtifact(artifact.id);
        });
    },
  };
}

export function useWorkPane(): WorkPaneState {
  const { services } = useAppComposition();
  const openArtifactIds = useAppStore((state) => state.workspace.openArtifactIds);
  const activeArtifactId = useAppStore((state) => state.workspace.activeArtifactId);
  const inspectorOverrides = useAppStore((state) => state.workspace.inspectorOverrides);
  const activateArtifact = useAppStore((state) => state.activateArtifact);
  const closeArtifact = useAppStore((state) => state.closeArtifact);
  const toggleInspector = useAppStore((state) => state.toggleInspector);
  const expandFolders = useAppStore((state) => state.expandFolders);

  const artifactCacheStream = useMemo(
    () => buildArtifactCacheStream(openArtifactIds, services.workspace),
    [openArtifactIds, services.workspace],
  );
  const artifactCacheState = useObservableState(artifactCacheStream);
  const artifactCache =
    artifactCacheState.status === "data" ? artifactCacheState.value : {};

  const treeLoadable = useObservableState(services.workspace.tree());
  const tree: BrowserTreeNode[] =
    treeLoadable.status === "data" ? treeLoadable.value : [];

  const openArtifacts = useMemo(
    () =>
      openArtifactIds
        .map((artifactId) => artifactCache[artifactId])
        .filter((artifact): artifact is Artifact => Boolean(artifact)),
    [artifactCache, openArtifactIds],
  );
  const activeArtifact =
    (activeArtifactId ? artifactCache[activeArtifactId] : null) ??
    resolveOpenArtifacts(openArtifacts, activeArtifactId, null);

  const breadcrumbFolders = useMemo<WorkPaneCrumb[]>(() => {
    if (!activeArtifact) return [];
    const folderId = activeArtifact.folderId ?? null;
    const ancestorIds = folderAncestorIds(tree, folderId);
    return ancestorIds
      .map((id) => {
        const title = findFolderTitle(tree, id);
        return title === null ? null : { id, title };
      })
      .filter((crumb): crumb is WorkPaneCrumb => crumb !== null);
  }, [activeArtifact, tree]);

  const inspectorOpen = activeArtifact ? Boolean(inspectorOverrides[activeArtifact.id]) : false;

  return {
    openArtifacts,
    activeArtifact,
    breadcrumbFolders,
    artifactTitle: activeArtifact?.title ?? null,
    inspectorOpen,
    onActivateArtifact: activateArtifact,
    onCloseArtifact: closeArtifact,
    onToggleInspector: () => {
      if (activeArtifact) toggleInspector(activeArtifact.id);
    },
    onJumpToFolder: (folderId: string) => {
      const ancestorIds = folderAncestorIds(tree, folderId);
      expandFolders([...ancestorIds, folderId]);
    },
  };
}

function buildArtifactCacheStream(
  ids: string[],
  workspace: Pick<WorkspaceService, "artifactById">,
): Observable<Record<string, Artifact | undefined>> {
  if (ids.length === 0) return of({});
  const streams = ids.map((id) =>
    workspace.artifactById(id).pipe(
      map((r) => [id, r.ok ? r.value : undefined] as const),
      startWith([id, undefined] as const),
    ),
  );
  return combineLatest(streams).pipe(
    map(
      (entries) =>
        Object.fromEntries(entries) as Record<string, Artifact | undefined>,
    ),
  );
}

function findFolderTitle(tree: BrowserTreeNode[], folderId: string): string | null {
  for (const node of tree) {
    if (node.kind === "folder") {
      if (node.id === folderId) return node.title;
      const nested = findFolderTitle(node.children, folderId);
      if (nested !== null) return nested;
    }
  }
  return null;
}

export function useRenameDialog(): RenameDialogState {
  const { services } = useAppComposition();
  const rename = useAppStore((state) => state.workspace.activeRename);
  const setRenameTitle = useAppStore((state) => state.setRenameTitle);
  const cancelRename = useAppStore((state) => state.cancelRename);
  const [pending, setPending] = useState(false);

  return {
    rename,
    pending,
    onDraftTitleChange: setRenameTitle,
    onCancel: cancelRename,
    onSave: async () => {
      if (!rename) return;
      try {
        setPending(true);
        if (rename.kind === "folder") {
          await services.workspace.renameFolder(rename.rowId, rename.draftTitle.trim());
        } else {
          await services.workspace.renameArtifact(rename.rowId, rename.draftTitle.trim());
        }
        cancelRename();
      } finally {
        setPending(false);
      }
    },
  };
}

export function useVoiceDraft(): VoiceDraftState {
  const { services } = useAppComposition();
  const draft = useAppStore((state) => state.workspace.activeVoiceDraft);
  const patchVoiceDraft = useAppStore((state) => state.patchVoiceDraft);
  const closeVoiceDraft = useAppStore((state) => state.closeVoiceDraft);
  const openArtifact = useAppStore((state) => state.openArtifact);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const [permissionError, setPermissionError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    return () => {
      recorderRef.current?.stop();
      streamRef.current?.getTracks().forEach((track) => track.stop());
      if (draft?.audioUrl) {
        URL.revokeObjectURL(draft.audioUrl);
      }
    };
  }, [draft?.audioUrl]);

  async function startRecording() {
    if (!navigator.mediaDevices?.getUserMedia) {
      setPermissionError("Audio capture is not available in this browser.");
      return;
    }

    try {
      setPermissionError(null);
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      streamRef.current = stream;
      chunksRef.current = [];
      const recorder = new MediaRecorder(stream);
      recorderRef.current = recorder;
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };
      recorder.onstop = () => {
        const blob = new Blob(chunksRef.current, {
          type: recorder.mimeType || "audio/webm",
        });
        const audioUrl = URL.createObjectURL(blob);
        patchVoiceDraft({
          phase: "review",
          audioBlob: blob,
          audioUrl,
          errorMessage: null,
        });
        stream.getTracks().forEach((track) => track.stop());
        streamRef.current = null;
        recorderRef.current = null;
      };
      recorder.start();
      patchVoiceDraft({ phase: "recording", errorMessage: null });
    } catch (error) {
      setPermissionError(error instanceof Error ? error.message : "Failed to start recording.");
    }
  }

  return {
    draft,
    permissionError,
    pending,
    onStartRecording: startRecording,
    onStopRecording: () => recorderRef.current?.stop(),
    onClose: closeVoiceDraft,
    onPatchDraft: patchVoiceDraft,
    onSave: async () => {
      if (!draft) {
        return;
      }
      try {
        setPending(true);
        const artifact = await services.workspace.uploadVoiceArtifact(draft);
        closeVoiceDraft();
        openArtifact(artifact.id);
      } catch (error) {
        patchVoiceDraft({
          phase: "failed",
          errorMessage: error instanceof Error ? error.message : "Failed to save voice note.",
        });
      } finally {
        setPending(false);
      }
    },
  };
}
