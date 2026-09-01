import { describe, expect, it, vi } from "vitest";
import { BridgeResourceTransport, WindowResourceBridgePort } from "@/modules/doc-surface/data/bridge-transport";
import type { ResourceBridgePort } from "@/modules/doc-surface/data/bridge-transport";
import type { StreamReceiver } from "@/modules/doc-surface/data/resource-client";

const identity = { subscriptionId: "s1", resource: "shard://test/resources/items?view=v1", epoch: "epoch1" };
const descriptor = { kind: "collection", schemaVersion: 1, schema: { type: "object" }, capabilities: ["snapshot", "observe"], observation: "ordered-changes", delivery: "deltas", limit: 100 };
const snapshot = { protocol: "shard-data/1", ...identity, op: "snapshot", seq: "0", records: [], complete: true };
const request = { binding: "items", inputs: {} };
const tick = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); };
class Port implements ResourceBridgePort {
  listeners = new Map<string, (value: unknown) => void>();
  acknowledge!: (value: unknown) => void;
  description = descriptor;
  call = vi.fn(async (method: string, _params: Record<string, unknown>): Promise<unknown> => {
    if (method === "hello") return { protocol: "bridge/2" };
    if (method === "resource.describe") return this.description;
    if (method === "resource.subscribe") return new Promise(resolve => { this.acknowledge = resolve; });
    if (method === "resource.read") return { resource: identity.resource, records: [], complete: true, sourceUpdatedAt: "2026-08-30T00:00:00Z" };
    return {};
  });
  listen(channel: string, listener: (value: unknown) => void) {
    this.listeners.set(channel, listener);
    return () => { this.listeners.delete(channel); };
  }
  push(channel: string, value: unknown) { this.listeners.get(channel)?.(value); }
}
function setup() {
  const port = new Port(), transport = new BridgeResourceTransport(port), controller = new AbortController();
  const receiver: StreamReceiver = { event: vi.fn(), error: vi.fn() };
  return { port, transport, controller, receiver };
}

describe("bridge/2 resource adapter", () => {
  it("buffers events and revocation before the subscribe acknowledgement", async () => {
    const { port, transport, controller, receiver } = setup();
    const opening = transport.open(request, receiver, controller.signal); await tick();
    port.push("resource.event", snapshot);
    port.push("resource.error", { ...identity, code: "forbidden", message: "Revoked" });
    expect(receiver.event).not.toHaveBeenCalled();
    port.acknowledge(identity); const opened = await opening;
    expect(receiver.event).toHaveBeenCalledWith(snapshot);
    expect(receiver.error).toHaveBeenCalledWith({ code: "forbidden", message: "Revoked" });
    opened.close(); expect(port.listeners.size).toBe(0);
    expect(port.call).toHaveBeenCalledWith("resource.unsubscribe", { subscriptionId: "s1" });
  });
  it("unsubscribes a late acknowledgement after cancellation", async () => {
    const { port, transport, controller, receiver } = setup();
    const opening = transport.open(request, receiver, controller.signal); await tick();
    controller.abort(); port.acknowledge(identity);
    await expect(opening).rejects.toMatchObject({ name: "AbortError" });
    expect(port.call).toHaveBeenCalledWith("resource.unsubscribe", { subscriptionId: "s1" });
    expect(port.listeners.size).toBe(0);
  });
  it("ignores controls for a retired epoch", async () => {
    const { port, transport, controller, receiver } = setup();
    const opening = transport.open(request, receiver, controller.signal); await tick();
    port.acknowledge(identity); const opened = await opening;
    port.push("resource.status", { ...identity, epoch: "old", status: "forbidden" });
    expect(receiver.error).not.toHaveBeenCalled();
    port.push("resource.status", { ...identity, status: "disconnected" });
    expect(receiver.error).toHaveBeenCalledWith(expect.objectContaining({ code: "resync-required" }));
    opened.close();
  });
  it("bounds the bytes buffered while waiting for an acknowledgement", async () => {
    const { port, transport, controller, receiver } = setup();
    const opening = transport.open(request, receiver, controller.signal); await tick();
    for (let i = 0; i < 6; i++) port.push("resource.event", { ...snapshot, padding: "x".repeat(900_000) });
    expect(receiver.error).toHaveBeenCalledOnce();
    expect(port.listeners.size).toBe(0);
    port.acknowledge(identity); await expect(opening).rejects.toMatchObject({ name: "AbortError" });
  });
  it("normalizes reads without inventing a source freshness timestamp", async () => {
    const { port, transport, controller, receiver } = setup();
    port.description = { ...descriptor, capabilities: ["snapshot"] };
    const opened = await transport.open(request, receiver, controller.signal);
    expect(receiver.event).toHaveBeenCalledWith(expect.objectContaining({ op: "snapshot", seq: "0", sourceUpdatedAt: "2026-08-30T00:00:00Z", records: [] }));
    expect(port.call.mock.calls.some(([method]) => method === "resource.subscribe")).toBe(false);
    opened.close();
  });
  it("validates query records and limits against the authorized descriptor", async () => {
    const page = { resource: identity.resource, complete: true, records: [{ id: "one", revision: "1", schemaVersion: 1, data: { title: "First" } }], nextCursor: "next" };
    const call = vi.fn(async (method: string) => method === "hello" ? { protocol: "bridge/2" } : method === "resource.describe" ? { ...descriptor, limit: 1, schema: { type: "object", properties: { title: { type: "string" } }, required: ["title"], additionalProperties: false } } : structuredClone(page));
    const transport = new BridgeResourceTransport({ call, listen: () => () => {} });
    const result = await transport.query(request, { limit: 1 });
    expect(result.nextCursor).toBe("next"); expect(Object.isFrozen(result.records)).toBe(true);
    expect(call).toHaveBeenCalledWith("resource.query", { ...request, query: { limit: 1 } }, undefined);
    page.records = [{ id: "bad", revision: "2", schemaVersion: 2, data: { title: "Bad version" } }];
    await expect(transport.query(request, { limit: 1 })).rejects.toThrow("schema version mismatch");
  });
});

describe("window bridge isolation", () => {
  it("accepts only the parent window and validates response envelopes", async () => {
    const parent = { postMessage: vi.fn() } as unknown as Window;
    const port = new WindowResourceBridgePort(window, parent);
    try {
      const response = port.call("hello", {});
      const message = { aladin: "bridge/2", type: "response", id: 1, ok: true, data: { protocol: "bridge/2" } };
      let resolved = false; void response.then(() => { resolved = true; });
      window.dispatchEvent(new MessageEvent("message", { source: window, data: message })); await tick();
      expect(resolved).toBe(false);
      window.dispatchEvent(new MessageEvent("message", { source: parent, data: message }));
      await expect(response).resolves.toEqual({ protocol: "bridge/2" });
      const invalid = port.call("theme.get", {});
      window.dispatchEvent(new MessageEvent("message", { source: parent, data: { ...message, id: 2, extra: true } }));
      await expect(invalid).rejects.toMatchObject({ code: "bad-request" });
    } finally { port.close(); }
  });
  it("rejects caller-supplied authority and pending calls on detach", async () => {
    const parent = { postMessage: vi.fn() } as unknown as Window;
    const port = new WindowResourceBridgePort(window, parent);
    await expect(port.call("resource.read", { binding: "items", environment: "published" })).rejects.toBeDefined();
    expect(parent.postMessage).not.toHaveBeenCalled();
    const pending = port.call("hello", {}); port.close();
    await expect(pending).rejects.toMatchObject({ code: "source-unavailable" });
  });
});
