import { ApiError } from "@/shared/api/client";
import type { ApiClient } from "@/shared/api/client";
import type { ShardChannel } from "@/app/state/shard-build-slice";
import { validateWithSchema } from "../data/schema-validation";
import { decodeJSON } from "../data/schema-profile";
import requestSchema from "../../../../../shared/shard-v2/schemas/bridge-request.json";
import responseSchema from "../../../../../shared/shard-v2/schemas/bridge-response.json";
import subscriptionSchema from "../../../../../shared/shard-v2/schemas/subscription.json";
import type { SubscriptionIdentity } from "../data/resource-store";

export type ShardReleaseMetadata = { protocol: "bridge/1"; available?: boolean } | { protocol: "bridge/2"; buildId: string; contractHash: string };
export interface HostResourceTarget { shardId: string; environment: ShardChannel; contractHash: string }
export interface HostResourceSession {
  call(method: string, params: Record<string, unknown>): Promise<unknown>;
  close(): void;
}
type Push = (channel: string, data: unknown) => void;
type Owner = { target: HostResourceTarget; push: Push; closed: boolean; token: string | null; abort: AbortController; subscriptions: Map<string, SubscriptionIdentity>; inflight: number };
type Pending = { owner: Owner; method: string; finish: (error?: unknown, value?: unknown) => void };
const failure = (code: string, message: string) => ({ code, message });

