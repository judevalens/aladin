import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Tldraw,
  type Editor,
  type TLComponents,
  type TLEditorSnapshot,
  type TLStateNodeConstructor,
} from "tldraw";
import "tldraw/tldraw.css";
import "../board.css";

import type { ApiClient } from "@/shared/api/client";
import type { UserArtifact } from "@/shared/api/models";
import { buildBoardTheme } from "../domain/board-theme";
import { boardCameraOptions } from "../domain/board-camera";
import {
  BoardContentContext,
  BoardFolderContext,
  createBoardContentSource,
} from "../domain/board-content";
import { BoardHostContext, type BoardHost } from "../domain/board-host";
import { BoardStatusContext, type BoardLoadState, type BoardStatus } from "../domain/board-status";
import { BoardToastContext, createToastStore } from "../domain/board-toasts";
import { addExcerpt } from "../domain/board-objects";
import {
  createBoardSaver,
  type BoardSaveState,
  type BoardSaver,
} from "../domain/board-persistence";
import { BoardLassoTool } from "../tools/lasso-tool";
import { CardShapeUtil } from "../shapes/card-shape";
import { DocWindowShapeUtil } from "../shapes/doc-window-shape";
import { ExcerptShapeUtil } from "../shapes/excerpt-shape";
import { TaskShapeUtil } from "../shapes/task-shape";
import { BoardChrome } from "./board-chrome";

// Everything handed to <Tldraw> must be identity-stable: a fresh object or array makes it
// recreate the editor, which disposes the store and drops unsaved shapes.
const BOARD_COMPONENTS: TLComponents = { InFrontOfTheCanvas: BoardChrome };
const BOARD_TOOLS: TLStateNodeConstructor[] = [BoardLassoTool];
const BOARD_SHAPES = [DocWindowShapeUtil, ExcerptShapeUtil, TaskShapeUtil, CardShapeUtil];
// Double-tap on the plane must never spawn a text shape (rule 4: pure plane) — and on an
// iPad a double-tap is a gesture, not a request for a text box.
const BOARD_OPTIONS = { longPressDurationMs: 400, createTextOnCanvasDoubleClick: false };
const NO_HOST: BoardHost = {};

/**
 * How much of its own frame the pane draws. `full` = header bar with title and status (the
 * desktop work pane, the spike). `plane` = just the plane and its floating chrome — the
 * iPad shell already shows the title in its tab strip, and 36px of duplicate header on a
 * surface whose whole point is the plane is 36px lost.
 */
export type BoardChromeMode = "full" | "plane";

/**
 * The board surface — one component, two hosts (the iPad's embedded web view and the
 * desktop work pane). Deliberately self-contained: no router, no app store, no embed
 * config — everything host-specific arrives through props.
 *
 * Persistence is the whole-snapshot REST spine: load once, then debounce-PATCH the full
 * `TLEditorSnapshot` into `artifacts.content`. Board content stays off the outbox sync
 * spine on purpose (the page_ydoc precedent: heavy content does not ride frames). The
 * debounce / retry / flush rules live in `createBoardSaver` (tested with fake timers).
 */
