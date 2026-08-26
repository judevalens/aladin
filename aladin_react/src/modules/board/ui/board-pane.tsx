import { useCallback, useEffect, useMemo, useState } from "react";
import { useEffect as useLayoutOrderedEffect } from "react";
import {
  Tldraw,
  defaultShapeUtils,
  getUserPreferences,
  inlineBase64AssetStore,
  registerColorsFromThemes,
  resolveThemes,
  setUserPreferences,
  type Editor,
  type TLComponents,
  type TLStateNodeConstructor,
} from "tldraw";
import { useSync } from "@tldraw/sync";
import "tldraw/tldraw.css";
import "../board.css";

import type { ApiClient } from "@/shared/api/client";
import { buildBoardTheme } from "../domain/board-theme";
import { boardCameraOptions } from "../domain/board-camera";
import {
  BoardContentContext,
  BoardFolderContext,
  createBoardContentSource,
} from "../domain/board-content";
import { BoardHostContext, useBoardHost, type BoardHost } from "../domain/board-host";
import { BoardStatusContext, type BoardStatus } from "../domain/board-status";
import { BoardToastContext, createToastStore } from "../domain/board-toasts";
import { addExcerpt } from "../domain/board-objects";
import { useBoardToasts } from "../domain/board-toasts";
import { BoardLassoTool } from "../tools/lasso-tool";
import { CardShapeUtil } from "../shapes/card-shape";
import { DocWindowShapeUtil } from "../shapes/doc-window-shape";
import { ExcerptShapeUtil } from "../shapes/excerpt-shape";
import { TaskShapeUtil } from "../shapes/task-shape";
import { BoardChrome } from "./board-chrome";

// Everything handed to <Tldraw> AND useSync must be identity-stable: a fresh object or
// array makes Tldraw recreate the editor (dropping unsaved work), and makes useSync tear
// down and redial the socket (the schema sits in its connection effect's deps).
const BOARD_COMPONENTS: TLComponents = { InFrontOfTheCanvas: BoardChrome };
const BOARD_TOOLS: TLStateNodeConstructor[] = [BoardLassoTool];
const BOARD_SHAPES = [DocWindowShapeUtil, ExcerptShapeUtil, TaskShapeUtil, CardShapeUtil];
// The sync store's schema is all-or-nothing: supply shapeUtils and you get ONLY what you
// supply — so the defaults ride along explicitly. (<Tldraw> adds defaults by itself.)
export const SYNC_SHAPE_UTILS = [...defaultShapeUtils, ...BOARD_SHAPES];
// Double-tap on the plane must never spawn a text shape (rule 4: pure plane).
const BOARD_OPTIONS = { longPressDurationMs: 400, createTextOnCanvasDoubleClick: false };
const NO_HOST: BoardHost = {};

/** How the pane reaches the board sync room server. Hosts build this identity-stable. */
export interface BoardSyncConfig {
  /** The room server's base, e.g. `ws://localhost:3502` — the pane appends `/board/{id}`. */
  url: string;
  /** Fresh bearer per (re)connect — useSync re-invokes the uri function on every dial. */
  getToken: () => string | Promise<string>;
}

/** Presence identity — the name and colour other devices see on your cursor. */
export interface BoardUser {
  name: string;
  color?: string;
}

/**
 * How much of its own frame the pane draws. `full` = header bar with the title (desktop,
 * spike). `plane` = just the plane and its floating chrome (the iPad shell's tab strip
 * already names the board).
 */
export type BoardChromeMode = "full" | "plane";

/**
 * The board surface — one component, two hosts (the iPad's embedded web view and the
 * desktop work pane), two store modes:
 *
 * - **Synced** (`sync` given): tldraw multiplayer against the board room server —
 *   record-level diffs over WS, presence, offline edits queued in memory and rebased on
 *   reconnect. The server owns persistence; this pane never PATCHes content.
 * - **Local** (`sync` absent — the /spike/board loop): tldraw's own IndexedDB persistence
 *   via `persistenceKey`. No backend anywhere.
 */
