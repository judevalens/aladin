// The host half of the shard bridge (protocol "bridge/1" — the kit's envelope).
//
// A shard iframe is sandboxed to an OPAQUE origin: postMessage is its only
// channel out, and this module is what answers. One host is attached per shard
// iframe (DocSurfaceUI); it filters strictly on the source WINDOW — the reliable
// identity check for opaque-origin frames, whose event.origin is "null" — and on
// the { aladin: "bridge/1" } envelope. Replies target "*" because an opaque
// origin cannot be named in targetOrigin; the source-window check is the gate
// (same accepted trade-off as the previous host, see SHARD_MODEL.md).
//
// Surface: hello / theme.* (M1) + kv.* — the shard-local-state plane
// (design/SHARD_LOCAL_STATE.md) proxied to the ShardKVPort (replica reads on
// desktop, REST on web; REST writes everywhere; published channel only — the
// host owns channel selection, shards never see it). nodes.* (the manifest-
// granted workspace plane) lands behind the same switch. Unknown methods are
// REJECTED with code "unknown-method" so a kit call fails fast instead of
// hanging into its 8s timeout.

import { ShardKVConflictError } from "@/shared/api/shard-api";
import type { ShardKVEntryWire } from "@/shared/api/shard-api";
import type { ShardKVPort } from "@/modules/doc-surface/bridge/shard-kv-port";

const BRIDGE = "bridge/1";

const METHODS = [
  "hello",
  "theme.get",
  "kv.get",
  "kv.list",
  "kv.set",
  "kv.delete",
  "kv.subscribe",
  "kv.unsubscribe",
] as const;
const CAPABILITIES = ["theme", "kv"] as const;

type BridgeRequest = {
  aladin?: string;
  type?: string;
  id?: number;
  method?: string;
  params?: Record<string, unknown>;
};

export interface BridgeHostDeps {
  pageId: string;
  /** The shard iframe's contentWindow (null while unmounted/reloading). */
  getWindow: () => Window | null | undefined;
  /** Current Aladin theme name; read at answer time, never cached. */
  getTheme: () => string;
  /** Shard local state storage (published channel). */
  kv: ShardKVPort;
}

export interface BridgeHost {
  attach(): void;
  detach(): void;
  /** Push a theme switch into the shard (kit re-stamps data-theme). */
  pushTheme(theme: string): void;
}

// The push payload for one key change on a kv subscription channel.
function kvPush(entry: ShardKVEntryWire): Record<string, unknown> {
  return { key: entry.key, value: entry.value, revision: entry.revision, deleted: !!entry.deleted };
}

