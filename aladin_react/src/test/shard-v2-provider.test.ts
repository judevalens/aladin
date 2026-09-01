import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import trace from "../../../shared/shard-v2/fixtures/stream.json";
import { ResourceClient } from "@/modules/doc-surface/data/resource-client";
import type { Mutation, MutationResult, OpenedResource, ResourceRequest, ResourceTransport, StreamReceiver } from "@/modules/doc-surface/data/resource-client";
import { EMPTY_REDUCTION, reduceResource } from "@/modules/doc-surface/data/resource-store";
import type { ResourceDescriptor, SubscriptionIdentity } from "@/modules/doc-surface/data/resource-store";
import { createUseResource } from "@/modules/doc-surface/data/use-resource";

const descriptor: ResourceDescriptor = {
  kind: "collection", schemaVersion: 1, schema: trace.resource.schema,
  capabilities: ["snapshot", "observe", "insert", "update", "delete"],
  observation: "ordered-changes", delivery: "deltas", limit: 100,
};
const record = { id: "a", revision: "1", schemaVersion: 1, data: { title: "before", value: 2 } };
const route = trace.route as SubscriptionIdentity;
const event = (seq: string, payload: Record<string, unknown>, identity = route) => ({ protocol: "shard-data/1", ...identity, seq, ...payload });
const snapshot = (identity = route, records = [record]) => event("0", { op: "snapshot", records, complete: true }, identity);

class MockTransport implements ResourceTransport {
  calls: { request: ResourceRequest; receiver: StreamReceiver; signal: AbortSignal; ack: (value: OpenedResource) => void; close: ReturnType<typeof vi.fn<() => void>> }[] = [];
  writes: Mutation[] = [];
  mutationResult: Promise<MutationResult> = Promise.resolve({ requestId: "request-1", record });
  open(request: ResourceRequest, receiver: StreamReceiver, signal: AbortSignal): Promise<OpenedResource> {
    return new Promise(ack => this.calls.push({ request, receiver, signal, ack, close: vi.fn() }));
  }
  mutate(_request: ResourceRequest, mutation: Mutation): Promise<MutationResult> {
    this.writes.push(mutation);
    return this.mutationResult;
  }
  acknowledge(index = 0, description = descriptor) {
    const call = this.calls[index];
    const identity = { ...route, subscriptionId: "sub-" + index, epoch: "epoch-" + index };
    call.ack({ descriptor: description, identity, close: call.close });
    return identity;
  }
  publish(index: number, value: unknown) { this.calls[index].receiver.event(value); }
}
const clients: ResourceClient[] = [];
const tick = async () => { for (let i = 0; i < 8; i++) await Promise.resolve(); };
function setup() {
  const transport = new MockTransport();
  const client = new ResourceClient(transport); clients.push(client);
  const handle = client.resource("itemsView");
  const release = handle.subscribe(() => {});
  return { transport, client, handle, release };
}
afterEach(() => { for (const client of clients.splice(0)) client.close(); vi.useRealTimers(); });

describe("deterministic shard-data/1 reducer", () => {
  it("replays the shared duplicate, retired-epoch and gap fixture", () => {
    let state = EMPTY_REDUCTION;
    trace.events.forEach((value, index) => {
      state = reduceResource(state, route, descriptor, value);
      expect({ ids: state.records.map(record => record.id), seq: state.seq, stale: state.stale }).toEqual(trace.expected[index]);
    });
    expect(state.records[0].data).toEqual({ title: "after" });
  });
  it("waits for the acknowledged epoch's seq-zero snapshot", () => {
    expect(reduceResource(EMPTY_REDUCTION, route, descriptor, event("1", { op: "insert", record })).stale).toBe(true);
    expect(reduceResource(EMPTY_REDUCTION, route, descriptor, event("1", { op: "snapshot", records: [], complete: true })).stale).toBe(true);
  });
  it("uses full replacement and never mutates input records", () => {
    const state = reduceResource(EMPTY_REDUCTION, route, descriptor, snapshot());
    const next = reduceResource(state, route, descriptor, event("1", { op: "update", record: { ...record, revision: "2", data: { title: "after" } } }));
    expect(next.records[0].data).toEqual({ title: "after" });
    expect(state.records[0].data.value).toBe(2);
    expect(Object.isFrozen(next.records[0].data)).toBe(true);
  });
  it("resynchronizes unknown updates and conflicting inserts", () => {
    const state = reduceResource(EMPTY_REDUCTION, route, descriptor, snapshot());
    expect(reduceResource(state, route, descriptor, event("1", { op: "insert", record })).stale).toBe(true);
    expect(reduceResource(state, route, descriptor, event("1", { op: "update", record: { ...record, id: "missing" } })).stale).toBe(true);
  });
  it("left-view only changes the view and absent deletion is a no-op", () => {
    const state = reduceResource(EMPTY_REDUCTION, route, descriptor, snapshot());
    const next = reduceResource(state, route, descriptor, event("1", { op: "delete", id: "a", reason: "left-view" }));
    expect(next.records).toEqual([]);
    expect(state.records).toHaveLength(1);
    expect(reduceResource(next, route, descriptor, event("2", { op: "delete", id: "a", reason: "deleted" })).stale).toBe(false);
  });
  it("does not infer sorted-window membership from raw deltas", () => {
    const state = reduceResource(EMPTY_REDUCTION, route, descriptor, snapshot());
    expect(reduceResource(state, route, { ...descriptor, delivery: "snapshots" }, event("1", { op: "delete", id: "a", reason: "left-view" })).stale).toBe(true);
  });
  it("compares sequence strings above the safe integer range exactly", () => {
    const initial = { ...reduceResource(EMPTY_REDUCTION, route, descriptor, snapshot()), seq: "9007199254740992" };
    const next = reduceResource(initial, route, descriptor, event("9007199254740993", { op: "delete", id: "missing", reason: "deleted" }));
    expect(next.seq).toBe("9007199254740993"); expect(next.stale).toBe(false);
  });
  it("keeps the last valid view when an event fails schema validation", () => {
    const state = reduceResource(EMPTY_REDUCTION, route, descriptor, snapshot());
    const next = reduceResource(state, route, descriptor, event("1", { op: "update", record: { ...record, data: { title: 42 } } }));
    expect(next.stale).toBe(true); expect(next.records).toBe(state.records);
  });
});

