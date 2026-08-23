import { useEffect, useMemo, useRef, useState } from "react";
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
import { addExcerpt } from "../domain/board-objects";
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
const BOARD_OPTIONS = { longPressDurationMs: 400 };
const NO_HOST: BoardHost = {};

type SaveState = "loading" | "saving" | "saved" | "error";

/**
 * The board surface — one component, two hosts (the iPad's embedded web view and the
 * desktop work pane). Deliberately self-contained: no router, no app store, no embed
 * config — everything host-specific arrives through props.
 *
 * Persistence is the whole-snapshot REST spine: load once, then debounce-PATCH the full
 * `TLEditorSnapshot` into `artifacts.content`. Board content stays off the outbox sync
 * spine on purpose (the page_ydoc precedent: heavy content does not ride frames).
 */
export function BoardPane({
  boardId,
  title,
  client,
  host = NO_HOST,
}: {
  boardId: string;
  title?: string;
  client: ApiClient;
  host?: BoardHost;
}) {
  const loadedRef = useRef(false);
  const saveTimerRef = useRef<number | null>(null);
  const cleanupRef = useRef<(() => void) | null>(null);
  const editorRef = useRef<Editor | null>(null);
  const [saveState, setSaveState] = useState<SaveState>("loading");
  const [message, setMessage] = useState("");
  const [folderId, setFolderId] = useState<string | null>(null);
  const content = useMemo(() => createBoardContentSource(client), [client]);

  // A debounce pending when the pane hides or unmounts would ride an iOS-throttled (or
  // never-firing) timer — flush it immediately instead. Changes made while visible always
  // have their timer armed by then, so this closes the whole at-hide window.
  const flushPendingSave = useMemo(
    () => () => {
      if (saveTimerRef.current === null || !editorRef.current) return;
      window.clearTimeout(saveTimerRef.current);
      saveTimerRef.current = null;
      void saveSnapshotRef.current?.(editorRef.current);
    },
    [],
  );
  const saveSnapshotRef = useRef<((editor: Editor) => Promise<void>) | null>(null);

  useEffect(() => {
    const onHide = () => {
      if (document.visibilityState === "hidden") flushPendingSave();
    };
    document.addEventListener("visibilitychange", onHide);
    return () => {
      document.removeEventListener("visibilitychange", onHide);
      flushPendingSave();
      cleanupRef.current?.();
      if (saveTimerRef.current) window.clearTimeout(saveTimerRef.current);
    };
  }, [flushPendingSave]);

  // Built once per mount (token values read from the live CSS vars at that moment) and
  // IDENTITY-STABLE: a fresh `themes` object per render makes Tldraw recreate the editor —
  // which disposes the store and silently drops every unsaved shape.
  const themes = useMemo(() => ({ default: buildBoardTheme() }), []);

  const saveSnapshot = useMemo(
    () => async (editor: Editor) => {
      setSaveState("saving");
      try {
        await client.fetch<UserArtifact>(`/api/artifacts/${encodeURIComponent(boardId)}`, {
          method: "PATCH",
          body: JSON.stringify({ content: JSON.stringify(editor.getSnapshot()) }),
        });
        setSaveState("saved");
        setMessage("");
      } catch (error) {
        setSaveState("error");
        setMessage(error instanceof Error ? error.message : "could not save the board");
      }
    },
    [boardId, client],
  );
  saveSnapshotRef.current = saveSnapshot;

  function scheduleSave(editor: Editor) {
    if (!loadedRef.current) return;
    if (saveTimerRef.current) window.clearTimeout(saveTimerRef.current);
    saveTimerRef.current = window.setTimeout(() => {
      saveTimerRef.current = null;
      void saveSnapshot(editor);
    }, 700);
  }

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
    setSaveState("loading");

    cleanupRef.current?.();
    let cancelled = false;
    const unlisten = editor.store.listen(() => scheduleSave(editor), {
      scope: "document",
      source: "user",
    });
    cleanupRef.current = () => {
      cancelled = true;
      unlisten();
    };

    void client
      .fetch<UserArtifact>(`/api/artifacts/${encodeURIComponent(boardId)}`)
      .then((record) => {
        if (cancelled) return;
        setFolderId(record.folderId ?? null);
        const snapshot = parseSnapshot(record.content);
        if (snapshot) {
          editor.loadSnapshot(snapshot);
          requestAnimationFrame(() => editor.zoomToFit());
        }
        loadedRef.current = true;
        setSaveState("saved");
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        loadedRef.current = true;
        setSaveState("error");
        setMessage(error instanceof Error ? error.message : "could not load the board");
      });

    return cleanupRef.current;
  }

  return (
    <div className="flex h-full w-full flex-col bg-bg">
      <div className="flex h-9 shrink-0 items-center gap-3 border-b border-line bg-panel px-3">
        <div className="min-w-0 flex-1 truncate font-display text-body font-medium text-ink">
          {title || "Board"}
        </div>
        <div className="font-mono text-meta uppercase text-ink-4">{saveStateLabel(saveState)}</div>
      </div>
      {message ? (
        <div className="border-b border-line bg-card px-3 py-2 font-mono text-small text-against">
          {message}
        </div>
      ) : null}
      <div className="min-h-0 flex-1">
        <BoardHostContext.Provider value={host}>
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

function saveStateLabel(state: SaveState) {
  switch (state) {
    case "loading":
      return "Loading";
    case "saving":
      return "Saving";
    case "saved":
      return "Saved";
    case "error":
      return "Offline";
  }
}
