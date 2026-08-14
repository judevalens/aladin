// copilot-agent — the Aladin copilot's agent loop, as a sidecar.
//
// The Go API posts one turn at a time to /turn and republishes the NDJSON
// events to the dock over its realtime hub; the Claude Agent SDK does the
// actual agentic work, consuming tools from the Go MCP server (:8090) with the
// calling user's own bearer. Approvals and cancels arrive as separate HTTP
// calls keyed by turnId while the /turn response is still streaming.
//
// Auth: every non-healthz route requires X-Agent-Secret (shared with the Go
// API). ANTHROPIC_API_KEY is read by the SDK from this process's env — the Go
// side never touches it.

import express from "express";
import { config } from "./src/config.js";
import { createTurn, getTurn, endTurn } from "./src/turns.js";
import { resolveApproval } from "./src/approvals.js";
import { runTurn } from "./src/agent.js";
import { probeMcpCached } from "./src/health.js";

if (config.agentSharedSecret === "local-dev-agent-secret") {
  console.warn(
    `[warn] copilot-agent is using the default shared secret — set COPILOT_AGENT_SHARED_SECRET before exposing :${config.port} beyond localhost`,
  );
}
if (config.authMode === "subscription") {
  console.log(
    "[copilot-agent] COPILOT_AUTH=subscription — using the local Claude Code login (~/.claude); API key ignored",
  );
} else if (!process.env.ANTHROPIC_API_KEY) {
  console.warn(
    "[warn] ANTHROPIC_API_KEY is not set — turns will ride the local Claude Code login if one exists (set COPILOT_AUTH=subscription to make that intentional), and fail otherwise",
  );
}

const app = express();
app.use(express.json({ limit: config.jsonLimit }));

app.get("/healthz", async (_req, res) => {
  res.json({
    ok: true,
    authMode: config.authMode,
    anthropicKey: Boolean(process.env.ANTHROPIC_API_KEY),
    // Reported, not gating — the per-turn fatal-init guard is the hard stop.
    mcp: await probeMcpCached(config.mcpUrl),
  });
});

// Shared-secret gate for everything else. Empty configured secret ⇒ disabled
// entirely (fail closed), mirroring the blocknote admin bridge.
app.use((req, res, next) => {
  if (!config.agentSharedSecret) {
    res.status(503).json({ error: "copilot-agent is disabled: no shared secret configured" });
    return;
  }
  if (req.get("X-Agent-Secret") !== config.agentSharedSecret) {
    res.status(401).json({ error: "bad or missing X-Agent-Secret" });
    return;
  }
  next();
});

app.post("/turn", async (req, res) => {
  const body = req.body ?? {};
  for (const field of ["turnId", "prompt", "userBearer"]) {
    if (typeof body[field] !== "string" || body[field] === "") {
      res.status(400).json({ error: `${field} is required` });
      return;
    }
  }
  const turn = createTurn(body.turnId);
  if (!turn) {
    res.status(409).json({ error: "turn already running" });
    return;
  }

  res.setHeader("Content-Type", "application/x-ndjson");
  res.setHeader("Cache-Control", "no-cache");
  res.flushHeaders();

  const writeEvent = (ev) => {
    if (!res.writableEnded) {
      res.write(`${JSON.stringify(ev)}\n`);
    }
  };
  // The Go API dropping the stream (its ctx timed out, the process died)
  // cancels the turn.
  res.on("close", () => endTurn(body.turnId));

  try {
    await runTurn(body, turn, writeEvent);
  } finally {
    endTurn(body.turnId);
    res.end();
  }
});

app.post("/turn/:turnId/cancel", (req, res) => {
  endTurn(req.params.turnId); // idempotent — unknown turn is a no-op
  res.json({ ok: true });
});

app.post("/turn/:turnId/approvals/:approvalId", (req, res) => {
  const turn = getTurn(req.params.turnId);
  if (!turn) {
    res.status(404).json({ error: "unknown turn" });
    return;
  }
  const approved = req.body?.approved === true;
  if (!resolveApproval(turn, req.params.approvalId, approved)) {
    res.status(404).json({ error: "unknown or already-resolved approval" });
    return;
  }
  res.json({ ok: true });
});

app.listen(config.port, () => {
  console.log(`[copilot-agent] listening on :${config.port} (mcp: ${config.mcpUrl}, model: ${config.model}, effort: ${config.effort})`);
});
