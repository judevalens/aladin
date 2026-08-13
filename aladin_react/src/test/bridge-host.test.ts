import { afterEach, describe, expect, it, vi } from "vitest";
import { createBridgeHost } from "@/modules/doc-surface/bridge/bridge-host";
import type { BridgeHost } from "@/modules/doc-surface/bridge/bridge-host";
import type { ShardKVPort, ShardKVPortChange } from "@/modules/doc-surface/bridge/shard-kv-port";
import { ShardKVConflictError } from "@/shared/api/shard-api";
import type { ShardKVEntryWire } from "@/shared/api/shard-api";

// The bridge host filters on the SOURCE WINDOW (opaque-origin frames have
// origin "null"), so the tests drive it with synthetic MessageEvents whose
// source is a stub "iframe window" capturing postMessage calls.

type Posted = { msg: Record<string, unknown>; origin: string };

function stubWindow() {
  const posted: Posted[] = [];
  const w = {
    postMessage: (msg: Record<string, unknown>, origin: string) => {
      posted.push({ msg, origin });
    },
  } as unknown as Window;
  return { w, posted };
}

function request(host: { source: Window }, body: Record<string, unknown>) {
  window.dispatchEvent(
    new MessageEvent("message", { data: { aladin: "bridge/1", type: "request", ...body }, source: host.source as unknown as MessageEventSource }),
  );
}

// An in-memory ShardKVPort with the real revision-guard semantics, plus a
// controllable change stream (what the replica would feed).
function fakeKVPort() {
  const store = new Map<string, { value: unknown; revision: number }>();
  let emit: ((c: ShardKVPortChange) => void) | null = null;
  let changeSubs = 0;
  const wire = (key: string): ShardKVEntryWire => ({
    shardId: "artifact-1",
    channel: "published",
    key,
    value: store.get(key)?.value,
    revision: store.get(key)?.revision ?? 0,
  });
  const port: ShardKVPort = {
    async list(_shard, prefix = "") {
      return [...store.keys()].filter((k) => k.startsWith(prefix)).map(wire);
    },
    async get(_shard, key) {
      return store.has(key) ? wire(key) : null;
    },
    async set(_shard, key, value, baseRevision) {
      const cur = store.get(key);
      const rev = cur?.revision ?? 0;
      if (baseRevision !== rev) throw new ShardKVConflictError(key, rev, cur?.value, false);
      store.set(key, { value, revision: rev + 1 });
      return wire(key);
    },
    async remove(_shard, key, baseRevision) {
      const cur = store.get(key);
      if (cur && baseRevision !== cur.revision) throw new ShardKVConflictError(key, cur.revision, cur.value, false);
      store.delete(key);
    },
    changes(_shard, cb) {
      changeSubs++;
      emit = cb;
      return () => {
        changeSubs--;
        emit = null;
      };
    },
  };
  return {
    port,
    store,
    pushChange: (c: ShardKVPortChange) => emit?.(c),
    get changeSubs() {
      return changeSubs;
    },
  };
}

async function flush() {
  await new Promise((r) => setTimeout(r, 0));
}

