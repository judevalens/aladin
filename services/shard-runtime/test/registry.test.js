import test from "node:test";
import assert from "node:assert/strict";
import { ReleaseRegistry, releaseMemoryMiB, releaseOperationTimeouts } from "../src/registry.js";

const schema = `type Query { value: String!, secret: String!, delayed: String! } type Mutation { setValue(value: String!): String! }`;
function release(hash, bundle, overrides = {}) {
  return {
    scopeKey: "user/shard/draft",
    releaseHash: hash,
    schema,
    bundle,
    manifest: {
      graphql: {
        schema: "graphql/schema.graphql",
        operations: {
          value: { document: "query Value { value }", exposure: ["app", "agent"] },
          secret: { document: "query Secret { secret }", exposure: ["agent"] },
          delayed: { document: "query Delayed { delayed }", exposure: ["app"] },
          set: { document: "mutation Set($value: String!) { setValue(value: $value) }", exposure: ["app"] }
        },
        resolvers: {
          value: { capabilities: [], budget: { maxOperations: 1, maxDocuments: 1, timeoutMs: 1000, memoryMiB: 32 } },
          secret: { capabilities: ["tasks:query"], budget: { maxOperations: 1, maxDocuments: 2, timeoutMs: 1000, memoryMiB: 32 } },
          delayed: { capabilities: ["tasks:query"], budget: { maxOperations: 1, maxDocuments: 2, timeoutMs: 2000, memoryMiB: 32 } },
          setValue: { capabilities: ["tasks:update"], budget: { maxOperations: 1, maxDocuments: 1, timeoutMs: 1000, memoryMiB: 32 } }
        }
      },
      lambdas: {
        rollup: { capabilities: ["tasks:query"], budget: { maxOperations: 1, maxDocuments: 2, timeoutMs: 1000, memoryMiB: 32 }, trigger: { kind: "manual" } }
      }
    },
    ...overrides
  };
}

const bundleV1 = `
export const resolvers = {
  value: () => "r1",
  secret: async (_, ctx) => (await ctx.capabilities.call("tasks:query", {limit: 2})).value,
  delayed: async (_, ctx) => (await ctx.capabilities.call("tasks:query", {wait: true})).value,
  setValue: async ({value}, ctx) => (await ctx.capabilities.call("tasks:update", {value})).value
};
export const lambdas = { rollup: async (input, ctx) => ({input, result: await ctx.capabilities.call("tasks:query", {})}) };
`;

test("uses the strictest release worker memory budget", () => {
  const value = release("memory", bundleV1);
  value.manifest.graphql.resolvers.value.budget.memoryMiB = 128;
  value.manifest.lambdas.rollup.budget.memoryMiB = 16;
  assert.equal(releaseMemoryMiB(value), 16);
  value.manifest.graphql.resolvers.value.budget.timeoutMs = 25;
  value.manifest.graphql.resolvers.secret.budget.timeoutMs = 250;
  assert.equal(releaseOperationTimeouts(value).get("value"), 25);
  assert.equal(releaseOperationTimeouts(value).get("secret"), 250);
});

test("prepares, atomically swaps, pins requests, and removes releases", async () => {
  const registry = new ReleaseRegistry({ capabilityExecutor: async ({ capability, input, releaseHash, scopeToken }) => ({ value: `${releaseHash}:${capability}:${input.value || "read"}:${scopeToken}` }) });
  try {
    await registry.prepare(release("r1", bundleV1));
    registry.activate("user/shard/draft", "r1");
    await registry.prepare(release("r1", bundleV1));
    assert.deepEqual(registry.activate("user/shard/draft", "r1"), { releaseHash: "r1" });
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "value" }), { data: { value: "r1" } });
    await registry.prepare(release("r2", bundleV1.replace('"r1"', '"r2"')));
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "value" }), { data: { value: "r1" } });
    registry.activate("user/shard/draft", "r2");
    await assert.rejects(() => registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "value" }), { code: "RELEASE_CHANGED" });
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r2", operationId: "value" }), { data: { value: "r2" } });
    registry.remove("user/shard/draft", "r2");
    await assert.rejects(() => registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r2", operationId: "value" }), { code: "RELEASE_CHANGED" });
  } finally { registry.close(); }
});

test("failed candidate leaves the active release serving", async () => {
  const registry = new ReleaseRegistry();
  try {
    await registry.prepare(release("r1", bundleV1));
    registry.activate("user/shard/draft", "r1");
    await assert.rejects(() => registry.prepare(release("broken", "export const resolvers = {}; export const lambdas = {};")), /does not export/);
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "value" }), { data: { value: "r1" } });
  } finally { registry.close(); }
});

test("enforces persisted exposure and passes only server scope to capabilities", async () => {
  const calls = [];
  const registry = new ReleaseRegistry({ capabilityExecutor: async (call) => { calls.push(call); return { value: "scoped", documentsRead: 1 }; } });
  try {
    await registry.prepare(release("r1", bundleV1));
    registry.activate("user/shard/draft", "r1");
    await assert.rejects(() => registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "secret", exposure: "app" }), { code: "FORBIDDEN" });
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "secret", exposure: "agent", scopeToken: "request-agent" }), { data: { secret: "scoped" } });
    assert.equal(calls[0].scopeToken, "request-agent");
    assert.equal(calls[0].capability, "tasks:query");
    assert.equal("namespace" in calls[0].input, false);
    assert.deepEqual(await registry.invokeLambda({ scopeKey: "user/shard/draft", releaseHash: "r1", name: "rollup", input: { day: 1 }, scopeToken: "request-lambda" }), { input: { day: 1 }, result: { value: "scoped", documentsRead: 1 } });
    assert.equal(calls[1].scopeToken, "request-lambda");
  } finally { registry.close(); }
});

