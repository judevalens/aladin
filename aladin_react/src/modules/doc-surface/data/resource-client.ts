import { decodeJSON, jsonKey, pointerParts, safeJSON, validateSchema } from "./schema-profile";
import { deepFreeze, EMPTY_REDUCTION, IDLE_STATE, reduceResource } from "./resource-store";
import type { ResourceDescriptor, ResourceError, ResourceState, SubscriptionIdentity } from "./resource-store";
import { LIMITS } from "./types";
import type { Binding, Capability, Data, ResourceRecord } from "./types";

export interface ResourceRequest { binding: string; inputs: Data }
export interface InsertInput { id?: string; data: Data; requestId: string }
export interface UpdateInput { id: string; data: Data; baseRevision: string; requestId: string }
export interface DeleteInput { id: string; baseRevision: string; requestId: string }
export type Mutation = ({ op: "insert" } & InsertInput) | ({ op: "update" } & UpdateInput) | ({ op: "delete" } & DeleteInput);
export interface MutationResult {
  requestId: string;
  record?: ResourceRecord;
  tombstone?: { id: string; revision: string };
}
export interface OpenedResource {
  descriptor: ResourceDescriptor;
  identity: SubscriptionIdentity;
  close(): void;
}
export interface StreamReceiver {
  event(value: unknown): void;
  error(error: ResourceError): void;
}
/**
 * Local adapter boundary, NOT a new wire protocol.
 * A bridge adapter implements open using describe + subscribe (or read for a
 * snapshot-only resource), and mutation using resource.insert/update/delete.
 * No principal, environment, endpoint, or contractHash is accepted from authors.
 */
export interface ResourceTransport {
  open(request: ResourceRequest, receiver: StreamReceiver, signal: AbortSignal): Promise<OpenedResource>;
  mutate(request: ResourceRequest, mutation: Mutation, signal: AbortSignal): Promise<MutationResult>;
}
export interface ResourceHandle {
  getSnapshot(): ResourceState;
  subscribe(listener: () => void): () => void;
  refresh(): void;
  readonly insert: ((input: InsertInput) => Promise<MutationResult>) | undefined;
  readonly update: ((input: UpdateInput) => Promise<MutationResult>) | undefined;
  readonly remove: ((input: DeleteInput) => Promise<MutationResult>) | undefined;
}
const capabilities: Capability[] = ["snapshot", "query", "insert", "update", "delete", "observe"];
const keyOf = jsonKey;
function errorOf(error: unknown): ResourceError {
  if (error && typeof error === "object" && "code" in error && typeof error.code === "string") {
    return { code: error.code, message: "message" in error ? String(error.message) : error.code };
  }
  return { code: "source-unavailable", message: error instanceof Error ? error.message : "Resource unavailable" };
}
function descriptorValid(descriptor: ResourceDescriptor): void {
  if (!["collection", "singleton"].includes(descriptor.kind) ||
      !Number.isSafeInteger(descriptor.schemaVersion) || descriptor.schemaVersion < 1 ||
      !Number.isInteger(descriptor.limit) || descriptor.limit < 1 || descriptor.limit > LIMITS.maxLimit ||
      !Array.isArray(descriptor.capabilities) || descriptor.capabilities.some(cap => !capabilities.includes(cap)) ||
      new Set(descriptor.capabilities).size !== descriptor.capabilities.length ||
      !descriptor.capabilities.includes("snapshot") ||
      !["deltas", "snapshots"].includes(descriptor.delivery) ||
      descriptor.observation === "refresh-snapshots" && descriptor.delivery !== "snapshots" ||
      descriptor.capabilities.includes("observe") && !["ordered-changes", "refresh-snapshots"].includes(descriptor.observation ?? "")) {
    throw { code: "invalid-schema", message: "Invalid resource descriptor" };
  }
  validateSchema(descriptor.schema);
}

