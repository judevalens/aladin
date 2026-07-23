import test from "node:test";
import assert from "node:assert/strict";
import { createTranslator } from "../src/translate.js";
import { stripMcpPrefix } from "../src/approvals.js";

test("strips the mcp server prefix from tool names", () => {
  assert.equal(stripMcpPrefix("mcp__aladin__get_page"), "get_page");
  assert.equal(stripMcpPrefix("mcp__aladin__preview_open"), "preview_open");
  assert.equal(stripMcpPrefix("search"), "search");
});

test("system init becomes a session event", () => {
  const tr = createTranslator({ resumed: true });
  const events = tr.translate({
    type: "system",
    subtype: "init",
    session_id: "sess-1",
    mcp_servers: [{ name: "aladin", status: "connected" }],
  });
  assert.deepEqual(events, [{ type: "session", sessionId: "sess-1", resumed: true }]);
  assert.equal(tr.sawSession(), true);
});

test("system init with OUR dead MCP server emits a fatal error event", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "system",
    subtype: "init",
    session_id: "sess-1",
    mcp_servers: [{ name: "aladin", status: "failed" }],
  });
  assert.equal(events.length, 2);
  assert.equal(events[0].type, "session");
  assert.equal(events[1].type, "error");
  assert.equal(events[1].fatal, true);
  assert.match(events[1].message, /aladin.*failed/);
  assert.match(events[1].message, /make mcp/);
});

test("system init with our server connected emits only the session event", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "system",
    subtype: "init",
    session_id: "sess-1",
    mcp_servers: [{ name: "aladin", status: "connected" }],
  });
  assert.deepEqual(events.map((e) => e.type), ["session"]);
});

test("a needs-auth OTHER server (local ~/.claude config) does NOT kill the turn", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "system",
    subtype: "init",
    session_id: "sess-1",
    mcp_servers: [
      { name: "aladin", status: "connected" },
      { name: "claude.ai Google Drive", status: "needs-auth" },
    ],
  });
  assert.deepEqual(events.map((e) => e.type), ["session"]);
});

test("our server absent entirely is fatal (not registered)", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "system",
    subtype: "init",
    session_id: "sess-1",
    mcp_servers: [{ name: "claude.ai Google Drive", status: "needs-auth" }],
  });
  assert.equal(events[1]?.type, "error");
  assert.equal(events[1].fatal, true);
  assert.match(events[1].message, /not registered/);
});

test("a thinking block start emits one thinking event; its deltas emit nothing", () => {
  const tr = createTranslator({});
  const start = {
    type: "stream_event",
    parent_tool_use_id: null,
    event: { type: "content_block_start", content_block: { type: "thinking" } },
  };
  assert.deepEqual(tr.translate(start), [{ type: "thinking" }]);
  const delta = {
    type: "stream_event",
    parent_tool_use_id: null,
    event: { type: "content_block_delta", delta: { type: "thinking_delta", thinking: "hmm" } },
  };
  assert.deepEqual(tr.translate(delta), []);
});

test("text deltas become token events, nested streams are dropped", () => {
  const tr = createTranslator({});
  const delta = {
    type: "stream_event",
    parent_tool_use_id: null,
    event: { type: "content_block_delta", delta: { type: "text_delta", text: "Hel" } },
  };
  assert.deepEqual(tr.translate(delta), [{ type: "token", delta: "Hel" }]);
  assert.deepEqual(tr.translate({ ...delta, parent_tool_use_id: "tu-1" }), []);
});

test("assistant tool_use blocks become tool_start with stripped names", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "assistant",
    parent_tool_use_id: null,
    message: {
      content: [
        { type: "text", text: "Let me look." },
        { type: "tool_use", id: "tu-1", name: "mcp__aladin__search", input: { query: "nvda" } },
      ],
    },
  });
  assert.deepEqual(events, [
    { type: "tool_start", name: "search", input: { query: "nvda" } },
  ]);
});

