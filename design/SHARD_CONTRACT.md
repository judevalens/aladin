# SHARD_CONTRACT.md — the shard contract & catalog

**Status: DRAFT rev 1 — for markup, not locked.**
Companion to `design/SHARD_LOCAL_STATE.md` (the kv plane) and `design/SHARD_MODEL.md`
(the anchor manifest). This doc formalizes the two declarations a shard carries and
adds the third piece: the catalog that makes shards and their data queryable.

## The frame

Shards are mini-apps whose data schemas are created **on the fly**, by agents, at
runtime. Pages are queryable because their table was designed once at dev time;
shard shapes cannot be designed in advance — so we design the **meta-schema**
instead. In one line:

> **Shards are schemas created on the fly; the contract is their catalog entry;
> the agent is the query planner; entities are the foreign keys.**

This is a system catalog (`pg_catalog` for shard data) whose planner is an LLM.
The querying agent compiles in two stages, like any planner: meta query (which
shards, what do they own) → schema lookup (the contract entry: patterns, shapes,
meanings) → data query (`read_shard_state`, containment probes). Because the
planner is a language model, **prose `meaning` fields are the type signatures** —
they are load-bearing, not documentation.

## Two declarations, different jobs

A shard carries two files. This is **additive** — the contract does not replace
the manifest, and existing shards without a contract keep working (they are
simply not queryable).

| | `anchors.json` (exists, unchanged) | `contract.json` (new) |
|---|---|---|
| declares | the **presentation** surface | the **data** surface |
| contents | regions: id/kind/route/meaning/binding, per-anchor refs | consumes, state patterns, emits (reserved) |
| changes when | the UI changes | the data model changes |
| validated by | `docsurface.ValidateManifest` | `docsurface.ValidateContract` (new, same posture) |
| verified by | route drive: anchors present in DOM | kv-write observation: writes match declared patterns |

Drift between them is validation's job: an anchor `ref` that appears in no
`consumes` entry (or vice versa) is a **warning** at verify/publish, never a
failure. Grant enforcement (`ids ⊆ anchors.json refs` in `ShardBridgeService`)
is **unchanged** by this doc; whether the grant ever moves to `consumes` is a
later, separate decision.

## contract.json

```jsonc
{
  "version": 1,
  "intent": "Track open positions and journal the reasoning behind each",
  "consumes": [
    {
      "ids": ["watchlist:9d2e…", "artifact-a1b2…"],
      "meaning": "the swing-candidates watchlist and the sizing-rules page drive the checklist"
    }
  ],
  "state": [
    {
      "path": "positions/*",
      "shape": { "ticker": "string", "qty": "number", "thesis": "string" },
      "meaning": "one open position per key; the * segment is a client uuid",
      "refs": ["entity-…"]
    },
    {
      "path": "settings",
      "shape": { "sortBy": "string" },
      "meaning": "display preferences"
    }
  ],
  "emits": []
}
```

Field rules:

- **`intent`** — one sentence, what the app is for. Feeds discovery search.
- **`consumes[]`** — workspace reads, lifted to first-class with a **why**.
  `ids` use the same forms as anchor refs (`artifact-…`, `record-…`,
  `research-…`, `watchlist:<uuid>`). `meaning` required.
- **`state[]`** — the kv namespace, declared as **patterns, not instances**
  (`/items/{id}` semantics: declare the route, not every row). `path` uses the
  kv key grammar with `*` as a full-segment wildcard, matching `useKV(prefix)`
  idiom. **`shape` is advisory** (open, never enforced — the regen finding:
  prose carries the weight); **`meaning` is required** (same asymmetry as
  anchor `kind`/`meaning`). Optional **`refs`** are entity ids — the hard joins
  that plug this shard's data into the designed ontology spine.
- **`emits`** — reserved, must be empty in v1. Shards writing back to the
  workspace is exec-model-A-out; the slot exists so that day needs no v2.

## The catalog

**The file is authored; the row is derived.** `contract.json` lives in the
shard's file set (agent-authored, versioned with the files, snapshotted to
`.history/`). Publish **extracts** it into `shard_contracts` — never a
hand-written row, never two writable sources. The catalog is a projection: it
can always be rebuilt by re-parsing every live shard's contract.

Publish is the extraction point because it gets the semantics free:

- the index describes **what's live** (the published channel — the only channel
  that syncs, same rule as kv);
- publish already parses + validates, so an invalid contract never pollutes the
  index;
- unpublish/delete drops the row; republish overwrites it. Idempotent.

The row: `page_id`, `intent`, `consumes jsonb`, `state jsonb`, and a
`meanings_tsv tsvector` flattening intent + every meaning for lexical search.

