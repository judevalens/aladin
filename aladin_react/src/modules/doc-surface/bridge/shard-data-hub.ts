import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import type { NodeViewWire, ShardApi } from "@/shared/api/shard-api";

// The workspace plane's liveness layer.
//
// Sync frames already reach this app: the outbox drain republishes every frame
// as a "*.frame" workspace event, and the composition's existing websocket
// subscription receives them. This hub is the demux: it knows which shard is
// subscribed to which entity ids, and when a frame mentions one of them it
// refetches THAT ONE entity through the granted endpoint and pushes it into the
// shard. Event-triggered single-entity fetch — the house pattern; never a list
// reload, never an invalidation subject.
//
// Four properties the naive version gets wrong, all handled here:
//   1. The websocket has NO replay cursor, so frames during a disconnect are
//      lost forever → on reconnect, refetch every subscribed id.
//   2. frame → refetch is async, so two frames can resolve out of order →
//      per-id seq guard, drop anything not newer than what was pushed.
//   3. A sync burst would trigger one refetch per frame per subscriber →
//      coalesce per tick: dedupe ids, one call per shard.
//   4. An iframe can vanish (LRU eviction, draft reload) without its shard ever
//      unsubscribing → subscriptions are keyed by shard and torn down by the
//      host, never trusted to the shard.

export interface ShardNodeSubscription {
  channel: string;
  ids: Set<string>;
  push: (node: NodeViewWire) => void;
}

export interface ShardDataHub {
  /** Register (or replace) one shard's subscription channel. */
  subscribe(shardId: string, sub: ShardNodeSubscription): void;
  unsubscribe(shardId: string, channel: string): void;
  /** Drop every subscription for a shard (host detach). */
  dropShard(shardId: string): void;
  /** Feed one realtime frame event in. */
  handleFrame(event: AppEventEnvelope): void;
  /** Websocket reconnected: nothing was replayed, so refetch everything. */
  refetchAll(): void;
}

// The frame payload shape (service.Frame): entities carry kind + id + seq.
interface FramePayload {
  entities?: Array<{ entityKind?: string; entityId?: string; seq?: number | string }>;
}

// A frame's entityId is the raw entity id; a shard may hold it under a
// qualified ref ("watchlist:<uuid>"). Match either form.
function refsMatching(ids: Set<string>, entityKind: string, entityId: string): string[] {
  const out: string[] = [];
  if (ids.has(entityId)) out.push(entityId);
  const qualified = `${entityKind}:${entityId}`;
  if (ids.has(qualified)) out.push(qualified);
  return out;
}

export function createShardDataHub(api: ShardApi): ShardDataHub {
  // shardId -> channel -> subscription
  const subs = new Map<string, Map<string, ShardNodeSubscription>>();
  // shardId -> ref -> last pushed seq (0 when unknown)
  const seen = new Map<string, Map<string, number>>();
  // shardId -> refs pending a refetch on the next tick
  const pending = new Map<string, Set<string>>();
  let scheduled = false;

  function seqFor(shardId: string): Map<string, number> {
    let m = seen.get(shardId);
    if (!m) {
      m = new Map();
      seen.set(shardId, m);
    }
    return m;
  }

  function queue(shardId: string, refs: string[]) {
    if (refs.length === 0) return;
    let set = pending.get(shardId);
    if (!set) {
      set = new Set();
      pending.set(shardId, set);
    }
    refs.forEach((r) => set!.add(r));
    if (scheduled) return;
    scheduled = true;
    // Coalesce a burst into one call per shard.
    queueMicrotask(() => {
      scheduled = false;
      const work = [...pending.entries()];
      pending.clear();
      work.forEach(([sid, refs]) => void flush(sid, [...refs]));
    });
  }

  async function flush(shardId: string, refs: string[]) {
    const channels = subs.get(shardId);
    if (!channels || channels.size === 0 || refs.length === 0) return;
    let nodes: NodeViewWire[];
    try {
      const res = await api.bridgeNodes(shardId, refs);
      nodes = res.nodes;
    } catch {
      return; // a denied/failed refetch must not tear the shard down
    }
    const seqs = seqFor(shardId);
    for (const node of nodes) {
      const seq = Number(node.seq ?? 0);
      // Seq 0 means the kind doesn't version its views (e.g. watchlists):
      // always push those; otherwise drop anything not strictly newer.
      if (seq !== 0 && seq <= (seqs.get(node.id) ?? 0)) continue;
      if (seq !== 0) seqs.set(node.id, seq);
      channels.forEach((sub) => {
        if (sub.ids.has(node.id)) sub.push(node);
      });
    }
  }

  return {
    subscribe(shardId, sub) {
      let channels = subs.get(shardId);
      if (!channels) {
        channels = new Map();
        subs.set(shardId, channels);
      }
      channels.set(sub.channel, sub);
    },
    unsubscribe(shardId, channel) {
      subs.get(shardId)?.delete(channel);
    },
    dropShard(shardId) {
      subs.delete(shardId);
      seen.delete(shardId);
      pending.delete(shardId);
    },
    handleFrame(event) {
      if (event.type !== "*.frame" || subs.size === 0) return;
      const payload = event.payload as FramePayload | null;
      const entities = payload?.entities;
      if (!entities?.length) return;
      for (const [shardId, channels] of subs) {
        const wanted = new Set<string>();
        for (const channel of channels.values()) {
          for (const entity of entities) {
            if (!entity.entityKind || !entity.entityId) continue;
            refsMatching(channel.ids, entity.entityKind, entity.entityId).forEach((r) => wanted.add(r));
          }
        }
        queue(shardId, [...wanted]);
      }
    },
    refetchAll() {
      for (const [shardId, channels] of subs) {
        const all = new Set<string>();
        channels.forEach((c) => c.ids.forEach((id) => all.add(id)));
        // Reconnect means our seq view may be stale in either direction; clear
        // it so the refetched values are pushed unconditionally.
        seen.delete(shardId);
        queue(shardId, [...all]);
      }
    },
  };
}

// createShardFrameHandler adapts the hub to the app-event processor (registered
// beside the shard-build handler in the composition).
export function createShardFrameHandler(hub: ShardDataHub) {
  return function handle(event: AppEventEnvelope) {
    hub.handleFrame(event);
  };
}
