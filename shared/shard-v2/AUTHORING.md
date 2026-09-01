# Authoring a Shard v2 resource app

For agent authoring, call `get_authoring_guide` first. It returns the data APIs
enabled on the connected backend, without presenting a runtime-version choice.
With resources enabled, `create_app` seeds a valid `contract.json` containing an
owned settings singleton, alongside `index.tsx` and `anchors.json`. It returns the
contract and matching guide. The starter writes no data and grants no agent access.
Extend or replace its declarations with the example below.

For complex backend composition, add optional `graphql` and `lambdas` sections.
GraphQL queries and mutations are named and persisted in `contract.json`; shard code calls
`executeGraphQL(operationId, variables)`. Resolver files may import
`defineResolver` from `@aladin/shard-runtime`. Each resolver declares binding-level
capabilities such as `tasks:query` plus operation, document, time and memory budgets.
The handler receives `(args, context)` and can call only
`context.capabilities.call("tasks:query", input)`. It never receives a database
client. Lambda handlers use the same bundle and budgets and currently support the
explicit `{ "kind": "manual" }` trigger. Build failure leaves the active release
unchanged.

Use `subscribeResource` for live views. The current GraphQL execution route is
request/response and rejects persisted `subscription` documents at contract
compile time.

For an existing app, pass `page_id` to get guidance matching its files and protected
release. Enabling resources does not migrate existing KV data or change its guide.
A resource app whose execution is disabled returns an unavailable-runtime message;
do not remove its contract or rewrite its storage to bypass that restriction.

This example requires resources enabled on the target backend. Start with a new
sandbox shard. Bulk-write the three files below with automatic build disabled,
then build once. Successful
draft builds activate draft code/data only. `publish_app` verifies the staged
published build and activates it; published data remains separate from draft.

The workspace normally displays the published release. Select **Preview draft**
to follow authoring builds with separate test data, and **Back to published** to
resume normal use. Unpublished shards show the draft warning automatically.
Publication returns open viewers to published; later draft builds do not redirect
their data. Publishing does not copy preview records into the published dataset.

## contract.json

```json
{
  "version": 2,
  "intent": "A small task list with persistent owned data",
  "resources": {
    "tasks": {
      "uri": "shard://self/resources/tasks",
      "kind": "collection",
      "meaning": "Tasks created and maintained by this shard",
      "schemaVersion": 1,
      "schema": {
        "type": "object",
        "properties": {"title": {"type": "string"}, "done": {"type": "boolean"}},
        "required": ["title", "done"],
        "additionalProperties": false
      },
      "source": {"provider": "shard.documents", "dataset": "tasks"},
      "operations": ["snapshot", "query", "insert", "update", "delete"],
      "observe": {"mode": "changes", "protocol": "shard-data/1"},
      "exposure": {
        "app": ["snapshot", "query", "observe", "insert", "update", "delete"],
        "agent": ["snapshot", "query"]
      },
      "query": {"filterFields": ["/done"], "sortFields": ["/title"], "maxLimit": 100}
    }
  },
  "bindings": {"tasks": {"resource": "tasks"}}
}
```

This is a persistent collection in the existing PostgreSQL engine. It is not
external workspace data and does not need another backend integration. A
singleton uses `kind: "singleton"` and has one record with ID `value`; insertion
is still explicit, not an automatic default write. Resource capabilities are
separate for the app and agent; this example intentionally gives the agent no
write access. Missing exposure grants nothing.

## anchors.json

```json
{
  "version": 1,
  "intent": "Task list",
  "anchors": [
    {"id": "tasks", "route": "#/", "meaning": "Current tasks", "binding": {"id": "tasks"}}
  ]
}
```

V2 anchors reference binding IDs. They do not embed schemas, provider parameters,
URLs or credentials. Narrative-only anchors can omit a binding.

## index.tsx

```tsx
import { useState } from "react";
import { createRoot } from "react-dom/client";
import { Page, Region, Button, useResource, resourceRequestId } from "@aladin/kit";

type Task = { title: string; done: boolean };
function App() {
  const tasks = useResource<Task>("tasks");
  const [message, setMessage] = useState("");
  async function add() {
    if (!tasks.insert) return;
    const command = { requestId: resourceRequestId(), data: { title: "Review design", done: false } };
    try { await tasks.insert(command); setMessage(""); }
    catch (error) { setMessage(String((error as {message?: string})?.message ?? error)); }
    // A production retry UI must retain command unchanged if its outcome is unknown.
  }
  return <Page><Region anchor="tasks" kind="collection">
    {tasks.loading && <p>Loading…</p>}
    {tasks.stale && <p>Reconnecting; displayed records may be out of date.</p>}
    {tasks.error && <p>{tasks.error.message}</p>}
    {message && <p>{message}</p>}
    <Button disabled={!tasks.insert || tasks.pending.length > 0} onClick={add}>Add task</Button>
    {tasks.records.map(record => <p key={record.id}>{record.data.title}</p>)}
  </Region></Page>;
}
createRoot(document.getElementById("root")!).render(<App />);
```

No author fetch/WS or bootstrap code is needed. The kit handles snapshots,
subscriptions, schema validation, reconnect and stale/revoked states. Updates
replace the complete data object and require the current opaque revision:

```ts
await tasks.update?.({ requestId: resourceRequestId(), id: record.id,
  baseRevision: record.revision, data: { title: record.data.title, done: true } });
await tasks.remove?.({ requestId: resourceRequestId(), id: record.id,
  baseRevision: record.revision });
```

Each example is a separate operation: after an update, use the refreshed revision
before deleting. On a conflict, refresh and let the user reconcile. Do not retry
commands with a new request ID after an unknown outcome. No optimistic writes or
automatic offline queue is provided. `resourceRequestId()` uses secure random
bytes and works in opaque iframe contexts without `crypto.randomUUID()`.

## External data and queries

For workspace data, use a resource with `source.provider: "workspace.nodes"` and
`source.params: {"ids": ["artifact-…"]}` containing actual authorized IDs. Declare
its NodeView schema (at least an object; useful fields include `id`, `kind`,
`title`, `data`), `snapshot` operation and read-only exposure. Add the same observe
block and `observe` grant for subscribed refresh snapshots. The builder checks
that IDs resolve and the real output fits the declared schema. This provider
never writes workspace objects through a resource mutation.

A binding can narrow those fixed IDs with a declared input, but cannot grant
other IDs. A third binding/resource using an existing provider needs no new
bridge method or reducer logic. Arbitrary APIs, joins and analytics require an
explicit registered backend provider with authorization and bounded queries.

`queryResource("tasks", {where: {field: "/done", op: "eq", value: false},
orderBy: [{field: "/title", direction: "asc"}], limit: 25})` queries on the server.
See CLIENT.md for read-current cursor pagination. The declaration controls which
fields can be queried; the client cannot ask for hidden projection fields.

V1 `useKV`, `useShardState`, `useNode/useNodes` and component `stateKey` persistence
are not translated into v2 resources. Use React state for temporary UI state and
declare a singleton/collection for saved state. Migrate existing saved keys only
through the reviewed procedure in MIGRATION.md.
