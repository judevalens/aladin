import { stripMcpPrefix } from "./approvals.js";

// Pure translation: Claude Agent SDK messages → the NDJSON events the Go API
// consumes (and republishes to the dock as copilot.* realtime events).
//
// Event vocabulary (one JSON object per line; exactly one `done`, always last):
//   session          {sessionId, resumed}
//   token            {delta}
//   tool_start       {name, input}
//   tool_result      {name, toolUseId, content, isError, approvalId?}
//   proposed_action  {approvalId, tool, input}        (emitted by canUseTool)
//   approval_resolved{approvalId, approved, timedOut} (emitted by canUseTool)
//   message          {text}
//   done             {sessionId, numTurns, usage, costUsd}
//   error            {message}

// Tool results can be huge (a whole page, a build log); the Go side only needs
// them for citations + approval settlement, so cap hard.
const maxToolResultChars = 32_000;

// createTranslator returns a stateful per-turn translator: translate(msg) →
// array of NDJSON-ready event objects. State: the pending tool_use names (to
// label results), accumulated assistant text (fallback final message), and the
// session id.
export function createTranslator({ resumed = false, approvalByToolUse } = {}) {
  const toolNamesByUseId = new Map();
  let sessionId = "";
  let sawSession = false;
  let assistantText = "";

  return {
    sawSession: () => sawSession,

    translate(msg) {
      switch (msg.type) {
        case "system":
          if (msg.subtype === "init") {
            sawSession = true;
            sessionId = msg.session_id ?? "";
            const out = [{ type: "session", sessionId, resumed }];
            // Fail LOUD if a configured MCP server didn't connect: without its
            // tools the model play-acts tool calls as prose instead of erroring
            // (it has no way to know the tools are gone). fatal tells the agent
            // loop to abort the turn after this event is written.
            for (const server of msg.mcp_servers ?? []) {
              if (server.status !== "connected") {
                out.push({
                  type: "error",
                  fatal: true,
                  message:
                    `The copilot's tool server ("${server.name}" MCP, status: ${server.status}) is unreachable — ` +
                    `no tools can run. Check that the MCP server is up (make mcp) and reachable from the copilot-agent sidecar, then try again.`,
                });
              }
            }
            return out;
          }
          return [];

        case "stream_event": {
          // Only top-level assistant text becomes tokens; subagent/tool-nested
          // streams (parent_tool_use_id set) are not the user-facing answer.
          if (msg.parent_tool_use_id) return [];
          const event = msg.event;
          if (
            event?.type === "content_block_delta" &&
            event.delta?.type === "text_delta" &&
            event.delta.text
          ) {
            return [{ type: "token", delta: event.delta.text }];
          }
          // A thinking block starting = the model is reasoning: one lightweight
          // event (not the deltas — nothing needs the text) so the dock can show
          // "reasoning…" instead of dead air, and the stuck-turn watchdog knows
          // the turn is alive.
          if (
            event?.type === "content_block_start" &&
            event.content_block?.type === "thinking"
          ) {
            return [{ type: "thinking" }];
          }
          return [];
        }

        case "assistant": {
          if (msg.parent_tool_use_id) return [];
          const out = [];
          for (const block of msg.message?.content ?? []) {
            if (block.type === "tool_use") {
              toolNamesByUseId.set(block.id, stripMcpPrefix(block.name));
              out.push({
                type: "tool_start",
                name: stripMcpPrefix(block.name),
                input: block.input ?? {},
              });
            } else if (block.type === "text" && block.text) {
              // Keep the LAST assistant text as the fallback final answer
              // (the result message is authoritative when present).
              assistantText = block.text;
            }
          }
          return out;
        }

        case "user": {
          if (msg.parent_tool_use_id) return [];
          const out = [];
          const content = msg.message?.content;
          if (!Array.isArray(content)) return [];
          for (const block of content) {
            if (block.type !== "tool_result") continue;
            const ev = {
              type: "tool_result",
              name: toolNamesByUseId.get(block.tool_use_id) ?? "",
              toolUseId: block.tool_use_id ?? "",
              content: capText(extractResultText(block.content)),
              isError: block.is_error === true,
            };
            const approvalId = approvalByToolUse?.get(block.tool_use_id);
            if (approvalId) {
              ev.approvalId = approvalId;
              approvalByToolUse.delete(block.tool_use_id);
            }
            out.push(ev);
          }
          return out;
        }

        case "result": {
          const out = [];
          if (msg.subtype === "success" && !isApiErrorResult(msg)) {
            out.push({ type: "message", text: msg.result || assistantText });
          } else if (msg.subtype === "success") {
            // The SDK reports some API failures (4xx billing/validation) as
            // subtype=success with the raw "API Error: …" text as the result.
            // That is not an answer — surface it as an error so Go never
            // persists it as an assistant message. No assistantText fallback:
            // partial prose before a hard API failure isn't an answer either.
            out.push({
              type: "error",
              code: "api_error",
              message: firstErrorLine(msg.result ?? "The model API call failed."),
            });
          } else {
            const detail =
              Array.isArray(msg.errors) && msg.errors.length > 0
                ? msg.errors.join("; ")
                : msg.subtype;
            out.push({
              type: "error",
              code: errorCodeFor(msg.subtype),
              message: errorMessageFor(msg.subtype, detail),
            });
          }
          out.push({
            type: "done",
            sessionId: msg.session_id ?? sessionId,
            numTurns: msg.num_turns ?? 0,
            usage: {
              inputTokens: msg.usage?.input_tokens ?? 0,
              outputTokens: msg.usage?.output_tokens ?? 0,
            },
            costUsd: msg.total_cost_usd ?? 0,
          });
          return out;
        }

        default:
          return [];
      }
    },
  };
}

// isApiErrorResult spots the SDK's error-as-success shape: is_error set, or the
// result text literally starting with "API Error" (anchored — a genuine answer
// merely mentioning API errors mid-sentence stays a message).
function isApiErrorResult(msg) {
  if (msg.is_error === true) return true;
  return /^API Error:?\s/i.test(msg.result ?? "");
}

function firstErrorLine(s) {
  const i = s.indexOf("\n");
  const line = i >= 0 ? s.slice(0, i) : s;
  return line.length > 300 ? `${line.slice(0, 300)}…` : line;
}

// errorCodeFor maps SDK result subtypes to the compact machine codes the dock
// switches on (e.g. max_turns renders a one-click Continue).
function errorCodeFor(subtype) {
  switch (subtype) {
    case "error_max_turns":
      return "max_turns";
    case "error_max_budget_usd":
      return "max_budget";
    default:
      return "run_failed";
  }
}

function errorMessageFor(subtype, detail) {
  switch (subtype) {
    case "error_max_turns":
      // Recoverable: the next turn resumes this SDK session with full context.
      return "I hit the step limit mid-task — say \"continue\" and I'll pick up exactly where I left off, or narrow the request.";
    case "error_max_budget_usd":
      return "That request hit the per-turn budget limit — try narrowing the request.";
    default:
      return detail || "The agent run failed.";
  }
}

// tool_result content is either a plain string or an array of content blocks;
// keep the text parts only.
function extractResultText(content) {
  if (typeof content === "string") return content;
  if (!Array.isArray(content)) return "";
  return content
    .filter((b) => b?.type === "text" && typeof b.text === "string")
    .map((b) => b.text)
    .join("\n");
}

function capText(text) {
  if (text.length <= maxToolResultChars) return text;
  return `${text.slice(0, maxToolResultChars)}\n…[truncated]`;
}
