import test from "node:test";
import assert from "node:assert/strict";
import { checkMcpHealth } from "../src/health.js";

test("mcp up → true, derived from the /mcp URL's origin", async () => {
  let requested = "";
  const fetchFn = async (url) => {
    requested = url;
    return { ok: true };
  };
  assert.equal(await checkMcpHealth("http://localhost:8090/mcp", fetchFn), true);
  assert.equal(requested, "http://localhost:8090/healthz");
});

test("mcp 5xx → false", async () => {
  assert.equal(await checkMcpHealth("http://localhost:8090/mcp", async () => ({ ok: false })), false);
});

test("mcp unreachable/timeout → false", async () => {
  const fetchFn = async () => {
    throw new Error("connect ECONNREFUSED");
  };
  assert.equal(await checkMcpHealth("http://localhost:8090/mcp", fetchFn), false);
});

test("garbage MCP url → false, not a crash", async () => {
  assert.equal(await checkMcpHealth("not a url", async () => ({ ok: true })), false);
});
