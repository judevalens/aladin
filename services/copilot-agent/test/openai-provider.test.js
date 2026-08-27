import test from "node:test";
import assert from "node:assert/strict";
import { createTurn, endTurn } from "../src/turns.js";
import { resolveApproval } from "../src/approvals.js";
import { openaiProvider } from "../src/providers/openai.js";

const baseBody = {
  turnId: "openai-turn",
  prompt: "hello",
  userBearer: "tok",
  systemPrompt: "You are the copilot.",
  model: "openai:gpt-5.1",
  effort: "high",
  gatedTools: ["publish_app"],
};

function fakeStream({ events = [], finalOutput = "", interruptions = [], state = {} }) {
  return {
    currentTurn: 1,
    finalOutput,
    interruptions,
    state,
    completed: Promise.resolve(),
    error: null,
    async *[Symbol.asyncIterator]() {
      for (const event of events) yield event;
    },
  };
}

async function run(body, deps) {
  const turn = createTurn(body.turnId);
  const events = [];
  try {
    await openaiProvider.runTurn(body, turn, (ev) => events.push(ev), {
      tools: [],
      turnTimeoutMs: 5_000,
      ...deps,
    });
  } finally {
    endTurn(body.turnId);
  }
  return events;
}

test("OpenAI provider streams tokens and final message through the normalized contract", async () => {
  const events = await run({ ...baseBody, turnId: "openai-text" }, {
    runFn: async () =>
      fakeStream({
        events: [
          { type: "raw_model_stream_event", data: { type: "response.output_text.delta", delta: "Hel" } },
          { type: "raw_model_stream_event", data: { type: "response.output_text.delta", delta: "lo" } },
        ],
        finalOutput: "Hello",
      }),
  });

  assert.deepEqual(
    events.map((e) => e.type),
    ["session", "token", "token", "message", "done"],
  );
  assert.equal(events[1].delta, "Hel");
  assert.equal(events[3].text, "Hello");
  assert.equal(events.at(-1).sessionId, events[0].sessionId);
});

test("OpenAI provider translates approval interruptions and resumes from run state", async () => {
  let call = 0;
  let approved = false;
  let resumedWithState = false;
  const state = {
    approve(interruption) {
      approved = interruption.name === "publish_app";
    },
    reject() {
      throw new Error("should approve");
    },
  };
  const turn = createTurn("openai-approval");
  const events = [];
  const running = openaiProvider.runTurn({ ...baseBody, turnId: "openai-approval" }, turn, (ev) => {
    events.push(ev);
    if (ev.type === "proposed_action") {
      resolveApproval(turn, ev.approvalId, true);
    }
  }, {
    tools: [{ name: "publish_app" }],
    approvalTimeoutMs: 5_000,
    turnTimeoutMs: 5_000,
    runFn: async (_agent, input) => {
      call += 1;
      if (call === 1) {
        return fakeStream({
          interruptions: [{ name: "publish_app", arguments: JSON.stringify({ id: "app-1" }) }],
          state,
        });
      }
      resumedWithState = input === state;
      return fakeStream({ finalOutput: "Published." });
    },
  });
  await running;
  endTurn("openai-approval");

  assert.equal(call, 2);
  assert.equal(approved, true);
  assert.equal(resumedWithState, true);
  assert.deepEqual(
    events.map((e) => e.type),
    ["session", "proposed_action", "approval_resolved", "message", "done"],
  );
  assert.equal(events[1].tool, "publish_app");
  assert.deepEqual(events[1].input, { id: "app-1" });
});
