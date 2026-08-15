import { useEffect, useMemo, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { combineLatest, map, of, startWith, type Observable } from "rxjs";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useShellSession } from "@/modules/auth/hooks/use-auth-state";
import type {
  BrowserTreeRow,
  FolderOption,
  RenameDraft,
  ResearchView,
  WorkTab,
} from "@/modules/workspace/domain";
import {
  activeArtifactIdOf,
  flattenFolderOptions,
  isContainerKind,
  tabKey,
  RESEARCH_VIEW_LABEL,
} from "@/modules/workspace/domain";
import {
  buildWorkspaceRows,
  createArtifactCommand,
  createFolderCommand,
  createResearchCommand,
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

export type WorkspaceDestination =
  | "markets"
  | "folders"
  | "sources"
  | "insights"
  | "entities"
  | "graph";

export interface WorkspaceShellState {
  selectedDestination: WorkspaceDestination;
  userEmail: string;
  logoutPending: boolean;
  createPending: boolean;
  onNavigate: (path: string) => void;
  onLogout: () => Promise<void>;
  onCreateFolder: () => Promise<void>;
  onCreateResearch: () => Promise<void>;
  onCreateNote: () => Promise<void>;
  onCreateLink: () => void;
  onCreateVoice: () => void;
  onCreateFile: () => void;
  linkDialogOpen: boolean;
  onCloseLinkDialog: () => void;
  onSubmitLink: (url: string, title?: string) => Promise<void>;
  fileDialogOpen: boolean;
  onCloseFileDialog: () => void;
  onSubmitFile: (file: File, title?: string) => Promise<void>;
}

export interface BrowserPaneState {
  loading: boolean;
  errorMessage: string | null;
  tree: BrowserTreeNode[];
  rows: BrowserTreeRow[];
  activeArtifactId: string | null;
  expandedFolderIds: string[];
  browserScrollTop: number;
  onToggleFolder: (folderId: string) => void;
  onOpenResearchView: (contextId: string, view: ResearchView) => void;
  onBrowserScroll: (scrollTop: number) => void;
  onOpenArtifact: (artifactId: string) => void;
  onStartRenameFolder: (folderId: string, title: string) => void;
  onStartRenameResearch: (nodeId: string, title: string) => void;
  onStartRenameArtifact: (artifactId: string, title: string) => void;
  onCreateFolderHere: (folderId: string) => void;
  onCreateResearchHere: (folderId: string) => void;
  onCreateNoteHere: (folderId: string) => void;
  /** Deletes a folder or research folder. Server-authoritative; the tree updates off the frame. */
  onDeleteFolder: (folderId: string) => Promise<void>;
  onDeleteArtifact: (artifactId: string) => Promise<void>;
}

export interface WorkPaneCrumb {
  id: string;
  title: string;
}

/**
 * One rendered tab: its stable key, the label to show, and the union member it came
 * from. Artifact tabs resolve their title from the artifact cache; research tabs from
 * the tree plus the view name (§11).
 */
export interface WorkPaneTab {
  key: string;
  label: string;
  tab: WorkTab;
  /** Set on artifact tabs once the artifact has loaded. */
  artifact?: Artifact;
  /**
   * The pieces `label` is built from. The strip renders `label` and ignores these; the tab
   * switcher needs them apart, because it puts the research folder in a group header and the
   * view name in the row. Splitting `label` back up on " · " would break on a folder title
   * that contains one — so the derivation hands over both forms rather than one to re-parse.
   */
  groupTitle?: string;
  viewLabel?: string;
  /** The folder an artifact tab lives in — disambiguates two same-named notes. */
  parentFolderTitle?: string | null;
}

export interface WorkPaneState {
  tabs: WorkPaneTab[];
  openArtifacts: Artifact[];
  activeTab: WorkPaneTab | null;
  activeArtifact: Artifact | null;
  breadcrumbFolders: WorkPaneCrumb[];
  artifactTitle: string | null;
  inspectorOpen: boolean;
  onActivateTab: (key: string) => void;
  onCloseTab: (key: string) => void;
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
  /** Milliseconds elapsed in the current recording (paused time excluded). */
  elapsedMs: number;
  /** Live input level, 0..1, for the meter while recording. */
  level: number;
  /** True while recording is paused (mic held, clock frozen). */
  paused: boolean;
  /** Folders the note can be saved into (root first). */
  folders: FolderOption[];
  onStartRecording: () => Promise<void>;
  onPauseRecording: () => void;
  onResumeRecording: () => void;
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
  const openVoiceDraft = useAppStore((state) => state.openVoiceDraft);
  const [createFolderPending, setCreateFolderPending] = useState(false);
  const [createArtifactPending, setCreateArtifactPending] = useState(false);
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);
  const [fileDialogOpen, setFileDialogOpen] = useState(false);

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
          createFolderCommand(tree, null),
        );
        useAppStore.getState().setFocusedFolder(folder.id);
      } finally {
        setCreateFolderPending(false);
      }
    },
    // The research create is server-owned (two rows + the frame in one tx), so there is
    // nothing to apply locally — the tree updates when the frame lands.
    onCreateResearch: async () => {
      try {
        setCreateFolderPending(true);
        await services.workspace.createResearch(createResearchCommand(tree, null));
      } finally {
        setCreateFolderPending(false);
      }
    },
    onCreateNote: async () => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.workspace.createArtifact(
          createArtifactCommand(tree, null, "note"),
        );
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
    onCreateLink: () => setLinkDialogOpen(true),
    onCreateVoice: () => {
      const focusedFolderId = useAppStore.getState().workspace.focusedFolderId;
      openVoiceDraft(createVoiceDraft(tree, focusedFolderId));
    },
    onCreateFile: () => setFileDialogOpen(true),
    linkDialogOpen,
    onCloseLinkDialog: () => setLinkDialogOpen(false),
    onSubmitLink: async (url, title) => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.workspace.createArtifact(
          createArtifactCommand(tree, null, "link", { sourceUrl: url, title }),
        );
        setLinkDialogOpen(false);
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
    fileDialogOpen,
    onCloseFileDialog: () => setFileDialogOpen(false),
    onSubmitFile: async (file, title) => {
      try {
        setCreateArtifactPending(true);
        const artifact = await services.workspace.uploadFileArtifact({
          file,
          folderId: null,
          title,
        });
        setFileDialogOpen(false);
        useAppStore.getState().openArtifact(artifact.id);
      } finally {
        setCreateArtifactPending(false);
      }
    },
  };
}

