import test from "node:test";
import assert from "node:assert/strict";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { createCodexTools } from "../src/providers/codex-tools.js";
import { codexProvider } from "../src/providers/codex.js";
import { createTurn, endTurn } from "../src/turns.js";
import { resolveApproval } from "../src/approvals.js";

function upstreamFixture() {
  return {
    calls: [], closed: false,
    connect: async () => {},
    listTools: async () => ({ tools: [
      { name: "get_quote", description: "Return a test quote for a symbol.", inputSchema: { type: "object", properties: { symbol: { type: "string" } }, required: ["symbol"] } },
      { name: "publish_app", description: "Publish a test artifact.", inputSchema: { type: "object", properties: { page_id: { type: "string" } }, required: ["page_id"] } },
    ] }),
    async callTool(params) {
      this.calls.push(params);
      return { content: [{ type: "text", text: "Fixture result: 123.45" }], structuredContent: { test: true, value: 123.45 } };
    },
    async close() { this.closed = true; },
  };
}

test("Codex MCP bridge forwards results and enforces approvals before upstream execution", async (t) => {
  for (const decision of ["approve", "reject", "timeout", "cancel"]) {
    await t.test(decision, async (t) => {
      const turn = createTurn(`bridge-${decision}`);
      const upstream = upstreamFixture();
      const events = [];
      let proposed;
      const proposal = new Promise((resolve) => { proposed = resolve; });
      const bridge = await createCodexTools({ userBearer: "private-user-token", gatedTools: ["publish_app"] }, turn, (event) => {
        events.push(event);
        if (event.type === "proposed_action") proposed(event);
      }, { mcpClient: upstream, approvalTimeoutMs: decision === "timeout" ? 20 : 1000 });
      const client = new Client({ name: "test", version: "1" });
      t.after(async () => { endTurn(turn.id); await client.close(); await bridge.close(); });
      await client.connect(new StreamableHTTPClientTransport(new URL(bridge.url), {
        requestInit: { headers: { Authorization: `Bearer ${bridge.token}` } },
      }));
      assert.equal((await fetch(bridge.url, { method: "POST" })).status, 401);
      assert.equal((await client.listTools()).tools.length, 2);
      const result = await client.callTool({ name: "get_quote", arguments: { symbol: "SPY" } });
      assert.deepEqual(result.structuredContent, { test: true, value: 123.45 });
      assert.equal(events.length, 0);
      const pending = client.callTool({ name: "publish_app", arguments: { page_id: "fixture-page" } });
      const event = await proposal;
      assert.deepEqual(event.input, { page_id: "fixture-page" });
      assert.equal(upstream.calls.length, 1, "gated call must not run before approval");
      if (decision === "cancel") endTurn(turn.id);
      else if (decision !== "timeout") resolveApproval(turn, event.approvalId, decision === "approve");
      const output = await pending;
      assert.equal(upstream.calls.length, decision === "approve" ? 2 : 1);
      assert.equal(output.isError === true, decision !== "approve");
      if (decision === "approve") assert.equal(output._meta["aladin/approvalId"], event.approvalId);
      await bridge.close();
      assert.equal(upstream.closed, true);
    });
  }
});

test("Codex MCP bridge fails closed on empty tools or upstream authentication errors", async () => {
  for (const empty of [true, false]) {
    const upstream = upstreamFixture();
    upstream.listTools = async () => { if (empty) return { tools: [] }; throw new Error("401 Unauthorized"); };
    const turn = createTurn(`bridge-failure-${empty}`);
    try {
      await assert.rejects(createCodexTools({ userBearer: "token" }, turn, () => {}, { mcpClient: upstream }), /Aladin MCP tools are unavailable/);
      assert.equal(upstream.closed, true);
    } finally { endTurn(turn.id); }
  }
});

