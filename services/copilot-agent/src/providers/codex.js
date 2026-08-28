import { spawn } from "node:child_process";
import { randomUUID } from "node:crypto";
import { createInterface } from "node:readline";
import { config } from "../config.js";
import { endTurn } from "../turns.js";
import { createCodexTools } from "./codex-tools.js";

const maxToolResultChars = 32_000;
const baseInstructions = "You are the Aladin copilot, embedded in the user's Aladin workspace. " +
  "Follow the application's instructions and current-surface context. Use Aladin MCP tools for workspace and market data. " +
  "The runtime working directory is an implementation detail, not the user's project or current view. " +
  "Do not infer current prices, news, holdings, or capabilities without tool results.";

export const codexProvider = {
  id: "codex",
  label: "Codex",
  defaultModel: "gpt-5.6-terra",
  defaultEffort: "high",
  models: [
    {
      id: "gpt-5.6-terra",
      label: "GPT-5.6 Terra",
      description: "Balanced Codex harness for everyday workspace and coding turns.",
    },
    {
      id: "gpt-5.6-sol",
      label: "GPT-5.6 Sol",
      description: "Frontier Codex harness for the hardest agentic coding turns.",
    },
    {
      id: "gpt-5.6-luna",
      label: "GPT-5.6 Luna",
      description: "Fast Codex harness for lightweight agentic turns.",
    },
  ],
  efforts: [
    { id: "low", label: "Low", description: "Fastest responses with lighter reasoning." },
    { id: "medium", label: "Medium", description: "Balanced reasoning for routine work." },
    { id: "high", label: "High", description: "Deep reasoning for harder agentic tasks." },
    { id: "xhigh", label: "X-High", description: "Extra reasoning for complex turns." },
  ],
  capabilities: {
    resume: true,
    mcp: true,
    approvalHold: true,
    effort: true,
    appServer: true,
  },
  authStatus() {
    return {
      codexCli: Boolean(config.codexCommand),
      authMode: "codex",
    };
  },
  async runTurn(body, turn, writeEvent, deps = {}) {
    return runCodexTurn(body, turn, writeEvent, deps);
  },
};

async function runCodexTurn(body, turn, writeEvent, deps = {}) {
  const timeoutMs = deps.turnTimeoutMs ?? config.turnTimeoutMs;
  let doneWritten = false;
  const emit = (ev) => {
    if (doneWritten) return;
    if (ev.type === "done") {
      if (doneWritten) return;
      doneWritten = true;
    }
    writeEvent(ev);
  };

  const timeout = setTimeout(() => endTurn(turn.id), timeoutMs);
  let client;
  let toolBridge;
  const fallbackSessionID = body.resumeSessionId || randomUUID();
  let sessionID = fallbackSessionID;
  let resumed = false;
  let turnStarted = false;
  let finalText = "";

  const translator = createCodexTranslator({
    emit,
    onSession(id, didResume) {
      sessionID = id || sessionID;
      resumed = didResume;
      emit({ type: "session", sessionId: sessionID, resumed });
    },
    onTurnStarted() {
      turnStarted = true;
    },
    onFinalText(text) {
      finalText = text;
    },
  });

  try {
    const execute = async () => {
      toolBridge = await (deps.createTools ?? createCodexTools)(body, turn, emit, deps);
      if (turn.abortController.signal.aborted) {
        await toolBridge.close();
        turn.abortController.signal.throwIfAborted();
      }
      client = deps.client ?? createCodexClient(toolBridge, deps);
      await client.start(translator.handleMessage);
      // Explicitly disable inherited personal MCP servers without editing local config.
      const inherited = await client.request("config/read", { cwd: config.codexCwd });
      turn.abortController.signal.throwIfAborted();
      const threadConfig = codexThreadConfig(toolBridge, inherited?.config?.mcp_servers);
      const thread = await startOrResumeThread(client, body, threadConfig);
      turn.abortController.signal.throwIfAborted();
      await checkAladinTools(client, thread.sessionID);
      turn.abortController.signal.throwIfAborted();
      translator.markSession(thread.sessionID, thread.resumed);

      const prompt = thread.resumed || !body.historyFallback
        ? body.prompt
        : `Previous conversation (durable history):\n${body.historyFallback}\n\n---\n\n${body.prompt}`;

      // Resuming restores the original instruction history. Append current
      // application context as a developer message before each new user turn.
      await client.request("thread/inject_items", {
        threadId: thread.sessionID,
        items: [{ type: "message", role: "developer", content: [{ type: "input_text",
          text: "Current Aladin application context for this turn (supersedes earlier surface context):\n\n" +
            (body.systemPrompt || "You are the Aladin copilot."),
        }] }],
      });
      turn.abortController.signal.throwIfAborted();
      await client.request("turn/start", {
        threadId: thread.sessionID,
        input: [{ type: "text", text: prompt }],
        model: localModel(body.model) || config.codexModel || codexProvider.defaultModel,
        effort: normalizeReasoningEffort(body.effort || config.effort),
        approvalPolicy: "on-request",
        sandboxPolicy: { type: "readOnly" },
      });

      if (client.failed) {
        await Promise.race([
          translator.completed,
          client.failed.then((error) => { throw error; }),
        ]);
      } else {
        await translator.completed;
      }
    };
    await Promise.race([execute(), abortPromise(turn.abortController.signal)]);
    if (finalText) emit({ type: "message", text: finalText });
    emit({
      type: "done",
      sessionId: sessionID,
      numTurns: turnStarted ? 1 : 0,
      usage: { inputTokens: 0, outputTokens: 0 },
      costUsd: 0,
    });
  } catch (err) {
    if (!turn.abortController.signal.aborted && !doneWritten) {
      emit({ type: "error", message: firstLine(err?.message ?? String(err)) });
    }
  } finally {
    clearTimeout(timeout);
    await Promise.allSettled([closeClient(client), toolBridge?.close()]);
    emit({
      type: "done",
      sessionId: sessionID,
      numTurns: 0,
      usage: { inputTokens: 0, outputTokens: 0 },
      costUsd: 0,
    });
  }
}

