package mcpserver

// The shared UI reference contains no storage API assumptions. Exactly one
// data guide is selected using runtime configuration and the target's files.
const shardSurfaceGuide = `This guide describes the capabilities available for this authoring target. Use them directly; do not ask the user to choose a runtime version.

A shard is a React app written in TypeScript/TSX. Its UI is ordinary React, semantic HTML and authored CSS. There is no Aladin component library. Do not import from "@aladin/kit" or imitate a generic dashboard template. Design the interface around the shard's actual job.

create_app returns a complete working index.tsx and the files required by this backend. Every index.tsx replacement must remain a full module with imports at the top and this mount at the bottom:
  import { createRoot } from "react-dom/client";
  createRoot(document.getElementById("root")!).render(<App />);
Never write a fragment, markdown or prose into a .tsx file.

Theme tokens are injected into every shard and update automatically when the host theme changes. No import is needed. Use token-backed Tailwind utilities as the primary styling system:
- surfaces: bg-bg/bg-panel/bg-card/bg-raise/bg-field; var(--color-bg), var(--color-panel), var(--color-card), var(--color-raise), var(--color-field)
- ink: text-ink/text-ink-2/text-ink-3/text-ink-4; var(--color-ink), var(--color-ink-2), var(--color-ink-3), var(--color-ink-4)
- accent and lines: text-amber/bg-amber/border-amber-line/border-line/border-line-2
- semantics: text-for/text-against/text-catalyst/text-echo
- type: font-display/font-mono/font-sans and text-meta/text-small/text-body/text-lead/text-title/text-display
- radii and shadows: rounded-tap/rounded-chip/rounded-control/rounded-card/rounded-modal and shadow-panel/shadow-modal/shadow-toast
Typography is part of the token system: use font-display for titles, font-sans for interface text, font-mono for data/code, and the named type steps instead of arbitrary font sizes. Use responsive Tailwind utilities for layout, spacing and interaction states. Write and import a .css file only when a custom visual, animation or selector is clearer than utilities; in that CSS use var(--color-*), var(--font-*), and the other injected properties instead of fixed palette values. CSS variables also work with currentColor for SVG.
The document shell applies live theme changes even if the shard imports no SDK. When code resolves a CSS variable into a concrete canvas/chart value during render, import useTheme from "@aladin/shard" and read the variable again when that hook changes.

Mark every addressable region with data-anchor and optional data-kind, then add the same ID to anchors.json:
  <section data-anchor="tasks" data-kind="collection">...</section>
Kinds include narrative, metric, chart, collection and control. This identity is plain DOM metadata, not a component.

For multiple views, use fragment routes only. Links must be href="#/path"; observe hashchange or use a local router that never calls pushState. Root-relative and ordinary relative links escape the authenticated shard document and fail verification.

Charts and other libraries are optional. Run install_lib first, import the package normally, and style it with injected CSS variables. Remote runtime scripts and arbitrary UI frameworks are not part of the authoring surface.

Animations may use CSS keyframes or Tailwind transition/animation utilities.

`

const legacyDataGuide = `Import the nonvisual compatibility APIs from "@aladin/shard". Shard-local storage persists per user, survives reload and syncs across the user's clients:
  import { useShardState, useKV, useNode, useNodes } from "@aladin/shard"
  const [value, setValue] = useShardState<T>("settings", initial)  // one key; setValue takes a value or (prev)=>next and retries safely if another client wrote first
  const { entries, put, remove, loading } = useKV("expenses/")     // a LIVE view of every key under a prefix — this is how a mini-app holds a collection
  Keys are stable paths: "settings", "filters", "layout/main", "scenario/base", "expenses/2026-08-01". A prefix IS a collection. Values are small JSON (16KiB max per key, 1MiB per shard). Use it for app/UI data — filters, layouts, entries, progress — never for knowledge that belongs in the workspace.

Workspace data (read-only, opt-in): declare the entity ids a region depends on in that anchor's "refs" in anchors.json, then const { node } = useNode("artifact-…") / useNodes([...]). Refs are the GRANT — a read of anything undeclared is refused, and publish fails if a ref doesn't resolve. Id forms: "artifact-…", "record-…", "research-…", and "watchlist:<uuid>" for kinds whose ids are bare uuids.

Preserve this shard's existing storage keys and data APIs. Do not add a resource contract or rewrite its persistence as part of a routine edit.
`