export function BoardPane({
  boardId,
  title,
  client,
  host = NO_HOST,
  chrome = "full",
  active = true,
  sync = null,
  user = null,
}: {
  boardId: string;
  title?: string;
  client: ApiClient;
  host?: BoardHost;
  chrome?: BoardChromeMode;
  /**
   * False while the host keeps this pane mounted but off screen (the iPad's one web view
   * holds every open board). A paused board drops focus — no key handling, no pointer
   * capture. Its room socket stays up, so background boards keep receiving.
   */
  active?: boolean;
  sync?: BoardSyncConfig | null;
  user?: BoardUser | null;
}) {
  // Presence identity rides tldraw's user preferences (what useSync reads by default).
  useEffect(() => {
    if (!user) return;
    const current = getUserPreferences();
    if (current.name !== user.name || (user.color && current.color !== user.color)) {
      setUserPreferences({ ...current, name: user.name, color: user.color ?? current.color });
    }
  }, [user]);

  // Built once per mount (token values read from the live CSS vars at that moment).
  const themes = useMemo(() => ({ default: buildBoardTheme() }), []);
  const content = useMemo(() => createBoardContentSource(client), [client]);
  const toasts = useMemo(createToastStore, []);
  const [folderId, setFolderId] = useState<string | null>(null);

  // The board's folder feeds the picker; metadata only — content never rides REST here.
  useEffect(() => {
    let alive = true;
    client
      .fetch<{ folderId?: string | null }>(`/api/artifacts/${encodeURIComponent(boardId)}`)
      .then((record) => {
        if (alive) setFolderId(record.folderId ?? null);
      })
      .catch(() => {
        if (alive) setFolderId(null);
      });
    return () => {
      alive = false;
    };
  }, [client, boardId]);

  // Sync errors are terminal (useSync never redials after one) — Retry remounts the
  // synced subtree, which builds a fresh hook and dials again.
  const [epoch, setEpoch] = useState(0);
  const retry = useCallback(() => setEpoch((n) => n + 1), []);

  const frame = (status: BoardStatus, canvas: React.ReactNode) => (
    <div className="flex h-full w-full flex-col bg-bg">
      {chrome === "full" ? (
        <div className="flex h-9 shrink-0 items-center gap-3 border-b border-line bg-panel px-3">
          <div className="min-w-0 flex-1 truncate font-display text-body font-medium text-ink">
            {title || "Board"}
          </div>
          <div className="font-mono text-meta uppercase text-ink-4">{headerLabel(status)}</div>
        </div>
      ) : null}
      <div className="relative min-h-0 flex-1">
        <BoardHostContext.Provider value={host}>
          <BoardStatusContext.Provider value={status}>
            <BoardToastContext.Provider value={toasts}>
              <BoardContentContext.Provider value={content}>
                <BoardFolderContext.Provider value={folderId}>{canvas}</BoardFolderContext.Provider>
              </BoardContentContext.Provider>
            </BoardToastContext.Provider>
          </BoardStatusContext.Provider>
        </BoardHostContext.Provider>
      </div>
    </div>
  );

  if (sync) {
    return (
      <SyncedBoard
        key={epoch}
        boardId={boardId}
        sync={sync}
        themes={themes}
        active={active}
        retry={retry}
        frame={frame}
      />
    );
  }
  return <LocalBoard boardId={boardId} themes={themes} active={active} frame={frame} />;
}

function SyncedBoard({
  boardId,
  sync,
  themes,
  active,
  retry,
  frame,
}: {
  boardId: string;
  sync: BoardSyncConfig;
  themes: Record<string, ReturnType<typeof buildBoardTheme>>;
  active: boolean;
  retry: () => void;
  frame: (status: BoardStatus, canvas: React.ReactNode) => React.ReactElement;
}) {
  // Re-invoked by useSync on every connection attempt — the token stays fresh across
  // reconnects. `sessionId`/`storeId` are reserved params the hook appends itself.
  const uri = useCallback(async () => {
    const token = await sync.getToken();
    return `${sync.url}/board/${encodeURIComponent(boardId)}?token=${encodeURIComponent(token)}`;
  }, [sync, boardId]);

  const store = useSync({
    uri,
    assets: inlineBase64AssetStore,
    shapeUtils: SYNC_SHAPE_UTILS,
    themes,
  });

  // Upstream trap (tldraw 5.3.2): useSync registers these themes' colours at render, but
  // its INTERNAL createTLStore re-registers the defaults — evicting the board's ink names
  // before the room's records arrive, so a board with "learn" strokes would fail hydration
  // validation. This effect runs right after useSync's (same component, later hook order)
  // and puts them back before any socket data lands. <Tldraw themes> keeps them fresh once
  // the canvas mounts.
  useLayoutOrderedEffect(() => {
    registerColorsFromThemes(resolveThemes(themes));
  });

  const status: BoardStatus =
    store.status === "loading"
      ? { mode: "synced", state: "loading", retry }
      : store.status === "error"
        ? { mode: "synced", state: "error", reason: syncErrorReason(store.error), retry }
        : store.connectionStatus === "offline"
          ? { mode: "synced", state: "offline", retry }
          : { mode: "synced", state: "online", retry };

  return frame(
    status,
    <>
      {store.status !== "synced-remote" ? (
        <div className="absolute inset-0 z-10 grid place-items-center bg-bg font-mono text-small text-ink-4">
          {store.status === "loading" ? "joining the board…" : null}
        </div>
      ) : null}
      {store.status === "synced-remote" ? (
        <BoardCanvas store={store.store} themes={themes} active={active} zoomToFitOnMount />
      ) : null}
    </>,
  );
}