function createCodexClient(toolBridge, deps) {
  const env = {
    ...process.env,
    ALADIN_COPILOT_MCP_TOKEN: toolBridge.token,
  };
  const child = (deps.spawnFn ?? spawn)(config.codexCommand, ["app-server"], {
    cwd: config.codexCwd,
    env,
    stdio: ["pipe", "pipe", "pipe"],
  });
  return new JsonRpcClient(child);
}

async function startOrResumeThread(client, body, threadConfig) {
  if (body.resumeSessionId) {
    try {
      const resumed = await client.request("thread/resume", {
        threadId: body.resumeSessionId,
        model: localModel(body.model) || config.codexModel || codexProvider.defaultModel,
        modelProvider: "openai",
        cwd: config.codexCwd,
        approvalPolicy: "on-request",
        sandbox: "read-only",
        baseInstructions,
        developerInstructions: body.systemPrompt || "You are the Aladin copilot.",
        config: threadConfig,
      });
      return { sessionID: resumed?.thread?.id || body.resumeSessionId, resumed: true };
    } catch (err) {
      if (!isMissingThreadError(err)) throw err;
    }
  }

  const started = await client.request("thread/start", {
    model: localModel(body.model) || config.codexModel || codexProvider.defaultModel,
    modelProvider: "openai",
    cwd: config.codexCwd,
    approvalPolicy: "on-request",
    sandbox: "read-only",
    baseInstructions,
    developerInstructions: body.systemPrompt || "You are the Aladin copilot.",
    serviceName: "Aladin Copilot",
    config: threadConfig,
  });
  return { sessionID: started?.thread?.id || randomUUID(), resumed: false };
}

function codexThreadConfig(toolBridge, inheritedServers = {}) {
  return {
    features: {
      shell_tool: false,
      unified_exec: false,
      shell_snapshot: false,
      apps: false,
      plugins: false,
      skill_search: false,
      multi_agent: false,
      browser_use: false,
      computer_use: false,
      image_generation: false,
      view_image: false,
      hooks: false,
      memories: false,
      code_mode: false,
      code_mode_host: false,
      retain_client_developer_messages: true,
    },
    project_doc_max_bytes: 0,
    web_search: "disabled",
    mcp_servers: {
      ...Object.fromEntries(Object.keys(inheritedServers).map((name) => [name, { enabled: false }])),
      aladin: {
        url: toolBridge.url,
        bearer_token_env_var: "ALADIN_COPILOT_MCP_TOKEN",
        enabled: true,
        required: true,
        tool_timeout_sec: Math.ceil(config.turnTimeoutMs / 1000),
        // Approval is enforced by the bridge, independently of model/runtime policy.
        tools: Object.fromEntries(toolBridge.tools.map(({ name }) => [name, { approval_mode: "approve" }])),
      },
    },
  };
}

