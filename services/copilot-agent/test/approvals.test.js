import test from "node:test";
import assert from "node:assert/strict";
import { createTurn, getTurn, endTurn } from "../src/turns.js";
import { makeCanUseTool, resolveApproval } from "../src/approvals.js";

function collect() {
  const events = [];
  return { events, emit: (ev) => events.push(ev) };
}

test("non-gated tools pass straight through", async () => {
  const turn = createTurn("t-pass");
  const { events, emit } = collect();
  const canUse = makeCanUseTool({ turn, gatedTools: ["publish_app"], emit, timeoutMs: 50 });

  const res = await canUse("mcp__aladin__search", { query: "x" }, { toolUseID: "tu" });
  assert.equal(res.behavior, "allow");
  assert.deepEqual(events, []);
  endTurn("t-pass");
});

test("gated tool holds until approved, then allows and maps the toolUse", async () => {
  const turn = createTurn("t-approve");
  const { events, emit } = collect();
  const canUse = makeCanUseTool({ turn, gatedTools: ["publish_app"], emit, timeoutMs: 5_000 });

  const pending = canUse("mcp__aladin__publish_app", { page_id: "p1" }, { toolUseID: "tu-9" });
  // The proposal is emitted synchronously before the hold.
  await new Promise((r) => setImmediate(r));
  assert.equal(events[0].type, "proposed_action");
  assert.equal(events[0].tool, "publish_app");

  assert.equal(resolveApproval(turn, events[0].approvalId, true), true);
  const res = await pending;
  assert.equal(res.behavior, "allow");
  assert.deepEqual(events[1], {
    type: "approval_resolved",
    approvalId: events[0].approvalId,
    approved: true,
    timedOut: false,
  });
  assert.equal(turn.approvalByToolUse.get("tu-9"), events[0].approvalId);
  endTurn("t-approve");
});

test("gated tool rejected → deny with dismissal guidance", async () => {
  const turn = createTurn("t-reject");
  const { events, emit } = collect();
  const canUse = makeCanUseTool({ turn, gatedTools: ["delete_file"], emit, timeoutMs: 5_000 });

  const pending = canUse("mcp__aladin__delete_file", {}, { toolUseID: "tu" });
  await new Promise((r) => setImmediate(r));
  resolveApproval(turn, events[0].approvalId, false);
  const res = await pending;
  assert.equal(res.behavior, "deny");
  assert.match(res.message, /declined/);
  assert.equal(turn.approvalByToolUse.size, 0);
  endTurn("t-reject");
});

test("gated tool times out → deny as expired", async () => {
  const turn = createTurn("t-timeout");
  const { events, emit } = collect();
  const canUse = makeCanUseTool({ turn, gatedTools: ["update_page"], emit, timeoutMs: 20 });

  const res = await canUse("mcp__aladin__update_page", {}, { toolUseID: "tu" });
  assert.equal(res.behavior, "deny");
  assert.match(res.message, /did not respond/);
  assert.equal(events[1].timedOut, true);
  endTurn("t-timeout");
});

test("ending the turn denies held approvals quietly", async () => {
  const turn = createTurn("t-end");
  const { events, emit } = collect();
  const canUse = makeCanUseTool({ turn, gatedTools: ["publish_app"], emit, timeoutMs: 5_000 });

  const pending = canUse("mcp__aladin__publish_app", {}, { toolUseID: "tu" });
  await new Promise((r) => setImmediate(r));
  endTurn("t-end");
  const res = await pending;
  assert.equal(res.behavior, "deny");
  // No approval_resolved after teardown — only the proposal was emitted.
  assert.equal(events.length, 1);
});

test("resolving an unknown approval reports false", () => {
  const turn = createTurn("t-unknown");
  assert.equal(resolveApproval(turn, "nope", true), false);
  endTurn("t-unknown");
  assert.equal(getTurn("t-unknown"), null);
});

test("duplicate turn ids are refused", () => {
  assert.ok(createTurn("t-dup"));
  assert.equal(createTurn("t-dup"), null);
  endTurn("t-dup");
});