export function BoardPane({
  boardId,
  title,
  client,
  host = NO_HOST,
  chrome = "full",
}: {
  boardId: string;
  title?: string;
  client: ApiClient;
  host?: BoardHost;
  chrome?: BoardChromeMode;
}) {
  const editorRef = useRef<Editor | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);
  const loadedRef = useRef(false);
  const [loadState, setLoadState] = useState<BoardLoadState>("loading");
  const [saveState, setSaveState] = useState<BoardSaveState>("saved");
  const [message, setMessage] = useState("");
  const [folderId, setFolderId] = useState<string | null>(null);
  const content = useMemo(() => createBoardContentSource(client), [client]);
  const toasts = useMemo(createToastStore, []);

  // Built once per mount (token values read from the live CSS vars at that moment) and
  // IDENTITY-STABLE: a fresh `themes` object per render makes Tldraw recreate the editor —
  // which disposes the store and silently drops every unsaved shape.
  const themes = useMemo(() => ({ default: buildBoardTheme() }), []);

  // One saver per board, owned by an effect so StrictMode's mount→unmount→mount creates a
  // fresh one rather than disposing the only one. It PATCHes whatever the editor holds at
  // save time; `load` arms it, the store listener dirties it, hide/unmount flush it.
  const saverRef = useRef<BoardSaver | null>(null);
  useEffect(() => {
    const saver = createBoardSaver({
      save: async () => {
        const editor = editorRef.current;
        if (!editor) return;
        await client.fetch<UserArtifact>(`/api/artifacts/${encodeURIComponent(boardId)}`, {
          method: "PATCH",
          body: JSON.stringify({ content: JSON.stringify(editor.getSnapshot()) }),
        });
      },
      onState: (state, error) => {
        setSaveState(state);
        setMessage(
          state === "error"
            ? error instanceof Error
              ? error.message
              : "could not save the board"
            : "",
        );
      },
    });
    // A remount after a successful load (StrictMode) must not leave the new saver unarmed.
    if (loadedRef.current) saver.arm();
    saverRef.current = saver;

    // A debounce pending when the pane hides or unmounts would ride an iOS-throttled (or
    // never-firing) timer — flush it immediately instead. `pagehide` covers the iPad's
    // web-content process going away; `visibilitychange` covers tab/pane switches.
    const flush = () => saver.flush();
    const onHide = () => {
      if (document.visibilityState === "hidden") flush();
    };
    document.addEventListener("visibilitychange", onHide);
    window.addEventListener("pagehide", flush);
    return () => {
      document.removeEventListener("visibilitychange", onHide);
      window.removeEventListener("pagehide", flush);
      flush();
      saver.dispose();
      if (saverRef.current === saver) saverRef.current = null;
    };
  }, [boardId, client]);

  // Fetch the board and hand it to the editor. Until this succeeds the pane is read-only in
  // effect: the saver stays unarmed, so no edit can PATCH (the data-loss guard). The store
  // listener and the in-flight load are released by tldraw through the cleanup `handleMount`
  // returns — not by an effect, where StrictMode's double-invoke would tear them down under a
  // live editor.
  const load = useCallback(
    (editor: Editor) => {
      let cancelled = false;
      setLoadState("loading");
      setMessage("");
      void client
        .fetch<UserArtifact>(`/api/artifacts/${encodeURIComponent(boardId)}`)
        .then((record) => {
          if (cancelled) return;
          setFolderId(record.folderId ?? null);
          const snapshot = parseSnapshot(record.content);
          if (snapshot) {
            editor.loadSnapshot(snapshot);
            // The session carries the camera the board was last left at — trust it. Only
            // a snapshot without one (older saves, imports) gets framed from scratch.
            if (!hasSessionCamera(snapshot)) requestAnimationFrame(() => editor.zoomToFit());
          }
          loadedRef.current = true;
          saverRef.current?.arm();
          setLoadState("ready");
        })
        .catch((error: unknown) => {
          if (cancelled) return;
          setLoadState("failed");
          setMessage(error instanceof Error ? error.message : "could not load the board");
        });
      return () => {
        cancelled = true;
      };
    },
    [boardId, client],
  );

  const retryLoad = () => {
    const editor = editorRef.current;
    if (editor) load(editor);
  };

  function handleMount(editor: Editor) {
    editorRef.current = editor;
    // Dev drive-by handle: the spike route verifies board behavior through this (there is
    // no tldraw UI to click once the chrome is ours). Never set in production builds.
    if (import.meta.env.DEV) {
      (window as unknown as { __boardEditor?: Editor }).__boardEditor = editor;
    }
    editor.setCameraOptions(boardCameraOptions);
    // ⌘V / system paste of text lands as an excerpt at the paste point (intake door 3).
    editor.registerExternalContentHandler("text", (info) => {
      const text = info.text.trim();
      if (text) addExcerpt(editor, { text, at: info.point });
    });

    cleanupRef.current?.();
    const unlisten = editor.store.listen(() => saverRef.current?.markDirty(), {
      scope: "document",
      source: "user",
    });
    const cancelLoad = load(editor);
    cleanupRef.current = () => {
      cancelLoad();
      unlisten();
    };
    return cleanupRef.current;
  }

  const failed = loadState === "failed";
  const status = useMemo<BoardStatus>(
    () => ({ load: loadState, save: saveState, message, retryLoad }),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- retryLoad reads refs only
    [loadState, saveState, message],
  );

  return (
    <div className="flex h-full w-full flex-col bg-bg">
      {chrome === "full" ? (
        <div className="flex h-9 shrink-0 items-center gap-3 border-b border-line bg-panel px-3">
          <div className="min-w-0 flex-1 truncate font-display text-body font-medium text-ink">
            {title || "Board"}
          </div>
          <div className="font-mono text-meta uppercase text-ink-4">
            {statusLabel(loadState, saveState)}
          </div>
        </div>
      ) : null}
      {chrome === "full" && (failed || message) ? (
        <div className="flex items-center gap-3 border-b border-line bg-card px-3 py-2 font-mono text-small text-against">
          <span className="min-w-0 flex-1 truncate">
            {failed ? `couldn't load this board — ${message || "no response"}` : message}
          </span>
          {failed ? (
            <button
              type="button"
              onClick={retryLoad}
              className="h-11 shrink-0 rounded-control border border-line px-3 font-display text-small text-ink-2 hover:bg-hover hover:text-ink"
            >
              Retry
            </button>
          ) : null}
        </div>
      ) : null}
      <div className="min-h-0 flex-1">
        <BoardHostContext.Provider value={host}>
          <BoardStatusContext.Provider value={status}>
            <BoardToastContext.Provider value={toasts}>
            <BoardContentContext.Provider value={content}>
              <BoardFolderContext.Provider value={folderId}>
                <Tldraw
                  hideUi
                  themes={themes}
                  colorScheme="dark"
                  components={BOARD_COMPONENTS}
                  tools={BOARD_TOOLS}
                  shapeUtils={BOARD_SHAPES}
                  options={BOARD_OPTIONS}
                  onMount={handleMount}
                />
              </BoardFolderContext.Provider>
            </BoardContentContext.Provider>
            </BoardToastContext.Provider>
          </BoardStatusContext.Provider>
        </BoardHostContext.Provider>
      </div>
    </div>
  );
}

function parseSnapshot(content: string): TLEditorSnapshot | null {
  const trimmed = content.trim();
  if (!trimmed) return null;
  try {
    const snapshot = JSON.parse(trimmed) as unknown;
    return isEditorSnapshot(snapshot) ? snapshot : null;
  } catch {
    return null;
  }
}

function isEditorSnapshot(value: unknown): value is TLEditorSnapshot {
  return typeof value === "object" && value !== null && "document" in value && "session" in value;
}

/** Whether the saved session restores a camera (tldraw keeps one per page state). */
export function hasSessionCamera(snapshot: TLEditorSnapshot): boolean {
  const states = (snapshot.session as { pageStates?: { camera?: unknown }[] } | undefined)
    ?.pageStates;
  return Array.isArray(states) && states.some((state) => state?.camera != null);
}

function statusLabel(load: BoardLoadState, save: BoardSaveState) {
  if (load === "loading") return "Loading";
  if (load === "failed") return "Not loaded";
  switch (save) {
    case "dirty":
    case "saving":
      return "Saving";
    case "saved":
      return "Saved";
    case "error":
      return "Retrying";
  }
}