describe("createBridgeHost", () => {
  let host: BridgeHost | null = null;
  afterEach(() => {
    host?.detach();
    host = null;
  });

  function attach(getTheme = () => "dark", kvFake = fakeKVPort()) {
    const { w, posted } = stubWindow();
    host = createBridgeHost({ pageId: "artifact-1", getWindow: () => w, getTheme, kv: kvFake.port });
    host.attach();
    return { w, posted, kvFake };
  }

  it("answers hello with protocol, theme, and capabilities", () => {
    const { w, posted } = attach(() => "cool");
    request({ source: w }, { id: 1, method: "hello" });
    expect(posted).toHaveLength(1);
    expect(posted[0].msg).toMatchObject({
      aladin: "bridge/1",
      type: "response",
      id: 1,
      ok: true,
      data: { protocol: "bridge/1", theme: "cool", capabilities: ["theme", "kv"] },
    });
  });

  it("answers theme.get with the CURRENT theme (read at answer time)", () => {
    let theme = "dark";
    const { w, posted } = attach(() => theme);
    theme = "light";
    request({ source: w }, { id: 2, method: "theme.get" });
    expect(posted[0].msg).toMatchObject({ id: 2, ok: true, data: { theme: "light" } });
  });

  it("rejects unknown methods with code unknown-method (fail fast, no timeout)", () => {
    const { w, posted } = attach();
    request({ source: w }, { id: 3, method: "nodes.get", params: { ids: ["x"] } });
    expect(posted[0].msg).toMatchObject({ id: 3, ok: false, code: "unknown-method" });
  });

  it("ignores messages from any other window (source check)", () => {
    const { posted } = attach();
    const stranger = stubWindow();
    request({ source: stranger.w }, { id: 4, method: "hello" });
    expect(posted).toHaveLength(0);
    expect(stranger.posted).toHaveLength(0);
  });

  it("ignores non-bridge envelopes from the right window", () => {
    const { w, posted } = attach();
    window.dispatchEvent(
      new MessageEvent("message", { data: { type: "aladin:bridge", cmd: "ping", id: 5 }, source: w as unknown as MessageEventSource }),
    );
    expect(posted).toHaveLength(0);
  });

  it("pushTheme pushes on the theme channel; detach stops answering", () => {
    const { w, posted } = attach();
    host!.pushTheme("soft");
    expect(posted[0].msg).toMatchObject({ type: "push", channel: "theme", data: { theme: "soft" } });
    host!.detach();
    request({ source: w }, { id: 6, method: "hello" });
    expect(posted).toHaveLength(1);
  });

  it("attach is idempotent (no double replies)", () => {
    const { w, posted } = attach();
    host!.attach();
    request({ source: w }, { id: 7, method: "hello" });
    expect(posted).toHaveLength(1);
    vi.restoreAllMocks();
  });

  it("kv.set round-trips and kv.get returns the entry", async () => {
    const { w, posted } = attach();
    request({ source: w }, { id: 10, method: "kv.set", params: { key: "filters", value: { q: "a" }, baseRevision: 0 } });
    await flush();
    expect(posted[0].msg).toMatchObject({ id: 10, ok: true, data: { revision: 1 } });
    request({ source: w }, { id: 11, method: "kv.get", params: { key: "filters" } });
    await flush();
    expect(posted[1].msg).toMatchObject({ id: 11, ok: true, data: { key: "filters", value: { q: "a" }, revision: 1 } });
  });

  it("kv.set with a stale baseRevision replies code=conflict carrying the current", async () => {
    const { w, posted } = attach();
    request({ source: w }, { id: 12, method: "kv.set", params: { key: "k", value: 1, baseRevision: 0 } });
    await flush();
    request({ source: w }, { id: 13, method: "kv.set", params: { key: "k", value: 2, baseRevision: 0 } });
    await flush();
    expect(posted[1].msg).toMatchObject({
      id: 13,
      ok: false,
      code: "conflict",
      data: { currentRevision: 1, currentValue: 1 },
    });
  });

  it("kv.subscribe seeds current entries then pushes prefix-matched live changes only", async () => {
    const { w, posted, kvFake } = attach();
    kvFake.store.set("scenario/base", { value: { x: 1 }, revision: 1 });
    kvFake.store.set("settings", { value: true, revision: 1 });
    request({ source: w }, { id: 14, method: "kv.subscribe", params: { prefix: "scenario/", channel: "kv:1" } });
    await flush();
    const seeds = posted.filter((p) => p.msg.type === "push");
    expect(seeds).toHaveLength(1);
    expect(seeds[0].msg).toMatchObject({ channel: "kv:1", data: { key: "scenario/base", revision: 1 } });

    kvFake.pushChange({ kind: "upsert", entry: { shardId: "artifact-1", channel: "published", key: "scenario/stress", value: 2, revision: 1 } });
    kvFake.pushChange({ kind: "upsert", entry: { shardId: "artifact-1", channel: "published", key: "settings", value: false, revision: 2 } });
    kvFake.pushChange({ kind: "delete", key: "scenario/base" });
    const pushes = posted.filter((p) => p.msg.type === "push");
    expect(pushes).toHaveLength(3); // seed + stress upsert + base delete; settings filtered
    expect(pushes[1].msg).toMatchObject({ data: { key: "scenario/stress" } });
    expect(pushes[2].msg).toMatchObject({ data: { key: "scenario/base", deleted: true } });
  });

  it("detach tears down kv subscriptions and the change stream", async () => {
    const { w, kvFake } = attach();
    request({ source: w }, { id: 15, method: "kv.subscribe", params: { prefix: "", channel: "kv:9" } });
    await flush();
    expect(kvFake.changeSubs).toBe(1);
    host!.detach();
    expect(kvFake.changeSubs).toBe(0);
  });
});
