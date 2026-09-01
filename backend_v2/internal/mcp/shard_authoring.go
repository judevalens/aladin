package mcpserver

import (
	"context"
	"errors"
	"os"

	"aladin/backend_v2/internal/service"
)

// The shared UI reference contains no storage API assumptions. Exactly one
// data guide is selected using runtime configuration and the target's files.
const kitComponentGuide = `This guide describes the capabilities available for this authoring target. Use them directly; do not ask the user to choose a runtime version.

A shard is a REACT app written in TypeScript/TSX. create_app returns a working index.tsx (current_index_tsx) and the files required by this backend — write_file a COMPLETE, valid index.tsx that EXTENDS it: a full module with the React imports at the top, your <App/> component, and the createRoot render at the bottom. Never write a fragment, markdown, or prose into a .tsx file. Import components from "@aladin/kit". Style with Tailwind + Aladin tokens ONLY — never arbitrary hex.

index.tsx MUST be a complete module and end with:
  import { createRoot } from "react-dom/client";
  createRoot(document.getElementById("root")!).render(<App />);

@aladin/kit exports (all token-styled, self-contained):
- Layout: <Page>, <Section> (centered, max-w-3xl), <Panel>, <Card>, <Toolbar>, <Divider/>
- Regions (wrap each addressable part; add a matching entry in anchors.json): <Region anchor="intro" kind="narrative|metric|chart|collection|control">…</Region>
- Routing (hash, for multi-view shards): <Route path="/x">…</Route>, <Link to="/x">…</Link>, useRoute()
- UI: <Button variant="primary|outline|ghost|danger" size="sm|md">, <Badge tone="neutral|amber|for|against">, <Callout tone="info|warn|for|against" title="…">, <Stat label={…} value={…} sub={…}/>, <Tabs tabs={[{id,label,content}]}/>, <Dialog open onClose title>, <Input>, <Textarea>, <Field label hint>
- Semantic colored text: <For>, <Against>, <Catalyst>, <Echo>
- Data display: <DataTable columns={[{key,label,render?,align?,width?}]} rows={…} rowKey={r=>r.id} onRowClick? empty?/>, <KeyValue items={[{label,value,hint?}]}/>, <MetricRow metrics={[{label,value,delta?,hint?}]}/>, <Sparkline points={[1,2,3]} tone?/>, <Delta value={-2.5} suffix?/>, <ProgressBar value max? label?/>
- App chrome + forms: <AppShell title nav={[{id,label,to}]} footer?>…</AppShell> (hash-routed sidebar), <SearchInput value onChange/>, <Select options={[{value,label}]} value onChange/>, <Checkbox checked onChange label?/>, <RadioGroup name options value onChange/>, <EmptyState title hint? action?/>, <LoadingState label?/>, useToast().show(msg, "neutral|for|against") with a single <Toasts/> mounted near the root
- Interactive components (in-memory state; persistence guidance below): <Quiz questions={[{id,prompt,choices:[{id,text}],answerId,explanation?}]} onComplete?/>, <Flashcards cards={[{id,front,back}]}/>, <Timer seconds label? onComplete?/>, <Checklist items={[{id,label}]} onChange?/>, <Stepper steps={[{id,title,content}]}/>

Theme: shards follow the app's theme automatically (utilities + tokens re-resolve on switch). Only when you compute a color at render time (tok(), chart colors, hand-drawn SVG) call useTheme() in that component so it re-renders on a switch.

Tokens (Tailwind classes): surfaces bg-bg/bg-panel/bg-card/bg-raise/bg-field; ink text-ink/text-ink-2/text-ink-3/text-ink-4; accent text-amber, border-amber-line; lines border-line; radius rounded-card/rounded-chip/rounded-modal; fonts font-display/font-mono/font-sans.

Charts: run install_lib "recharts" first, import from "recharts", theme via the kit: <XAxis {...chartAxis()}/>, <CartesianGrid {...chartGrid()}/>, <Tooltip {...chartTooltip()}/>, and series colors from chartSeries()[i]. (import { chartAxis, chartGrid, chartTooltip, chartSeries } from "@aladin/kit")

Animations: Tailwind (transition-*, hover:*, animate-pulse) or your own CSS keyframes in a .css file you write_file and import.

`

