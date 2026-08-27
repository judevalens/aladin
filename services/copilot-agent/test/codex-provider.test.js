import test from "node:test";
import assert from "node:assert/strict";
import { createTurn, endTurn } from "../src/turns.js";
import { codexProvider } from "../src/providers/codex.js";

const baseBody = {
  turnId: "codex-turn",
  prompt: "hello",
  userBearer: "tok",
  systemPrompt: "You are the copilot.",
  model: "codex:gpt-5.6-terra",
  effort: "high",
  gatedTools: ["publish_app"],
};

const createTools = async () => ({
  url: "http://127.0.0.1:12345/mcp", token: "bridge-token",
  tools: [{ name: "get_quote" }, { name: "publish_app" }], close: async () => {},
});

class FakeCodexClient {
  constructor(script = {}) {
    this.script = script;
    this.requests = [];
    this.responses = [];
    this.handler = async () => {};
    this.closed = false;
  }

  async start(handler) {
    this.handler = handler;
  }

  async request(method, params) {
    this.requests.push({ method, params });
    if (method === "config/read") return { config: { mcp_servers: { personal: { url: "https://example.test/mcp" } } } };
    if (method === "mcpServerStatus/list") return { data: this.script.mcpServers ?? [{ name: "aladin", tools: { get_quote: {} } }] };
    if (method === "thread/resume") {
      if (this.script.resumeError) throw this.script.resumeError;
      return { thread: { id: params.threadId } };
    }
    if (method === "thread/start") {
      return { thread: { id: this.script.threadId ?? "codex-thread-1" } };
    }
    if (method === "turn/start") {
      await this.runTurn(params.threadId);
      return {};
    }
    return {};
  }

  respond(id, result) {
    this.responses.push({ id, result });
  }

  error(id, code, message) {
    this.responses.push({ id, error: { code, message } });
  }

  async runTurn(threadId) {
    for (const msg of this.script.messages ?? []) {
      await this.handler(
        typeof msg === "function" ? msg(threadId) : msg,
        this,
      );
    }
  }

  close() {
    this.closed = true;
  }
}

async function run(body, client, deps = {}) {
  const turn = createTurn(body.turnId);
  const events = [];
  try {
    await codexProvider.runTurn(body, turn, (ev) => events.push(ev), {
      client,
      createTools,
      turnTimeoutMs: 5_000,
      approvalTimeoutMs: 5_000,
      ...deps,
    });
  } finally {
    endTurn(body.turnId);
  }
  return events;
}

test("Codex provider streams app-server deltas and final message through the normalized contract", async () => {
  const client = new FakeCodexClient({
    threadId: "thr-codex-text",
    messages: [
      (threadId) => ({
        method: "turn/started",
        params: { threadId, turn: { id: "turn-1", items: [], status: "running" } },
      }),
      {
        method: "item/agentMessage/delta",
        params: { threadId: "thr-codex-text", turnId: "turn-1", itemId: "msg-1", delta: "Hel" },
      },
      {
        method: "item/agentMessage/delta",
        params: { threadId: "thr-codex-text", turnId: "turn-1", itemId: "msg-1", delta: "lo" },
      },
      {
        method: "turn/completed",
        params: { threadId: "thr-codex-text", turn: { id: "turn-1", items: [], status: "completed" } },
      },
    ],
  });

  const events = await run({ ...baseBody, turnId: "codex-text" }, client);

  assert.deepEqual(
    events.map((e) => e.type),
    ["session", "thinking", "token", "token", "message", "done"],
  );
  assert.equal(events[0].sessionId, "thr-codex-text");
  assert.equal(events[2].delta, "Hel");
  assert.equal(events[4].text, "Hello");
  assert.equal(client.requests.find((r) => r.method === "turn/start").params.model, "gpt-5.6-terra");
  const params = client.requests.find((r) => r.method === "thread/start").params;
  assert.equal(params.developerInstructions, baseBody.systemPrompt);
  assert.match(params.baseInstructions, /Aladin copilot/);
  assert.equal(params.sandbox, "read-only");
  assert.equal(params.config.features.shell_tool, false);
  assert.equal(params.config.web_search, "disabled");
  assert.equal(params.config.mcp_servers.personal.enabled, false);
  assert.equal(params.config.mcp_servers.aladin.url, "http://127.0.0.1:12345/mcp");
  assert.equal(params.config.mcp_servers.aladin.required, true);
});