describe("shared resource client lifecycle", () => {
  it("shares equal binding inputs and releases only after the last consumer", async () => {
    const { client, transport, release } = setup();
    const another = client.resource("itemsView");
    const releaseOther = another.subscribe(() => {});
    expect(transport.calls).toHaveLength(1);
    const identity = transport.acknowledge(); transport.publish(0, snapshot(identity)); await tick();
    release(); expect(transport.calls[0].close).not.toHaveBeenCalled();
    releaseOther(); expect(transport.calls[0].close).toHaveBeenCalledOnce();
    expect(transport.calls[0].signal.aborted).toBe(true);
  });
  it("buffers a snapshot that arrives before acknowledgement", async () => {
    const { transport, handle } = setup();
    const identity = { ...route, subscriptionId: "sub-0", epoch: "epoch-0" };
    transport.publish(0, snapshot(identity)); await tick();
    expect(handle.getSnapshot().records).toEqual([]);
    transport.acknowledge(); await tick();
    expect(handle.getSnapshot().records).toHaveLength(1);
  });
  it("disconnect retains stale data and reconnect obtains a new epoch", async () => {
    vi.useFakeTimers();
    const { transport, handle } = setup();
    const first = transport.acknowledge(); transport.publish(0, snapshot(first)); await tick();
    transport.calls[0].receiver.error({ code: "source-unavailable", message: "Disconnected" });
    expect(handle.getSnapshot().stale).toBe(true); expect(handle.getSnapshot().records).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(250);
    const second = transport.acknowledge(1); transport.publish(1, snapshot(second, [])); await tick();
    transport.publish(0, snapshot(first)); await tick();
    expect(handle.getSnapshot().records).toEqual([]); expect(handle.getSnapshot().stale).toBe(false);
  });
  it("revocation clears data and cannot be revived by refresh or late events", async () => {
    const { transport, handle } = setup();
    const identity = transport.acknowledge(); transport.publish(0, snapshot(identity)); await tick();
    transport.calls[0].receiver.error({ code: "forbidden", message: "Access revoked" });
    handle.refresh(); transport.publish(0, snapshot(identity)); await tick();
    expect(handle.getSnapshot().records).toEqual([]);
    expect(handle.getSnapshot().capabilities).toEqual([]);
    expect(handle.getSnapshot().status).toBe("forbidden");
    expect(transport.calls).toHaveLength(1);
  });
  it("overflow retires the entire generation instead of skipping operations", async () => {
    const { transport, handle } = setup();
    for (let i = 0; i < 1001; i++) transport.publish(0, snapshot());
    expect(handle.getSnapshot().error?.code).toBe("resync-required");
    expect(transport.calls[0].signal.aborted).toBe(true);
    transport.acknowledge(); await tick();
    expect(transport.calls[0].close).toHaveBeenCalledOnce();
  });
  it("does not apply mutation responses or automatically retry commands", async () => {
    const { transport, handle } = setup();
    const identity = transport.acknowledge(); transport.publish(0, snapshot(identity)); await tick();
    const mutation = { id: "a", baseRevision: "1", requestId: "request-1", data: { title: "after" } };
    await handle.update!(mutation);
    expect(transport.writes).toHaveLength(1);
    expect(handle.getSnapshot().records[0].data.title).toBe("before");
    expect(handle.getSnapshot().pending).toEqual(["request-1"]);
    const next = transport.acknowledge(1);
    transport.publish(1, snapshot(next, [{ ...record, revision: "2", data: { title: "after", value: 2 } }])); await tick();
    expect(handle.getSnapshot().pending).toEqual([]);
    expect(handle.getSnapshot().records[0].data.title).toBe("after");
  });
  it("deduplicates in-flight request IDs and rejects a changed payload", async () => {
    const { transport, handle } = setup();
    const identity = transport.acknowledge(); transport.publish(0, snapshot(identity)); await tick();
    const command = { id: "a", data: { title: "x" }, baseRevision: "1", requestId: "request-1" };
    const first = handle.update!(command), second = handle.update!(command);
    expect(first).toBe(second);
    await expect(handle.update!({ ...command, data: { title: "different" } })).rejects.toMatchObject({ code: "conflict" });
    await first; expect(transport.writes).toHaveLength(1);
  });
  it("a read-only resource has no write methods", async () => {
    const { transport, handle } = setup();
    const identity = transport.acknowledge(0, { ...descriptor, capabilities: ["snapshot"], observation: undefined });
    transport.publish(0, snapshot(identity)); await tick();
    expect(handle.update).toBeUndefined(); expect(handle.insert).toBeUndefined(); expect(handle.remove).toBeUndefined();
    expect(handle.getSnapshot().status).toBe("ready");
  });
  it("closes late opens after the last consumer has detached", async () => {
    const { transport, release } = setup();
    release(); transport.acknowledge(); await tick();
    expect(transport.calls[0].close).toHaveBeenCalledOnce();
  });
  it("the React adapter never shows previous-account data as current", async () => {
    const transport = new MockTransport(), client = new ResourceClient(transport); clients.push(client);
    const useResource = createUseResource(client);
    const hook = renderHook(({ account }) => useResource("itemsView", { account }), { initialProps: { account: "A" } });
    await act(async () => { const identity = transport.acknowledge(); transport.publish(0, snapshot(identity)); await tick(); });
    expect(hook.result.current.records).toHaveLength(1);
    hook.rerender({ account: "B" });
    expect(hook.result.current.records).toEqual([]);
    expect(transport.calls[0].signal.aborted).toBe(true);
    hook.unmount(); expect(transport.calls[1].signal.aborted).toBe(true);
  });
});

