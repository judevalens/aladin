import requestSchema from "../../../../../shared/shard-v2/schemas/bridge-request.json";
import responseSchema from "../../../../../shared/shard-v2/schemas/bridge-response.json";
import descriptorSchema from "../../../../../shared/shard-v2/schemas/descriptor.json";
import subscriptionSchema from "../../../../../shared/shard-v2/schemas/subscription.json";
import snapshotSchema from "../../../../../shared/shard-v2/schemas/snapshot.json";
import { decodeJSON, object, safeJSON } from "./schema-profile";
import { validateWithSchema } from "./schema-validation";
import { LIMITS } from "./types";
import { resourceRequestId } from "./request-id";
import { validateEvent } from "./event-validation";
import { deepFreeze } from "./resource-store";
import type { Data, Query, ResourceSnapshot } from "./types";
import type { ResourceDescriptor, SubscriptionIdentity } from "./resource-store";
import type { Mutation, MutationResult, OpenedResource, ResourceRequest, ResourceTransport, StreamReceiver } from "./resource-client";

export interface ResourceBridgePort {
  call(method: string, params: Record<string, unknown>, signal?: AbortSignal): Promise<unknown>;
  listen(channel: string, listener: (value: unknown) => void): () => void;
}

/** One port per shard. Accept responses only from the exact parent window. */
const bridgeSequences = new WeakMap<Window, number>();
// Theme and resource clients share one bridge/2 request-ID namespace.
export function allocateBridgeRequestID(scope: Window): number {
  const next = (bridgeSequences.get(scope) ?? 0) + 1;
  if (!Number.isSafeInteger(next)) throw new Error("Bridge request IDs exhausted; reload");
  bridgeSequences.set(scope, next);
  return next;
}
export class WindowResourceBridgePort implements ResourceBridgePort {
  private closed = false;
  private pending = new Map<number, { resolve(value: unknown): void; reject(error: unknown): void }>();
  private listeners = new Map<string, Set<(value: unknown) => void>>();
  constructor(private scope: Window, private parent: Window) { scope.addEventListener("message", this.receive); }
  private receive = (event: MessageEvent) => {
    if (event.source !== this.parent || !object(event.data) || event.data.aladin !== "bridge/2") return;
    let message: Record<string, unknown>;
    try { message = decodeJSON(JSON.stringify(event.data)) as Record<string, unknown>; } catch { return; }
    if (message.type === "response") {
      const pending = this.pending.get(message.id as number);
      if (!pending) return;
      try { validateWithSchema(responseSchema, message); }
      catch { pending.reject({ code: "bad-request", message: "Malformed bridge response" }); return; }
      if (message.ok) pending.resolve(message.data);
      else pending.reject({ code: message.code, message: message.error, data: message.data });
    } else if (message.type === "push" && typeof message.channel === "string") {
      for (const listener of this.listeners.get(message.channel) ?? []) listener(message.data);
    }
  };
  call(method: string, params: Record<string, unknown>, signal?: AbortSignal): Promise<unknown> {
    if (this.closed) return Promise.reject({ code: "source-unavailable", message: "Bridge is closed" });
    if (this.pending.size >= 32) return Promise.reject({ code: "rate-limited", message: "Too many pending resource requests" });
    const id = allocateBridgeRequestID(this.scope);
    const request = { aladin: "bridge/2", type: "request", id, method, params };
    try { safeJSON(request); validateWithSchema(requestSchema, request); }
    catch (error) { return Promise.reject(error); }
    if (signal?.aborted) return Promise.reject(new DOMException("Aborted", "AbortError"));
    return new Promise((resolve, reject) => {
      const finish = (error: unknown, value?: unknown) => {
        if (!this.pending.delete(id)) return;
        clearTimeout(timer);
        signal?.removeEventListener("abort", abort);
        if (error) reject(error); else resolve(value);
      };
      const abort = () => finish(new DOMException("Aborted", "AbortError"));
      const timeoutMs = method === "graphql.execute" || method === "lambda.invoke" ? 35000 : 8000;
      const timer = setTimeout(() => finish({ code: "source-unavailable", message: "Bridge request timed out; mutation outcome may be unknown" }), timeoutMs);
      this.pending.set(id, { resolve: value => finish(null, value), reject: error => finish(error) });
      signal?.addEventListener("abort", abort, { once: true });
      try { this.parent.postMessage(request, "*"); } catch (error) { finish(error); }
    });
  }
  listen(channel: string, listener: (value: unknown) => void): () => void {
    if (this.closed) throw new Error("Bridge is closed");
    let listeners = this.listeners.get(channel);
    if (!listeners) { listeners = new Set(); this.listeners.set(channel, listeners); }
    listeners.add(listener);
    return () => { listeners!.delete(listener); if (!listeners!.size) this.listeners.delete(channel); };
  }
  close(): void {
    this.closed = true;
    this.scope.removeEventListener("message", this.receive);
    for (const pending of [...this.pending.values()]) pending.reject({ code: "source-unavailable", message: "Bridge detached" });
    this.listeners.clear();
  }
}