test("Codex separates assistant items without changing Markdown inside token chunks", async () => {
  const final = "## Result\n\n**Market** snapshot\n\n```\nline 1\nline 2\n```";
  const client = new FakeCodexClient({ messages: [
    { method: "item/started", params: { item: { type: "agentMessage", id: "comment", phase: "commentary" } } },
    { method: "item/agentMessage/delta", params: { itemId: "comment", delta: "Checking tools." } },
    { method: "item/completed", params: { item: { type: "agentMessage", id: "comment", phase: "commentary", text: "Checking tools." } } },
    { method: "item/started", params: { item: { type: "mcpToolCall", id: "quote", tool: "get_quote" } } },
    { method: "item/completed", params: { item: { type: "mcpToolCall", id: "quote", tool: "get_quote", status: "completed" } } },
    { method: "item/started", params: { item: { type: "agentMessage", id: "answer", phase: "final_answer" } } },
    ...["## Result\n\n*", "*Market** snapshot\n\n`", "``\nline 1\nline 2\n```"].map((delta) => ({
      method: "item/agentMessage/delta", params: { itemId: "answer", delta },
    })),
    { method: "item/completed", params: { item: { type: "agentMessage", id: "answer", phase: "final_answer", text: final } } },
    { method: "turn/completed", params: { turn: { status: "completed" } } },
  ] });
  const events = await run({ ...baseBody, turnId: "codex-markdown" }, client);
  assert.equal(events.filter((event) => event.type === "token").map((event) => event.delta).join(""), "Checking tools.\n\n" + final);
  assert.equal(events.find((event) => event.type === "message").text, final);
});

test("Codex streams completion-only messages and missing suffixes exactly once", async () => {
  const item = { type: "agentMessage", id: "answer", phase: "final_answer", text: "A complete **answer**." };
  for (const prefix of ["", "A complete ", item.text]) {
    const client = new FakeCodexClient({ messages: [
      { method: "item/completed", params: { item: { type: "agentMessage", id: "comment", text: "Checking." } } },
      { method: "item/agentMessage/delta", params: { itemId: item.id, delta: prefix } },
      { method: "item/completed", params: { item } },
      { method: "item/completed", params: { item } },
      { method: "turn/completed", params: { turn: { status: "completed" } } },
    ] });
    const events = await run({ ...baseBody, turnId: "codex-completion" }, client);
    assert.equal(events.filter((event) => event.type === "token").map((event) => event.delta).join(""), "Checking.\n\n" + item.text);
    assert.equal(events.find((event) => event.type === "message").text, item.text);
  }
});

test("Codex retains progress during private reasoning, compaction and recoverable errors", async () => {
  const client = new FakeCodexClient({ messages: [
    { method: "turn/started", params: {} },
    ...["reasoning", "contextCompaction", "plan"].map((type) => ({ method: "item/started", params: { item: { type, id: type } } })),
    { method: "error", params: { willRetry: true, error: { message: "Retrying stream" } } },
    { method: "item/completed", params: { item: { type: "agentMessage", id: "answer", text: "Recovered." } } },
    { method: "turn/completed", params: { turn: { status: "completed" } } },
  ] });
  const events = await run({ ...baseBody, turnId: "codex-progress" }, client);
  assert.equal(events.filter((event) => event.type === "thinking").length, 5);
  assert.equal(events.some((event) => event.type === "error"), false);
  assert.equal(events.find((event) => event.type === "message").text, "Recovered.");
  assert.equal(events.filter((event) => event.type === "done").length, 1);
});

test("Codex still reports terminal errors", async () => {
  const client = new FakeCodexClient({ messages: [{ method: "error", params: { willRetry: false, error: { message: "Request failed" } } }] });
  const events = await run({ ...baseBody, turnId: "codex-terminal-error" }, client);
  assert.deepEqual(events.map((event) => event.type), ["session", "error", "done"]);
});

test("Codex provider denies native shell approvals outside the Aladin tool scope", async () => {
  const client = new FakeCodexClient({
    threadId: "thr-codex-approval",
    messages: [
      {
        method: "item/commandExecution/requestApproval",
        id: 42,
        params: {
          threadId: "thr-codex-approval",
          turnId: "turn-1",
          itemId: "cmd-1",
          command: "npm test",
          cwd: "/repo",
          reason: "run tests",
        },
      },
      {
        method: "turn/completed",
        params: { threadId: "thr-codex-approval", turn: { id: "turn-1", items: [], status: "completed" } },
      },
    ],
  });

  const turn = createTurn("codex-approval");
  const events = [];
  await codexProvider.runTurn({ ...baseBody, turnId: "codex-approval" }, turn, (ev) => {
    events.push(ev);
  }, {
    client,
    createTools,
    turnTimeoutMs: 5_000,
    approvalTimeoutMs: 5_000,
  });
  endTurn("codex-approval");

  assert.deepEqual(
    events.map((e) => e.type),
    ["session", "done"],
  );
  assert.deepEqual(client.responses, [{ id: 42, result: { decision: "decline" } }]);
});

