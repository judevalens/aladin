import {
  Agent,
  MCPServerStreamableHttp,
  MemorySession,
  getAllMcpTools,
  run,
} from "@openai/agents";
import { randomUUID } from "node:crypto";
import { config } from "../config.js";
import { createApproval } from "../approvals.js";

const sessions = new Map();

export const openaiProvider = {
  id: "openai",
  label: "OpenAI",
  defaultModel: "gpt-5.1",
  defaultEffort: "high",
  models: [
    {
      id: "gpt-5.1",
      label: "GPT-5.1",
      description: "General-purpose OpenAI agent for workspace tasks.",
    },
    {
      id: "gpt-5.1-codex",
      label: "GPT-5.1 Codex",
      description: "OpenAI coding-oriented agent model for implementation-heavy turns.",
    },
    {
      id: "gpt-5-mini",
      label: "GPT-5 Mini",
      description: "Faster OpenAI agent model for lightweight turns.",
    },
  ],
  efforts: [
    { id: "low", label: "Low", description: "Fastest responses with minimal thinking." },
    { id: "medium", label: "Medium", description: "Balanced reasoning for routine work." },
    { id: "high", label: "High", description: "Deep reasoning for harder agentic tasks." },
    { id: "xhigh", label: "X-High", description: "Deeper reasoning for complex turns." },
    { id: "max", label: "Max", description: "Maximum effort for the hardest long-running tasks." },
  ],
  capabilities: {
    resume: true,
    mcp: true,
    approvalHold: true,
    effort: true,
  },
  authStatus() {
    return { openaiKey: Boolean(process.env.OPENAI_API_KEY) };
  },
  async runTurn(body, turn, writeEvent, deps = {}) {
    return runOpenAITurn(body, turn, writeEvent, deps);
  },
};

async function runOpenAITurn(body, turn, writeEvent, deps = {}) {
  const timeoutMs = deps.turnTimeoutMs ?? config.turnTimeoutMs;
  let doneWritten = false;
  const emit = (ev) => {
    if (ev.type === "done") {
      if (doneWritten) return;
      doneWritten = true;
    }
    writeEvent(ev);
  };

  const timeout = setTimeout(() => turn.abortController.abort(), timeoutMs);
  const sessionID = sessionIDFor(body);
  const { session, resumed } = sessionFor(sessionID, body.resumeSessionId);
  emit({ type: "session", sessionId: sessionID, resumed });

  let closeTools = async () => {};
  try {
    const toolSetup = await openAITools(body, deps);
    const tools = toolSetup.tools;
    closeTools = toolSetup.close;
    const agent = new (deps.AgentClass ?? Agent)({
      name: "Aladin Copilot",
      instructions: body.systemPrompt || "You are the Aladin copilot.",
      model: localModel(body.model) || config.openAIModel || openaiProvider.defaultModel,
      tools,
      mcpServers: [],
      modelSettings: modelSettings(body.effort || config.effort),
    });

    let input = resumed ? body.prompt : promptWithFallback(body);
    let final = "";
    let turns = 0;
    for (;;) {
      const stream = await (deps.runFn ?? run)(agent, input, {
        stream: true,
        maxTurns: body.maxTurns || config.maxTurns,
        signal: turn.abortController.signal,
        session,
      });
      turns += stream.currentTurn ?? 0;
      for await (const event of stream) {
        const text = translateOpenAIEvent(event, emit);
        if (text) final = text;
      }
      await stream.completed;
      if (stream.error) throw stream.error;
      if (!stream.interruptions?.length) {
        final = String(stream.finalOutput ?? final ?? "");
        if (final) emit({ type: "message", text: final });
        emit({
          type: "done",
          sessionId: sessionID,
          numTurns: turns,
          usage: { inputTokens: 0, outputTokens: 0 },
          costUsd: 0,
        });
        return;
      }

      const state = stream.state;
      for (const interruption of stream.interruptions) {
        await resolveOpenAIApproval(turn, interruption, state, emit, deps.approvalTimeoutMs ?? config.approvalTimeoutMs);
      }
      input = state;
    }
  } catch (err) {
    if (!turn.abortController.signal.aborted && !doneWritten) {
      emit({ type: "error", message: firstLine(err?.message ?? String(err)) });
    }
  } finally {
    try {
      await closeTools();
    } catch {
      // best effort
    }
    clearTimeout(timeout);
    emit({
      type: "done",
      sessionId: sessionID,
      numTurns: 0,
      usage: { inputTokens: 0, outputTokens: 0 },
      costUsd: 0,
    });
  }
}

