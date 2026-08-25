// The board sync room server — tldraw multiplayer (@tldraw/sync-core) for board artifacts,
// third listener in this process (`ws@8`; Hocuspocus's bundled ws@7 owns :COLLAB_PORT, the
// same reason the converter and collab already sit on separate ports).
//
// Shape of the thing:
//  - One `TLSocketRoom` per open board, keyed by artifact id. THE singleton rule is
//    absolute: two rooms for one board silently overwrite each other's edits, so the map
//    holds PROMISES — a second connect during an async open awaits the first.
//  - Room truth = one SQLite file per board (`SQLiteSyncStorage`, transactional, restart-
//    safe). First open seeds from the artifact's legacy `content` snapshot, so every board
//    made under the PATCH regime carries over.
//  - Postgres stays near-canonical: storage `onChange` schedules a debounced projection of
//    the room snapshot into `artifacts.content` (same discipline as collab.js's
//    `writeProjection`: debounce + sweep, best-effort, never throws into the WS path). A
//    lost SQLite file re-seeds from this projection — worst case one debounce window.
//  - Auth on upgrade: bearer from `?token=` through the same Go `/api/auth/resolve` the
//    page collab uses, THEN per-room authorization (the artifact must exist, be a board,
//    and belong to the principal). Failures still complete the WS handshake and close with
//    tldraw's coded reason, so the client surfaces the right terminal error.

import http from "node:http";
import { mkdirSync } from "node:fs";
import path from "node:path";
import { DatabaseSync } from "node:sqlite";
import { WebSocketServer } from "ws";
import {
  NodeSqliteWrapper,
  SQLiteSyncStorage,
  TLSocketRoom,
  TLSyncErrorCloseEventCode,
  TLSyncErrorCloseEventReason,
} from "@tldraw/sync-core";
import { errorMessage } from "../errors.js";
import { createBoardSchema } from "./board-schema.js";

/** Board room ids become file names — strip anything path-like (template's guard). */
export function sanitizeRoomId(artifactId) {
  return artifactId.replace(/[^a-zA-Z0-9_-]/g, "_");
}

/**
 * A room snapshot as the REST/MCP world knows board content: the legacy
 * `{document: {store, schema}}` JSON (session omitted — sessions are per-device now).
 */
export function roomSnapshotToArtifactContent(snapshot) {
  const store = {};
  for (const doc of snapshot.documents ?? []) {
    if (doc?.state?.id) store[doc.state.id] = doc.state;
  }
  return JSON.stringify({ document: { store, schema: snapshot.schema } });
}

/**
 * The seed for a fresh room: the document part of the artifact's legacy content, or null.
 * Both `store` and `schema` are required — a real `editor.getSnapshot()` always carries
 * both, and a schema-less store can neither be migrated nor bound into the room's
 * metadata (SQLiteSyncStorage stringifies it verbatim).
 */
export function parseLegacyContent(content) {
  const trimmed = (content ?? "").trim();
  if (!trimmed) return null;
  try {
    const parsed = JSON.parse(trimmed);
    const document = parsed?.document;
    if (
      document &&
      typeof document === "object" &&
      "store" in document &&
      document.schema &&
      typeof document.schema === "object"
    ) {
      return document;
    }
    return null;
  } catch {
    return null;
  }
}

