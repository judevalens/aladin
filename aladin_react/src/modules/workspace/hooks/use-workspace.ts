import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useShellSession } from "@/modules/auth/hooks/use-auth";
import type { BrowserTreeRow } from "@/modules/workspace/domain";
import { useObservableState } from "@/shared/flow/use-observable-state";

export function useWorkspaceShell() {
  const { services } = useAppComposition();
  const location = useLocation();
  const navigate = useNavigate();
  const shellSession = useShellSession(navigate);
  const focusedFolderId = useAppStore((state) => state.workspace.focusedFolderId);
  const scopeFolderId = useAppStore((state) => state.workspace.scopeFolderId);
  const openVoiceDraft = useAppStore((state) => state.openVoiceDraft);
  const setFocusedFolder = useAppStore((state) => state.setFocusedFolder);
  const [createFolderPending, setCreateFolderPending] = useState(false);
  const [createArtifactPending, setCreateArtifactPending] = useState(false);

  const treeLoadable = useObservableState(
    () => services.data.workspace.observeTree(),
    () => services.data.workspace.getTreeSnapshot(),
    [services.data.workspace],
  );

  useEffect(() => {
    void services.data.workspace.ensureTreeLoaded();
  }, [services.data.workspace]);

  const tree = treeLoadable.data;

  return {
    selectedDestination: services.workspace.resolveWorkspaceDestination(location.pathname),
    userEmail: shellSession.userEmail,
    logoutPending: shellSession.logoutPending,
    createPending: createFolderPending || createArtifactPending,
    onNavigate: (path: string) => navigate(path),
    onLogout: shellSession.onLogout,
    onCreateFolder: async () => {
      try {
        setCreateFolderPending(true);
        const folder = await services.data.workspace.createFolder(
          services.workspace.createFolderCommand(tree, focusedFolderId ?? scopeFolderId),
        );
        setFocusedFolder(folder.id);
      } finally {
        setCreateFolderPending(false);
      }
    },
    onCreateNote: async () => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.data.workspace.createArtifact(
          services.workspace.createArtifactCommand(tree, focusedFolderId ?? scopeFolderId, "note"),
        );
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
    onCreateLink: async () => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.data.workspace.createArtifact(
          services.workspace.createArtifactCommand(tree, focusedFolderId ?? scopeFolderId, "link"),
        );
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
    onCreateVoice: () => {
      openVoiceDraft(services.workspace.createVoiceDraft(tree, focusedFolderId ?? scopeFolderId));
    },
  };
}

export function useBrowserPane() {
  const { services } = useAppComposition();
  const workspace = useAppStore((state) => state.workspace);
  const openArtifact = useAppStore((state) => state.openArtifact);
  const setFocusedFolder = useAppStore((state) => state.setFocusedFolder);
  const toggleFolder = useAppStore((state) => state.toggleFolder);
  const navigateScope = useAppStore((state) => state.navigateScope);
  const navigateScopeBack = useAppStore((state) => state.navigateScopeBack);
  const startRename = useAppStore((state) => state.startRename);

  const treeLoadable = useObservableState(
    () => services.data.workspace.observeTree(),
    () => services.data.workspace.getTreeSnapshot(),
    [services.data.workspace],
  );

  useEffect(() => {
    void services.data.workspace.ensureTreeLoaded();
  }, [services.data.workspace]);

  const tree = treeLoadable.data;
  const rows = useMemo(
    () => services.workspace.buildWorkspaceRows(tree, workspace.scopeFolderId, workspace.expandedFolderIds),
    [services.workspace, tree, workspace.scopeFolderId, workspace.expandedFolderIds],
  );
  const scopeBreadcrumbs = useMemo(
    () => services.workspace.buildWorkspaceBreadcrumbs(tree, workspace.scopeFolderId),
    [services.workspace, tree, workspace.scopeFolderId],
  );

  return {
    loading: treeLoadable.status === "loading" || treeLoadable.status === "idle",
    errorMessage: treeLoadable.errorMessage,
    rows,
    breadcrumbs: scopeBreadcrumbs,
    activeArtifactId: workspace.activeArtifactId,
    expandedFolderIds: workspace.expandedFolderIds,
    canNavigateBack: workspace.scopeBackStack.length > 0,
    onNavigateBack: navigateScopeBack,
    onFolderPrimaryAction: (row: BrowserTreeRow) => {
      if (!row.folderId) return;
      if (row.scopeCandidate) {
        navigateScope(row.folderId, true, services.workspace.folderAncestorIds(tree, row.folderId));
        return;
      }
      toggleFolder(row.folderId);
    },
    onSelectFolder: (folderId: string) => setFocusedFolder(folderId),
    onOpenArtifact: (artifactId: string) => {
      void services.data.workspace.ensureArtifactLoaded(artifactId);
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
      void services.data.workspace.createFolder(services.workspace.createFolderCommand(tree, folderId));
    },
    onCreateNoteHere: (folderId: string) => {
      void services.data.workspace
        .createArtifact(services.workspace.createArtifactCommand(tree, folderId, "note"))
        .then((artifact) => {
          openArtifact(artifact.id);
        });
    },
  };
}

export function useWorkPane() {
  const { services } = useAppComposition();
  const openArtifactIds = useAppStore((state) => state.workspace.openArtifactIds);
  const activeArtifactId = useAppStore((state) => state.workspace.activeArtifactId);
  const activateArtifact = useAppStore((state) => state.activateArtifact);
  const closeArtifact = useAppStore((state) => state.closeArtifact);

  const artifactCache = useObservableState(
    () => services.data.workspace.observeArtifacts(),
    () => services.data.workspace.getArtifactsSnapshot(),
    [services.data.workspace],
  );

  useEffect(() => {
    void services.data.workspace.ensureArtifactsLoaded(openArtifactIds);
  }, [openArtifactIds, services.data.workspace]);

  const openArtifacts = useMemo(
    () => openArtifactIds.map((artifactId) => artifactCache[artifactId]).filter(Boolean),
    [artifactCache, openArtifactIds],
  );
  const activeArtifact =
    (activeArtifactId ? artifactCache[activeArtifactId] : null) ??
    services.workspace.resolveOpenArtifacts(openArtifacts, activeArtifactId, null);

  return {
    openArtifacts,
    activeArtifact,
    onActivateArtifact: activateArtifact,
    onCloseArtifact: closeArtifact,
  };
}

export function useRenameDialog() {
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
          await services.data.workspace.renameFolder(rename.rowId, rename.draftTitle.trim());
        } else {
          await services.data.workspace.renameArtifact(rename.rowId, rename.draftTitle.trim());
        }
        cancelRename();
      } finally {
        setPending(false);
      }
    },
  };
}

export function useVoiceDraft() {
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
        const artifact = await services.data.workspace.uploadVoiceArtifact(draft);
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