async function checkAladinTools(client, threadId) {
  let cursor;
  let connected = false;
  do {
    const page = await client.request("mcpServerStatus/list", { threadId, cursor, limit: 100, detail: "toolsAndAuthOnly" });
    const aladin = page.data?.find((server) => server.name === "aladin");
    if (aladin && Object.keys(aladin.tools ?? {}).length > 0) connected = true;
    const unexpected = page.data?.find((server) => server.name !== "aladin" && Object.keys(server.tools ?? {}).length > 0);
    if (unexpected) {
      throw new Error(`Codex exposed a non-Aladin MCP server (${unexpected.name}, ${unexpected.runtimeStatus}). Refusing to run outside the copilot's tool scope.`);
    }
    cursor = page.nextCursor;
  } while (cursor);
  if (connected) return;
  throw new Error("The copilot's Aladin MCP tools did not connect to Codex. Check the MCP server and retry.");
}

function createCodexTranslator({
  emit,
  onSession,
  onTurnStarted,
  onFinalText,
}) {
  let completedResolve;
  const completed = new Promise((resolve) => {
    completedResolve = resolve;
  });
  const toolNamesByItem = new Map();
  const assistantMessages = new Map();
  let lastStreamedItem;
  let finalMessage;
  let sessionEmitted = false;

  const assistantMessage = (id, phase) => {
    const key = id ?? "assistant";
    if (!assistantMessages.has(key)) assistantMessages.set(key, { id: key, text: "", phase });
    const message = assistantMessages.get(key);
    if (phase) message.phase = phase;
    return message;
  };

  const appendText = (message, delta) => {
    if (!delta) return;
    // Each Codex item is its own Markdown document; token chunks within an
    // item must stay verbatim, but commentary and answers need a boundary.
    const separator = lastStreamedItem != null && lastStreamedItem !== message.id ? "\n\n" : "";
    emit({ type: "token", delta: separator + delta });
    lastStreamedItem = message.id;
    message.text += delta;
  };

  const updateFinalText = (message) => {
    if (message.phase === "final_answer") finalMessage = message;
    onFinalText(finalMessage?.text || message.text);
  };

  const markSession = (sessionID, resumed) => {
    if (sessionEmitted) return;
    sessionEmitted = true;
    onSession(sessionID, resumed);
  };

  const handleMessage = async (msg, client) => {
    if (Object.hasOwn(msg, "id") && msg.method) {
      await handleServerRequest(msg, client);
      return;
    }
    if (!msg.method) return;
    const params = msg.params ?? {};
    switch (msg.method) {
      case "turn/started":
        onTurnStarted();
        emit({ type: "thinking" });
        break;
      case "item/agentMessage/delta":
        if (params.delta) {
          const message = assistantMessage(params.itemId);
          appendText(message, params.delta);
          updateFinalText(message);
        }
        break;
      case "item/reasoning/textDelta":
      case "item/reasoning/summaryTextDelta":
      case "item/reasoning/summaryPartAdded":
        emit({ type: "thinking" });
        break;
      case "item/started":
        if (params.item?.type === "agentMessage") assistantMessage(params.item.id, params.item.phase);
        if (["reasoning", "contextCompaction", "plan"].includes(params.item?.type)) emit({ type: "thinking" });
        translateItemStarted(params.item, emit, toolNamesByItem);
        break;
      case "item/completed":
        translateItemCompleted(params.item, emit, toolNamesByItem);
        if (params.item?.type === "agentMessage" && params.item.text) {
          const message = assistantMessage(params.item.id, params.item.phase);
          // Some runtimes deliver only a completion, or a final suffix after
          // the last delta. Do not stream an already-delivered prefix twice.
          if (params.item.text.startsWith(message.text)) appendText(message, params.item.text.slice(message.text.length));
          message.text = params.item.text;
          updateFinalText(message);
        }
        break;
      case "turn/completed":
        if (params.turn?.status === "failed") {
          emit({ type: "error", message: firstLine(params.turn?.error?.message || "Codex turn failed.") });
        }
        completedResolve();
        break;
      case "error":
        if (params.willRetry) {
          emit({ type: "thinking" });
          break;
        }
        emit({ type: "error", message: firstLine(params.error?.message || params.message || "Codex app-server error.") });
        completedResolve();
        break;
      default:
        break;
    }
  };

  return { completed, handleMessage, markSession };
}