/** One physical connection per app composition, multiplexing isolated frames. */
export function createResourceHostHub(api: ApiClient, socketURL: string, token: () => string | null, makeSocket = (url: string) => new WebSocket(url)) {
  const owners = new Set<Owner>();
  const pending = new Map<number, Pending>();
  let socket: WebSocket | undefined, connecting: Promise<WebSocket> | undefined;
  let sequence = 0;
  let watch: ReturnType<typeof setInterval> | undefined;

  function send(ws: WebSocket, value: unknown) {
    const raw = JSON.stringify(value);
    const size = new TextEncoder().encode(raw).byteLength;
    if (size > (1 << 20) || ws.bufferedAmount + size > (4 << 20)) {
      ws.close(); throw failure("resync-required", "Host socket send queue exceeded its limit");
    }
    ws.send(raw);
  }

  function invalidate(owner: Owner, code: string) {
    for (const identity of owner.subscriptions.values()) owner.push("resource.error", { ...identity, code, message: "Resource connection closed" });
    owner.subscriptions.clear();
  }
  function closeOwner(owner: Owner) {
    if (owner.closed) return;
    owner.closed = true;
    for (const id of owner.subscriptions.keys()) {
      if (socket?.readyState === WebSocket.OPEN) { try { sendUnsubscribe(owner, id); } catch { /* Closing the socket also releases server subscriptions. */ } }
    }
    owner.subscriptions.clear(); owner.abort.abort(); owners.delete(owner);
    // Retain pending WS acknowledgements so a late subscribe is cleaned up.
    if (!owners.size) {
      if (watch) clearInterval(watch); watch = undefined;
      socket?.close();
    }
  }
  function sendUnsubscribe(owner: Owner, subscriptionId: string) {
    if (socket) send(socket, { target: owner.target, request: { aladin: "bridge/2", type: "request", id: ++sequence, method: "resource.unsubscribe", params: { subscriptionId } } });
  }
  function ensureSocket(): Promise<WebSocket> {
    if (socket?.readyState === WebSocket.OPEN) return Promise.resolve(socket);
    if (connecting) return connecting;
    const credential = token();
    const url = socketURL + (credential ? "?access_token=" + encodeURIComponent(credential) : "");
    const ws = makeSocket(url); socket = ws;
    connecting = new Promise<WebSocket>((resolve, reject) => {
      const timeout = setTimeout(() => { reject(failure("source-unavailable", "Connection timed out")); ws.close(); }, 8000);
      ws.onopen = () => { clearTimeout(timeout); connecting = undefined; resolve(ws); };
      ws.onclose = event => {
        clearTimeout(timeout);
        if (socket !== ws) return;
        socket = undefined; connecting = undefined;
        const error = failure(event.code === 1008 ? "forbidden" : "resync-required", "Resource connection closed");
        reject(error);
        for (const value of [...pending.values()]) value.finish(error);
        for (const owner of owners) invalidate(owner, error.code);
      };
      ws.onerror = () => ws.close();
      ws.onmessage = event => {
        if (socket !== ws) return;
        try {
          const message = decodeJSON(String(event.data)) as Record<string, unknown>;
          if (message.aladin !== "bridge/2") throw new Error("Wrong protocol");
          if (message.type === "response") {
            validateWithSchema(responseSchema, message);
            const value = pending.get(message.id as number);
            if (!value) return;
            if (message.ok && value.method === "resource.subscribe") {
              validateWithSchema(subscriptionSchema, message.data);
              const identity = message.data as SubscriptionIdentity;
              if (value.owner.closed) sendUnsubscribe(value.owner, identity.subscriptionId);
              else value.owner.subscriptions.set(identity.subscriptionId, identity);
            }
            value.finish(message.ok ? undefined : failure(String(message.code), String(message.error)), message.data);
          } else if (message.type === "push" && ["resource.event", "resource.error", "resource.status"].includes(String(message.channel))) {
            const data = message.data as SubscriptionIdentity;
            for (const owner of owners) {
              const identity = owner.subscriptions.get(data?.subscriptionId);
              if (identity && data.epoch === identity.epoch && data.resource === identity.resource) owner.push(String(message.channel), data);
            }
          } else throw new Error("Invalid socket envelope");
        } catch { ws.close(); }
      };
    });
    return connecting;
  }
  async function call(owner: Owner, method: string, params: Record<string, unknown>) {
    if (owner.closed || owner.token !== token()) { invalidate(owner, "forbidden"); closeOwner(owner); throw failure("forbidden", "Session changed"); }
    if (owner.inflight >= 32) throw failure("rate-limited", "Too many pending requests");
    const request = { aladin: "bridge/2", type: "request", id: ++sequence, method, params };
    validateWithSchema(requestSchema, request);
    owner.inflight++;
    try {
      if (method === "resource.subscribe" || method === "resource.unsubscribe") {
        if (method === "resource.unsubscribe") {
          if (!owner.subscriptions.delete(String(params.subscriptionId))) return true;
        } else if (owner.subscriptions.size + owner.inflight > 32) throw failure("rate-limited", "Subscription limit reached");
        const ws = await ensureSocket();
        if (owner.closed) throw failure("forbidden", "Session closed");
        return await new Promise<unknown>((resolve, reject) => {
          const timer = setTimeout(() => {
            // Closing retires unknown server-side subscriptions after a lost ack.
            pending.get(request.id)?.finish(failure("resync-required", "Subscription acknowledgement timed out")); ws.close();
          }, 8000);
          pending.set(request.id, { owner, method, finish(error, value) { clearTimeout(timer); pending.delete(request.id); if (error) reject(error); else resolve(value); } });
          try { send(ws, { target: owner.target, request }); } catch (error) { pending.get(request.id)?.finish(error); }
        });
      }
      let response: unknown;
      try {
        response = await api.fetch(`/api/shards/${encodeURIComponent(owner.target.shardId)}/v2/${owner.target.environment}/request`, { method: "POST", headers: { "X-Shard-Contract": owner.target.contractHash }, body: JSON.stringify(request), signal: AbortSignal.any([owner.abort.signal, AbortSignal.timeout(10000)]) });
      } catch (error) {
        if (error instanceof ApiError && error.body) response = error.body; else throw error;
      }
      if (owner.closed || owner.token !== token()) throw failure("forbidden", "Session changed");
      validateWithSchema(responseSchema, response);
      const result = response as { id: number; ok: boolean; data?: unknown; code?: string; error?: string };
      if (result.id !== request.id) throw failure("source-unavailable", "Response identity mismatch");
      if (!result.ok) throw failure(result.code!, result.error!);
      return result.data;
    } finally { owner.inflight--; }
  }
  return {
    async release(shardId: string, environment: ShardChannel): Promise<ShardReleaseMetadata> {
      const result = await api.fetch<ShardReleaseMetadata>(`/api/shards/${encodeURIComponent(shardId)}/release?channel=${environment}`);
      if (result.protocol !== "bridge/1" && (result.protocol !== "bridge/2" || !result.buildId || !result.contractHash)) throw new Error("Invalid shard release metadata");
      return result;
    },
    session(target: HostResourceTarget, push: Push): HostResourceSession {
      const owner: Owner = { target: { ...target }, push, closed: false, token: token(), abort: new AbortController(), subscriptions: new Map(), inflight: 0 };
      owners.add(owner);
      if (!watch) watch = setInterval(() => { for (const item of owners) if (item.token !== token()) { invalidate(item,"forbidden"); closeOwner(item); } }, 1000);
      return { call: (method, params) => call(owner, method, params), close: () => closeOwner(owner) };
    },
    close() { for (const owner of [...owners]) { invalidate(owner, "forbidden"); closeOwner(owner); } },
  };
}
export type ResourceHostHub = ReturnType<typeof createResourceHostHub>;
