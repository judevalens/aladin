// blocknote-converter — stateless HTTP sidecar exposing BlockNote's
// markdown<->blocks converter for the Go MCP server. The Go side is the
// only client.
//
// Endpoints:
//   POST /md-to-blocks         {markdown: string}        -> {blocks: Block[]}
//   POST /blocks-to-md         {blocks: Block[]}         -> {markdown: string}
//   POST /blocks-to-md-batch   {blocks: Block[][]}       -> {markdowns: string[]}
//   GET  /healthz                                        -> "ok"
//
// We construct a single ServerBlockNoteEditor at startup and reuse it per
// request. ServerBlockNoteEditor exists precisely for headless conversions
// in Node and does not require a DOM.

import express from "express";
import { ServerBlockNoteEditor } from "@blocknote/server-util";

const port = Number.parseInt(process.env.PORT ?? "3500", 10);
const editor = ServerBlockNoteEditor.create();

const app = express();
app.use(express.json({ limit: "10mb" }));

app.get("/healthz", (_req, res) => {
  res.type("text/plain").send("ok");
});

app.post("/md-to-blocks", async (req, res) => {
  try {
    const markdown = typeof req.body?.markdown === "string" ? req.body.markdown : "";
    const blocks = await editor.tryParseMarkdownToBlocks(markdown);
    res.json({ blocks });
  } catch (err) {
    res.status(400).json({ error: errorMessage(err) });
  }
});

app.post("/blocks-to-md", async (req, res) => {
  try {
    const blocks = Array.isArray(req.body?.blocks) ? req.body.blocks : null;
    if (blocks === null) {
      return res.status(400).json({ error: "blocks must be an array" });
    }
    const markdown = await editor.blocksToMarkdownLossy(blocks);
    res.json({ markdown });
  } catch (err) {
    res.status(400).json({ error: errorMessage(err) });
  }
});

app.post("/blocks-to-md-batch", async (req, res) => {
  try {
    const batches = Array.isArray(req.body?.blocks) ? req.body.blocks : null;
    if (batches === null) {
      return res.status(400).json({ error: "blocks must be an array of arrays" });
    }
    const markdowns = [];
    for (const blocks of batches) {
      if (!Array.isArray(blocks)) {
        return res.status(400).json({ error: "each element must be an array of blocks" });
      }
      markdowns.push(await editor.blocksToMarkdownLossy(blocks));
    }
    res.json({ markdowns });
  } catch (err) {
    res.status(400).json({ error: errorMessage(err) });
  }
});

function errorMessage(err) {
  if (err instanceof Error) return err.message;
  return String(err);
}

const server = app.listen(port, "0.0.0.0", () => {
  // Stable startup line — the Go side waits for /healthz, but a clean
  // ready log keeps the converter's stdout debuggable.
  console.log(`blocknote-converter listening on :${port}`);
});

function shutdown(signal) {
  console.log(`blocknote-converter received ${signal}, shutting down`);
  server.close(() => process.exit(0));
}

process.on("SIGINT", () => shutdown("SIGINT"));
process.on("SIGTERM", () => shutdown("SIGTERM"));