export function useBrowserPane(): BrowserPaneState {
  const { services } = useAppComposition();
  const workspace = useAppStore((state) => state.workspace);
  const openArtifact = useAppStore((state) => state.openArtifact);
  const toggleFolder = useAppStore((state) => state.toggleFolder);
  const expandFolders = useAppStore((state) => state.expandFolders);
  const setBrowserScrollTop = useAppStore((state) => state.setBrowserScrollTop);
  const startRename = useAppStore((state) => state.startRename);

  const treeLoadable = useObservableState(services.workspace.tree());

  const tree: BrowserTreeNode[] =
    treeLoadable.status === "data" ? treeLoadable.value : [];
  const rows = useMemo(
    () => buildWorkspaceRows(tree, workspace.expandedFolderIds, null),
    [tree, workspace.expandedFolderIds],
  );

  return {
    loading: treeLoadable.status !== "data",
    errorMessage:
      treeLoadable.status === "error" ? treeLoadable.error.message : null,
    tree,
    rows,
    activeArtifactId: activeArtifactIdOf(workspace),
    expandedFolderIds: workspace.expandedFolderIds,
    browserScrollTop: workspace.browserScrollTop,
    onToggleFolder: (folderId: string) => {
      toggleFolder(folderId);
    },
    // §11: the structural slots are typed children in the tree, so opening one is just
    // "open that view's tab". Selecting the research folder itself only expands it —
    // making the click ALSO open a tab meant collapsing re-opened it.
    onOpenResearchView: (contextId: string, view: ResearchView) => {
      useAppStore.getState().openResearchTab(contextId, view);
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
    onStartRenameResearch: (nodeId: string, title: string) =>
      startRename({
        kind: "research",
        rowId: nodeId,
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
    onCreateResearchHere: (folderId: string) => {
      void services.workspace.createResearch(createResearchCommand(tree, folderId));
    },
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
    // Delete THEN close: a tab closed before a failed request would look like the delete
    // worked. The tree itself needs no help — the server tombstones the node and the sync
    // frame removes it from the local replica, which is what the tree reads.
    onDeleteFolder: async (folderId: string) => {
      await services.workspace.deleteFolder(folderId);
      closeTabsForContext(folderId);
    },
    onDeleteArtifact: async (artifactId: string) => {
      await services.workspace.deleteArtifact(artifactId);
      useAppStore.getState().closeTab(artifactId);
    },
  };
}

/**
 * Closes every research view tab belonging to a deleted folder. Their keys are
 * `research:<contextId>:<view>`, so membership is derived rather than tracked — the same rule
 * the tab strip's grouping uses.
 */
function closeTabsForContext(contextId: string) {
  const store = useAppStore.getState();
  store.workspace.openTabs
    .filter((tab) => tab.kind === "research" && tab.contextId === contextId)
    .forEach((tab) => store.closeTab(tabKey(tab)));
}

export function useWorkPane(): WorkPaneState {
  const { services } = useAppComposition();
  const openTabs = useAppStore((state) => state.workspace.openTabs);
  const activeTabKey = useAppStore((state) => state.workspace.activeTabKey);
  const inspectorOverrides = useAppStore((state) => state.workspace.inspectorOverrides);
  const activateTab = useAppStore((state) => state.activateTab);
  const closeTab = useAppStore((state) => state.closeTab);

  // Only artifact tabs need the artifact cache; research views read their own data.
  const openArtifactIds = useMemo(
    () => openTabs.flatMap((tab) => (tab.kind === "artifact" ? [tab.artifactId] : [])),
    [openTabs],
  );
  const activeArtifactId = useMemo(
    () => activeArtifactIdOf({ openTabs, activeTabKey }),
    [openTabs, activeTabKey],
  );
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

  // §12: tabs belonging to one research folder must be CONTIGUOUS. Membership is
  // derived from the tab's contextId — never managed — so grouping is an ordering
  // invariant rather than a mode with its own UI.
  const tabs = useMemo<WorkPaneTab[]>(() => {
    const rendered = openTabs.map((tab) => {
      if (tab.kind === "artifact") {
        const artifact = artifactCache[tab.artifactId];
        return {
          key: tabKey(tab),
          label: artifact?.title ?? "Untitled",
          tab,
          artifact,
          parentFolderTitle: artifact?.folderId
            ? findFolderTitle(tree, artifact.folderId)
            : null,
        };
      }
      const title = findFolderTitle(tree, tab.contextId) ?? "Research";
      const viewLabel = RESEARCH_VIEW_LABEL[tab.view];
      return {
        key: tabKey(tab),
        label: `${title} · ${viewLabel}`,
        tab,
        groupTitle: title,
        viewLabel,
      };
    });

    // Stable grouping: research tabs cluster by contextId in first-seen order; loose
    // artifact tabs sit at the far right, past every group (§12).
    const groupOrder: string[] = [];
    rendered.forEach(({ tab }) => {
      if (tab.kind === "research" && !groupOrder.includes(tab.contextId)) {
        groupOrder.push(tab.contextId);
      }
    });
    const rank = (t: WorkPaneTab) =>
      t.tab.kind === "research" ? groupOrder.indexOf(t.tab.contextId) : groupOrder.length;
    return rendered
      .map((t, index) => ({ t, index }))
      .sort((a, b) => rank(a.t) - rank(b.t) || a.index - b.index)
      .map(({ t }) => t);
  }, [openTabs, artifactCache, tree]);

  const activeTab = tabs.find((t) => t.key === activeTabKey) ?? null;

  return {
    tabs,
    openArtifacts,
    activeTab,
    activeArtifact,
    breadcrumbFolders,
    artifactTitle: activeArtifact?.title ?? null,
    inspectorOpen,
    onActivateTab: activateTab,
    onCloseTab: closeTab,
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
    if (isContainerKind(node.kind)) {
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
        } else if (rename.kind === "research") {
          await services.workspace.renameResearch(rename.rowId, rename.draftTitle.trim());
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

/**
 * Turns a getUserMedia/MediaRecorder failure into a message that tells the user
 * what to actually do. macOS surfaces a denied mic as NotAllowedError/SecurityError;
 * a machine with no input device as NotFoundError.
 */
function describeMicError(error: unknown): string {
  const name = error instanceof DOMException ? error.name : "";
  if (name === "NotAllowedError" || name === "SecurityError") {
    return "Microphone access is blocked. Enable it under System Settings → Privacy & Security → Microphone, then try again.";
  }
  if (name === "NotFoundError" || name === "OverconstrainedError") {
    return "No microphone was found. Connect an input device and try again.";
  }
  if (name === "NotReadableError") {
    return "The microphone is in use by another app. Close it and try again.";
  }
  return error instanceof Error ? error.message : "Couldn't start recording.";
}

/**
 * Picks a recording container the *playback* engine can also decode. WKWebView
 * (macOS desktop) can't decode WebM/Opus but supports MP4/AAC, so we prefer mp4;
 * Chromium only records webm, so it falls through to that. Returns undefined to
 * let the browser choose when it supports none of our candidates.
 */
function pickAudioMimeType(): string | undefined {
  if (typeof MediaRecorder === "undefined" || !MediaRecorder.isTypeSupported) {
    return undefined;
  }
  for (const candidate of ["audio/mp4", "audio/webm;codecs=opus", "audio/webm"]) {
    if (MediaRecorder.isTypeSupported(candidate)) {
      return candidate;
    }
  }
  return undefined;
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
  const timerRef = useRef<number | null>(null);
  // Elapsed time is accumulated across record segments so pauses don't count.
  const accumulatedMsRef = useRef<number>(0);
  const segmentStartRef = useRef<number>(0);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const analyserRef = useRef<AnalyserNode | null>(null);
  const meterDataRef = useRef<Uint8Array<ArrayBuffer> | null>(null);
  const rafRef = useRef<number | null>(null);
  const audioUrlRef = useRef<string | null>(null);
  const [permissionError, setPermissionError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const [elapsedMs, setElapsedMs] = useState(0);
  const [level, setLevel] = useState(0);
  const [paused, setPaused] = useState(false);

  const treeLoadable = useObservableState(services.workspace.tree());
  const folders = useMemo<FolderOption[]>(() => {
    const tree = treeLoadable.status === "data" ? treeLoadable.value : [];
    return [
      { id: null, title: "Workspace", depth: 0 },
      ...flattenFolderOptions(tree).map((f) => ({ ...f, depth: f.depth + 1 })),
    ];
  }, [treeLoadable]);

  // Keep the latest preview URL in a ref so unmount cleanup can revoke it without
  // re-running (and re-tearing-down the recorder) every time the URL changes.
  useEffect(() => {
    audioUrlRef.current = draft?.audioUrl ?? null;
  }, [draft?.audioUrl]);

  function startClock() {
    segmentStartRef.current = Date.now();
    if (timerRef.current === null) {
      timerRef.current = window.setInterval(() => {
        setElapsedMs(accumulatedMsRef.current + (Date.now() - segmentStartRef.current));
      }, 200);
    }
  }

  function stopClock() {
    if (timerRef.current !== null) {
      window.clearInterval(timerRef.current);
      timerRef.current = null;
    }
  }

  function runMeterLoop() {
    const analyser = analyserRef.current;
    const data = meterDataRef.current;
    if (!analyser || !data) return;
    const tick = () => {
      analyser.getByteTimeDomainData(data);
      let peak = 0;
      for (let i = 0; i < data.length; i += 1) {
        const v = Math.abs(data[i] - 128) / 128;
        if (v > peak) peak = v;
      }
      setLevel(peak);
      rafRef.current = requestAnimationFrame(tick);
    };
    tick();
  }

  function stopMeterLoop() {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    setLevel(0);
  }

  function openMeter(stream: MediaStream) {
    try {
      const ctx = new AudioContext();
      audioCtxRef.current = ctx;
      const source = ctx.createMediaStreamSource(stream);
      const analyser = ctx.createAnalyser();
      analyser.fftSize = 512;
      source.connect(analyser);
      analyserRef.current = analyser;
      meterDataRef.current = new Uint8Array(analyser.frequencyBinCount);
    } catch {
      // Metering is best-effort; recording still works without a level meter.
    }
  }

  function closeMeter() {
    stopMeterLoop();
    analyserRef.current = null;
    meterDataRef.current = null;
    audioCtxRef.current?.close().catch(() => {});
    audioCtxRef.current = null;
  }

  function teardown() {
    stopClock();
    closeMeter();
    setPaused(false);
    if (recorderRef.current && recorderRef.current.state !== "inactive") {
      recorderRef.current.stop();
    }
    streamRef.current?.getTracks().forEach((track) => track.stop());
    streamRef.current = null;
    recorderRef.current = null;
  }

  // Cleanup only on unmount — not on every draft.audioUrl change.
  useEffect(() => {
    return () => {
      teardown();
      if (audioUrlRef.current) {
        URL.revokeObjectURL(audioUrlRef.current);
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function startRecording() {
    if (!navigator.mediaDevices?.getUserMedia) {
      setPermissionError("Audio capture isn't available here. Record from the desktop app.");
      return;
    }

    // Revoke a prior take's preview URL before starting a fresh one.
    if (audioUrlRef.current) {
      URL.revokeObjectURL(audioUrlRef.current);
      audioUrlRef.current = null;
    }

    try {
      setPermissionError(null);
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      streamRef.current = stream;
      chunksRef.current = [];
      const mimeType = pickAudioMimeType();
      const recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
      recorderRef.current = recorder;
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          chunksRef.current.push(event.data);
        }
      };
      recorder.onstop = () => {
        stopClock();
        closeMeter();
        setPaused(false);
        const blob = new Blob(chunksRef.current, {
          type: recorder.mimeType || "audio/webm",
        });
        const audioUrl = URL.createObjectURL(blob);
        audioUrlRef.current = audioUrl;
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
      accumulatedMsRef.current = 0;
      setElapsedMs(0);
      setPaused(false);
      openMeter(stream);
      startClock();
      runMeterLoop();
      patchVoiceDraft({ phase: "recording", errorMessage: null });
    } catch (error) {
      teardown();
      setPermissionError(describeMicError(error));
    }
  }

  function pauseRecording() {
    const recorder = recorderRef.current;
    if (!recorder || recorder.state !== "recording") return;
    recorder.pause();
    accumulatedMsRef.current += Date.now() - segmentStartRef.current;
    stopClock();
    stopMeterLoop();
    setElapsedMs(accumulatedMsRef.current);
    setPaused(true);
  }

  function resumeRecording() {
    const recorder = recorderRef.current;
    if (!recorder || recorder.state !== "paused") return;
    recorder.resume();
    startClock();
    runMeterLoop();
    setPaused(false);
  }

  return {
    draft,
    permissionError,
    pending,
    elapsedMs,
    level,
    paused,
    folders,
    onStartRecording: startRecording,
    onPauseRecording: pauseRecording,
    onResumeRecording: resumeRecording,
    onStopRecording: () => recorderRef.current?.stop(),
    onClose: () => {
      teardown();
      closeVoiceDraft();
    },
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
