// Two-client convergence + persistence test through the real Hocuspocus
// server. Gated behind COLLAB_IT=1 because it needs a live Postgres with the
// M8 schema. Normal `npm test` (hermetic) skips it.
//
//   COLLAB_IT=1 npm test
//   COLLAB_IT=1 node --test --test-force-exit test/collab.test.js

import { test } from "node:test";
import assert from "node:assert";
import pg from "pg";
import * as Y from "yjs";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { ServerBlockNoteEditor } from "@blocknote/server-util";
import { WebSocket } from "ws";
import { createCollabServer } from "../src/services/collab.js";

const RUN = process.env.COLLAB_IT === "1";
const DB =
  process.env.BLOCKNOTE_DATABASE_URL ??
  "postgres://aladin:password@localhost:5433/aladin";
const ADMIN_USER = "00000000-0000-0000-0000-000000000001";
const editor = ServerBlockNoteEditor.create();

function stubResolver() {
  return async () => ({
    userId: ADMIN_USER,
    actorType: "user_session",
    actorId: ADMIN_USER,
    scopes: [],
  });
}

test(
  "two clients converge and Y.Doc state persists to page_ydoc",
  { skip: !RUN },
  async () => {
    const pool = new pg.Pool({ connectionString: DB });
    const pageId = `test-collab-${Date.now()}`;

    // page_ydoc.page_id → artifacts.id → users.id. Seed a throwaway page
    // artifact owned by the default admin so the FK + store succeed.
    await pool.query(
      `INSERT INTO artifacts (id, user_id, type, title, content)
         VALUES ($1, $2::uuid, 'page', 'collab test', '')`,
      [pageId, ADMIN_USER],
    );

    const collab = createCollabServer({
      pool,
      resolveToken: async () => ({
        userId: ADMIN_USER,
        actorType: "user_session",
        actorId: ADMIN_USER,
        scopes: [],
      }),
      port: 0, // ephemeral
      debounceMs: 100,
      maxDebounceMs: 300,
    });
    await collab.listen();
    const port = collab.address.port;
    const url = `ws://127.0.0.1:${port}`;

    const docA = new Y.Doc();
    const docB = new Y.Doc();
    const provA = new HocuspocusProvider({
      url,
      name: pageId,
      document: docA,
      token: "test",
      WebSocketPolyfill: WebSocket,
    });
    const provB = new HocuspocusProvider({
      url,
      name: pageId,
      document: docB,
      token: "test",
      WebSocketPolyfill: WebSocket,
    });

    try {
      await waitFor(() => provA.synced && provB.synced, 5000, "both synced");

      // A writes → B sees it.
      docA.getMap("root").set("greeting", "hello from A");
      await waitFor(
        () => docB.getMap("root").get("greeting") === "hello from A",
        5000,
        "B sees A's write",
      );

      // Concurrent inserts → both survive deterministically.
      docA.getArray("list").push(["a"]);
      docB.getArray("list").push(["b"]);
      await waitFor(
        () =>
          docA.getArray("list").length === 2 &&
          docB.getArray("list").length === 2,
        5000,
        "both lists converge to length 2",
      );
      assert.deepEqual(docA.getArray("list").toArray().sort(), ["a", "b"]);
      assert.deepEqual(docB.getArray("list").toArray().sort(), ["a", "b"]);

      // Persistence: full Y.Doc state landed in page_ydoc after debounce.
      await waitFor(
        async () => {
          const { rows } = await pool.query(
            "SELECT state FROM page_ydoc WHERE page_id = $1",
            [pageId],
          );
          return rows.length === 1 && (rows[0].state?.length ?? 0) > 0;
        },
        5000,
        "state persisted to page_ydoc",
      );
    } finally {
      provA.destroy();
      provB.destroy();
      await collab.destroy();
      // Cascades to page_ydoc via FK.
      await pool.query("DELETE FROM artifacts WHERE id = $1", [pageId]);
      await pool.end();
    }
  },
);

test(
  "MCP bridge op broadcasts to live clients and lands in the projection",
  { skip: !RUN },
  async () => {
    const pool = new pg.Pool({ connectionString: DB });
    const pageId = `test-bridge-${Date.now()}`;
    await pool.query(
      `INSERT INTO artifacts (id, user_id, type, title, content)
         VALUES ($1, $2::uuid, 'page', 'bridge test', '')`,
      [pageId, ADMIN_USER],
    );

    const collab = createCollabServer({
      pool,
      resolveToken: stubResolver(),
      port: 0,
      debounceMs: 100,
      maxDebounceMs: 300,
      projectionDebounceMs: 100,
    });
    await collab.listen();
    const url = `ws://127.0.0.1:${collab.address.port}`;

    const docA = new Y.Doc();
    const docB = new Y.Doc();
    const provA = new HocuspocusProvider({
      url,
      name: pageId,
      document: docA,
      token: "test",
      WebSocketPolyfill: WebSocket,
    });
    const provB = new HocuspocusProvider({
      url,
      name: pageId,
      document: docB,
      token: "test",
      WebSocketPolyfill: WebSocket,
    });
    const textOf = (doc) =>
      editor
        .yDocToBlocks(doc, "document")
        .map((b) => b.content?.[0]?.text ?? "");

    try {
      await waitFor(() => provA.synced && provB.synced, 5000, "both synced");

      // A server-side bridge op mutates the same in-memory doc the clients
      // hold → both see it live (the M8.6 acceptance criterion).
      const res = await collab.applyOperation({
        page_id: pageId,
        op: "replace_all",
        blocks: [{ type: "paragraph", content: "hello from the bridge" }],
        agent: { id: "agent-1", name: "Claude" },
      });
      assert.equal(res.blockIds.length, 1, "replace_all yields one block");

      await waitFor(
        () =>
          textOf(docA).join() === "hello from the bridge" &&
          textOf(docB).join() === "hello from the bridge",
        5000,
        "both clients see the bridge replace_all",
      );

      // insert_blocks appends; untouched block ids stay stable.
      const ins = await collab.applyOperation({
        page_id: pageId,
        op: "insert_blocks",
        blocks: [{ type: "paragraph", content: "second" }],
      });
      assert.equal(ins.affectedBlockIds.length, 1);
      await waitFor(
        () => textOf(docA).length === 2 && textOf(docB).length === 2,
        5000,
        "both clients see the inserted block",
      );

      // M8.7 projection caught up in page_documents.
      await waitFor(
        async () => {
          const { rows } = await pool.query(
            "SELECT blocks, search_text FROM page_documents WHERE artifact_id = $1",
            [pageId],
          );
          return (
            rows.length === 1 &&
            Array.isArray(rows[0].blocks) &&
            rows[0].blocks.length === 2 &&
            rows[0].search_text.includes("second")
          );
        },
        5000,
        "projection reflects the bridge edits",
      );
    } finally {
      provA.destroy();
      provB.destroy();
      await collab.destroy();
      await pool.query("DELETE FROM artifacts WHERE id = $1", [pageId]);
      await pool.end();
    }
  },
);

async function waitFor(cond, timeoutMs, label) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    if (await cond()) return;
    await new Promise((r) => setTimeout(r, 50));
  }
  throw new Error(`waitFor timed out: ${label}`);
}