test("preserves structured capability failure codes for authored handlers", async () => {
  const catches = bundleV1.replace(
    'secret: async (_, ctx) => (await ctx.capabilities.call("tasks:query", {limit: 2})).value',
    'secret: async (_, ctx) => { try { await ctx.capabilities.call("tasks:query", {}); return "unexpected"; } catch (error) { return error.code; } }'
  );
  const registry = new ReleaseRegistry({ capabilityExecutor: async () => { throw Object.assign(new Error("conflict"), { code: "conflict" }); } });
  try {
    await registry.prepare(release("r1", catches));
    registry.activate("user/shard/draft", "r1");
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "secret", exposure: "agent", scopeToken: "scope" }), { data: { secret: "conflict" } });
  } finally { registry.close(); }
});

test("old release drains after an in-flight request during activation", async () => {
  let releaseCall;
  const gate = new Promise((resolve) => { releaseCall = resolve; });
  const registry = new ReleaseRegistry({ capabilityExecutor: async ({ releaseHash }) => { if (releaseHash === "r1") await gate; return { value: releaseHash }; } });
  try {
    await registry.prepare(release("r1", bundleV1));
    registry.activate("user/shard/draft", "r1");
    const inFlight = registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "delayed", scopeToken: "request-r1" });
    await registry.prepare(release("r2", bundleV1.replace('"r1"', '"r2"')));
    registry.activate("user/shard/draft", "r2");
    releaseCall();
    assert.deepEqual(await inFlight, { data: { delayed: "r1" } });
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r2", operationId: "value" }), { data: { value: "r2" } });
  } finally { registry.close(); }
});

test("isolates same-hash candidates by scope and hides parent environment", async () => {
  const prior = process.env.SHARD_RUNTIME_SECRET;
  process.env.SHARD_RUNTIME_SECRET = "must-not-reach-worker";
  const environmentBundle = `export const resolvers = { value: () => process.env.SHARD_RUNTIME_SECRET || "clear", secret: () => "secret", delayed: () => "delayed", setValue: ({value}) => value }; export const lambdas = { rollup: async () => ({}) };`;
  const registry = new ReleaseRegistry();
  try {
    const first = release("same", environmentBundle);
    const second = release("same", environmentBundle, { scopeKey: "other/shard/draft" });
    await registry.prepare(first);
    await registry.prepare(second);
    registry.activate(first.scopeKey, first.releaseHash);
    registry.activate(second.scopeKey, second.releaseHash);
    assert.deepEqual(await registry.execute({ scopeKey: first.scopeKey, releaseHash: "same", operationId: "value" }), { data: { value: "clear" } });
    assert.deepEqual(await registry.execute({ scopeKey: second.scopeKey, releaseHash: "same", operationId: "value" }), { data: { value: "clear" } });
  } finally {
    registry.close();
    if (prior === undefined) delete process.env.SHARD_RUNTIME_SECRET; else process.env.SHARD_RUNTIME_SECRET = prior;
  }
});

test("terminates a timed-out worker and can warm the same release again", async () => {
  const hanging = bundleV1.replace('value: () => "r1"', 'value: async () => await new Promise(() => {})');
  const timed = release("r1", hanging);
  for (const resolver of Object.values(timed.manifest.graphql.resolvers)) resolver.budget.timeoutMs = 20;
  const registry = new ReleaseRegistry();
  try {
    await registry.prepare(timed); registry.activate("user/shard/draft", "r1");
    await assert.rejects(() => registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "value" }), { code: "TIMEOUT" });
    await registry.prepare(release("r1", bundleV1)); registry.activate("user/shard/draft", "r1");
    assert.deepEqual(await registry.execute({ scopeKey: "user/shard/draft", releaseHash: "r1", operationId: "value" }), { data: { value: "r1" } });
  } finally { registry.close(); }
});

test("counts snapshot documents and rejects peer requests when a worker times out", async () => {
  const counted = release("counted", bundleV1);
  counted.manifest.graphql.resolvers.secret.budget.maxDocuments = 1;
  const countingRegistry = new ReleaseRegistry({ capabilityExecutor: async () => ({ value: "too-many", records: [{}, {}] }) });
  try {
    await countingRegistry.prepare(counted);
    countingRegistry.activate(counted.scopeKey, counted.releaseHash);
    const result = await countingRegistry.execute({ scopeKey: counted.scopeKey, releaseHash: counted.releaseHash, operationId: "secret", exposure: "agent", scopeToken: "scope" });
    assert.match(result.errors?.[0]?.message || "", /document budget exceeded/);
  } finally { countingRegistry.close(); }

  const hanging = release("timeout-peers", bundleV1.replace('value: () => "r1"', 'value: async () => await new Promise(() => {})'));
  for (const resolver of Object.values(hanging.manifest.graphql.resolvers)) resolver.budget.timeoutMs = 20;
  const peerRegistry = new ReleaseRegistry({ capabilityExecutor: async () => await new Promise(() => {}) });
  try {
    await peerRegistry.prepare(hanging);
    peerRegistry.activate(hanging.scopeKey, hanging.releaseHash);
    const peer = peerRegistry.invokeLambda({ scopeKey: hanging.scopeKey, releaseHash: hanging.releaseHash, name: "rollup", scopeToken: "scope" });
    const peerOutcome = assert.rejects(() => Promise.race([peer, new Promise((_, reject) => setTimeout(() => reject(new Error("peer request remained pending")), 200))]), /closed/);
    await assert.rejects(() => peerRegistry.execute({ scopeKey: hanging.scopeKey, releaseHash: hanging.releaseHash, operationId: "value" }), { code: "TIMEOUT" });
    await peerOutcome;
  } finally { peerRegistry.close(); }
});
