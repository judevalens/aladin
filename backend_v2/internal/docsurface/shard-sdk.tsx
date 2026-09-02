// @aladin/shard — nonvisual runtime SDK for authored shards.
//
// Theme colors, typography, spacing and radii are injected into every shard as
// CSS variables and Tailwind utilities. UI remains ordinary React/HTML/CSS.

import { useEffect, useState, useSyncExternalStore } from "react";
export {
  useResource,
  queryResource,
  resourceRequestId,
  executeGraphQL,
  invokeLambda,
} from "./resource-client.generated.js";
import { allocateBridgeRequestID } from "./resource-client.generated.js";

// --- L5 bridge (host ↔ shard channel: nodes.get / nodes.subscribe) -----------
//
// A shard is sandboxed (opaque origin); it reaches workspace/graph data only
// through this postMessage bridge. The SHARD code is identical in preview and
// production — it posts to window.parent; in the live app the HOST answers (auth +
// data, scoped to the shard's manifest refs), and in the headless preview a small
// emulator answers with stub nodes so data-wired shards still render. Messages are
// namespaced { aladin: "bridge/1" }; everything else is ignored.

const BRIDGE = typeof document !== "undefined" && document.getElementById("aladin-resource-bootstrap") ? "bridge/2" : "bridge/1";

// Node is a workspace/graph entity a shard depends on (declared in anchors.json
// `refs`). Shape is intentionally generic; `data` carries the entity payload.
export type Node = { id: string; type?: string; title?: string; data?: unknown };

let _seq = 0;
const _pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
const _subs = new Map<string, (n: Node) => void>();
let _wired = false;

// --- theme (host-pushed) ------------------------------------------------------
// The document shell stamps <html data-theme> for a correct first paint and
// applies every host theme push, even when this SDK is not imported. This SDK
// also observes the push so useTheme consumers re-render when they compute
// concrete values from CSS at render time.
let _theme = "";
const _themeSubs = new Set<() => void>();

function applyTheme(theme: unknown) {
  if (typeof theme !== "string" || theme === "" || theme === _theme) return;
  _theme = theme;
  document.documentElement.dataset.theme = theme;
  _themeSubs.forEach((fn) => fn());
}

// BridgeError carries the host's structured failure: `code` discriminates
// ("conflict" | "quota" | "too-large" | "unknown-method" | …) and `data` holds
// the code-specific payload (a conflict's { currentRevision, currentValue }).
export class BridgeError extends Error {
  code: string;
  data: unknown;
  constructor(message: string, code: string, data: unknown) {
    super(message);
    this.name = "BridgeError";
    this.code = code;
    this.data = data;
  }
}

function ensureWired() {
  if (_wired || typeof window === "undefined") return;
  _wired = true;
  window.addEventListener("message", (e: MessageEvent) => {
    const m = e.data as { aladin?: string; type?: string; id?: number; ok?: boolean; data?: unknown; error?: string; code?: string; channel?: string };
    if (!m || m.aladin !== BRIDGE) return;
    if (m.type === "response" && m.id != null && _pending.has(m.id)) {
      const p = _pending.get(m.id)!;
      _pending.delete(m.id);
      if (m.ok) p.resolve(m.data);
      else p.reject(new BridgeError(m.error || "bridge error", m.code || "error", m.data));
    } else if (m.type === "push" && m.channel === "theme") {
      applyTheme((m.data as { theme?: string } | null)?.theme);
    } else if (m.type === "push" && m.channel && _subs.has(m.channel)) {
      _subs.get(m.channel)!(m.data as Node);
    }
  });
  // Seed from the served document, then reconcile with the host (covers a theme
  // switch that happened while this frame was hidden in the keep-alive set, and
  // hosts that serve no stamp). Fire-and-forget: previews without a theme-aware
  // emulator just keep the stamp.
  _theme = document.documentElement.dataset.theme || "";
  post("theme.get", {})
    .then((d) => applyTheme((d as { theme?: string } | null)?.theme))
    .catch(() => {});
}