export function createBoardSyncServer({
  pool,
  resolveToken,
  port,
  dataDir,
  projectionDebounceMs = 2000,
  projectionSweepMs = 5000,
}) {
  const schema = createBoardSchema();
  mkdirSync(dataDir, { recursive: true });

  /** artifactId -> Promise<Entry>; Entry = { room, db, storage, projection, artifactId } */
  const rooms = new Map();

  // ── Projection: room snapshot → artifacts.content, debounced per room ──
  function createProjection(artifactId, getSnapshot) {
    const state = { dirty: false, timer: null };
    async function flush() {
      if (!state.dirty) return;
      state.dirty = false;
      if (state.timer) {
        clearTimeout(state.timer);
        state.timer = null;
      }
      try {
        const content = roomSnapshotToArtifactContent(getSnapshot());
        await pool.query(
          `UPDATE artifacts SET content = $2, updated_at = now() WHERE id = $1`,
          [artifactId, content],
        );
      } catch (err) {
        // Best-effort: the room's SQLite file is the truth; the projection heals on the
        // next change or the sweep. Never throws into the sync path.
        console.error(`board-sync projection ${artifactId}: ${errorMessage(err)}`);
        state.dirty = true;
      }
    }
    return {
      schedule() {
        state.dirty = true;
        if (state.timer) return;
        state.timer = setTimeout(() => {
          state.timer = null;
          void flush();
        }, projectionDebounceMs);
        // A pending timer must not hold the process open at shutdown; destroy() flushes.
        state.timer.unref?.();
      },
      flush,
      get dirty() {
        return state.dirty;
      },
    };
  }

  // Restart-orphan sweep, same rationale as collab.js's projection sweep.
  const sweep = setInterval(() => {
    for (const entryPromise of rooms.values()) {
      entryPromise.then((entry) => void entry.projection.flush()).catch(() => {});
    }
  }, projectionSweepMs);
  sweep.unref?.();

  // ── Rooms ──
  async function buildRoom(artifactId) {
    const file = path.join(dataDir, `${sanitizeRoomId(artifactId)}.sqlite`);
    const db = new DatabaseSync(file);
    const sql = new NodeSqliteWrapper(db);
    let seed;
    if (!SQLiteSyncStorage.hasBeenInitialized(sql)) {
      const { rows } = await pool.query(`SELECT content FROM artifacts WHERE id = $1`, [
        artifactId,
      ]);
      seed = parseLegacyContent(rows[0]?.content) ?? undefined;
    }
    const entry = { artifactId, db, room: null, storage: null, projection: null };
    entry.projection = createProjection(artifactId, () => entry.storage.getSnapshot());
    entry.storage = new SQLiteSyncStorage({
      sql,
      snapshot: seed,
      onChange: () => entry.projection.schedule(),
    });
    entry.room = new TLSocketRoom({
      schema,
      storage: entry.storage,
      log: {
        warn: (...args) => console.warn(`board-sync ${artifactId}:`, ...args),
        error: (...args) => console.error(`board-sync ${artifactId}:`, ...args),
      },
      onSessionRemoved(room, { numSessionsRemaining }) {
        if (numSessionsRemaining > 0) return;
        // Last one out closes the room; the file re-opens (already initialized, no seed)
        // on the next connect.
        void entry.projection.flush().finally(() => {
          if (!room.isClosed()) room.close();
          try {
            db.close();
          } catch {
            /* already closed */
          }
          if (rooms.get(artifactId) === promiseFor(entry)) rooms.delete(artifactId);
        });
      },
    });
    return entry;
  }

  // Map entries are promises; remember which promise an entry belongs to so a room closing
  // itself only evicts ITS map slot, never a newer room's.
  const promises = new WeakMap();
  function promiseFor(entry) {
    return promises.get(entry);
  }
  async function openRoom(artifactId) {
    const existing = rooms.get(artifactId);
    if (existing) {
      const entry = await existing.catch(() => null);
      if (entry && !entry.room.isClosed()) return entry;
      if (rooms.get(artifactId) === existing) rooms.delete(artifactId);
    }
    const promise = buildRoom(artifactId);
    rooms.set(artifactId, promise);
    try {
      const entry = await promise;
      promises.set(entry, promise);
      return entry;
    } catch (err) {
      if (rooms.get(artifactId) === promise) rooms.delete(artifactId);
      throw err;
    }
  }

  // ── Auth: token → principal, then room-level authorization ──
  async function denialFor(artifactId, token) {
    let principal;
    try {
      principal = await resolveToken(token);
    } catch {
      return { reason: TLSyncErrorCloseEventReason.NOT_AUTHENTICATED };
    }
    let rows;
    try {
      ({ rows } = await pool.query(`SELECT user_id, type FROM artifacts WHERE id = $1`, [
        artifactId,
      ]));
    } catch (err) {
      console.error(`board-sync authz ${artifactId}: ${errorMessage(err)}`);
      return { reason: TLSyncErrorCloseEventReason.UNKNOWN_ERROR };
    }
    const row = rows[0];
    if (!row || row.type !== "board") return { reason: TLSyncErrorCloseEventReason.NOT_FOUND };
    if (String(row.user_id) !== String(principal.userId)) {
      return { reason: TLSyncErrorCloseEventReason.FORBIDDEN };
    }
    return { principal };
  }

  // ── The listener ──
  const server = http.createServer((req, res) => {
    // The WS port answers a plain GET for doctors and curiosity.
    res.writeHead(200, { "content-type": "text/plain" });
    res.end("board-sync ok\n");
  });
  const wss = new WebSocketServer({ noServer: true });

  server.on("upgrade", (req, socket, head) => {
    void handleUpgrade(req, socket, head).catch((err) => {
      console.error(`board-sync upgrade: ${errorMessage(err)}`);
      socket.destroy();
    });
  });

  async function handleUpgrade(req, socket, head) {
    const url = new URL(req.url ?? "/", "http://localhost");
    const match = url.pathname.match(/^\/board\/([^/]+)$/);
    if (!match) {
      socket.write("HTTP/1.1 404 Not Found\r\n\r\n");
      socket.destroy();
      return;
    }
    const artifactId = decodeURIComponent(match[1]);
    const sessionId = url.searchParams.get("sessionId") ?? "";
    const token = url.searchParams.get("token") ?? "";

    // Resolve auth AND the room BEFORE completing the handshake: the client cannot send
    // its connect request until the upgrade finishes, so attaching the room's listeners
    // inside the handleUpgrade callback leaves no window to drop messages in (the
    // fastify template needs a buffering workaround precisely because it upgrades first).
    const denial = sessionId
      ? await denialFor(artifactId, token)
      : { reason: TLSyncErrorCloseEventReason.NOT_AUTHENTICATED };
    let entry = null;
    if (!denial.reason) {
      try {
        entry = await openRoom(artifactId);
      } catch (err) {
        console.error(`board-sync open ${artifactId}: ${errorMessage(err)}`);
        denial.reason = TLSyncErrorCloseEventReason.UNKNOWN_ERROR;
      }
    }

    wss.handleUpgrade(req, socket, head, (ws) => {
      if (denial.reason) {
        // Complete-then-close so the client receives tldraw's coded reason instead of a
        // bare failed connection it would retry forever.
        ws.close(TLSyncErrorCloseEventCode, denial.reason);
        return;
      }
      if (entry.room.isClosed()) {
        ws.close(TLSyncErrorCloseEventCode, TLSyncErrorCloseEventReason.UNKNOWN_ERROR);
        return;
      }
      entry.room.handleSocketConnect({
        sessionId,
        socket: ws,
        meta: { userId: denial.principal.userId },
      });
    });
  }

  return {
    async listen() {
      await new Promise((resolve, reject) => {
        server.once("error", reject);
        server.listen(port, "0.0.0.0", () => {
          server.off("error", reject);
          resolve();
        });
      });
      return server.address();
    },
    /** {rooms, sessions} for /healthz. */
    async stats() {
      let sessions = 0;
      let count = 0;
      for (const promise of rooms.values()) {
        const entry = await promise.catch(() => null);
        if (entry && !entry.room.isClosed()) {
          count += 1;
          sessions += entry.room.getNumActiveSessions();
        }
      }
      return { rooms: count, sessions };
    },
    /** Test/admin: the live room snapshot (opens the room if needed). */
    async getRoomSnapshot(artifactId) {
      const entry = await openRoom(artifactId);
      return entry.storage.getSnapshot();
    },
    /** Test/admin: server-side mutation (projection fires through storage onChange). */
    async updateRoomStore(artifactId, updater) {
      const entry = await openRoom(artifactId);
      await entry.room.updateStore(updater);
    },
    async destroy() {
      clearInterval(sweep);
      for (const promise of [...rooms.values()]) {
        const entry = await promise.catch(() => null);
        if (!entry) continue;
        await entry.projection.flush().catch(() => {});
        if (!entry.room.isClosed()) entry.room.close();
        try {
          entry.db.close();
        } catch {
          /* already closed */
        }
      }
      rooms.clear();
      await new Promise((resolve) => {
        wss.close(() => server.close(() => resolve()));
      });
    },
  };
}