test("tool results carry text content, the tool name, and any approvalId", () => {
  const approvals = new Map([["tu-2", "app-1"]]);
  const tr = createTranslator({ approvalByToolUse: approvals });
  tr.translate({
    type: "assistant",
    parent_tool_use_id: null,
    message: {
      content: [
        { type: "tool_use", id: "tu-2", name: "mcp__aladin__publish_app", input: {} },
      ],
    },
  });
  const events = tr.translate({
    type: "user",
    parent_tool_use_id: null,
    message: {
      content: [
        {
          type: "tool_result",
          tool_use_id: "tu-2",
          content: [{ type: "text", text: '{"ok":true}' }],
        },
      ],
    },
  });
  assert.deepEqual(events, [
    {
      type: "tool_result",
      name: "publish_app",
      toolUseId: "tu-2",
      content: '{"ok":true}',
      isError: false,
      approvalId: "app-1",
    },
  ]);
  assert.equal(approvals.size, 0, "approval mapping is consumed");
});

test("successful result emits message then done with usage", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "result",
    subtype: "success",
    result: "NVDA looks strong.",
    session_id: "sess-9",
    num_turns: 3,
    total_cost_usd: 0.12,
    usage: { input_tokens: 100, output_tokens: 40 },
  });
  assert.deepEqual(events, [
    { type: "message", text: "NVDA looks strong." },
    {
      type: "done",
      sessionId: "sess-9",
      numTurns: 3,
      usage: { inputTokens: 100, outputTokens: 40 },
      costUsd: 0.12,
    },
  ]);
});

test("empty result falls back to the last assistant text", () => {
  const tr = createTranslator({});
  tr.translate({
    type: "assistant",
    parent_tool_use_id: null,
    message: { content: [{ type: "text", text: "Fallback answer." }] },
  });
  const events = tr.translate({
    type: "result",
    subtype: "success",
    result: "",
    session_id: "s",
    num_turns: 1,
    total_cost_usd: 0,
    usage: {},
  });
  assert.equal(events[0].text, "Fallback answer.");
});

test("error results emit error then done", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "result",
    subtype: "error_max_turns",
    session_id: "s",
    num_turns: 24,
    total_cost_usd: 1,
    usage: {},
    errors: [],
  });
  assert.equal(events[0].type, "error");
  assert.equal(events[0].code, "max_turns");
  assert.match(events[0].message, /step limit/);
  assert.equal(events[1].type, "done");
});

test("an API-error-as-success result becomes an error, never a message", () => {
  const tr = createTranslator({});
  // Accumulate assistant prose first — it must NOT be used as a fallback answer.
  tr.translate({
    type: "assistant",
    parent_tool_use_id: null,
    message: { content: [{ type: "text", text: "Partial prose before the failure." }] },
  });
  const events = tr.translate({
    type: "result",
    subtype: "success",
    result: 'API Error: 400 {"type":"error","error":{"type":"invalid_request_error"}}',
    session_id: "s",
    num_turns: 3,
    total_cost_usd: 0,
    usage: {},
  });
  assert.deepEqual(events.map((e) => e.type), ["error", "done"]);
  assert.equal(events[0].code, "api_error");
  assert.match(events[0].message, /^API Error: 400/);
  assert.equal(events.some((e) => e.type === "message"), false);
});

test("a genuine answer that merely mentions API errors stays a message", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "result",
    subtype: "success",
    result: "You can retry when an API Error: 400 appears in the logs.",
    session_id: "s",
    num_turns: 1,
    total_cost_usd: 0,
    usage: {},
  });
  assert.equal(events[0].type, "message");
});

test("is_error on a success result is classified as an api error", () => {
  const tr = createTranslator({});
  const events = tr.translate({
    type: "result",
    subtype: "success",
    is_error: true,
    result: "Overloaded",
    session_id: "s",
    num_turns: 1,
    total_cost_usd: 0,
    usage: {},
  });
  assert.equal(events[0].type, "error");
  assert.equal(events[0].code, "api_error");
});

test("huge tool results are capped", () => {
  const tr = createTranslator({});
  tr.translate({
    type: "assistant",
    parent_tool_use_id: null,
    message: { content: [{ type: "tool_use", id: "tu-3", name: "get_page", input: {} }] },
  });
  const [ev] = tr.translate({
    type: "user",
    parent_tool_use_id: null,
    message: {
      content: [
        { type: "tool_result", tool_use_id: "tu-3", content: "x".repeat(50_000) },
      ],
    },
  });
  assert.ok(ev.content.length < 33_000);
  assert.match(ev.content, /\[truncated\]$/);
});
