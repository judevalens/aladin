# Copilot Client-Side Agent ("BYO Claude") — design proposal

Status: **PROPOSED** (2026-07-22) — not scheduled. Written down so the current
architecture keeps the door open; build when Aladin has users beyond the author.

## Idea

Move the copilot's agent loop from the server into the **desktop app**, so each
user's dock is powered by **their own Claude Code login / Anthropic
subscription** on their own machine. The Aladin server stops paying for and
running inference entirely — it serves tools (MCP) and persists threads;
"bring your own agent."

## Why

- **Cost**: inference is the copilot's only marginal cost. Client-side, each
  user's subscription absorbs it; the server bill is zero regardless of usage.
- **Privacy**: conversations and tool traffic run between the user's machine
  and their own Anthropic account; the server sees only tool calls it already
  authorizes and the final persisted turns.
- **Capability**: the user's local runtime is whatever Claude Code they run —
  model upgrades arrive with their subscription, not our deploys.
- **Fit**: Aladin's likely early adopters (trader-developers) are
  disproportionately already Claude Code users.

## Current architecture (server agent — shipped 2026-07)

```
dock ──POST /api/copilot/message──▶ Go API (orchestrator)
                                       │ builds systemPrompt + surface context,
                                       │ resolves sdk_session_id, gatedTools
                                       ▼
                              copilot-agent sidecar (server, :3550)
                                       │ Claude Agent SDK query(), per-turn
                                       ▼
                              Go MCP server (:8090) ◀─ user's bearer
                                       │
dock ◀── copilot.* events ── realtime hub ◀── Go API ◀── NDJSON stream
```

Approvals: dock → REST → Go → sidecar `canUseTool` hold. Sessions: SDK
transcripts on the **server** sidecar's disk; `sdk_session_id` per thread in
Postgres (mig 00028).

## Proposed architecture (local agent mode)

```
dock ──GET /api/copilot/prepare-turn──▶ Go API      (1) server builds context
dock ──POST /turn (NDJSON)───▶ LOCAL copilot-agent  (2) user's machine, user's
                                       │                Claude login
                                       ▼
                     server MCP endpoint (/mcp) ◀── user's bearer  (unchanged)
dock ◀── NDJSON stream directly (no hub hop)
dock ──POST /api/copilot/complete-turn──▶ Go API    (3) persist message,
                                                        citations, session id
```

### What already lines up (built, no rework needed)

- **MCP server is remote-capable**: streamable HTTP + per-user bearer auth
  (`service.ResolveBearerPrincipal`). A laptop sidecar pointing at
  `https://<server>/mcp` is the same code path as today — `agent.js` already
  accepts a per-turn `mcpUrl`.
- **The sidecar is portable by construction**: self-contained Node, no DB, no
  server secrets, clean NDJSON contract. Tauri supports bundled sidecar
  binaries — ship it with the app, or detect the user's installed Claude Code
  (the `COPILOT_AUTH=subscription` mode is the same auth mechanism).
- **Sessions belong client-side anyway**: SDK transcripts on the user's disk
  gain durability (no container volume concerns) and privacy.
- **Some plumbing disappears**: approvals need no REST relay (the dock answers
  `canUseTool` directly over localhost); token streaming needs no realtime-hub
  hop.

### What needs building

1. **Split the Go orchestrator into two endpoints** (the only real work):
   - `POST /api/copilot/prepare-turn` → `{systemPrompt (incl. surface
     context), resumeSessionId, historyFallback, gatedTools, model}`.
     Prompt logic stays server-side and versioned — clients never fork it.
   - `POST /api/copilot/complete-turn` → persists the final assistant message
     + citations + new `sdk_session_id`; threads stay canonical in Postgres
     and sync across devices/modes.
2. **Dock agent-location setting**: `server` (today's path — the floor for
   users without Claude Code) vs `local`. Local mode drives the turn itself:
   prepare → local sidecar stream → complete.
3. **Tauri packaging**: bundle copilot-agent as a sidecar binary (or a
   "use my Claude Code" detection path), lifecycle managed by the app.
4. **Failure UX**: local sidecar missing/not logged in → clear error with a
   one-click fallback to server mode.

### Security notes

- The approval gate becomes **UX, not enforcement**, in local mode — the
  client already holds the user's bearer and could call MCP tools directly
  regardless. True enforcement is (and remains) the MCP server's auth +
  principal scoping. Server-side gating of genuinely dangerous ops, if ever
  needed, belongs in the MCP tool layer, not the agent.
- `X-Agent-Secret` is meaningless client-side (both halves run on the user's
  machine); local mode can drop it or bind the sidecar to localhost only.
- The server must treat `complete-turn` bodies as untrusted client input
  (validate thread ownership, cap sizes) — same posture as any client write.

### Mode comparison

| | Server agent (today) | Local agent (proposed) |
|---|---|---|
| Inference cost | server's API key | user's subscription/key |
| Works without Claude Code | yes | no (falls back to server) |
| Sessions live | server volume | user's disk |
| Approval relay | REST → sidecar hold | in-process, direct |
| Event path | sidecar → Go → hub → WS | sidecar → dock direct |
| Prompt/policy source | Go (per turn) | Go (prepare-turn) — unchanged |

## Non-goals

- Replacing server mode. It stays as the default floor.
- Moving tool logic client-side. The MCP server remains the single tool
  surface (see also the OpenClaw bridge — every consumer shares it).
- A literal interactive-TUI loop. The local agent is the same headless SDK
  runtime; interactive Claude Code remains a separate, complementary way to
  drive the same MCP tools.

## Related

- `~/.claude/plans/copilot-agent-sdk-sidecar.md` — the server-agent migration
  this builds on (SDK sidecar, MCP tool surface, NDJSON contract).
- `COPILOT_AUTH=subscription` (services/copilot-agent) — the smaller shipped
  win: the *server* sidecar riding the local Claude login on a dev machine.