function translateItemStarted(item, emit, toolNamesByItem) {
  if (!item) return;
  if (item.type === "mcpToolCall") {
    const name = item.tool || "tool";
    toolNamesByItem.set(item.id, name);
    emit({ type: "tool_start", name, input: item.arguments ?? {} });
    return;
  }
  if (item.type === "dynamicToolCall") {
    const name = item.tool || "tool";
    toolNamesByItem.set(item.id, name);
    emit({ type: "tool_start", name, input: item.arguments ?? {} });
    return;
  }
  if (item.type === "commandExecution") {
    toolNamesByItem.set(item.id, "command");
    emit({ type: "tool_start", name: "command", input: { command: item.command, cwd: item.cwd } });
    return;
  }
  if (item.type === "fileChange") {
    toolNamesByItem.set(item.id, "file_change");
    emit({ type: "tool_start", name: "file_change", input: { changes: item.changes ?? [] } });
  }
}

function translateItemCompleted(item, emit, toolNamesByItem) {
  if (!item) return;
  if (item.type === "mcpToolCall") {
    emit({
      type: "tool_result",
      name: item.tool || toolNamesByItem.get(item.id) || "tool",
      toolUseId: item.id,
      approvalId: item.result?._meta?.["aladin/approvalId"],
      content: capText(resultText(item.result ?? item.error)),
      isError: Boolean(item.error) || item.status === "failed",
    });
    return;
  }
  if (item.type === "dynamicToolCall") {
    emit({
      type: "tool_result",
      name: item.tool || toolNamesByItem.get(item.id) || "tool",
      toolUseId: item.id,
      content: capText(resultText(item.contentItems)),
      isError: item.status === "failed" || item.success === false,
    });
    return;
  }
  if (item.type === "commandExecution") {
    emit({
      type: "tool_result",
      name: "command",
      toolUseId: item.id,
      content: capText(item.aggregatedOutput ?? ""),
      isError: item.status === "failed" || (item.exitCode != null && item.exitCode !== 0),
    });
    return;
  }
  if (item.type === "fileChange") {
    emit({
      type: "tool_result",
      name: "file_change",
      toolUseId: item.id,
      content: `${item.status || "completed"} ${Array.isArray(item.changes) ? item.changes.length : 0} file change(s)`,
      isError: item.status === "failed" || item.status === "declined",
    });
  }
}

async function handleServerRequest(msg, client) {
  switch (msg.method) {
    case "item/commandExecution/requestApproval":
    case "item/fileChange/requestApproval":
      // Workspace writes go through Aladin tools and their approval gate only.
      client.respond(msg.id, { decision: "decline" });
      return;
    case "item/permissions/requestApproval":
      client.respond(msg.id, { permissions: {}, scope: "turn" });
      return;
    case "mcpServer/elicitation/request":
      client.respond(msg.id, {
        action: "decline",
        content: null,
        _meta: null,
      });
      return;
    case "item/tool/requestUserInput":
      client.respond(msg.id, {
        answers: [],
      });
      return;
    case "item/tool/call":
      client.respond(msg.id, {
        contentItems: [{ type: "inputText", text: "Dynamic client tools are not enabled in Aladin copilot." }],
        success: false,
      });
      return;
    default:
      client.error(msg.id, -32601, `Unsupported Codex app-server request: ${msg.method}`);
  }
}

class JsonRpcClient {
  constructor(child) {
    this.child = child;
    this.nextID = 1;
    this.pending = new Map();
    this.messageHandler = async () => {};
    this.exited = false;
    this.failure = null;
    // Resolves with an error, so an early process exit cannot produce an
    // unhandled rejection before the turn starts awaiting completion.
    this.failed = new Promise((resolve) => { this.resolveFailure = resolve; });
  }