## Discoverability — two sources, two question types

**Source A — declared (the catalog).** Answers *"does X"* — capability/purpose
queries ("shards that teach maths"). tsvector over intent + meanings + the
publish summary. Cheap, structured, and `find_shards` falls straight out of it.

**Source B — observed (the data).** Answers *"related to X"* when nothing
declared says so (the flashcards whose kv holds derivative rules). Two
mechanisms, both per-row and always fresh — no event-driven projection needed:

- `search_tsv tsvector GENERATED ALWAYS AS (jsonb_to_tsvector('english', value,
  '["string"]')) STORED` on `shard_kv` + GIN — Postgres natively flattens
  string leaves; freshness is by construction, not by frame-driven refresh.
- GIN `jsonb_path_ops` on `value` for containment probes
  (`value @> '{"ticker":"AAPL"}'`) — precise cross-shard structural search with
  zero schema foreknowledge.

Published channel only, filtered at query time. Range/sort queries inside
values are **out** (that's the designed-vs-on-the-fly line at the index layer;
a declared-hot pattern earning an expression index is a later optimization).

The agent-facing surface:

- **`find_shards(query?)`** — search the catalog (A) and optionally kv text (B);
  returns page_id, intent, matched meanings/patterns.
- **`read_shard_state(page_id, prefix?)`** — the contract + live published kv
  values. Works without a contract (raw keys), better with one (interpreted).
- The federated `search` tool's shard leg upgrades from title-ILIKE to a
  provider over the catalog — same tool surface, smarter hits.

## Verification — observed vs declared

The M4 posture, extended to data: **the contract promises, the harness checks.**
During `verify_app`'s route drive, diff draft-channel kv before/after:

- a write matching **no declared pattern** → warning ("undeclared state");
- a declared pattern with **zero live keys** → info ("declared, never written");
- anchors-refs vs consumes drift → warning.

All advisory. Publish reports them and refuses only on the existing hard
failures. The pressure point is the *editing agent* seeing observed-vs-declared
side by side while context is hot.

## Lifecycle

**Mutation — data outlives its schema.** There is no `ALTER TABLE` here and
**the platform never migrates shard data** (exec model A: no per-shard server
code, and N ad-hoc shapes can't be understood well enough to transform). The
shard migrates itself: defensive reads in shard code (the localStorage
tradition), taught by the authoring guide. Advisory shapes absorb the rest —
mismatch is a verify warning, never a break. Old contracts stay interpretable
via `.history/`.

**Deletion — the data and the replicas.** kv lifecycle follows the shard's:
soft-delete with the tree, per-key tombstones ride **frames through the sync
spine** (kv keys are synced entities living in desktop SQLite replicas —
tombstoning is replica convergence, not janitorial cleanup), hard purge on the
tree's retention. Catalog row drops immediately (discovery reflects live things
only). *Gap on main today: no kv delete path exists at all — deleting a shard
orphans its kv rows in Postgres and in every replica.*

**The durable-data rule.** If a shard's kv would ever hold data you'd grieve,
it belonged in the workspace (an artifact/record the shard **consumes**), not in
shard-local state. kv dies with the app, by design; the contract's `consumes`
side is where durable things live. The authoring guide states this so agents
put data in the right store from the start.

## Ontology posture

**No taxonomy.** The same decision the entity model already made, made again:

1. **Designed** (stable): the entity spine + kind convention; this catalog
   meta-schema. These are the only things we design.
2. **Registered** (on the fly): each shard's shapes, declared into the catalog;
   entity refs as the hard joins. Local schemas, global keys — shapes are never
   unified, they are anchored.
3. **Observed** (derived, advisory, reversible): verification keeps
   declarations honest; tiered entity resolution over kv text later derives the
   links agents forgot. Never blocks publish.
4. **Emergent**: kit `stateKey` components (Quiz/Checklist/Flashcards…) already
   impose shared shapes by library adoption. Recurring bespoke shapes get
   promoted into the kit as named kinds — the vocabulary standardizes through
   the dependency, never by mandate.

Degrades gracefully by construction: subtract the LLM and the mechanics
(catalog, patterns, GIN/tsvector, entity refs, grant) still give a 2015-era
system — FTS discovery, containment probes, human interpretation. Everything
the smart layer adds is pure upside on mechanics that stand alone.

## Non-goals (v1)

- `emits` behavior of any kind.
- Embeddings/semantic search — only if lexical proves too coarse (then via the
  ingestion pipeline, not bespoke).
- An MCP kv **write** tool.
- Per-shard expression indexes; catalog revision history beyond `.history/`.
- Any controlled vocabulary for shapes, kinds, or topics.
- Moving grant enforcement from anchor refs to `consumes`.