test("Codex refreshes the Aladin surface instructions on resume without replaying history", async () => {
  const client = new FakeCodexClient({ messages: [{ method: "turn/completed", params: { turn: { status: "completed" } } }] });
  const body = { ...baseBody, turnId: "codex-context", resumeSessionId: "old-thread", historyFallback: "Old page", systemPrompt: "The user is viewing Markets with SPY and QQQ." };
  await run(body, client);
  assert.equal(client.requests.find((r) => r.method === "thread/resume").params.developerInstructions, body.systemPrompt);
  assert.equal(client.requests.find((r) => r.method === "turn/start").params.input[0].text, body.prompt);
  const injected = client.requests.find((r) => r.method === "thread/inject_items").params.items[0];
  assert.equal(injected.role, "developer");
  assert.match(injected.content[0].text, /Markets with SPY and QQQ/);
});

test("Codex preserves current instructions and durable history when a thread is missing", async () => {
  const client = new FakeCodexClient({ resumeError: new Error("thread not found"), messages: [{ method: "turn/completed", params: { turn: { status: "completed" } } }] });
  const events = await run({ ...baseBody, turnId: "codex-fallback", resumeSessionId: "missing", historyFallback: "Earlier conversation" }, client);
  assert.equal(events[0].resumed, false);
  assert.equal(client.requests.find((r) => r.method === "thread/start").params.developerInstructions, baseBody.systemPrompt);
  assert.match(client.requests.find((r) => r.method === "turn/start").params.input[0].text, /Earlier conversation/);
});

test("Codex refuses a turn when its Aladin tool inventory is missing", async () => {
  const client = new FakeCodexClient({ mcpServers: [] });
  const events = await run({ ...baseBody, turnId: "codex-no-tools" }, client);
  assert.deepEqual(events.map((e) => e.type), ["error", "done"]);
  assert.match(events[0].message, /Aladin MCP tools did not connect/);
  assert.equal(client.requests.some((r) => r.method === "turn/start"), false);
  assert.equal(client.closed, true);
});

test("Codex does not start a runtime when connecting to Aladin MCP fails", async () => {
  const client = new FakeCodexClient();
  const events = await run({ ...baseBody, turnId: "codex-mcp-error" }, client, { createTools: async () => { throw new Error("Aladin MCP: 401 unauthorized"); } });
  assert.deepEqual(events.map((e) => e.type), ["error", "done"]);
  assert.match(events[0].message, /401 unauthorized/);
  assert.equal(client.requests.length, 0);
});

test("Codex refuses inherited personal tools even when Aladin is connected", async () => {
  const client = new FakeCodexClient({ mcpServers: [
    { name: "aladin", tools: { get_quote: {} } },
    { name: "personal", tools: { local_shell: {} } },
  ] });
  const events = await run({ ...baseBody, turnId: "codex-scope" }, client);
  assert.match(events[0].message, /non-Aladin MCP/);
  assert.equal(client.requests.some((request) => request.method === "turn/start"), false);
});

test("Codex tool results carry the approval ID that settles the dock card", async () => {
  const client = new FakeCodexClient({ messages: [
    { method: "item/completed", params: { item: { id: "tool-1", type: "mcpToolCall", tool: "publish_app", status: "completed", result: { content: [{ type: "text", text: "Published" }], _meta: { "aladin/approvalId": "approved-1" } } } } },
    { method: "turn/completed", params: { turn: { status: "completed" } } },
  ] });
  const events = await run({ ...baseBody, turnId: "codex-tool-result" }, client);
  assert.equal(events.find((event) => event.type === "tool_result").approvalId, "approved-1");
});

test("Codex closes the runtime and tool bridge when initialization times out", async () => {
  const client = new FakeCodexClient();
  client.start = () => new Promise(() => {});
  let closed = false;
  const events = await run({ ...baseBody, turnId: "codex-init-timeout" }, client, {
    turnTimeoutMs: 10,
    createTools: async () => ({ ...await createTools(), close: async () => { closed = true; } }),
  });
  assert.deepEqual(events.map((e) => e.type), ["done"]);
  assert.equal(client.closed, true);
  assert.equal(closed, true);
});