describe("binding dependencies", () => {
  const bindings = {
    preferences: { resource: "preferences" },
    itemsView: { resource: "items", params: { account: { binding: "preferences", pointer: "/data/account" } } },
  };
  const preferences: ResourceDescriptor = { ...descriptor, kind: "singleton", schema: { type: "object", properties: { account: { type: "string" } } } };
  const preferenceRecord = (account: string) => ({ id: "value", revision: "1", schemaVersion: 1, data: { account } });
  it("waits for a singleton value, retires changed views, and releases dependencies", async () => {
    const transport = new MockTransport(), client = new ResourceClient(transport, bindings); clients.push(client);
    const handle = client.resource("itemsView"), release = handle.subscribe(() => {});
    expect(transport.calls.map(call => call.request.binding)).toEqual(["preferences"]);
    const dependency = transport.acknowledge(0, preferences);
    transport.publish(0, snapshot(dependency, [])); await tick();
    expect(transport.calls).toHaveLength(1);
    transport.publish(0, event("1", { op: "insert", record: preferenceRecord("A") }, dependency)); await tick();
    expect(transport.calls[1].request.binding).toBe("itemsView");
    const first = transport.acknowledge(1); transport.publish(1, snapshot(first)); await tick();
    expect(handle.getSnapshot().records).toHaveLength(1);
    transport.publish(0, event("2", { op: "update", record: preferenceRecord("B") }, dependency)); await tick();
    expect(handle.getSnapshot().records).toEqual([]);
    expect(transport.calls[1].signal.aborted).toBe(true);
    expect(transport.calls).toHaveLength(3);
    transport.publish(1, snapshot(first)); await tick();
    expect(handle.getSnapshot().records).toEqual([]);
    transport.acknowledge(2); await tick(); release();
    expect(transport.calls.every(call => call.signal.aborted)).toBe(true);
  });
  it("propagates dependency revocation without opening the dependent view", async () => {
    const transport = new MockTransport(), client = new ResourceClient(transport, bindings); clients.push(client);
    const handle = client.resource("itemsView"); handle.subscribe(() => {});
    transport.calls[0].receiver.error({ code: "forbidden", message: "Revoked" }); await tick();
    expect(handle.getSnapshot().status).toBe("forbidden");
    expect(transport.calls).toHaveLength(1);
  });
  it("rejects dependency cycles before starting subscriptions", () => {
    expect(() => new ResourceClient(new MockTransport(), { self: { resource: "x", params: { account: { binding: "self", pointer: "/data/account" } } } })).toThrow("cycle");
  });
});