const shardAuthoringLoop = `Source inspection: use list_dir to discover files and grep_files to find code within this shard (not workspace search). Narrow searches with glob and check truncation/files_skipped; a partial search cannot establish absence. Use read_file with optional one-based inclusive start_line/end_line for exact context. Its hash always covers the entire file, even for a range. Prefer edit_file with expected_hash from that read and exact old_string/new_string; on a conflict, read again and reconsider the edit. Line numbers locate code but are not edit coordinates. Omit range arguments for an exact whole-file read. Preserve existing data APIs and keys.

Loop: for several files, call write_file or edit_file with build:false and then build_app once. Read every build diagnostic and fix the exact file until build.ok is true. Open the preview, exercise meaningful interactions and inspect its snapshot/console. Then run verify_app; it checks that every declared anchor is present on its route, sources resolve, links stay inside the shard, and nothing threw. Fix every failure before publish_app. A successful build alone is not evidence that the shard works.`

const resourceAuthoringGuide = `Shard data is managed through declared resources. create_app seeds contract.json and anchors.json automatically. Extend those files; keep their schema-version fields intact. The runtime details are handled by the backend, not a choice to put to the user.

Available data sources:
- shard.documents: the shard's own persistent collections and singletons. Supports create, full replacement, delete, declared-field filtering/sorting, cursor pagination and subscribed live snapshots. No extra storage integration is needed for journals, trackers, forms, saved settings or learning progress.
- workspace.nodes: read-only snapshots of explicitly granted artifacts, records, research and watchlists. Supported observable entity kinds can refresh live. The source takes fixed authorized IDs, not arbitrary workspace queries; it does not provide joins, structured filtering, cursor pagination or workspace mutations.
- Other managed sources or advanced analytics require an installed backend provider. Do not infer that an available Copilot market tool is automatically a shard resource provider.

Contract authoring:
- Each resource declares uri (shard://self/resources/<id>), kind (collection or singleton), meaning, schemaVersion, an object-root JSON Schema, source, operations and separate exposure.app/exposure.agent capabilities. Always include snapshot. Omitted exposure grants nothing. For owned data use source:{provider:"shard.documents",dataset:"tasks"}; for workspace data use source:{provider:"workspace.nodes",params:{ids:["artifact-..."]}} with real authorized IDs and a matching NodeView object schema.
- Declare bindings by resource ID. An anchor references binding:{id:"tasks"}; sources and grants belong in the contract. A new resource using an existing source needs no new bridge method.
- Example owned collection declaration, to add under contract.resources:
  "tasks": {"uri":"shard://self/resources/tasks","kind":"collection","meaning":"Tasks managed by this shard","schemaVersion":1,"schema":{"type":"object","properties":{"title":{"type":"string"},"done":{"type":"boolean"}},"required":["title","done"],"additionalProperties":false},"source":{"provider":"shard.documents","dataset":"tasks"},"operations":["snapshot","query","insert","update","delete"],"observe":{"mode":"changes","protocol":"shard-data/1"},"exposure":{"app":["snapshot","query","observe","insert","update","delete"],"agent":["snapshot","query"]},"query":{"filterFields":["/done"],"sortFields":["/title"],"maxLimit":100}}
  Then add bindings.tasks:{resource:"tasks"}. Bindings can project fields with select; never treat a projected read as a full replacement write. Optional inputsSchema/params resolve declared inputs and singleton dependencies; the backend reauthorizes them.

Import only the nonvisual client from @aladin/shard:
  import { useResource, queryResource, resourceRequestId } from "@aladin/shard"
  const tasks = useResource("tasks", inputs?)
  // records: [{id,revision,schemaVersion,data}], status/loading/stale/error,
  // capabilities/pending/nextCursor/refresh, permitted insert/update/remove.
  await tasks.insert?.({requestId:resourceRequestId(),data:{title:"Review",done:false}})
  await tasks.update?.({requestId:resourceRequestId(),id:record.id,baseRevision:record.revision,data:completeData})
  await tasks.remove?.({requestId:resourceRequestId(),id:record.id,baseRevision:record.revision})
  const page = await queryResource("tasks", {limit:25,orderBy:[{field:"/title",direction:"asc"}]}, inputs?)
  // Query needs the query capability and declared query.filterFields/sortFields.
  // Next page: retain query/inputs and add cursor:page.nextCursor.

The shard SDK owns transport, schema validation, subscriptions, stale state and recovery. Add observe:{mode:"changes",protocol:"shard-data/1"} and the observe grant for subscribed refresh snapshots. This reports current state, not a lossless event log. Records are bounded; do not scan a large dataset client-side to emulate backend analytics. Singleton ID is value; insertion is explicit, never an automatic default write. Use React state for temporary UI state and resources for saved settings or progress.

Updates replace complete data. Retain the original requestId and exact command for an explicit retry after an unknown outcome. No automatic optimistic/offline writes. Query pages are read-current and separate from the live hook; discard them on view/session changes. Credentials, backend URLs, environment and release hash never come from authored binding params or TSX. Public browser-compatible HTTPS/WSS data can be used subject to CORS, but it is outside managed resource guarantees; never embed private credentials.

Published resources are also accessible through the registered find_shard_resources, describe_shard_resource, read_shard_resource, query_shard_resource and mutate_shard_resource tools with their granted agent capabilities, even while the UI is closed. App and agent write permission are separate; a declared resource is not automatically agent-writable.

Build captures code+contract+anchors together. Draft builds activate draft only; build_app stages publishable code and publish_app verifies the exact build before activation. Preview uses real draft data (mount effects can mutate it), separate from published data. A renderer failure blocks publication. Incompatible schema changes require a reviewed migration; never rewrite another shard's storage to make an ordinary edit.
`

