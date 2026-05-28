// blocknote — combined Node service for Aladin's BlockNote needs.
//
// M8a (now): markdown <-> blocks conversion (the former blocknote-converter).
// M8b:       mounts Hocuspocus on /collaboration for real-time Y.Doc sync.
// M8c:       adds the MCP admin bridge on /admin/apply.
//
// Layered: server.js boots Express + middleware and wires routes to thin
// handlers (src/handlers); handlers call services (src/services); the error
// boundary (src/middleware) guarantees a handler bug returns 500 rather than
// crashing the process.

import express from "express";
import { config } from "./src/config.js";
import { trace } from "./src/middleware/trace.js";
import {
  wrap,
  errorHandler,
  installProcessHandlers,
} from "./src/middleware/error-boundary.js";
import * as converter from "./src/handlers/converter.js";
import { healthz } from "./src/handlers/health.js";

installProcessHandlers();

const app = express();
app.use(express.json({ limit: config.jsonLimit }));
app.use(trace);

app.get("/healthz", healthz);
app.post("/md-to-blocks", wrap(converter.mdToBlocks));
app.post("/blocks-to-md", wrap(converter.blocksToMd));
app.post("/blocks-to-md-batch", wrap(converter.blocksToMdBatch));

// Terminal error handler — must be last.
app.use(errorHandler);

const server = app.listen(config.port, "0.0.0.0", () => {
  console.log(`blocknote service listening on :${config.port}`);
});

function shutdown(signal) {
  console.log(`blocknote service received ${signal}, shutting down`);
  server.close(() => process.exit(0));
}

process.on("SIGINT", () => shutdown("SIGINT"));
process.on("SIGTERM", () => shutdown("SIGTERM"));