class ResourceCore {
  state: ResourceState = IDLE_STATE;
  readonly listeners = new Set<() => void>();
  private reduction = EMPTY_REDUCTION;
  private lifetime = new AbortController();
  private generation = 0;
  private controller?: AbortController;
  private opened?: OpenedResource;
  private retry?: ReturnType<typeof setTimeout>;
  private attempts = 0;
  private disposed = false;
  private terminal = false;
  private gated = false;
  private dependencyKey: string | undefined;
  readonly dependencyCleanups: (() => void)[] = [];
  private notificationScheduled = false;
  private pending = new Map<string, { payload: string; committed: boolean; promise: Promise<MutationResult> }>();

  constructor(private transport: ResourceTransport, readonly request: ResourceRequest) {}

  private publish(patch: Partial<ResourceState>): void {
    this.state = deepFreeze({ ...this.state, ...patch });
    if (!this.notificationScheduled) {
      this.notificationScheduled = true;
      queueMicrotask(() => {
        this.notificationScheduled = false;
        if (!this.disposed) for (const listener of [...this.listeners]) listener();
      });
    }
  }
  private stopGeneration(): void {
    this.generation++;
    this.controller?.abort();
    try { this.opened?.close(); } catch { /* detach is best effort; server also expires leases */ }
    this.opened = undefined;
    if (this.retry) clearTimeout(this.retry);
    this.retry = undefined;
  }
  start(): void {
    if (this.disposed || this.terminal || this.gated) return;
    this.stopGeneration();
    const generation = this.generation;
    const controller = new AbortController();
    this.controller = controller;
    this.reduction = EMPTY_REDUCTION;
    const confirmations = [...this.pending.entries()].filter(([, entry]) => entry.committed).map(([id]) => id);
    this.publish({ status: "loading", loading: true, stale: this.state.updatedAt !== null, error: null });
    let queued: unknown[] = [], queuedBytes = 0, scheduled = false;
    const current = () => !this.disposed && generation === this.generation;
    const drain = () => {
      scheduled = false;
      if (!current() || !this.opened) return;
      const events = queued; queued = []; queuedBytes = 0;
      for (const value of events) {
        if (!current()) break;
        const next = reduceResource(this.reduction, this.opened!.identity, this.opened!.descriptor, value);
        if (next === this.reduction) continue;
        if (next.stale) { this.fail({ code: "resync-required", message: "Resource stream lost continuity" }); break; }
        this.reduction = next;
        this.attempts = 0;
        for (const id of confirmations) this.pending.delete(id);
        const now = new Date().toISOString();
        this.publish({
          records: next.records, nextCursor: next.nextCursor, loading: false, stale: false, error: null,
          status: this.opened!.descriptor.capabilities.includes("observe") ? "live" : "ready",
          updatedAt: now, receivedAt: now, sourceUpdatedAt: next.sourceUpdatedAt, pending: [...this.pending.keys()],
        });
      }
    };
    const receiver: StreamReceiver = {
      event: value => {
        if (!current()) return;
        try {
          const encoded = JSON.stringify(value);
          queuedBytes += new TextEncoder().encode(encoded).byteLength;
          if (queued.length >= LIMITS.queuedEvents || queuedBytes > LIMITS.queuedBytes) {
            this.fail({ code: "resync-required", message: "Resource event queue exceeded its limit" }); return;
          }
          queued.push(JSON.parse(encoded));
        } catch { this.fail({ code: "resync-required", message: "Invalid resource event" }); return; }
        if (!scheduled) { scheduled = true; queueMicrotask(drain); }
      },
      error: error => { if (current()) this.fail(error); },
    };
    void this.transport.open(this.request, receiver, controller.signal).then(opened => {
      if (!current()) { opened.close(); return; }
      this.opened = opened; // Preserve close even when descriptor validation fails.
      descriptorValid(opened.descriptor);
      this.opened = { ...opened, descriptor: deepFreeze(structuredClone(opened.descriptor)) };
      this.publish({ capabilities: [...opened.descriptor.capabilities] });
      drain(); // Includes events received before the acknowledgement.
    }).catch(error => { if (current()) this.fail(errorOf(error)); });
  }
  dependencyState(ready: boolean, key: string, error?: ResourceError): void {
    if (this.disposed || this.terminal) return;
    if (error?.code === "forbidden" || error?.code === "not-found") { this.fail(error); return; }
    if (!ready) {
      this.gated = true;
      this.stopGeneration();
      this.publish({ ...IDLE_STATE, status: "loading", loading: true, error: error ?? null });
      return;
    }
    if (this.gated || key !== this.dependencyKey) {
      this.gated = false;
      this.dependencyKey = key;
      // A dependency may select a different account: old records cannot be
      // presented as the new parameter generation's current data.
      this.publish({ ...IDLE_STATE, status: "loading", loading: true });
      this.start();
    }
  }
  private fail(error: ResourceError): void {
    this.stopGeneration();
    if (["forbidden", "not-found", "contract-changed"].includes(error.code)) {
      this.terminal = true;
      this.lifetime.abort();
      this.pending.clear();
      this.publish({ ...IDLE_STATE, status: "forbidden", error });
      return;
    }
    const terminal = ["bad-request", "invalid-schema", "unsupported-capability", "contract-changed"].includes(error.code);
    this.terminal = terminal;
    this.publish({ loading: false, stale: true, status: terminal ? "error" : "stale", error });
    if (!terminal && !this.disposed) this.retry = setTimeout(() => this.start(), Math.min(250 * 2 ** Math.min(this.attempts++, 7), 30_000));
  }
  mutate(mutation: Mutation): Promise<MutationResult> {
    if (this.disposed || this.terminal || !this.state.capabilities.includes(mutation.op)) {
      return Promise.reject({ code: "forbidden", message: "Mutation is not permitted" });
    }
    // Never auto-retry a mutation. The caller keeps this same requestId when
    // reconciling an unknown outcome; durable idempotency belongs to the server.
    safeJSON(mutation);
    if (!mutation.requestId) return Promise.reject({ code: "bad-request", message: "requestId is required" });
    const copy = deepFreeze(structuredClone(mutation));
    const payload = keyOf(copy), existing = this.pending.get(copy.requestId);
    if (existing) {
      return existing.payload === payload ? existing.promise : Promise.reject({ code: "conflict", message: "requestId payload mismatch" });
    }
    const entry = { payload, committed: false, promise: undefined as unknown as Promise<MutationResult> };
    const promise = Promise.resolve().then(() => this.transport.mutate(this.request, copy, this.lifetime.signal)).then(result => {
      if (!this.disposed && !this.terminal) {
        entry.committed = true;
        this.start(); // Confirm through a fresh authoritative snapshot, not the mutation response.
      }
      return result;
    }).catch(error => {
      this.pending.delete(copy.requestId);
      if (!this.disposed && !this.terminal) {
        const typed = errorOf(error);
        this.publish({ pending: [...this.pending.keys()], error: typed });
        if (["forbidden", "not-found", "contract-changed"].includes(typed.code)) this.fail(typed);
      }
      throw error;
    });
    entry.promise = promise;
    this.pending.set(copy.requestId, entry);
    this.publish({ pending: [...this.pending.keys()] });
    return promise;
  }
  dispose(): void {
    this.disposed = true;
    this.stopGeneration();
    this.lifetime.abort();
    this.pending.clear();
    for (const release of this.dependencyCleanups.splice(0)) release();
    this.state = IDLE_STATE;
    for (const listener of [...this.listeners]) listener();
    this.listeners.clear();
  }
}