async function openAITools(body, deps) {
  if (deps.tools) return { tools: deps.tools, close: async () => {} };
  const server = deps.mcpServer ?? new MCPServerStreamableHttp({
    name: "aladin",
    url: body.mcpUrl || config.mcpUrl,
    requestInit: {
      headers: { Authorization: `Bearer ${body.userBearer}` },
    },
  });
  await server.connect();
  const tools = await (deps.getToolsFn ?? getAllMcpTools)({
    mcpServers: [server],
    includeServerInToolNames: false,
  });
  const gated = new Set(body.gatedTools ?? []);
  for (const tool of tools) {
    if (gated.has(tool.name)) {
      tool.needsApproval = true;
    }
  }
  return {
    tools,
    close: async () => {
      await server.close();
    },
  };
}

async function resolveOpenAIApproval(turn, interruption, state, emit, timeoutMs) {
  const name = interruption.name || interruption.toolName || interruption.rawItem?.name || "tool";
  const input = parseArgs(interruption.arguments ?? interruption.rawItem?.arguments);
  const { approvalId, promise } = createApproval(turn, timeoutMs);
  emit({ type: "proposed_action", approvalId, tool: name, input });
  const { approved, timedOut, aborted } = await promise;
  if (aborted) {
    state.reject?.(interruption, { message: "The turn was cancelled." });
    return;
  }
  emit({ type: "approval_resolved", approvalId, approved, timedOut });
  if (approved) {
    state.approve(interruption);
  } else {
    state.reject(interruption, {
      message: timedOut
        ? "The user did not respond to the approval request in time. Tell them it expired; they can ask again."
        : "The user declined this action. Do not retry it; tell the user it was dismissed.",
    });
  }
}

function translateOpenAIEvent(event, emit) {
  if (event?.type === "raw_model_stream_event") {
    const data = event.data ?? {};
    if (typeof data.delta === "string" && data.type?.includes("output_text")) {
      emit({ type: "token", delta: data.delta });
    }
    if (data.type?.includes("reasoning")) {
      emit({ type: "thinking" });
    }
    return "";
  }
  if (event?.type !== "run_item_stream_event") return "";
  const item = event.item ?? {};
  if (event.name === "tool_called") {
    emit({
      type: "tool_start",
      name: item.name || item.rawItem?.name || "tool",
      input: rawJSON(item.arguments ?? item.rawItem?.arguments),
    });
  }
  if (event.name === "tool_output") {
    emit({
      type: "tool_result",
      name: item.name || item.rawItem?.name || "tool",
      content: outputText(item.output ?? item.rawItem?.output ?? item.rawItem),
      isError: false,
    });
  }
  if (event.name === "reasoning_item_created") {
    emit({ type: "thinking" });
  }
  if (event.name === "message_output_created") {
    return outputText(item.text ?? item.rawItem?.content ?? item.rawItem);
  }
  return "";
}

function sessionIDFor(body) {
  if (body.resumeSessionId && sessions.has(body.resumeSessionId)) return body.resumeSessionId;
  return randomUUID();
}

function sessionFor(sessionID, resumeSessionID) {
  if (resumeSessionID && sessions.has(resumeSessionID)) {
    return { session: sessions.get(resumeSessionID), resumed: true };
  }
  const session = new MemorySession();
  sessions.set(sessionID, session);
  return { session, resumed: false };
}

function promptWithFallback(body) {
  if (!body.historyFallback) return body.prompt;
  return `Previous conversation (durable history):\n${body.historyFallback}\n\n---\n\n${body.prompt}`;
}

function localModel(model) {
  if (typeof model !== "string") return "";
  const trimmed = model.trim();
  const prefix = "openai:";
  return trimmed.startsWith(prefix) ? trimmed.slice(prefix.length) : trimmed;
}

function modelSettings(effort) {
  const reasoning = normalizeReasoningEffort(effort);
  return reasoning ? { reasoning: { effort: reasoning } } : {};
}

function normalizeReasoningEffort(effort) {
  switch (effort) {
    case "low":
    case "medium":
    case "high":
    case "xhigh":
    case "max":
      return effort;
    default:
      return "";
  }
}

function parseArgs(raw) {
  if (!raw) return {};
  if (typeof raw === "object") return raw;
  try {
    return JSON.parse(raw);
  } catch {
    return { value: String(raw) };
  }
}

function rawJSON(raw) {
  if (raw instanceof Uint8Array) return raw;
  if (raw == null) return undefined;
  if (typeof raw === "object") return raw;
  try {
    return JSON.parse(raw);
  } catch {
    return { value: String(raw) };
  }
}

function outputText(output) {
  if (typeof output === "string") return output;
  if (Array.isArray(output)) return output.map(outputText).filter(Boolean).join("\n");
  if (output && typeof output === "object") {
    if (typeof output.text === "string") return output.text;
    if (typeof output.output === "string") return output.output;
    if (typeof output.content === "string") return output.content;
    if (Array.isArray(output.content)) return outputText(output.content);
  }
  return output == null ? "" : JSON.stringify(output);
}

function firstLine(s) {
  const i = s.indexOf("\n");
  const line = i >= 0 ? s.slice(0, i) : s;
  return line.length > 300 ? `${line.slice(0, 300)}…` : line;
}
