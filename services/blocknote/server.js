// blocknote — combined Node service for Aladin's BlockNote needs.
//
// M8a: markdown <-> blocks conversion (the former blocknote-converter), HTTP
//      on :PORT (default 3500).
// M8b (now): Hocuspocus real-time Y.Doc collaboration on its own server,
//      :COLLAB_PORT (default 3501), persisting to page_ydoc; auth via the
//      Go API. Separate port because Hocuspocus bundles ws@7 and the Express
//      stack uses ws@8 — separate servers avoid the socket version mismatch.
// M8c: adds the MCP admin bridge on the HTTP server (/admin/apply).
//
// One process, two listeners. Layered: server.js boots both and wires Express
// routes to thin handlers (src/handlers) that call services (src/services);
// the error boundary (src/middleware) keeps a handler bug from crashing the
// process.

import express from "express";
import pg from "pg";
import { config } from "./src/config.js";
import { trace } from "./src/middleware/trace.js";
import {
  wrap,
  errorHandler,
  installProcessHandlers,
} from "./src/middleware/error-boundary.js";
import * as converter from "./src/handlers/converter.js";
import { healthz } from "./src/handlers/health.js";
import { createCollabAdminHandlers } from "./src/handlers/collab-admin.js";
import { requireAdminSecret } from "./src/middleware/admin-auth.js";
import { createAuthResolver } from "./src/services/auth.js";
import { createCollabServer } from "./src/services/collab.js";

installProcessHandlers();

// Shared Postgres pool for collab persistence (and, in M8c, the admin bridge).
const pool = new pg.Pool({ connectionString: config.databaseUrl });

// --- Collaboration (Hocuspocus, :COLLAB_PORT) ------------------------------
// Created before the HTTP routes so the MCP admin bridge can call into the
// live Y.Docs in-process.
const collab = createCollabServer({
  pool,
  resolveToken: createAuthResolver(config.authResolveUrl),
  port: config.collabPort,
  debounceMs: config.storeDebounceMs,
  maxDebounceMs: config.storeMaxDebounceMs,
});
const collabAdmin = createCollabAdminHandlers(collab);

// --- Converter (HTTP, :PORT) + MCP admin bridge ----------------------------
const app = express();
app.use(express.json({ limit: config.jsonLimit }));
app.use(trace);

app.get("/healthz", healthz);
app.post("/md-to-blocks", wrap(converter.mdToBlocks));
app.post("/blocks-to-md", wrap(converter.blocksToMd));
app.post("/blocks-to-md-batch", wrap(converter.blocksToMdBatch));

// M8c MCP admin bridge — shared-secret guarded; mutates/reads the live Y.Docs.
const adminGuard = requireAdminSecret(config.adminSharedSecret);
app.post("/admin/apply", adminGuard, wrap(collabAdmin.apply));
app.get("/admin/page/:id", adminGuard, wrap(collabAdmin.getPage));

// Terminal error handler — must be last.
app.use(errorHandler);

const httpServer = app.listen(config.port, "0.0.0.0", () => {
  console.log(`blocknote converter + admin bridge listening on :${config.port}`);
});

await collab.listen();
console.log(`blocknote collab (Hocuspocus) listening on :${config.collabPort}`);

async function shutdown(signal) {
  console.log(`blocknote service received ${signal}, shutting down`);
  try {
    await collab.destroy(); // flushes pending stores + closes connections
  } catch (err) {
    console.error("collab shutdown error:", err);
  }
  try {
    await pool.end();
  } catch (err) {
    console.error("pool shutdown error:", err);
  }
  httpServer.close(() => process.exit(0));
}

process.on("SIGINT", () => shutdown("SIGINT"));
process.on("SIGTERM", () => shutdown("SIGTERM"));