/**
 * One instance per mounted shard session. Identical binding+input combinations
 * share one stream. Different inputs never share cached records.
 */
export class ResourceClient {
  private cores = new Map<string, ResourceCore>();
  private closed = false;
  constructor(private readonly transport: ResourceTransport, private readonly bindings: Record<string, Binding> = {}) {
    // Do not assume externally supplied binding metadata has been compiled.
    const active = new Set<string>(), done = new Set<string>();
    const visit = (id: string) => {
      if (active.has(id)) throw new Error("Binding dependency cycle");
      if (done.has(id)) return;
      active.add(id);
      for (const dependency of this.dependencies(id)) {
        if (!Object.hasOwn(bindings, dependency.binding)) throw new Error("Unknown binding dependency");
        visit(dependency.binding);
      }
      active.delete(id); done.add(id);
    };
    for (const id of Object.keys(bindings)) visit(id);
  }

  private dependencies(binding: string): { binding: string; pointer: string }[] {
    return Object.values(this.bindings[binding]?.params ?? {}).flatMap(value =>
      value && typeof value === "object" && !Array.isArray(value) && typeof value.binding === "string" && typeof value.pointer === "string"
        ? [{ binding: value.binding, pointer: value.pointer }] : []);
  }

  private attachDependencies(core: ResourceCore): boolean {
    const refs = this.dependencies(core.request.binding);
    if (!refs.length) return false;
    const handles = new Map(refs.map(ref => [ref.binding, this.resource(ref.binding)]));
    const evaluate = () => {
      const values: unknown[] = [];
      for (const ref of refs) {
        const state = handles.get(ref.binding)!.getSnapshot();
        if (state.stale || state.loading || !["live", "ready"].includes(state.status) || !state.records.length) {
          core.dependencyState(false, "", state.error ?? undefined); return;
        }
        let value: unknown = state.records[0];
        for (const part of pointerParts(ref.pointer)) {
          if (!value || typeof value !== "object" || !Object.hasOwn(value, part)) {
            core.dependencyState(false, ""); return;
          }
          value = (value as Record<string, unknown>)[part];
        }
        values.push(value);
      }
      // Values are used only to detect changes. The backend resolves and
      // authorizes the actual parameters from its contract and source data.
      core.dependencyState(true, keyOf(values));
    };
    for (const handle of handles.values()) core.dependencyCleanups.push(handle.subscribe(evaluate));
    evaluate();
    return true;
  }

