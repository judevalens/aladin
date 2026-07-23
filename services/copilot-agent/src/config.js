// Central env parsing (mirrors services/blocknote/src/config.js).

// COPILOT_AUTH picks how the SDK authenticates:
//   "api"          (default) — ANTHROPIC_API_KEY from env (required in containers)
//   "subscription"           — force the local Claude Code login (~/.claude);
//                              the API key is stripped from this process's env so
//                              the SDK can't silently prefer it. Personal/dev use
//                              on a logged-in machine only.
const authMode = process.env.COPILOT_AUTH === "subscription" ? "subscription" : "api";
if (authMode === "subscription") {
  delete process.env.ANTHROPIC_API_KEY;
}

export const config = {
  authMode,
  port: Number.parseInt(process.env.PORT ?? "3550", 10),
  jsonLimit: process.env.JSON_LIMIT ?? "5mb",

  // Shared secret guarding every non-healthz route. Defaults to a known
  // local-dev value so `make backend` works out of the box; set
  // COPILOT_AGENT_SHARED_SECRET in Docker/prod. Empty string disables the
  // service (fail closed).
  agentSharedSecret:
    process.env.COPILOT_AGENT_SHARED_SECRET ?? "local-dev-agent-secret",

  // The Go MCP server the agent consumes its tools from. The per-turn user
  // bearer rides in the request body, not here.
  mcpUrl: process.env.ALADIN_MCP_URL ?? "http://localhost:8090/mcp",

  // Model + loop bounds. COPILOT_MODEL mirrors the Go-side default so either
  // process can pin it.
  model: process.env.COPILOT_MODEL ?? "claude-sonnet-5",
  // Fallback only — the Go API passes maxTurns per turn (keep in sync with
  // maxCopilotTurns in internal/service/copilot.go). Sized for deep shard
  // authoring; hitting it is recoverable ("continue" resumes the session).
  maxTurns: Number.parseInt(process.env.MAX_TURNS ?? "60", 10),

  // A turn's hard deadline must contain one full approval hold (below), plus
  // real agent work either side of it.
  turnTimeoutMs: Number.parseInt(
    process.env.TURN_TIMEOUT_MS ?? String(15 * 60 * 1000),
    10,
  ),
  // How long canUseTool holds a gated tool open waiting for the user's
  // approve/reject before denying as timed out.
  approvalTimeoutMs: Number.parseInt(
    process.env.APPROVAL_TIMEOUT_MS ?? String(10 * 60 * 1000),
    10,
  ),
};