/**
 * Normalizes bridge reads/subscriptions into the same client reducer.
 * The host remains responsible for grants and environment/release selection.
 */
export class BridgeResourceTransport implements ResourceTransport {
  private hello?: Promise<void>;
  constructor(private readonly bridge: ResourceBridgePort, private readonly expected?: { contractHash: string; buildId: string }) {}
  private ready(): Promise<void> {
    if (!this.hello) this.hello = this.bridge.call("hello", {}).then(value => {
      if (!object(value) || value.protocol !== "bridge/2") throw { code: "unsupported-capability", message: "Host does not support bridge/2" };
      if (this.expected && (value.contractHash !== this.expected.contractHash || value.buildId !== this.expected.buildId)) throw { code: "contract-changed", message: "Code and resource release do not match; reload the shard" };
    }).catch(error => { this.hello = undefined; throw error; });
    return this.hello;
  }
  async open(request: ResourceRequest, receiver: StreamReceiver, signal: AbortSignal): Promise<OpenedResource> {
    await this.ready();
    const params = { binding: request.binding, inputs: request.inputs };
    const description = await this.bridge.call("resource.describe", { binding: request.binding, inputs: request.inputs }, signal);
    validateWithSchema(descriptorSchema, description);
    const descriptor = description as ResourceDescriptor;
    if (!descriptor.capabilities.includes("observe")) {
      const result = await this.bridge.call("resource.read", params, signal);
      validateWithSchema(snapshotSchema, result);
      const snapshot = result as { resource: string; records: unknown[]; complete: true; nextCursor?: string; sourceUpdatedAt?: string };
      const identity = { subscriptionId: "read-" + resourceRequestId(), epoch: resourceRequestId(), resource: snapshot.resource };
      receiver.event({ protocol: "shard-data/1", ...identity, seq: "0", op: "snapshot", records: snapshot.records, complete: true, ...(snapshot.sourceUpdatedAt ? { sourceUpdatedAt: snapshot.sourceUpdatedAt } : {}), ...(snapshot.nextCursor ? { nextCursor: snapshot.nextCursor } : {}) });
      return { descriptor, identity, close() {} };
    }
    let identity: SubscriptionIdentity | undefined, closed = false, earlyBytes = 0;
    let early: { channel: string; value: unknown }[] = [];
    const releases: (() => void)[] = [];
    const unsubscribe = () => {
      if (identity) void this.bridge.call("resource.unsubscribe", { subscriptionId: identity.subscriptionId }).catch(() => {});
    };
    const close = () => {
      if (closed) return;
      closed = true; for (const release of releases) release();
      signal.removeEventListener("abort", close); early = []; earlyBytes = 0;
      unsubscribe();
    };
    const receive = (channel: string, value: unknown) => {
      if (closed) return;
      if (!identity) {
        try {
          const encoded = JSON.stringify(value);
          earlyBytes += new TextEncoder().encode(encoded).byteLength;
          if (early.length >= LIMITS.queuedEvents || earlyBytes > LIMITS.queuedBytes) throw new Error("queue overflow");
          early.push({ channel, value: decodeJSON(encoded) });
        } catch {
          receiver.error({ code: "resync-required", message: "Invalid or overflowing pre-acknowledgement queue" }); close();
        }
        return;
      }
      if (!object(value) || value.subscriptionId !== identity.subscriptionId) return;
      if (channel === "resource.event") { receiver.event(value); return; }
      // Control messages must belong to the same view and subscription epoch.
      if (value.resource !== identity.resource || value.epoch !== identity.epoch) return;
      if (channel === "resource.error") {
        receiver.error({ code: typeof value.code === "string" ? value.code : "source-unavailable", message: typeof value.message === "string" ? value.message : "Resource error" });
      } else if (["stale", "disconnected", "resync-required", "forbidden"].includes(String(value.status))) {
        receiver.error({ code: value.status === "forbidden" ? "forbidden" : "resync-required", message: "Resource stream " + String(value.status) });
      }
    };
    for (const channel of ["resource.event", "resource.error", "resource.status"]) {
      releases.push(this.bridge.listen(channel, value => receive(channel, value)));
    }
    signal.addEventListener("abort", close, { once: true });
    if (signal.aborted) { close(); throw new DOMException("Aborted", "AbortError"); }
    try {
      // Retain the acknowledgement even after cancellation so a late server-side
      // subscription can still be explicitly unsubscribed.
      const result = await this.bridge.call("resource.subscribe", params);
      validateWithSchema(subscriptionSchema, result);
      identity = result as SubscriptionIdentity;
      if (closed) { unsubscribe(); throw new DOMException("Aborted", "AbortError"); }
      for (const message of early) receive(message.channel, message.value);
      early = []; earlyBytes = 0;
      return { descriptor, identity, close };
    } catch (error) { close(); throw error; }
  }
  async mutate(request: ResourceRequest, mutation: Mutation, signal: AbortSignal): Promise<MutationResult> {
    await this.ready();
    const { op, ...command } = mutation;
    const result = await this.bridge.call("resource." + op, { binding: request.binding, inputs: request.inputs, ...command }, signal);
    if (!object(result) || result.requestId !== mutation.requestId) throw { code: "source-unavailable", message: "Unconfirmed mutation outcome", requestId: mutation.requestId };
    // This result is never applied to the view; only the stream/refetch is.
    return result as unknown as MutationResult;
  }
  async query<T extends Data = Data>(request: ResourceRequest, query: Query, signal?: AbortSignal): Promise<ResourceSnapshot<T>> {
    await this.ready();
    const params = { binding: request.binding, inputs: request.inputs, query };
    const description = await this.bridge.call("resource.describe", { binding: request.binding, inputs: request.inputs }, signal);
    validateWithSchema(descriptorSchema, description);
    const descriptor = description as ResourceDescriptor;
    const result = await this.bridge.call("resource.query", params, signal);
    validateWithSchema(snapshotSchema, result);
    const snapshot = result as ResourceSnapshot<T>;
    validateEvent({ ...snapshot, protocol: "shard-data/1", subscriptionId: "query", epoch: "query", seq: "0", op: "snapshot" }, descriptor, descriptor.schema);
    if (snapshot.records.length > (query.limit ?? descriptor.limit)) throw { code: "invalid-schema", message: "Query response exceeds its view limit" };
    return deepFreeze(snapshot);
  }

  async executeGraphQL<T = unknown>(operationId: string, variables: Data = {}, signal?: AbortSignal): Promise<{ data?: T; errors?: { message: string }[] }> {
    await this.ready();
    const result = await this.bridge.call("graphql.execute", { operationId, variables }, signal);
    if (!object(result) || (result.data === undefined && !Array.isArray(result.errors))) throw { code: "source-unavailable", message: "Invalid GraphQL response" };
    return deepFreeze(result as { data?: T; errors?: { message: string }[] });
  }

  async invokeLambda<T = unknown>(name: string, input: Data = {}, signal?: AbortSignal): Promise<T> {
    await this.ready();
    return deepFreeze(await this.bridge.call("lambda.invoke", { name, input }, signal) as T);
  }
}
