// The board sync server against real wire + Postgres. Gated behind BOARD_SYNC_IT=1
// (needs the sandbox DB: `make test-db-up`, and BLOCKNOTE_DATABASE_URL pointing at it).
//
//   BOARD_SYNC_IT=1 BLOCKNOTE_DATABASE_URL=postgres://aladin:password@localhost:5444/aladin \
//     node --test --test-force-exit test/board-sync-it.test.js
//
// Full two-client convergence lives in aladin_react's vitest E2E (the real useSync client);
// here we prove the server's own contract: auth closes with tldraw's coded reasons, a
// legacy snapshot seeds the room, and edits project back into artifacts.content.
import { test } from "node:test";
import assert from "node:assert";
import { mkdtempSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import pg from "pg";
import { WebSocket } from "ws";
import { createBoardSchema } from "../src/services/board-schema.js";
import { createBoardSyncServer } from "../src/services/board-sync.js";

const RUN = process.env.BOARD_SYNC_IT === "1";
const DB =
  process.env.BLOCKNOTE_DATABASE_URL ??
  "postgres://aladin:password@localhost:5433/aladin";
const OWNER = "00000000-0000-0000-0000-000000000001";
const STRANGER = "00000000-0000-0000-0000-000000000002";

function stubResolver() {
  return async (token) => {
    if (token === "owner-token") return { userId: OWNER };
    if (token === "stranger-token") return { userId: STRANGER };
    throw new Error("bad token");
  };
}

const SEED_SHAPE = {
  typeName: "shape",
  type: "aladin-task",
  id: "shape:seeded",
  index: "a1",
  parentId: "page:page",
  x: 10,
  y: 10,
  rotation: 0,
  isLocked: false,
  opacity: 1,
  meta: {},
  props: { w: 364, h: 112, text: "carried over", meta: "open", checked: false },
};

function seedContent() {
  // As the PATCH-era client wrote it: editor.getSnapshot()'s document part always carries
  // the serialized schema alongside the store.
  return JSON.stringify({
    document: {
      store: { [SEED_SHAPE.id]: SEED_SHAPE },
      schema: createBoardSchema().serialize(),
    },
    session: {},
  });
}

function closeEvent(url) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    const timer = setTimeout(() => reject(new Error("no close within 5s")), 5000);
    ws.on("close", (code, reason) => {
      clearTimeout(timer);
      resolve({ code, reason: reason.toString() });
    });
    ws.on("error", () => {});
  });
}

function opensAndStays(url, ms = 400) {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    let closed = null;
    ws.on("close", (code, reason) => {
      closed = { code, reason: reason.toString() };
    });
    ws.on("error", (err) => reject(err));
    ws.on("open", () => {
      setTimeout(() => {
        const ok = closed === null && ws.readyState === WebSocket.OPEN;
        ws.close();
        ok ? resolve() : reject(new Error(`closed early: ${JSON.stringify(closed)}`));
      }, ms);
    });
  });
}

test("board sync server", { skip: !RUN }, async (t) => {
  const pool = new pg.Pool({ connectionString: DB });
  const boardId = `it-board-${Date.now()}`;
  // The artifacts FK needs real users; namespaced fixed ids, ON CONFLICT tolerant, so
  // parallel suites on the shared sandbox never fight (test-isolation house rules).
  for (const [id, email] of [
    [OWNER, "board-sync-owner@test.local"],
    [STRANGER, "board-sync-stranger@test.local"],
  ]) {
    await pool.query(
      `INSERT INTO users (id, email, created_at) VALUES ($1::uuid, $2, now())
       ON CONFLICT (id) DO NOTHING`,
      [id, email],
    );
  }
  await pool.query(
    `INSERT INTO artifacts (id, user_id, type, title, content, metadata, created_at, updated_at)
     VALUES ($1, $2::uuid, 'board', 'IT board', $3, '{}'::jsonb, now(), now())`,
    [boardId, OWNER, seedContent()],
  );

  const server = createBoardSyncServer({
    pool,
    resolveToken: stubResolver(),
    port: 0,
    dataDir: mkdtempSync(path.join(os.tmpdir(), "board-rooms-")),
    projectionDebounceMs: 50,
  });
  const address = await server.listen();
  const base = `ws://127.0.0.1:${address.port}/board/${boardId}`;

  t.after(async () => {
    await server.destroy();
    await pool.query(`DELETE FROM artifacts WHERE id = $1`, [boardId]);
    await pool.end();
  });

  await t.test("bad token closes with NOT_AUTHENTICATED at tldraw's code", async () => {
    const { code, reason } = await closeEvent(`${base}?sessionId=s1&token=nope`);
    assert.equal(code, 4099);
    assert.equal(reason, "NOT_AUTHENTICATED");
  });

  await t.test("a stranger is FORBIDDEN", async () => {
    const { code, reason } = await closeEvent(`${base}?sessionId=s2&token=stranger-token`);
    assert.equal(code, 4099);
    assert.equal(reason, "FORBIDDEN");
  });

  await t.test("a non-board artifact is NOT_FOUND", async () => {
    const otherId = `${boardId}-page`;
    await pool.query(
      `INSERT INTO artifacts (id, user_id, type, title, content, metadata, created_at, updated_at)
       VALUES ($1, $2::uuid, 'page', 'IT page', '', '{}'::jsonb, now(), now())`,
      [otherId, OWNER],
    );
    const address2 = address; // same server
    const { code, reason } = await closeEvent(
      `ws://127.0.0.1:${address2.port}/board/${otherId}?sessionId=s3&token=owner-token`,
    );
    assert.equal(code, 4099);
    assert.equal(reason, "NOT_FOUND");
    await pool.query(`DELETE FROM artifacts WHERE id = $1`, [otherId]);
  });

  await t.test("the owner's socket opens and stays open", async () => {
    await opensAndStays(`${base}?sessionId=s4&token=owner-token`);
  });

  await t.test("the legacy snapshot seeded the room", async () => {
    const snapshot = await server.getRoomSnapshot(boardId);
    const ids = snapshot.documents.map((d) => d.state.id);
    assert.ok(ids.includes("shape:seeded"), `room lacks seeded shape: ${ids.join(",")}`);
  });

  await t.test("a server-side edit projects back into artifacts.content", async () => {
    await server.updateRoomStore(boardId, (store) => {
      store.put({ ...SEED_SHAPE, id: "shape:added", x: 200 });
    });
    // debounce 50ms — poll briefly
    let content = "";
    for (let i = 0; i < 40; i++) {
      await new Promise((r) => setTimeout(r, 50));
      const { rows } = await pool.query(`SELECT content FROM artifacts WHERE id = $1`, [
        boardId,
      ]);
      content = rows[0].content;
      if (content.includes("shape:added")) break;
    }
    assert.ok(content.includes("shape:added"), "projection never landed");
    const parsed = JSON.parse(content);
    assert.ok(parsed.document.store["shape:seeded"], "projection lost the seed");
  });
});
