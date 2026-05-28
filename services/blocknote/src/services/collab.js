// Hocuspocus collaboration server. Owns the in-memory Y.Doc per page, syncs
// to connected clients, and persists the full Y.Doc binary to the page_ydoc
// table via the stock @hocuspocus/extension-database (debounced store).
//
// Uses the @hocuspocus/server `Server` wrapper (its own HTTP + WebSocket
// server on its own bundled `ws`). We deliberately do NOT hand-roll a `ws`
// WebSocketServer + handleConnection: the project's other `ws` is v8 while
// Hocuspocus bundles v7, and passing a v8 socket into Hocuspocus's v7
// message handling silently breaks the sync handshake. Letting Server own
// its socket sidesteps the version mismatch — at the cost of a second port
// (collab WS) alongside the converter's HTTP port. One process, two ports.
//
// Dependency-injected (pool, resolveToken) so the collab test can supply a
// test Postgres pool and a stub resolver without standing up the Go API.

import { Server } from "@hocuspocus/server";
import { Database } from "@hocuspocus/extension-database";

export function createCollabServer({
  pool,
  resolveToken,
  port,
  debounceMs,
  maxDebounceMs,
}) {
  return new Server({
    port: port ?? 3501,

    // Write-efficiency knobs only — durability lives on the clients +
    // y-indexeddb. See the plan's durability model.
    debounce: debounceMs ?? 2000,
    maxDebounce: maxDebounceMs ?? 10000,

    extensions: [
      new Database({
        // documentName is the Aladin page id.
        fetch: async ({ documentName }) => {
          const { rows } = await pool.query(
            "SELECT state FROM page_ydoc WHERE page_id = $1",
            [documentName],
          );
          // BYTEA comes back as a Buffer (a Uint8Array); null => new doc.
          return rows[0]?.state ?? null;
        },
        store: async ({ documentName, state }) => {
          await pool.query(
            `INSERT INTO page_ydoc (page_id, state, updated_at)
                 VALUES ($1, $2, now())
             ON CONFLICT (page_id)
                 DO UPDATE SET state = EXCLUDED.state, updated_at = now()`,
            [documentName, state],
          );
        },
      }),
    ],

    // Returned value becomes the connection context (read by awareness +
    // write-authz, and by the M8c admin bridge). Throwing rejects the
    // connection — Hocuspocus closes it with an auth error.
    onAuthenticate: async ({ token }) => {
      const principal = await resolveToken(token);
      return { principal };
    },
  });
}