function post(method: string, params: Record<string, unknown>): Promise<unknown> {
  if (typeof window === "undefined") return Promise.reject(new Error("bridge: no window"));
  ensureWired();
  const id = BRIDGE === "bridge/2" ? allocateBridgeRequestID(window) : ++_seq;
  return new Promise((resolve, reject) => {
    _pending.set(id, { resolve, reject });
    (window.parent || window).postMessage({ aladin: BRIDGE, type: "request", id, method, params }, "*");
    setTimeout(() => {
      if (_pending.has(id)) {
        _pending.delete(id);
        reject(new Error("bridge: timeout on " + method));
      }
    }, 8000);
  });
}

// bridge is the low-level client. Most shards use the useNode/useNodes hooks.
export const bridge = {
  getNodes(ids: string[]): Promise<Node[]> {
    return post("nodes.get", { ids }).then((d) => (d as Node[]) || []);
  },
  getNode(id: string): Promise<Node | null> {
    return post("nodes.get", { ids: [id] }).then((d) => ((d as Node[]) || [])[0] ?? null);
  },
  // subscribe pushes the current value then updates; returns an unsubscribe fn.
  subscribe(ids: string[], cb: (n: Node) => void): () => void {
    ensureWired();
    const channel = "sub:" + ++_seq;
    _subs.set(channel, cb);
    post("nodes.subscribe", { ids, channel }).catch(() => {});
    return () => {
      _subs.delete(channel);
      post("nodes.unsubscribe", { channel }).catch(() => {});
    };
  },
};

// useTheme returns the active Aladin theme name ("dark", "light", …) and
// re-renders on host theme switches. Utilities and var()-based styles follow the
// theme with NO code — reach for this hook only when a value is resolved at
// render time (tok(), chartSeries(), hand-computed colors).
export function useTheme(): string {
  return useSyncExternalStore(
    (onChange) => {
      _themeSubs.add(onChange);
      return () => _themeSubs.delete(onChange);
    },
    () => _theme,
    () => _theme,
  );
}

// Wire at import so every SDK-using shard receives theme pushes immediately —
// not only after its first bridge call.
ensureWired();

// --- shard local state (kv) ---------------------------------------------------
//
// The shard's private key/value document store (design/SHARD_LOCAL_STATE.md),
// served by the host bridge: path-shaped keys, per-key revisions, prefix
// subscriptions. The host owns channel selection (the app binds your real data;
// previews get a scratch sandbox) — shard code never sees channels. Most shards
// use the hooks; `kv` is the imperative client.

export type KVEntry = { key: string; value: unknown; revision: number; deleted?: boolean };

export const kv = {
  get(key: string): Promise<KVEntry | null> {
    return post("kv.get", { key }).then((d) => (d as KVEntry) ?? null);
  },
  list(prefix = ""): Promise<KVEntry[]> {
    return post("kv.list", { prefix }).then((d) => ((d as { entries?: KVEntry[] })?.entries ?? []));
  },
  set(key: string, value: unknown, baseRevision: number): Promise<{ revision: number }> {
    return post("kv.set", { key, value, baseRevision }) as Promise<{ revision: number }>;
  },
  remove(key: string, baseRevision: number): Promise<void> {
    return post("kv.delete", { key, baseRevision }).then(() => undefined);
  },
  // subscribe pushes the current entries under prefix, then every change
  // (deleted:true for tombstones); returns an unsubscribe fn.
  subscribe(prefix: string, cb: (entry: KVEntry) => void): () => void {
    ensureWired();
    const channel = "kv:" + ++_seq;
    _subs.set(channel, cb as unknown as (n: Node) => void);
    post("kv.subscribe", { prefix, channel }).catch(() => {});
    return () => {
      _subs.delete(channel);
      post("kv.unsubscribe", { channel }).catch(() => {});
    };
  },
};

