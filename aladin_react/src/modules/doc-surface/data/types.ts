/** Shared v2 wire types. Runtime validation lives in contract.ts. */
export type JSONValue = null | boolean | number | string | JSONValue[] | { [key: string]: JSONValue };
export type Data = { [key: string]: JSONValue };
export type Schema = Record<string, unknown>;
export type Operation = "snapshot" | "query" | "insert" | "update" | "delete";
export type Capability = Operation | "observe";
export interface Resource {
  uri: string;
  kind: "collection" | "singleton";
  meaning: string;
  schemaVersion: number;
  schema: Schema;
  source: { provider: string; version?: 1; dataset?: string; params?: Data };
  operations: Operation[];
  observe?: { mode: "changes"; protocol: "shard-data/1" };
  exposure?: { app?: Capability[]; agent?: Capability[] };
  query?: { filterFields?: string[]; sortFields?: string[]; maxLimit?: number };
}
export type Predicate =
  | { and: Predicate[] }
  | { or: Predicate[] }
  | { field: string; op: "eq" | "gt" | "gte" | "lt" | "lte"; value: string | number | boolean | null }
  | { field: string; op: "in"; value: (string | number | boolean | null)[] }
  | { field: string; op: "exists"; value: boolean };
export interface Query {
  where?: Predicate;
  orderBy?: { field: string; direction: "asc" | "desc" }[];
  limit?: number;
  cursor?: string | null;
}
export interface Binding {
  resource: string;
  params?: Data;
  inputsSchema?: Schema;
  query?: Query;
  select?: string[];
}
export interface Contract {
  version: 2;
  intent: string;
  resources: Record<string, Resource>;
  bindings: Record<string, Binding>;
  graphql?: {
    schema: string;
    operations: Record<string, { document: string; exposure: ("app" | "agent")[] }>;
    resolvers: Record<string, RuntimeHandler>;
  };
  lambdas?: Record<string, RuntimeHandler & { trigger: { kind: "manual" } }>;
}
export interface RuntimeHandler {
  file: string;
  export: string;
  capabilities: string[];
  budget: { maxOperations: number; maxDocuments: number; timeoutMs: number; memoryMiB: number };
}
export interface ProviderProfile {
  version: number;
  operations: Operation[];
  observation?: "ordered-changes" | "refresh-snapshots";
  owned?: boolean;
  paramsSchema: Schema;
}
export type Registry = Record<string, ProviderProfile>;
export interface ResourceRecord<T extends Data = Data> {
  id: string;
  revision: string;
  schemaVersion: number;
  data: T;
}
export interface ResourceSnapshot<T extends Data = Data> {
  resource: string;
  records: ResourceRecord<T>[];
  complete: true;
  nextCursor?: string;
  sourceUpdatedAt?: string;
}
export interface Routing {
  protocol: "shard-data/1";
  subscriptionId: string;
  resource: string;
  epoch: string;
  seq: string;
  sourceUpdatedAt?: string;
}
export type ResourceEvent = Routing & (
  | { op: "snapshot"; records: ResourceRecord[]; complete: true; nextCursor?: string }
  | { op: "insert" | "update"; record: ResourceRecord }
  | { op: "delete"; id: string; revision?: string; reason: "deleted" | "left-view" }
);
export type Command = { resource: string; requestId: string; contractHash: string } & (
  | { op: "insert"; id?: string; data: Data }
  | { op: "update"; id: string; data: Data; baseRevision: string }
  | { op: "delete"; id: string; baseRevision: string }
);
export interface CompiledContract {
  contract: Contract;
  bindingOrder: string[];
  outputSchemas: Record<string, Schema>;
}
export const LIMITS = Object.freeze({
  jsonBytes: 1 << 20,
  jsonDepth: 64,
  recordBytes: 64 << 10,
  defaultLimit: 100,
  maxLimit: 500,
  queuedEvents: 1000,
  queuedBytes: 4 << 20,
  subscriptions: 32,
});
