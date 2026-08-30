/**
 * The gate for board multiplayer: TWO real useSync clients against the REAL room server
 * (imported straight from services/blocknote), one room — a record created on client A
 * must materialize on client B, and a board-coloured ink stroke must survive validation
 * on the wire. No Postgres: the server's pool is stubbed; rooms live in a temp dir.
 */
// @vitest-environment jsdom
import { mkdtempSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { useEffect, useMemo } from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { WebSocket as NodeWebSocket } from "ws";
import { useSync } from "@tldraw/sync";
import { ClientWebSocketAdapter } from "@tldraw/sync-core";
import {
  DefaultColorStyle,
  NoteShapeUtil,
  FrameShapeUtil,
  toRichText,
  inlineBase64AssetStore,
  registerColorsFromThemes,
  resolveThemes,
} from "tldraw";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

// eslint-disable-next-line import/no-relative-packages -- the E2E spans both packages on purpose
import { createBoardSyncServer } from "../../../services/blocknote/src/services/board-sync.js";
import { SYNC_SHAPE_UTILS } from "@/modules/board/ui/board-pane";
import { buildBoardStudioTheme } from "@/modules/board/domain/board-appearance";
import { TASK_DEFAULTS } from "@/modules/board/shapes/shape-types";

const OWNER = "00000000-0000-0000-0000-0000000000aa";
const BOARD_ID = "e2e-board";

/** The pool the server sees: one owned board artifact, projections captured in memory. */
function stubPool() {
  const state = { content: "", updates: 0 };
  return {
    state,
    async query(sql: string, params: unknown[]) {
      if (sql.includes("SELECT user_id, type")) {
        return { rows: [{ user_id: OWNER, type: "board" }] };
      }
      if (sql.includes("SELECT content")) {
        return { rows: [{ content: state.content }] };
      }
      if (sql.startsWith("UPDATE artifacts")) {
        state.content = String(params[1]);
        state.updates += 1;
        return { rows: [] };
      }
      throw new Error(`stub pool: unexpected query ${sql}`);
    },
  };
}

let server: ReturnType<typeof createBoardSyncServer>;
let pool: ReturnType<typeof stubPool>;
let base = "";
const themes = { default: buildBoardStudioTheme() };

beforeAll(async () => {
  (globalThis as { WebSocket?: unknown }).WebSocket = NodeWebSocket;
  (window as unknown as { __tldraw_socket_debug?: boolean }).__tldraw_socket_debug = true;
  pool = stubPool();
  server = createBoardSyncServer({
    pool,
    resolveToken: async (token: string) => {
      if (token !== "good") throw new Error("bad token");
      return { userId: OWNER };
    },
    port: 0,
    dataDir: mkdtempSync(path.join(os.tmpdir(), "board-e2e-")),
    projectionDebounceMs: 50,
  });
  const address = (await server.listen()) as { port: number };
  base = `ws://127.0.0.1:${address.port}/board/${BOARD_ID}?token=good`;
});

afterAll(async () => {
  await server.destroy();
});

/**
 * One synced client. In the app each device is its own browser tab, so useSync's per-tab
 * sessionId is naturally unique; two clients in ONE jsdom tab would share it and replace
 * each other's session forever — so the test names its own via the `connect` escape hatch.
 */
function client(session: string) {
  return renderHook(() => {
    // Identity-stable, like every option into useSync — an inline arrow would re-run its
    // connection effect on every render, tearing the socket down mid-handshake.
    const connect = useMemo(
      () =>
        ({ storeId }: { storeId: string }) =>
          new ClientWebSocketAdapter(async () => `${base}&sessionId=${session}&storeId=${storeId}`),
      [],
    );
    const store = useSync({
      connect,
      assets: inlineBase64AssetStore,
      shapeUtils: SYNC_SHAPE_UTILS,
      themes,
    });
    // Mirrors SyncedBoard's guard: useSync's internal createTLStore re-registers default
    // colours, evicting the board's — re-assert before any record arrives.
    useEffect(() => {
      registerColorsFromThemes(resolveThemes(themes));
    });
    return store;
  });
}

describe("board multiplayer end to end", () => {
  it("two clients converge, board colours validate, and the projection lands", async () => {
    const a = client("session-a");
    const b = client("session-b");
    await waitFor(() => expect(a.result.current.status).toBe("synced-remote"), {
      timeout: 8000,
    });
    await waitFor(() => expect(b.result.current.status).toBe("synced-remote"), {
      timeout: 8000,
    });
    const storeA = (a.result.current as { store: { put(r: unknown[]): void } }).store;
    const storeB = (
      b.result.current as { store: { get(id: string): unknown; allRecords(): unknown[] } }
    ).store;

    const task = {
      typeName: "shape",
      type: "aladin-task",
      id: "shape:e2e-task",
      index: "a2",
      parentId: "page:page",
      x: 40,
      y: 40,
      rotation: 0,
      isLocked: false,
      opacity: 1,
      meta: { boardTint: "sage" },
      props: { ...TASK_DEFAULTS, text: "made on A" },
    };
    const ink = {
      typeName: "shape",
      type: "draw",
      id: "shape:e2e-ink",
      index: "a3",
      parentId: "page:page",
      x: 0,
      y: 0,
      rotation: 0,
      isLocked: false,
      opacity: 1,
      meta: {},
      props: {
        // 5.3 draw segments are a prebuilt path string, no point list (board trap #4).
        segments: [{ type: "free", path: "M0 0 L10 10" }],
        color: "learn",
        fill: "none",
        dash: "draw",
        size: "m",
        isComplete: true,
        isClosed: false,
        isPen: true,
        scale: 1,
        scaleX: 1,
        scaleY: 1,
      },
    };
    expect(DefaultColorStyle.values, "client registry lost the board colours").toContain("learn");
    const note = {
      ...task, type: "note", id: "shape:e2e-note", index: "a4", meta: {},
      props: { ...NoteShapeUtil.prototype.getDefaultProps(), color: "yellow", richText: toRichText("Research hypothesis") },
    };
    const frame = {
      ...task, type: "frame", id: "shape:e2e-frame", index: "a5", meta: {},
      props: { ...FrameShapeUtil.prototype.getDefaultProps(), name: "Method and evidence" },
    };
    storeA.put([task, ink, note, frame]);

    await waitFor(
      () => {
        expect(storeB.get("shape:e2e-task"), "task did not arrive on B").toBeTruthy();
        expect(storeB.get("shape:e2e-ink"), "learn-coloured ink did not arrive on B").toBeTruthy();
        expect(storeB.get(note.id)).toEqual(note);
        expect(storeB.get(frame.id)).toEqual(frame);
        expect(storeB.get(task.id)).toMatchObject({ meta: { boardTint: "sage" } });
      },
      { timeout: 8000 },
    );

    await waitFor(
      () => {
        expect(pool.state.content, "projection never reached artifacts.content").toContain(
          "shape:e2e-task",
        );
      },
      { timeout: 8000 },
    );

    a.unmount();
    b.unmount();

    const reopened = client("session-reopened");
    await waitFor(() => expect(reopened.result.current.status).toBe("synced-remote"), { timeout: 8000 });
    const restored = (reopened.result.current as { store: { get(id: string): unknown } }).store;
    expect(restored.get(note.id)).toEqual(note);
    expect(restored.get(frame.id)).toEqual(frame);
    expect(restored.get(task.id)).toMatchObject({ meta: { boardTint: "sage" } });
    reopened.unmount();
  }, 30000);
});
