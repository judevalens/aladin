import { query } from "@anthropic-ai/claude-agent-sdk";
import { config } from "../config.js";
import { makeCanUseTool } from "../approvals.js";
import { createTranslator } from "../translate.js";

export const claudeProvider = {
  id: "claude",
  label: "Claude",
  defaultModel: "claude-opus-5",
  defaultEffort: "high",
  models: [
    {
      id: "claude-opus-5",
      label: "Opus 5",
      description: "Best reasoning for shard authoring and hard workspace tasks.",
    },
    {
      id: "claude-sonnet-5",
      label: "Sonnet 5",
      description: "Fast everyday coding and research assistant work.",
    },
    {
      id: "claude-fable-5",
      label: "Fable 5",
      description: "Quick lightweight answers when speed matters most.",
    },
  ],
  efforts: [
    { id: "low", label: "Low", description: "Fastest responses with minimal thinking." },
    { id: "medium", label: "Medium", description: "Balanced reasoning for routine work." },
    { id: "high", label: "High", description: "Deep reasoning; Claude Code default." },
    { id: "xhigh", label: "X-High", description: "Deeper reasoning for harder agentic tasks." },
    { id: "max", label: "Max", description: "Maximum effort for the hardest long-running tasks." },
  ],
  capabilities: {
    resume: true,
    mcp: true,
    approvalHold: true,
    effort: true,
  },
  authStatus() {
    return {
      authMode: config.authMode,
      anthropicKey: Boolean(process.env.ANTHROPIC_API_KEY),
    };
  },
  async runTurn(body, turn, writeEvent, deps = {}) {
    return runClaudeTurn(body, turn, writeEvent, deps);
  },
};

// runClaudeTurn drives ONE copilot turn: a Claude Agent SDK query() consuming tools
// from the Go MCP server, translated into NDJSON events on the response
// stream. Holds gated tools open via canUseTool until the Go API relays the
// user's approve/reject.
//
// Resume semantics: `resumeSessionId` resumes the thread's prior SDK session
// (full tool-call continuity). If the session is gone (sidecar restarted,
// volume dropped) the turn retries ONCE from scratch, prepending the caller's
// text-history fallback — degraded but correct; the durable conversation lives
// in the Go API's Postgres either way.
async function runClaudeTurn(body, turn, writeEvent, deps = {}) {
  const queryFn = deps.queryFn ?? query;
  const timeoutMs = deps.turnTimeoutMs ?? config.turnTimeoutMs;

  // The stream contract is "exactly one done, always last": swallow any
  // duplicate, and guarantee a trailing done on every exit path.
  let doneWritten = false;
  const emit = (ev) => {
    if (ev.type === "done") {
      if (doneWritten) return;
      doneWritten = true;
    }
    writeEvent(ev);
  };

  const timeout = setTimeout(() => turn.abortController.abort(), timeoutMs);
  try {
    const attempt = (resume) =>
      runQuery({
        queryFn,
        body,
        turn,
        writeEvent: emit,
        resume,
        approvalTimeoutMs: deps.approvalTimeoutMs ?? config.approvalTimeoutMs,
      });

    const resume = body.resumeSessionId || undefined;
    try {
      await attempt(resume);
    } catch (err) {
      // A resume attempt that died before the session initialized most likely
      // means the session file is gone — fall back to a fresh session with the
      // text-history preamble. Any other failure is a real error.
      if (resume && !err.sawSession && !turn.abortController.signal.aborted && !doneWritten) {
        await attempt(undefined);
      } else {
        throw err;
      }
    }
  } catch (err) {
    if (!turn.abortController.signal.aborted && !doneWritten) {
      emit({ type: "error", message: firstLine(err?.message ?? String(err)) });
    }
  } finally {
    clearTimeout(timeout);
    emit({
      type: "done",
      sessionId: "",
      numTurns: 0,
      usage: { inputTokens: 0, outputTokens: 0 },
      costUsd: 0,
    });
  }
}

async function runQuery({ queryFn, body, turn, writeEvent, resume, approvalTimeoutMs }) {
  const translator = createTranslator({
    resumed: Boolean(resume),
    approvalByToolUse: turn.approvalByToolUse,
  });

  // Without a session to resume, prepend the durable text history so a fresh
  // SDK session still knows the conversation so far.
  let prompt = body.prompt;
  if (!resume && body.historyFallback) {
    prompt = `Previous conversation (durable history):\n${body.historyFallback}\n\n---\n\n${body.prompt}`;
  }

  const q = queryFn({
    prompt,
    options: {
      systemPrompt: body.systemPrompt,
      model: localModel(body.model) || config.model,
      effort: body.effort || config.effort,
      maxTurns: body.maxTurns || config.maxTurns,
      resume,
      includePartialMessages: true,
      // No built-in tools (Read/Write/Bash/…) — the agent's whole tool surface
      // is the Aladin MCP server, reached with the caller's own bearer.
      tools: [],
      mcpServers: {
        aladin: {
          type: "http",
          url: body.mcpUrl || config.mcpUrl,
          headers: { Authorization: `Bearer ${body.userBearer}` },
        },
      },
      // Use ONLY the aladin server above. Without this, COPILOT_AUTH=subscription
      // inherits the user's local ~/.claude MCP config (personal Drive/etc.
      // servers), which pollutes the tool surface and — in needs-auth state —
      // would trip the init guard and kill the turn.
      strictMcpConfig: true,
      // Every MCP tool call routes through canUseTool (nothing is pre-allowed):
      // non-gated tools pass straight through; gated ones hold for approval.
      canUseTool: makeCanUseTool({
        turn,
        gatedTools: body.gatedTools,
        emit: writeEvent,
        timeoutMs: approvalTimeoutMs,
      }),
      abortController: turn.abortController,
    },
  });

  try {
    for await (const msg of q) {
      for (const ev of translator.translate(msg)) {
        writeEvent(ev);
        if (ev.fatal) {
          // Unrecoverable turn condition (e.g. the MCP tool server never
          // connected) — the error is already on the stream; stop the query
          // instead of letting the model improvise without tools.
          turn.abortController.abort();
          return;
        }
      }
    }
  } catch (err) {
    err.sawSession = translator.sawSession();
    throw err;
  }
  if (!translator.sawSession()) {
    // The generator ended without ever initializing (e.g. resume of a missing
    // session exits cleanly) — treat like a failed attempt so the caller can
    // fall back to a fresh session.
    const err = new Error("agent run ended before the session initialized");
    err.sawSession = false;
    throw err;
  }
}

function localModel(model) {
  if (typeof model !== "string") return "";
  const trimmed = model.trim();
  const prefix = "claude:";
  return trimmed.startsWith(prefix) ? trimmed.slice(prefix.length) : trimmed;
}

function firstLine(s) {
  const i = s.indexOf("\n");
  const line = i >= 0 ? s.slice(0, i) : s;
  return line.length > 300 ? `${line.slice(0, 300)}…` : line;
}
