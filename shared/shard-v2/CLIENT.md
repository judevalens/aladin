# Shard client data-stream library

The client is bundled into the nonvisual `@aladin/shard` SDK, with `useResource`, `queryResource` and
`resourceRequestId` exports. The host reads protected release metadata before
mounting a bridge/2 iframe; the shard SDK creates one lazy session from the matching
inert bootstrap. Author components do not construct sockets or bridge clients.
See [AUTHORING.md](AUTHORING.md) for a complete runnable example.

## Responsibilities

| Layer | Owns |
| --- | --- |
| Author component | Binding name, declared inputs, rendering, explicit mutations |
| React adapter | `useSyncExternalStore`, component attach/detach, input changes |
| ResourceClient | Shared subscriptions, dependency gates, pending mutations, recovery |
| Pure reducer | Validation, exact routing/sequence checks, immutable records |
| Bridge transport | Handshake, describe/read/subscribe, bounded pre-ack queue, unsubscribe |
| Host/backend | Grants, release/environment, parameter resolution, queries, projection, persistence |

One `ResourceClient` belongs to one mounted shard session. Never share it between
users, shard releases, environments, or host frames. The bootstrap owner closes
the client and bridge port on unmount or session replacement.

```ts
// Host/vendor integration, not code every author needs to repeat.
const port = new WindowResourceBridgePort(window, window.parent);
const transport = new BridgeResourceTransport(port);
const client = new ResourceClient(transport, compiledContract.bindings);
export const useResource = createUseResource(client);

// On session teardown:
client.close();
port.close();
```

## Component usage

```tsx
function Tasks({ account }: { account: string }) {
  const tasks = useResource("accountTasks", { account });

  if (tasks.status === "forbidden") return <p>Access is no longer available.</p>;
  if (tasks.loading && !tasks.records.length) return <p>Loading…</p>;
  return <>
    {tasks.stale && <p>Reconnecting; these records may be out of date.</p>}
    {tasks.error && <p>{tasks.error.message}</p>}
    <button onClick={tasks.refresh}>Refresh</button>
    {tasks.records.map(record => <div key={record.id}>
      {String(record.data.title)}
    </div>)}
  </>;
}
```

The hook returns `records`, `status`, `loading`, `stale`, `error`, `capabilities`,
`pending`, `updatedAt`, `sourceUpdatedAt`, `receivedAt`, `nextCursor`, `refresh`,
and permitted `insert`, `update`, `remove` methods. Read-only resources expose
undefined write methods. `updatedAt` and `receivedAt` describe local application
of an accepted event; only `sourceUpdatedAt` describes supplied source freshness.

Plain JavaScript consumers can call `client.resource(binding, inputs)`, read
`getSnapshot()`, and attach with `subscribe(listener)`. Creating a handle does
not open a stream. Equal binding+inputs share one stream regardless of object key
order; the last consumer releases it. Inputs are copied and snapshots are frozen.

## Mutations

```ts
const requestId = resourceRequestId(); // Retain with the command until resolved.
await tasks.update?.({
  requestId,
  id: record.id,
  baseRevision: record.revision,
  data: { title: "Review design", done: true }, // Complete resource data.
});
```

Updates replace all data, including removal of omitted optional properties.
Do not spread a projected record into a write: fetch or construct the complete
write schema. The server enforces this and checks baseRevision atomically.
Insert accepts optional ID, data and requestId. Remove requires ID, baseRevision
and requestId. Do not send a user, endpoint, environment or contract hash.

A command response never edits the view. A successful acknowledgement starts a
new authoritative snapshot; pending request IDs clear only after that snapshot.
Identical in-flight request IDs share a promise; changed payloads conflict.
The client never retries commands automatically, queues offline edits, or applies
optimistic writes. On an unknown timeout outcome, retain the original command
and request ID for an explicit server-idempotent retry. The durable 24-hour
idempotency record belongs to the backend, not this client cache.

## Subscription and dependency lifecycle

1. The adapter describes the authorized binding. Snapshot-only sources use read;
   observable sources use subscribe, registering push listeners before its ack.
2. The ack establishes subscriptionId, canonical resource URI and epoch. The
   reducer waits for snapshot sequence `0`; subsequent sequence numbers must be
   consecutive. Messages from other tuples and duplicate sequences are ignored.
3. Missing updates, conflicting inserts, gaps, malformed frames or queue overflow
   retire the generation. Recovery retains the last valid view as stale and
   opens a new snapshot with bounded retry backoff (250 ms to 30 s).
4. Revocation/not-found/contract-changed clears records, capabilities and pending state, aborts
   writes and prevents refresh from reviving the view. A new authorized session
   is required. Detaching the last listener also clears the cached view.
5. Binding dependency pointers wait for a non-stale singleton value. If missing,
   the dependent stream does not open. Changing the value clears the old view
   and opens a new generation; late messages cannot repopulate it. Dependencies
   are subscribed using their own default inputs; parent inputs are not copied
   implicitly. The backend independently resolves and authorizes all parameters.

Queues are bounded by both count (1000) and bytes (4 MiB), including the pre-ack
queue. Sorted/filtered views use replacement snapshots; raw deltas cannot safely
infer window membership. Client subscription creation unwinds on failure.

## GraphQL and lambdas

Persisted GraphQL queries/mutations and manual lambdas use the protected host bridge:

```ts
import { executeGraphQL, invokeLambda } from "@aladin/shard";

const graph = await executeGraphQL<{ project: { open: number } }>(
  "projectExecutionGraph",
  { projectId },
);
await invokeLambda("dailyRollup", { day: "2026-08-31" });
```

The client cannot send raw GraphQL text. The operation ID, exposure, schema,
resolver code and capabilities all come from the pinned release. A release mismatch
returns `contract-changed` and requires a reload. Live data uses
`subscribeResource`; persisted GraphQL subscriptions are not exposed by this
transport.

## Querying and pagination

```ts
import { queryResource } from "@aladin/shard";

const first = await queryResource("tasks", {
  where: { field: "/done", op: "eq", value: false },
  orderBy: [{ field: "/title", direction: "asc" }],
  limit: 25,
});
if (first.nextCursor) {
  const next = await queryResource("tasks", {
    where: { field: "/done", op: "eq", value: false },
    orderBy: [{ field: "/title", direction: "asc" }],
    limit: 25,
    cursor: first.nextCursor,
  });
}
```

`queryResource<T>(binding, query, inputs?, signal?)` performs one bounded backend
query, validates records against the authorized projected schema, and returns a
frozen snapshot. It does not replace or append to the hook's live view. Pass the
same filters, order, limit and inputs on the next page; only the cursor changes.
Cursors are opaque, expire after 15 minutes and become invalid when their
principal/release/view changes. Pages are read-current, not a transactionally
consistent export. Restart pagination if its view changes or the cursor expires.

Both helpers accept a record-data type parameter for component ergonomics;
runtime schemas remain authoritative. Filtering/sorting runs in the backend on
declared fields; additional filters cannot remove a binding's authored scope.
A query needs the `query` capability. Aggregations/joins need an appropriate
registered backend provider, not a growing client-side scan.

A one-shot result belongs to the calling component. Abort requests on teardown
and discard retained pages when the session/view changes; only the live hook
owns automatic revocation and stale-state handling. See BACKEND.md for the
service's authorization, persistence and refresh guarantees.
