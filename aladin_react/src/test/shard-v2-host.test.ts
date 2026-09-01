import { afterEach, describe, expect, it, vi } from "vitest";
import { createResourceHostHub } from "@/modules/doc-surface/bridge/resource-host-hub";
import { createBridgeV2Host } from "@/modules/doc-surface/bridge/bridge-v2-host";
import type { ApiClient } from "@/shared/api/client";

class Socket {
  readyState = 0;
  bufferedAmount = 0;
  onopen?: () => void;
  onclose?: (event: { code: number }) => void;
  onmessage?: (event: { data: string }) => void;
  onerror?: () => void;
  sent: { target: unknown; request: { id: number; method: string; params: Record<string, unknown> } }[] = [];
  send(raw: string) { this.sent.push(JSON.parse(raw)); }
  open() { this.readyState = 1; this.onopen?.(); }
  close(code = 1000) { this.readyState = 3; this.onclose?.({ code }); }
  receive(value: unknown) { this.onmessage?.({ data: JSON.stringify(value) }); }
  ack(index: number, data: unknown) { this.receive({ aladin: "bridge/2", type: "response", id: this.sent[index].request.id, ok: true, data }); }
}
const target = { shardId: "artifact-one", environment: "draft" as const, contractHash: "hash-one" };
const identity = (id: string) => ({ subscriptionId: id, epoch: "epoch", resource: `shard://artifact-one/resources/tasks?view=${id}` });
const flush = async () => { for (let i = 0; i < 6; i++) await Promise.resolve(); };
const cleanups: (() => void)[] = [];
afterEach(() => { for (const close of cleanups.splice(0)) close(); vi.useRealTimers(); });
function setup() {
  let token = "session-one";
  const sockets: Socket[] = [];
  const fetch = vi.fn(async (_path: string, init?: RequestInit) => {
    const req = JSON.parse(String(init?.body));
    return { aladin: "bridge/2", type: "response", id: req.id, ok: true, data: { protocol: "bridge/2", buildId: "build-one", contractHash: "hash-one" } };
  });
  const hub = createResourceHostHub({ fetch } as unknown as ApiClient, "ws://localhost/api/shard-resources/ws", () => token, () => {
    const socket = new Socket(); sockets.push(socket); return socket as unknown as WebSocket;
  });
  cleanups.push(hub.close);
  return { hub, sockets, fetch, setToken: (value: string) => { token = value; } };
}
describe("v2 host isolation", () => {
  it("shares one socket, routes only matching subscriptions, and isolates detach", async () => {
    const { hub, sockets } = setup();
    const pushA = vi.fn(), pushB = vi.fn();
    const a = hub.session(target, pushA), b = hub.session({ ...target, shardId: "artifact-two" }, pushB);
    const readyA = a.call("resource.subscribe", { binding: "tasks" });
    const readyB = b.call("resource.subscribe", { binding: "tasks" });
    expect(sockets).toHaveLength(1);
    const ws = sockets[0]; ws.open(); await flush();
    ws.ack(0, identity("a")); ws.ack(1, identity("b"));
    await Promise.all([readyA, readyB]);
    ws.receive({ aladin: "bridge/2", type: "push", channel: "resource.event", data: identity("a") });
    expect(pushA).toHaveBeenCalledOnce(); expect(pushB).not.toHaveBeenCalled();
    ws.receive({ aladin: "bridge/2", type: "push", channel: "resource.event", data: { ...identity("a"), epoch: "wrong" } });
    expect(pushA).toHaveBeenCalledOnce();
    expect(await b.call("resource.unsubscribe", { subscriptionId: "a" })).toBe(true);
    expect(ws.sent).toHaveLength(2);
    a.close(); expect(ws.readyState).toBe(1);
    expect(ws.sent[2].request.method).toBe("resource.unsubscribe");
    ws.close(1006);
    expect(pushB).toHaveBeenCalledWith("resource.error", expect.objectContaining({ code: "resync-required" }));
  });
  it("cleans a subscribe acknowledged after its frame detaches", async () => {
    const { hub, sockets } = setup();
    const a = hub.session(target, vi.fn());
    hub.session({ ...target, shardId: "other" }, vi.fn());
    const ready = a.call("resource.subscribe", { binding: "tasks" });
    const ws = sockets[0]; ws.open(); await flush(); a.close();
    ws.ack(0, identity("late")); await ready;
    expect(ws.sent[1].request.params.subscriptionId).toBe("late");
  });
  it("rejects subscriptions when the browser send queue is full", async () => {
    const { hub, sockets } = setup();
    const a = hub.session(target, vi.fn());
    const ready = a.call("resource.subscribe", { binding: "tasks" });
    const assertion = expect(ready).rejects.toBeDefined();
    sockets[0].bufferedAmount = 4 << 20; sockets[0].open();
    await assertion;
    expect(sockets[0].sent).toHaveLength(0);
  });
  it("pins HTTP context and purges a stream when the account changes", async () => {
    const { hub, sockets, fetch, setToken } = setup();
    const push = vi.fn(), a = hub.session(target, push);
    await a.call("hello", {});
    expect(fetch.mock.calls[0][0]).toContain("/artifact-one/v2/draft/request");
    expect(fetch.mock.calls[0][1]?.headers).toEqual({ "X-Shard-Contract": "hash-one" });
    const ready = a.call("resource.subscribe", { binding: "tasks" });
    const ws = sockets[0]; ws.open(); await flush(); ws.ack(0, identity("a")); await ready;
    setToken("another-account");
    await expect(a.call("hello", {})).rejects.toMatchObject({ code: "forbidden" });
    expect(push).toHaveBeenCalledWith("resource.error", expect.objectContaining({ code: "forbidden" }));
  });
  it("rejects forged inputs and other windows; drops replies after detach", async () => {
    const { hub, fetch } = setup();
    const posted: unknown[] = [];
    const frame = { postMessage: (m: unknown) => posted.push(m) } as unknown as Window;
    const host = createBridgeV2Host({ target, buildId: "build-one", getWindow: () => frame, getTheme: () => "light", hub });
    host.attach(); cleanups.push(host.detach);
    const request = (source: Window, params: object, method = "hello") => window.dispatchEvent(new MessageEvent("message", { source, data: { aladin: "bridge/2", type: "request", id: 1, method, params } }));
    request(window, {}); await flush(); expect(fetch).not.toHaveBeenCalled();
    request(frame, { environment: "published", binding: "tasks" }, "resource.read"); await flush();
    expect(fetch).not.toHaveBeenCalled(); expect(posted[0]).toMatchObject({ ok: false });
    request(frame, {}); host.detach(); await flush(); expect(posted).toHaveLength(1);
  });
});
