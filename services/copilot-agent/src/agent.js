import { query } from "@anthropic-ai/claude-agent-sdk";
import { config } from "./config.js";
import { makeCanUseTool } from "./approvals.js";
import { createTranslator } from "./translate.js";

// runTurn drives ONE copilot turn: a Claude Agent SDK query() consuming tools
// from the Go MCP server, translated into NDJSON events on the response
// stream. Holds gated tools open via canUseTool until the Go API relays the
// user's approve/reject.
//
// Resume semantics: `resumeSessionId` resumes the thread's prior SDK session
// (full tool-call continuity). If the session is gone (sidecar restarted,
// volume dropped) the turn retries ONCE from scratch, prepending the caller's
// text-history fallback — degraded but correct; the durable conversation lives
// in the Go API's Postgres either way.
export async function runTurn(body, turn, writeEvent, deps = {}) {
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
      model: body.model || config.model,
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

function firstLine(s) {
  const i = s.indexOf("\n");
  const line = i >= 0 ? s.slice(0, i) : s;
  return line.length > 300 ? `${line.slice(0, 300)}…` : line;
}