  async start(onMessage) {
    this.messageHandler = onMessage;
    const rl = createInterface({ input: this.child.stdout });
    rl.on("line", (line) => this.onLine(line));
    rl.on("error", (error) => this.fail(error));
    rl.once("close", () => this.fail(new Error("Codex app-server output stream closed.")));
    this.child.stderr?.on("data", (chunk) => {
      const text = String(chunk).trim();
      if (text) console.warn(`[codex app-server] ${text}`);
    });
    this.child.on("exit", (code, signal) => {
      this.exited = true;
      const status = signal || (code ?? "unknown");
      this.fail(new Error(`Codex app-server exited (${status})`));
    });
    const fail = (err) => this.fail(err);
    this.child.on("error", fail);
    this.child.stdin.on("error", fail);
    this.child.stdout.on("error", fail);
    await this.request("initialize", {
      clientInfo: { name: "aladin-copilot", title: "Aladin Copilot", version: "0.1.0" },
      capabilities: { experimentalApi: true, requestAttestation: false },
    });
    this.notify("initialized", {});
  }

  request(method, params = {}) {
    const id = this.nextID++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      try {
        this.send({ method, id, params });
      } catch (err) {
        this.pending.delete(id);
        reject(err);
      }
    });
  }

  notify(method, params = {}) {
    this.send({ method, params });
  }

  respond(id, result) {
    this.send({ id, result });
  }

  error(id, code, message) {
    this.send({ id, error: { code, message } });
  }

  close() {
    if (this.exited) return;
    this.child.kill("SIGTERM");
  }

  send(msg) {
    if (this.failure) throw this.failure;
    this.child.stdin.write(`${JSON.stringify(msg)}\n`);
  }

  fail(error) {
    if (this.failure) return;
    this.failure = error;
    this.resolveFailure(error);
    for (const { reject } of this.pending.values()) reject(error);
    this.pending.clear();
  }

  onLine(line) {
    let msg;
    try {
      msg = JSON.parse(line);
    } catch {
      return;
    }
    if (Object.hasOwn(msg, "id") && !msg.method) {
      const pending = this.pending.get(msg.id);
      if (!pending) return;
      this.pending.delete(msg.id);
      if (msg.error) {
        pending.reject(new Error(msg.error.message || `Codex app-server error ${msg.error.code}`));
      } else {
        pending.resolve(msg.result);
      }
      return;
    }
    Promise.resolve(this.messageHandler(msg, this)).catch((err) => {
      console.warn(`[codex app-server] handler failed: ${err?.message ?? err}`);
      this.fail(err);
    });
  }
}

async function closeClient(client) {
  if (!client) return;
  if (typeof client.close === "function") {
    await client.close();
  }
}

function abortPromise(signal) {
  if (signal.aborted) return Promise.reject(new Error("turn aborted"));
  return new Promise((_, reject) => {
    signal.addEventListener("abort", () => reject(new Error("turn aborted")), { once: true });
  });
}

function localModel(model) {
  if (typeof model !== "string") return "";
  const trimmed = model.trim();
  const prefix = "codex:";
  return trimmed.startsWith(prefix) ? trimmed.slice(prefix.length) : trimmed;
}

function normalizeReasoningEffort(effort) {
  switch (effort) {
    case "low":
    case "medium":
    case "high":
    case "xhigh":
      return effort;
    case "max":
    case "ultra":
      return "xhigh";
    default:
      return codexProvider.defaultEffort;
  }
}

function isMissingThreadError(err) {
  return /not found|unknown|missing/i.test(err?.message ?? "");
}

function resultText(value) {
  if (typeof value === "string") return value;
  if (Array.isArray(value)) return value.map(resultText).filter(Boolean).join("\n");
  if (value && typeof value === "object") {
    if (typeof value.text === "string") return value.text;
    if (Array.isArray(value.content)) return resultText(value.content);
    if (value.structuredContent != null) return JSON.stringify(value.structuredContent);
    if (value.message) return resultText(value.message);
    if (value.error) return resultText(value.error);
  }
  return value == null ? "" : JSON.stringify(value);
}

function capText(text) {
  text = String(text ?? "");
  if (text.length <= maxToolResultChars) return text;
  return `${text.slice(0, maxToolResultChars)}\n...[truncated]`;
}

function firstLine(s) {
  const i = s.indexOf("\n");
  const line = i >= 0 ? s.slice(0, i) : s;
  return line.length > 300 ? `${line.slice(0, 300)}...` : line;
}
