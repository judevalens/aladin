import test from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { setTimeout as delay } from "node:timers/promises";
import { createEventStream } from "../src/event-stream.js";

class Response extends EventEmitter {
  writableEnded = false;
  destroyed = false;
  events = [];
  write(line) {
    this.events.push(JSON.parse(line));
    return true;
  }
}

test("quiet turns send transport heartbeats, not fake model progress", async (t) => {
  const response = new Response();
  const stream = createEventStream(response, { heartbeatMs: 10 });
  t.after(stream.dispose);
  stream.writeEvent({ type: "proposed_action", approvalId: "approval-1" });
  await delay(45);
  assert.ok(response.events.length >= 3);
  assert.ok(response.events.slice(1).every((event) => event.type === "heartbeat"));
  stream.writeEvent({ type: "message", text: "Finished." });
  stream.writeEvent({ type: "done" });
  const events = [...response.events];
  await delay(25);
  stream.writeEvent({ type: "token", delta: "late" });
  assert.deepEqual(response.events, events);
  assert.equal(response.listenerCount("close"), 0);
  assert.equal(response.listenerCount("finish"), 0);
});

for (const terminal of ["close", "finish", "dispose", "destroyed", "writableEnded"]) {
  test(`heartbeat cleanup on ${terminal}`, async (t) => {
    const response = new Response();
    const stream = createEventStream(response, { heartbeatMs: 10 });
    t.after(stream.dispose);
    if (terminal === "dispose") stream.dispose();
    else if (terminal === "close" || terminal === "finish") response.emit(terminal);
    else response[terminal] = true;
    await delay(25);
    stream.writeEvent({ type: "token", delta: "late" });
    assert.deepEqual(response.events, []);
    assert.equal(response.listenerCount("close"), 0);
    assert.equal(response.listenerCount("finish"), 0);
  });
}