const kitKeyValueGuide = `Shard-local storage (the shard's own little database — persists per user, survives reload, syncs across the user's clients):
  const [value, setValue] = useShardState<T>("settings", initial)  // one key; setValue takes a value or (prev)=>next and retries safely if another client wrote first
  const { entries, put, remove, loading } = useKV("expenses/")     // a LIVE view of every key under a prefix — this is how a mini-app holds a collection
  Keys are stable paths: "settings", "filters", "layout/main", "scenario/base", "expenses/2026-08-01". A prefix IS a collection. Values are small JSON (16KiB max per key, 1MiB per shard). Use it for app/UI data — filters, layouts, entries, progress — never for knowledge that belongs in the workspace.

Workspace data (read-only, opt-in): declare the entity ids a region depends on in that anchor's "refs" in anchors.json, then const { node } = useNode("artifact-…") / useNodes([...]). Refs are the GRANT — a read of anything undeclared is refused, and publish fails if a ref doesn't resolve. Id forms: "artifact-…", "record-…", "research-…", and "watchlist:<uuid>" for kinds whose ids are bare uuids.

Interactive component persistence: Quiz, Flashcards, Timer, Checklist and Stepper also accept an optional stateKey. With it their state persists; without it state remains in memory. A persisted timer stores its target timestamp.

Preserve this shard's existing storage keys and data APIs. Do not add a resource contract or rewrite its persistence as part of a routine edit.
`

const kitAuthoringLoop = `Loop: after each write_file/edit_file, READ the returned build log — if it has errors, fix the exact file and write again until build.ok is true. Then verify_app (it checks that every anchor you declared is really in the DOM on its route, that declared sources resolve, and that nothing threw) before publish_app. Keep components small and valid; prefer kit primitives over hand-rolled markup.`