  resource(binding: string, inputs: Data = {}): ResourceHandle {
    if (this.closed) throw new Error("Resource client is closed");
    if (!/^[A-Za-z][A-Za-z0-9_-]{0,63}$/.test(binding)) throw new Error("Invalid binding ID");
    safeJSON(inputs);
    const request = deepFreeze({ binding, inputs: decodeJSON(JSON.stringify(inputs)) as Data });
    const key = binding + ":" + keyOf(request.inputs);
    const get = () => this.cores.get(key);
    const mutation = (value: Mutation) => {
      const core = get();
      return core ? core.mutate(value) : Promise.reject({ code: "source-unavailable", message: "Resource has no active consumers" });
    };
    return {
      getSnapshot: () => get()?.state ?? IDLE_STATE,
      subscribe: listener => {
        if (this.closed) throw new Error("Resource client is closed");
        let core = get();
        const created = !core;
        if (!core) {
          if (this.cores.size >= LIMITS.subscriptions) throw new Error("Resource subscription limit exceeded");
          core = new ResourceCore(this.transport, request);
          this.cores.set(key, core);
        }
        const notify = () => listener();
        core.listeners.add(notify);
        try {
          if (created && !this.attachDependencies(core)) core.start();
        } catch (error) {
          core.listeners.delete(notify);
          if (created) { core.dispose(); this.cores.delete(key); }
          throw error;
        }
        let released = false;
        return () => {
          if (released) return;
          released = true;
          core!.listeners.delete(notify);
          if (!core!.listeners.size) { core!.dispose(); this.cores.delete(key); }
        };
      },
      refresh: () => { get()?.start(); },
      get insert() { return get()?.state.capabilities.includes("insert") ? (input: InsertInput) => mutation({ ...input, op: "insert" }) : undefined; },
      get update() { return get()?.state.capabilities.includes("update") ? (input: UpdateInput) => mutation({ ...input, op: "update" }) : undefined; },
      get remove() { return get()?.state.capabilities.includes("delete") ? (input: DeleteInput) => mutation({ ...input, op: "delete" }) : undefined; },
    };
  }
  close(): void {
    this.closed = true;
    for (const core of this.cores.values()) core.dispose();
    this.cores.clear();
  }
}