test("Codex bridge authenticates to the upstream MCP server over HTTP", async (t) => {
  const sourceTurn = createTurn("mcp-source");
  const turn = createTurn("mcp-http");
  const upstream = upstreamFixture();
  const source = await createCodexTools({ userBearer: "fixture" }, sourceTurn, () => {}, { mcpClient: upstream });
  let bridge;
  const client = new Client({ name: "test", version: "1" });
  t.after(async () => {
    endTurn(turn.id); endTurn(sourceTurn.id);
    await client.close(); await bridge?.close(); await source.close();
  });
  await assert.rejects(createCodexTools({ userBearer: "wrong-token", mcpUrl: source.url }, turn, () => {}), /401/);
  bridge = await createCodexTools({ userBearer: source.token, mcpUrl: source.url }, turn, () => {});
  await client.connect(new StreamableHTTPClientTransport(new URL(bridge.url), {
    requestInit: { headers: { Authorization: `Bearer ${bridge.token}` } },
  }));
  await client.callTool({ name: "get_quote", arguments: { symbol: "SPY" } });
  assert.deepEqual(upstream.calls[0].arguments, { symbol: "SPY" });
});

// Real Codex runtime and model, but isolated fixture data and no workspace writes.
test("live Codex understands Aladin context, uses MCP, and refreshes context on resume", { skip: process.env.CODEX_LIVE_TEST !== "1", timeout: 120_000 }, async () => {
  let resumeSessionId;
  for (const surface of ["Markets", "Study Board"]) {
    const turn = createTurn(`codex-live-${surface}`);
    const upstream = upstreamFixture();
    const events = [];
    try {
      await codexProvider.runTurn({
        turnId: turn.id, userBearer: "fixture-only", resumeSessionId,
        systemPrompt: `You are the Aladin copilot. The user is currently viewing ${surface}. Use get_quote when asked for a quote. These are test tools and fixture values, not real market data.`,
        prompt: "Name the application I am using and the surface I am currently viewing. Then fetch a test SPY quote with get_quote and report the fixture result. Keep it brief.",
        model: "codex:gpt-5.6-sol", effort: "low", gatedTools: ["publish_app"],
      }, turn, (event) => events.push(event), { mcpClient: upstream, turnTimeoutMs: 55_000 });
      assert.deepEqual(events.filter((event) => event.type === "error"), [], surface);
      assert.equal(upstream.calls.some((call) => call.name === "get_quote"), true);
      const final = events.find((event) => event.type === "message")?.text ?? "";
      assert.match(final, /Aladin/i);
      assert.ok(final.includes(surface), final);
      assert.match(final, /123\.45/);
      assert.equal(events.some((event) => event.type === "tool_start" && ["command", "file_change"].includes(event.name)), false);
      resumeSessionId = events.find((event) => event.type === "session")?.sessionId;
      assert.ok(resumeSessionId);
    } finally { endTurn(turn.id); }
  }
});

test("live Codex requests approval before a fixture publish and settles the tool result", { skip: process.env.CODEX_LIVE_TEST !== "1", timeout: 60_000 }, async () => {
  const turn = createTurn("codex-live-approval");
  const upstream = upstreamFixture();
  const events = [];
  try {
    await codexProvider.runTurn({
      turnId: turn.id, userBearer: "fixture-only", gatedTools: ["publish_app"],
      model: "codex:gpt-5.6-sol", effort: "low",
      systemPrompt: "You are the Aladin copilot. When asked to publish, call publish_app; its approval UI collects confirmation. All tools are isolated test fixtures.",
      prompt: "Publish the test artifact fixture-page using publish_app.",
    }, turn, (event) => {
      events.push(event);
      if (event.type === "proposed_action") {
        assert.equal(upstream.calls.length, 0);
        resolveApproval(turn, event.approvalId, true);
      }
    }, { mcpClient: upstream, turnTimeoutMs: 55_000 });
    assert.deepEqual(events.filter((event) => event.type === "error"), []);
    const approval = events.find((event) => event.type === "proposed_action");
    assert.ok(approval, "Codex must request approval before publishing");
    assert.equal(upstream.calls[0].name, "publish_app");
    assert.equal(events.find((event) => event.type === "tool_result")?.approvalId, approval.approvalId);
  } finally { endTurn(turn.id); }
});
