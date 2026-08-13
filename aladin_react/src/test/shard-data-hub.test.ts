import { describe, expect, it } from "vitest";
import { createShardDataHub } from "@/modules/doc-surface/bridge/shard-data-hub";
import type { NodeViewWire, ShardApi } from "@/shared/api/shard-api";
import type { AppEventEnvelope } from "@/shared/realtime/app-event";

// The hub is where the four liveness hazards get handled: the replay-less
// websocket, out-of-order refetches, frame storms, and iframes that vanish
// without unsubscribing. Each is pinned here.

function frame(entities: Array<{ entityKind: string; entityId: string; seq?: number }>): AppEventEnvelope {
  return {
    eventId: `evt-${Math.random()}`,
    type: "*.frame",
    subscriptionKey: { stream: "workspace", resourceKind: "*", resourceId: "*" },
    payload: { entities },
    occurredAt: new Date().toISOString(),
  };
}

function fakeApi(seed: Record<string, NodeViewWire> = {}) {
  const calls: string[][] = [];
  const table = new Map(Object.entries(seed));
  const api = {
    bridgeNodes: async (_shardId: string, ids: string[]) => {
      calls.push([...ids]);
      return { nodes: ids.map((id) => table.get(id)).filter(Boolean) as NodeViewWire[], missing: [] };
    },
  } as unknown as ShardApi;
  return { api, calls, table };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe("createShardDataHub", () => {
  it("refetches only subscribed entities and pushes them", async () => {
    const { api, calls } = fakeApi({
      "artifact-a": { id: "artifact-a", kind: "artifact", title: "A", seq: "2" },
    });
    const hub = createShardDataHub(api);
    const pushed: NodeViewWire[] = [];
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["artifact-a"]), push: (n) => pushed.push(n) });

    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
    await flush();
    expect(calls).toEqual([["artifact-a"]]);
    expect(pushed).toHaveLength(1);

    // An unrelated entity triggers no work at all.
    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-other" }]));
    await flush();
    expect(calls).toHaveLength(1);
  });

  it("matches qualified refs (watchlist:<id>) against a bare frame id", async () => {
    const { api, calls } = fakeApi({
      "watchlist:w-1": { id: "watchlist:w-1", kind: "watchlist", title: "Semis" },
    });
    const hub = createShardDataHub(api);
    const pushed: NodeViewWire[] = [];
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["watchlist:w-1"]), push: (n) => pushed.push(n) });

    hub.handleFrame(frame([{ entityKind: "watchlist", entityId: "w-1" }]));
    await flush();
    expect(calls).toEqual([["watchlist:w-1"]]);
    expect(pushed[0].title).toBe("Semis");
  });

  it("coalesces a frame storm into ONE refetch per shard", async () => {
    const { api, calls } = fakeApi({
      "artifact-a": { id: "artifact-a", kind: "artifact", title: "A", seq: "1" },
      "artifact-b": { id: "artifact-b", kind: "artifact", title: "B", seq: "1" },
    });
    const hub = createShardDataHub(api);
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["artifact-a", "artifact-b"]), push: () => {} });

    for (let i = 0; i < 10; i++) {
      hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
      hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-b" }]));
    }
    await flush();
    expect(calls).toHaveLength(1);
    expect(calls[0].sort()).toEqual(["artifact-a", "artifact-b"]);
  });

  it("drops stale pushes by seq (out-of-order refetch)", async () => {
    const { api, table } = fakeApi({
      "artifact-a": { id: "artifact-a", kind: "artifact", title: "v5", seq: "5" },
    });
    const hub = createShardDataHub(api);
    const pushed: NodeViewWire[] = [];
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["artifact-a"]), push: (n) => pushed.push(n) });

    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
    await flush();
    expect(pushed).toHaveLength(1);

    // A late refetch resolving an OLDER version must not overwrite v5.
    table.set("artifact-a", { id: "artifact-a", kind: "artifact", title: "v3", seq: "3" });
    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
    await flush();
    expect(pushed).toHaveLength(1);

    // A newer one does push.
    table.set("artifact-a", { id: "artifact-a", kind: "artifact", title: "v6", seq: "6" });
    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
    await flush();
    expect(pushed.map((n) => n.title)).toEqual(["v5", "v6"]);
  });

  it("always pushes kinds that carry no seq (e.g. watchlists)", async () => {
    const { api } = fakeApi({ "watchlist:w-1": { id: "watchlist:w-1", kind: "watchlist", title: "Semis" } });
    const hub = createShardDataHub(api);
    const pushed: NodeViewWire[] = [];
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["watchlist:w-1"]), push: (n) => pushed.push(n) });

    hub.handleFrame(frame([{ entityKind: "watchlist", entityId: "w-1" }]));
    await flush();
    hub.handleFrame(frame([{ entityKind: "watchlist", entityId: "w-1" }]));
    await flush();
    expect(pushed).toHaveLength(2);
  });

  it("refetchAll reconciles every subscription after a reconnect gap", async () => {
    const { api, calls } = fakeApi({
      "artifact-a": { id: "artifact-a", kind: "artifact", title: "A", seq: "1" },
    });
    const hub = createShardDataHub(api);
    const pushed: NodeViewWire[] = [];
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["artifact-a"]), push: (n) => pushed.push(n) });

    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
    await flush();
    expect(pushed).toHaveLength(1);

    // Same seq, but a reconnect clears the seq view so the value is re-pushed
    // (the client can't know what it missed).
    hub.refetchAll();
    await flush();
    expect(calls).toHaveLength(2);
    expect(pushed).toHaveLength(2);
  });

  it("dropShard stops all work for an evicted iframe", async () => {
    const { api, calls } = fakeApi({
      "artifact-a": { id: "artifact-a", kind: "artifact", title: "A", seq: "1" },
    });
    const hub = createShardDataHub(api);
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["artifact-a"]), push: () => {} });
    hub.dropShard("shard-1");

    hub.handleFrame(frame([{ entityKind: "artifact", entityId: "artifact-a" }]));
    hub.refetchAll();
    await flush();
    expect(calls).toHaveLength(0);
  });

  it("ignores non-frame events", async () => {
    const { api, calls } = fakeApi();
    const hub = createShardDataHub(api);
    hub.subscribe("shard-1", { channel: "sub:1", ids: new Set(["artifact-a"]), push: () => {} });
    hub.handleFrame({ ...frame([{ entityKind: "artifact", entityId: "artifact-a" }]), type: "artifact.build-status" });
    await flush();
    expect(calls).toHaveLength(0);
  });
});
