import { validateEvent } from "./event-validation";
import type { Capability, Data, Resource, ResourceEvent, ResourceRecord, Schema } from "./types";

export interface ResourceError {
  code: string;
  message: string;
}
export interface ResourceDescriptor {
  kind: Resource["kind"];
  schemaVersion: number;
  schema: Schema; // Effective projected schema supplied by the authorized host.
  capabilities: Capability[];
  observation?: "ordered-changes" | "refresh-snapshots";
  delivery: "deltas" | "snapshots";
  limit: number;
}
export interface SubscriptionIdentity {
  subscriptionId: string;
  resource: string;
  epoch: string;
}
export interface ResourceState<T extends Data = Data> {
  readonly records: readonly ResourceRecord<T>[];
  readonly status: "idle" | "loading" | "live" | "ready" | "stale" | "error" | "forbidden";
  readonly loading: boolean;
  readonly stale: boolean;
  readonly error: ResourceError | null;
  readonly capabilities: readonly Capability[];
  readonly pending: readonly string[];
  readonly updatedAt: string | null;
  readonly sourceUpdatedAt: string | null;
  readonly receivedAt: string | null;
  readonly nextCursor: string | null;
}
export interface Reduction {
  records: readonly ResourceRecord[];
  seq: string | null;
  sourceUpdatedAt: string | null;
  initialized: boolean;
  nextCursor: string | null;
  stale: boolean;
}
export const EMPTY_REDUCTION: Reduction = Object.freeze({ records: Object.freeze([]), seq: null, sourceUpdatedAt: null, initialized: false, nextCursor: null, stale: false });
export const IDLE_STATE: ResourceState = Object.freeze({
  records: Object.freeze([]), status: "idle", loading: false, stale: false, error: null,
  capabilities: Object.freeze([]), pending: Object.freeze([]), updatedAt: null,
  sourceUpdatedAt: null, receivedAt: null, nextCursor: null,
});

export function deepFreeze<T>(value: T): T {
  if (value && typeof value === "object" && !Object.isFrozen(value)) {
    for (const child of Object.values(value)) deepFreeze(child);
    Object.freeze(value);
  }
  return value;
}

// Envelope ID ordering is Unicode code-point order, not locale or UTF-16 order.
export function compareIDs(a: string, b: string): number {
  const left = Array.from(a), right = Array.from(b);
  for (let i = 0; i < Math.min(left.length, right.length); i++) {
    const difference = left[i].codePointAt(0)! - right[i].codePointAt(0)!;
    if (difference) return difference;
  }
  return left.length - right.length;
}

/** Pure reducer. No fetches, sockets, React, optimistic writes, or source cases. */
export function reduceResource(
  state: Reduction,
  identity: SubscriptionIdentity,
  descriptor: ResourceDescriptor,
  incoming: unknown,
): Reduction {
  if (state.stale) return state; // This generation is retired until a new epoch.
  if (incoming && typeof incoming === "object") {
    const raw = incoming as Partial<ResourceEvent>;
    // A complete mismatched routing tuple is an old/foreign message, not a fault.
    if (typeof raw.subscriptionId === "string" && raw.subscriptionId !== identity.subscriptionId ||
        typeof raw.resource === "string" && raw.resource !== identity.resource ||
        typeof raw.epoch === "string" && raw.epoch !== identity.epoch) return state;
  }
  let event: ResourceEvent;
  try { event = validateEvent(incoming, descriptor, descriptor.schema); }
  catch { return { ...state, stale: true }; }
  const sequence = BigInt(event.seq);
  if (state.seq !== null && sequence <= BigInt(state.seq)) return state;
  if (!state.initialized) {
    if (event.op !== "snapshot" || sequence !== 0n) return { ...state, stale: true };
  } else if (sequence !== BigInt(state.seq!) + 1n) return { ...state, stale: true };
  if (event.op !== "snapshot" && descriptor.delivery !== "deltas") return { ...state, stale: true };
  let records: ResourceRecord[];
  let nextCursor = state.nextCursor;
  switch (event.op) {
    case "snapshot":
      if (event.records.length > descriptor.limit) return { ...state, stale: true };
      records = structuredClone(event.records);
      nextCursor = event.nextCursor ?? null;
      break;
    case "insert":
      if (state.records.some(record => record.id === event.record.id) || state.records.length >= descriptor.limit) return { ...state, stale: true };
      records = [...state.records, structuredClone(event.record)].sort((a, b) => compareIDs(a.id, b.id));
      break;
    case "update":
      if (!state.records.some(record => record.id === event.record.id)) return { ...state, stale: true };
      records = state.records.map(record => record.id === event.record.id ? structuredClone(event.record) : record);
      break;
    case "delete":
      records = state.records.filter(record => record.id !== event.id);
      break;
  }
  return deepFreeze({ records, seq: event.seq, sourceUpdatedAt: event.sourceUpdatedAt ?? null, initialized: true, nextCursor, stale: false });
}