export function createBridgeHost(deps: BridgeHostDeps): BridgeHost {
  let listener: ((e: MessageEvent) => void) | null = null;
  // channel -> subscribed prefix. One replica change stream per attached host,
  // demuxed to channels by prefix — and torn down with the host (LRU eviction /
  // draft reload), never trusted to the shard's own unsubscribe.
  const kvSubs = new Map<string, string>();
  let stopChanges: (() => void) | null = null;

  function reply(to: Window, id: number | undefined, ok: boolean, body: Record<string, unknown>) {
    to.postMessage({ aladin: BRIDGE, type: "response", id, ok, ...body }, "*");
  }

  function push(channel: string, data: unknown) {
    const w = deps.getWindow();
    if (w) w.postMessage({ aladin: BRIDGE, type: "push", channel, data }, "*");
  }

  function ensureChangeStream() {
    if (stopChanges) return;
    stopChanges = deps.kv.changes(deps.pageId, (change) => {
      for (const [channel, prefix] of kvSubs) {
        const key = change.kind === "upsert" ? change.entry.key : change.key;
        if (!key.startsWith(prefix)) continue;
        if (change.kind === "upsert") push(channel, kvPush(change.entry));
        else push(channel, { key, value: null, revision: 0, deleted: true });
      }
    });
  }

  function fail(to: Window, id: number | undefined, err: unknown) {
    if (err instanceof ShardKVConflictError) {
      reply(to, id, false, {
        error: err.message,
        code: "conflict",
        data: {
          key: err.key,
          currentRevision: err.currentRevision,
          currentValue: err.currentValue,
          deleted: err.deleted,
        },
      });
      return;
    }
    const message = err instanceof Error ? err.message : String(err);
    let code = "error";
    if (/quota/i.test(message)) code = "quota";
    else if (/too large/i.test(message)) code = "too-large";
    else if (/invalid (key|channel)|baseRevision/i.test(message)) code = "bad-request";
    reply(to, id, false, { error: message, code });
  }

  function onMessage(event: MessageEvent) {
    // Opaque-origin frames report origin "null"; the source window is the check.
    if (!event.source || event.source !== deps.getWindow()) return;
    const m = event.data as BridgeRequest | null;
    if (!m || m.aladin !== BRIDGE || m.type !== "request") return;
    const to = event.source as Window;
    const p = m.params ?? {};
    switch (m.method) {
      case "hello":
        reply(to, m.id, true, {
          data: {
            protocol: BRIDGE,
            theme: deps.getTheme(),
            capabilities: [...CAPABILITIES],
            methods: [...METHODS],
          },
        });
        return;
      case "theme.get":
        reply(to, m.id, true, { data: { theme: deps.getTheme() } });
        return;
      case "kv.get":
        deps.kv
          .get(deps.pageId, String(p.key ?? ""))
          .then((entry) => reply(to, m.id, true, { data: entry ? kvPush(entry) : null }))
          .catch((err: unknown) => fail(to, m.id, err));
        return;
      case "kv.list":
        deps.kv
          .list(deps.pageId, String(p.prefix ?? ""))
          .then((entries) => reply(to, m.id, true, { data: { entries: entries.map(kvPush) } }))
          .catch((err: unknown) => fail(to, m.id, err));
        return;
      case "kv.set":
        deps.kv
          .set(deps.pageId, String(p.key ?? ""), p.value, Number(p.baseRevision ?? 0))
          .then((entry) =>
            reply(to, m.id, true, { data: { revision: entry.revision, updatedAt: entry.updatedAt } }),
          )
          .catch((err: unknown) => fail(to, m.id, err));
        return;
      case "kv.delete":
        deps.kv
          .remove(deps.pageId, String(p.key ?? ""), Number(p.baseRevision ?? 0))
          .then(() => reply(to, m.id, true, { data: true }))
          .catch((err: unknown) => fail(to, m.id, err));
        return;
      case "kv.subscribe": {
        const channel = String(p.channel ?? "");
        const prefix = String(p.prefix ?? "");
        if (!channel) {
          reply(to, m.id, false, { error: "channel is required", code: "bad-request" });
          return;
        }
        kvSubs.set(channel, prefix);
        ensureChangeStream();
        reply(to, m.id, true, { data: true });
        // Contract: push current entries, then live updates.
        deps.kv
          .list(deps.pageId, prefix)
          .then((entries) => entries.forEach((e) => push(channel, kvPush(e))))
          .catch(() => {});
        return;
      }
      case "kv.unsubscribe":
        kvSubs.delete(String(p.channel ?? ""));
        reply(to, m.id, true, { data: true });
        return;
      default:
        reply(to, m.id, false, {
          error: `unknown method: ${String(m.method)}`,
          code: "unknown-method",
        });
    }
  }

  return {
    attach() {
      if (listener) return;
      listener = onMessage;
      window.addEventListener("message", listener);
    },
    detach() {
      if (!listener) return;
      window.removeEventListener("message", listener);
      listener = null;
      kvSubs.clear();
      stopChanges?.();
      stopChanges = null;
    },
    pushTheme(theme: string) {
      push("theme", { theme });
    },
  };
}