const starterResourceContractJSON = `{
  "version": 2,
  "intent": "Saved settings for this shard; extend with resources for its purpose.",
  "resources": {
    "settings": {
      "uri": "shard://self/resources/settings",
      "kind": "singleton",
      "meaning": "Settings saved by this shard",
      "schemaVersion": 1,
      "schema": {"type": "object"},
      "source": {"provider": "shard.documents", "dataset": "settings"},
      "operations": ["snapshot", "insert", "update", "delete"],
      "observe": {"mode": "changes", "protocol": "shard-data/1"},
      "exposure": {"app": ["snapshot", "observe", "insert", "update", "delete"]}
    }
  },
  "bindings": {"settings": {"resource": "settings"}}
}
`

func (t docToolServer) resourceAuthoringEnabled() bool {
	return t.releases != nil && t.releases.Enabled()
}

const runtimeAuthoringGuide = `Authored backend runtime is enabled on this target. GraphQL operations are named and persisted; raw GraphQL text is never accepted.

Add this exact shape under contract.graphql, adapting names and fields:
  "graphql": {
    "schema": "graphql/schema.graphql",
    "operations": {
      "taskSummary": {
        "document": "query TaskSummary { taskSummary { total open } }",
        "exposure": ["app", "agent"]
      }
    },
    "resolvers": {
      "Query.taskSummary": {
        "file": "resolvers/taskSummary.ts",
        "export": "default",
        "capabilities": ["tasks:query"],
        "budget": {"maxOperations": 1, "maxDocuments": 100, "timeoutMs": 1500, "memoryMiB": 32}
      }
    }
  }

graphql/schema.graphql:
  type Query { taskSummary: TaskSummary! }
  type TaskSummary { total: Int!, open: Int! }

resolvers/taskSummary.ts:
  import { defineResolver } from "@aladin/shard-runtime";
  export default defineResolver(async (_args: unknown, ctx: any) => {
    const result = await ctx.capabilities.call("tasks:query", {query: {limit: 100}});
    const records = result.records || [];
    return {total: records.length, open: records.filter((item: any) => !item.data.done).length};
  });

Shard UI:
  import { executeGraphQL } from "@aladin/shard";
  const result = await executeGraphQL<{taskSummary:{total:number;open:number}}>("taskSummary", {});

Resolver capability names are <binding>:<operation>. Every resolver declares only the capabilities it calls plus explicit operation, document, time and memory budgets. It receives context.capabilities.call(capability,input), never a database client, storage namespace or credential. Use useResource for live views because this GraphQL route is request/response and persisted subscription documents are rejected.

Manual lambdas use resolver-style TypeScript, the same capability/budget declarations, a {"kind":"manual"} trigger, and invokeLambda(name,input) from "@aladin/shard". Published agent-exposed GraphQL operations are available through execute_shard_operation.
`

func (t docToolServer) runtimeAuthoringEnabled() bool { return t.graphql != nil && t.graphql.Enabled() }

func shardAuthoringGuide(resources, graphql bool) string {
	data := legacyDataGuide
	if resources {
		data = resourceAuthoringGuide
	}
	if resources && graphql {
		data += "\n" + runtimeAuthoringGuide
	}
	return shardSurfaceGuide + data + "\n" + shardAuthoringLoop
}

const unavailableShardGuide = `This shard's data runtime is unavailable on this backend. Do not build or publish it, remove its contract, or rewrite its saved data to use another API. Ask for the configured runtime to be restored. Existing saved data must be preserved.`
