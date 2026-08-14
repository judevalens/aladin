import test from "node:test";
import assert from "node:assert/strict";
import { createTurn, endTurn } from "../src/turns.js";
import { runTurn } from "../src/agent.js";

// fakeQuery builds a query()-compatible async generator from canned SDK
// messages (or an Error to throw mid-stream). Captures the options it was
// called with so tests can assert the wiring.
function fakeQuery(script) {
  const calls = [];
  const fn = ({ prompt, options }) => {
    calls.push({ prompt, options });
    const scripted = Array.isArray(script) ? script : script(calls.length);
    return (async function* () {
      for (const item of scripted) {
        if (item instanceof Error) throw item;
        yield item;
      }
    })();
  };
  fn.calls = calls;
  return fn;
}

const init = {
  type: "system",
  subtype: "init",
  session_id: "sess-new",
  mcp_servers: [{ name: "aladin", status: "connected" }],
};
const okResult = {
  type: "result",
  subtype: "success",
  result: "Done.",
  session_id: "sess-new",
  num_turns: 1,
  total_cost_usd: 0.01,
  usage: { input_tokens: 10, output_tokens: 5 },
};

const baseBody = {
  turnId: "turn-1",
  prompt: "hello",
  userBearer: "tok",
  systemPrompt: "You are the copilot.",
  gatedTools: ["publish_app"],
};

async function run(body, queryFn) {
  const turn = createTurn(body.turnId);
  const events = [];
  try {
    await runTurn(body, turn, (ev) => events.push(ev), { queryFn, turnTimeoutMs: 5_000 });
  } finally {
    endTurn(body.turnId);
  }
  return events;
}

test("happy path: session → message → exactly one trailing done", async () => {
  const q = fakeQuery([init, okResult]);
  const events = await run({ ...baseBody, turnId: "t-happy", model: "claude-opus-5", effort: "xhigh" }, q);

  assert.deepEqual(
    events.map((e) => e.type),
    ["session", "message", "done"],
  );
  assert.equal(events[0].sessionId, "sess-new");
  assert.equal(events[0].resumed, false);
  assert.equal(events[2].usage.inputTokens, 10);

  // Wiring: MCP server gets the user bearer; built-ins disabled; system prompt set.
  const opts = q.calls[0].options;
  assert.deepEqual(opts.tools, []);
  assert.equal(opts.mcpServers.aladin.headers.Authorization, "Bearer tok");
  assert.equal(opts.systemPrompt, "You are the copilot.");
  assert.equal(opts.model, "claude-opus-5");
  assert.equal(opts.effort, "xhigh");
  assert.equal(typeof opts.canUseTool, "function");
});

test("resume failure falls back to a fresh session with history preamble", async () => {
  const q = fakeQuery((attempt) =>
    attempt === 1 ? [new Error("No conversation found with session ID")] : [init, okResult],
  );
  const events = await run(
    { ...baseBody, turnId: "t-resume", resumeSessionId: "sess-old", historyFallback: "User: hi\nAssistant: hey" },
    q,
  );

  assert.equal(q.calls.length, 2);
  assert.equal(q.calls[0].options.resume, "sess-old");
  assert.equal(q.calls[1].options.resume, undefined);
  assert.match(q.calls[1].prompt, /Previous conversation/);
  assert.match(q.calls[1].prompt, /User: hi/);

  const session = events.find((e) => e.type === "session");
  assert.equal(session.resumed, false);
  assert.equal(events.at(-1).type, "done");
});

test("failure after the session initialized is a real error, not a retry", async () => {
  const q = fakeQuery((attempt) =>
    attempt === 1 ? [init, new Error("boom mid-run")] : [init, okResult],
  );
  const events = await run(
    { ...baseBody, turnId: "t-err", resumeSessionId: "sess-old" },
    q,
  );

  assert.equal(q.calls.length, 1, "no fallback retry after init");
  const err = events.find((e) => e.type === "error");
  assert.match(err.message, /boom/);
  assert.equal(events.at(-1).type, "done");
  assert.equal(events.filter((e) => e.type === "done").length, 1);
});

test("clean end without a result still closes with exactly one done", async () => {
  const q = fakeQuery([init]); // generator ends with no result message
  const events = await run({ ...baseBody, turnId: "t-noresult" }, q);
  assert.equal(events.at(-1).type, "done");
  assert.equal(events.filter((e) => e.type === "done").length, 1);
});

test("a dead MCP server fails the turn loudly: error then done, query aborted", async () => {
  const initFailedMcp = {
    ...init,
    mcp_servers: [{ name: "aladin", status: "failed" }],
  };
  // The script would continue past init (a message the model would "improvise")
  // — the fatal guard must abort before any of it streams.
  const q = fakeQuery([initFailedMcp, okResult]);
  const events = await run({ ...baseBody, turnId: "t-dead-mcp" }, q);

  assert.deepEqual(
    events.map((e) => e.type),
    ["session", "error", "done"],
  );
  assert.match(events[1].message, /aladin.*failed/);
  // The scripted success result must NOT have reached the stream.
  assert.equal(events.some((e) => e.type === "message"), false);
});

test("abort (cancel) yields a quiet done without an error event", async () => {
  const turn = createTurn("t-cancel");
  const events = [];
  const q = ({ options }) =>
    (async function* () {
      yield init;
      options.abortController.abort();
      const err = new Error("aborted");
      err.name = "AbortError";
      throw err;
    })();
  await runTurn({ ...baseBody, turnId: "t-cancel" }, turn, (ev) => events.push(ev), {
    queryFn: q,
    turnTimeoutMs: 5_000,
  });
  endTurn("t-cancel");

  assert.equal(events.find((e) => e.type === "error"), undefined);
  assert.equal(events.at(-1).type, "done");
});

test("history preamble is only used when not resuming", async () => {
  const q = fakeQuery([init, okResult]);
  await run(
    { ...baseBody, turnId: "t-hist", resumeSessionId: "sess-old", historyFallback: "User: hi" },
    q,
  );
  assert.equal(q.calls[0].prompt, "hello", "resume attempt sends the bare prompt");
});