function LocalBoard({
  boardId,
  themes,
  active,
  frame,
}: {
  boardId: string;
  themes: Record<string, ReturnType<typeof buildBoardTheme>>;
  active: boolean;
  frame: (status: BoardStatus, canvas: React.ReactNode) => React.ReactElement;
}) {
  return frame(
    { mode: "local", state: "online", retry: () => {} },
    <BoardCanvas persistenceKey={`aladin-board-${boardId}`} themes={themes} active={active} />,
  );
}

function BoardCanvas({
  store,
  persistenceKey,
  themes,
  active,
  zoomToFitOnMount = false,
}: {

  store?: ReturnType<typeof useSync> extends infer R
    ? R extends { status: "synced-remote"; store: infer S }
      ? S
      : never
    : never;
  persistenceKey?: string;
  themes: Record<string, ReturnType<typeof buildBoardTheme>>;
  active: boolean;
  zoomToFitOnMount?: boolean;
}) {
  const [editor, setEditor] = useState<Editor | null>(null);
  const host = useBoardHost();
  const toasts = useBoardToasts();

  // Off screen: give up focus (no key handling, no pointer capture). The socket lives on.
  useEffect(() => {
    if (!editor) return;
    editor.updateInstanceState({ isFocused: active });
  }, [editor, active]);

  // The capture inbox: while this board is the active one, reader excerpts land here as
  // cited excerpt objects. Drains on front AND on every new capture while fronted.
  useEffect(() => {
    const captures = host.captures;
    if (!captures || !editor || !active) return;
    const drain = () => {
      const items = captures.take();
      if (items.length === 0) return;
      for (const item of items) {
        addExcerpt(editor, {
          text: item.text,
          sourceArtifactId: item.sourceArtifactId,
          sourceTitle: item.sourceTitle,
          page: item.page,
        });
      }
      host.haptic?.("light");
      toasts.show({
        text:
          items.length === 1
            ? `Excerpt landed — ${items[0].sourceTitle} · p. ${items[0].page}`
            : `${items.length} excerpts landed`,
      });
    };
    drain();
    return captures.subscribe(drain);
  }, [host, editor, active, toasts]);

  function handleMount(mounted: Editor) {
    setEditor(mounted);
    if (import.meta.env.DEV) {
      (window as unknown as { __boardEditor?: Editor }).__boardEditor = mounted;
    }
    mounted.setCameraOptions(boardCameraOptions);
    // ⌘V / system paste of text lands as an excerpt at the paste point (intake door 3).
    mounted.registerExternalContentHandler("text", (info) => {
      const text = info.text.trim();
      if (text) addExcerpt(mounted, { text, at: info.point });
    });
    // Session state (camera) is per-device and starts fresh on a synced board — frame the
    // content once so the board opens showing itself.
    if (zoomToFitOnMount && mounted.getCurrentPageShapeIds().size > 0) {
      requestAnimationFrame(() => mounted.zoomToFit());
    }
  }

  return (
    <Tldraw
      hideUi
      themes={themes}
      colorScheme="dark"
      components={BOARD_COMPONENTS}
      tools={BOARD_TOOLS}
      shapeUtils={BOARD_SHAPES}
      options={BOARD_OPTIONS}
      store={store}
      persistenceKey={store ? undefined : persistenceKey}
      onMount={handleMount}
    />
  );
}

function headerLabel(status: BoardStatus): string {
  if (status.mode === "local") return "Local";
  switch (status.state) {
    case "loading":
      return "Joining";
    case "online":
      return "Live";
    case "offline":
      return "Offline";
    case "error":
      return "Not connected";
  }
}

/** Maps tldraw's terminal close reasons to something a person can act on. */
export function syncErrorReason(error: unknown): string {
  const reason =
    error && typeof error === "object" && "reason" in error ? String(error.reason) : "";
  switch (reason) {
    case "NOT_AUTHENTICATED":
      return "the session expired — sign in again";
    case "FORBIDDEN":
      return "this board belongs to another account";
    case "NOT_FOUND":
      return "this board no longer exists";
    case "CLIENT_TOO_OLD":
      return "this app is older than the server — update the app";
    case "SERVER_TOO_OLD":
      return "the server is older than this app — update the server";
    case "RATE_LIMITED":
      return "rate limited — try again in a moment";
    case "ROOM_FULL":
      return "the board is full";
    default:
      return "couldn't reach the board server";
  }
}