const kitResourceGuide = `Shard data is managed through declared resources. create_app seeds contract.json and anchors.json automatically. Extend those files; keep their schema-version fields intact. The runtime details are handled by the backend, not a choice to put to the user.

Available data sources:
- shard.documents: the shard's own persistent collections and singletons. Supports create, full replacement, delete, declared-field filtering/sorting, cursor pagination and subscribed refresh snapshots. No extra storage integration is needed for journals, trackers, forms, saved settings or learning progress.
- workspace.nodes: read-only snapshots of explicitly granted artifacts, records, research and watchlists. Supported observable entity kinds can refresh live. The source takes fixed authorized IDs, not arbitrary workspace queries; it does not provide joins, structured filtering, cursor pagination or workspace mutations.
- Other managed sources or advanced analytics require an installed backend provider. Do not infer that an available Copilot market tool is automatically a shard resource provider.

Contract authoring:
- Each resource declares uri (shard://self/resources/<id>), kind (collection or singleton), meaning, schemaVersion, an object-root JSON Schema, source, operations and separate exposure.app/exposure.agent capabilities. Always include snapshot. Omitted exposure grants nothing. For owned data use source:{provider:"shard.documents",dataset:"tasks"}; for workspace data use source:{provider:"workspace.nodes",params:{ids:["artifact-..."]}} with real authorized IDs and a matching NodeView object schema.
- Declare bindings by resource ID. An anchor references binding:{id:"tasks"}; sources and grants belong in the contract. A new resource using an existing source needs no new bridge method.
- Example owned collection declaration, to add under contract.resources:
  "tasks": {"uri":"shard://self/resources/tasks","kind":"collection","meaning":"Tasks managed by this shard","schemaVersion":1,"schema":{"type":"object","properties":{"title":{"type":"string"},"done":{"type":"boolean"}},"required":["title","done"],"additionalProperties":false},"source":{"provider":"shard.documents","dataset":"tasks"},"operations":["snapshot","query","insert","update","delete"],"observe":{"mode":"changes","protocol":"shard-data/1"},"exposure":{"app":["snapshot","query","observe","insert","update","delete"],"agent":["snapshot","query"]},"query":{"filterFields":["/done"],"sortFields":["/title"],"maxLimit":100}}
  Then add bindings.tasks:{resource:"tasks"}. Bindings can project fields with select; never treat a projected read as a full replacement write. Optional inputsSchema/params resolve declared inputs and singleton dependencies; the backend reauthorizes them.

Import from @aladin/kit:
  const tasks = useResource("tasks", inputs?)
  // records: [{id,revision,schemaVersion,data}], status/loading/stale/error,
  // capabilities/pending/nextCursor/refresh, permitted insert/update/remove.
  await tasks.insert?.({requestId:resourceRequestId(),data:{title:"Review",done:false}})
  await tasks.update?.({requestId:resourceRequestId(),id:record.id,baseRevision:record.revision,data:completeData})
  await tasks.remove?.({requestId:resourceRequestId(),id:record.id,baseRevision:record.revision})
  const page = await queryResource("tasks", {limit:25,orderBy:[{field:"/title",direction:"asc"}]}, inputs?)
  // Query needs the query capability and declared query.filterFields/sortFields.
  // Next page: retain query/inputs and add cursor:page.nextCursor.

The kit owns transport, schema validation, subscriptions, stale state and recovery. Add observe:{mode:"changes",protocol:"shard-data/1"} and the observe grant for subscribed refresh snapshots. This reports current state, not a lossless event log. Records are bounded; do not scan a large dataset client-side to emulate backend analytics. Singleton ID is value; insertion is explicit, never an automatic default write. Use React state for temporary UI state; use resources for saved settings/progress, including custom learning components. The listed interactive components keep their internal state in memory.

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

func shardAuthoringGuide(resources bool) string {
	data := kitKeyValueGuide
	if resources {
		data = kitResourceGuide
	}
	return kitComponentGuide + data + "\n" + kitAuthoringLoop
}

const unavailableShardGuide = `This shard's data runtime is unavailable on this backend. Do not build or publish it, remove its contract, or rewrite its saved data to use another API. Ask for the configured runtime to be restored. Existing saved data must be preserved.`

// Global discovery follows backend configuration. Editing follows the actual
// shard as well: enabling resources never silently migrates an existing app.
func (t docToolServer) existingShardAuthoringGuide(ctx context.Context, id string) (string, string, error) {
	contract, err := t.store.ReadFile(ctx, id, "contract.json")
	hasContract := err == nil
	if err != nil && !errors.Is(err, service.ErrNotFound) && !os.IsNotExist(err) {
		return "", "", err
	}
	if !hasContract && t.releases != nil {
		// A missing authoring file cannot turn an active resource shard into a
		// key/value app. Recover its authoring context from protected metadata.
		for _, channel := range []service.BuildChannel{service.ChannelDraft, service.ChannelPublished} {
			release, err := t.releases.Active(ctx, id, channel)
			if err == nil {
				return shardAuthoringGuide(true) + "\nThe authoring contract file is missing. Restore contract.json from the returned protected contract before building; do not change this shard's storage API.\n", string(release.Source), nil
			}
			if service.ResourceErrorCode(err) == "unsupported-capability" {
				return unavailableShardGuide, "", nil
			}
			if !errors.Is(err, service.ErrNotFound) {
				return "", "", err
			}
		}
	}
	if hasContract && !t.resourceAuthoringEnabled() {
		return unavailableShardGuide, string(contract), nil
	}
	return shardAuthoringGuide(hasContract), string(contract), nil
}