// conflictOf narrows a rejection to the conflict payload, or null.
function conflictOf(err: unknown): { currentRevision: number; currentValue: unknown } | null {
  if (err instanceof BridgeError && err.code === "conflict") {
    const d = (err.data ?? {}) as { currentRevision?: number; currentValue?: unknown };
    return { currentRevision: d.currentRevision ?? 0, currentValue: d.currentValue };
  }
  return null;
}

/**
 * useShardState — persistent widget state under one key. Renders instantly from
 * local state (the shard is the single writer of its own view); persists
 * write-through with the per-key revision guard, and on a conflict (another
 * client edited the same key) re-applies your updater to the stored current and
 * retries — generated code never handles concurrency by hand. Live pushes from
 * other clients adopt automatically (revision-guarded).
 */
export function useShardState<T>(
  key: string,
  initial: T,
): [T, (next: T | ((prev: T) => T)) => void, { loading: boolean; error: string | null }] {
  const [value, setValue] = useState<T>(initial);
  const [meta, setMeta] = useState<{ loading: boolean; error: string | null }>({ loading: true, error: null });
  const stateRef = useState(() => ({ revision: 0, value: initial, alive: true, chain: Promise.resolve() }))[0];

  useEffect(() => {
    stateRef.alive = true;
    kv.get(key)
      .then((entry) => {
        if (!stateRef.alive) return;
        if (entry) {
          stateRef.revision = entry.revision;
          stateRef.value = entry.value as T;
          setValue(entry.value as T);
        }
        setMeta({ loading: false, error: null });
      })
      .catch((e: Error) => {
        if (stateRef.alive) setMeta({ loading: false, error: e.message });
      });
    const unsub = kv.subscribe(key, (entry) => {
      if (!stateRef.alive || entry.key !== key) return;
      if (entry.revision <= stateRef.revision) return; // echo / stale
      stateRef.revision = entry.revision;
      if (!entry.deleted) {
        stateRef.value = entry.value as T;
        setValue(entry.value as T);
      }
    });
    return () => {
      stateRef.alive = false;
      unsub();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const set = (next: T | ((prev: T) => T)) => {
    const updater = typeof next === "function" ? (next as (prev: T) => T) : null;
    const desired = updater ? updater(stateRef.value) : (next as T);
    stateRef.value = desired;
    setValue(desired);
    // Serialize persists; on conflict re-apply the updater to the stored
    // current (bounded retries), else last-write-wins with the fresh revision.
    stateRef.chain = stateRef.chain.then(async () => {
      let attempt = 0;
      let target = stateRef.value;
      for (;;) {
        try {
          const res = await kv.set(key, target, stateRef.revision);
          stateRef.revision = res.revision;
          if (stateRef.alive) setMeta({ loading: false, error: null });
          return;
        } catch (err) {
          const conflict = conflictOf(err);
          if (!conflict || attempt >= 3) {
            if (stateRef.alive) setMeta({ loading: false, error: err instanceof Error ? err.message : String(err) });
            return;
          }
          attempt++;
          stateRef.revision = conflict.currentRevision;
          target = updater ? updater(conflict.currentValue as T) : target;
          stateRef.value = target;
          if (stateRef.alive) setValue(target);
        }
      }
    });
  };

  return [value, set, meta];
}

/**
 * useKV — a live view of every key under a prefix (the shard's mini-app data:
 * "expenses/", "annotations/"…). put/remove are revision-guarded internally; a
 * put that loses a race retries once against the stored revision (the user's
 * whole-document action wins).
 */
export function useKV(prefix: string): {
  entries: Record<string, unknown>;
  put(key: string, value: unknown): void;
  remove(key: string): void;
  loading: boolean;
  error: string | null;
} {
  const [entries, setEntries] = useState<Record<string, unknown>>({});
  const [meta, setMeta] = useState<{ loading: boolean; error: string | null }>({ loading: true, error: null });
  const revsRef = useState(() => new Map<string, number>())[0];

  useEffect(() => {
    setEntries({});
    revsRef.clear();
    setMeta({ loading: true, error: null });
    const unsub = kv.subscribe(prefix, (entry) => {
      const known = revsRef.get(entry.key) ?? 0;
      if (entry.revision <= known) return;
      revsRef.set(entry.key, entry.revision);
      setEntries((prev) => {
        const next = { ...prev };
        if (entry.deleted) delete next[entry.key];
        else next[entry.key] = entry.value;
        return next;
      });
      setMeta({ loading: false, error: null });
    });
    // Subscribe seeds current entries; an empty prefix set still ends loading.
    kv.list(prefix)
      .then(() => setMeta((m) => ({ ...m, loading: false })))
      .catch((e: Error) => setMeta({ loading: false, error: e.message }));
    return unsub;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [prefix]);

  const write = (key: string, value: unknown, isDelete: boolean) => {
    const attempt = (baseRevision: number, retried: boolean) => {
      const op = isDelete ? kv.remove(key, baseRevision) : kv.set(key, value, baseRevision).then(() => undefined);
      op.catch((err: unknown) => {
        const conflict = conflictOf(err);
        if (conflict && !retried) {
          revsRef.set(key, conflict.currentRevision);
          attempt(conflict.currentRevision, true);
          return;
        }
        setMeta({ loading: false, error: err instanceof Error ? err.message : String(err) });
      });
    };
    attempt(revsRef.get(key) ?? 0, false);
  };

  return {
    entries,
    put: (key, value) => write(key, value, false),
    remove: (key) => write(key, undefined, true),
    loading: meta.loading,
    error: meta.error,
  };
}

export type NodeState = { node: Node | null; loading: boolean; error: string | null };

// useNode fetches a single node and live-updates it via subscription. id may be
// null/undefined to render nothing. Use the returned {node, loading, error}.
export function useNode(id: string | null | undefined): NodeState {
  const [state, setState] = useState<NodeState>({ node: null, loading: !!id, error: null });
  useEffect(() => {
    if (!id) {
      setState({ node: null, loading: false, error: null });
      return;
    }
    let alive = true;
    setState({ node: null, loading: true, error: null });
    bridge
      .getNode(id)
      .then((n) => {
        if (alive) setState({ node: n, loading: false, error: null });
      })
      .catch((e: Error) => {
        if (alive) setState({ node: null, loading: false, error: e.message });
      });
    const unsub = bridge.subscribe([id], (n) => {
      if (alive && n && n.id === id) setState({ node: n, loading: false, error: null });
    });
    return () => {
      alive = false;
      unsub();
    };
  }, [id]);
  return state;
}

// useNodes is the multi-id form: returns {nodes, loading, error} with nodes keyed
// in the requested order (missing ids omitted).
export function useNodes(ids: string[]): { nodes: Node[]; loading: boolean; error: string | null } {
  const key = ids.join(",");
  const [state, setState] = useState<{ nodes: Node[]; loading: boolean; error: string | null }>({
    nodes: [],
    loading: ids.length > 0,
    error: null,
  });
  useEffect(() => {
    if (ids.length === 0) {
      setState({ nodes: [], loading: false, error: null });
      return;
    }
    let alive = true;
    setState({ nodes: [], loading: true, error: null });
    bridge
      .getNodes(ids)
      .then((ns) => {
        if (alive) setState({ nodes: ns, loading: false, error: null });
      })
      .catch((e: Error) => {
        if (alive) setState({ nodes: [], loading: false, error: e.message });
      });
    const unsub = bridge.subscribe(ids, (n) => {
      if (!alive) return;
      setState((s) => ({ ...s, loading: false, nodes: s.nodes.map((x) => (x.id === n.id ? n : x)) }));
    });
    return () => {
      alive = false;
      unsub();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  return state;
}
