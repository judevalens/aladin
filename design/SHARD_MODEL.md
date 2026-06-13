# The Shard Model — graph, manifest, projection

> The complete conceptual model behind Shards, captured from the design sessions of
> 2026-06-11/12. This is the thinking, not the implementation plan (that lives at
> `~/.claude/plans/shard-authoring-loop.md`). Read this to understand *why* the
> system is shaped the way it is; read the plan to build it.

**The model in one sentence:** the knowledge graph is the source of truth, the
manifest is each shard's declared interface to it, and the UI is a disposable,
regenerable **projection** of the graph — addressable, data-bound,
provenance-carrying, and steerable.

---

## Invariant 0 — we do not guarantee good UI

The foundational assumption everything else is built on, stated before anything
else so no later section can quietly over-promise:

> **Generated UI quality is stochastic and unguaranteeable. The architecture
> does not make UI good — it makes stochastic, live UI ergonomic, safe,
> steerable, and durable.**

We depend on the model's intelligence for quality, and we refuse to pretend
otherwise. The system splits cleanly along the deterministic/stochastic axis:

| Stochastic — bet on the model, never guaranteed | Deterministic — guaranteed by the architecture |
|---|---|
| is the UI good, useful, well-designed? | it can only touch declared entities (the grant) |
| did it pick the right transform (weighted vs simple mean)? | displayed knowledge traces to the graph (conservation, partial) |
| is the meaning/derivation prose accurate? | anchors resolve, stay unique, survive regeneration |
| did it choose the right data, the right layout? | the value provably depends on its sources (grounding) |
| | it cannot escape the sandbox (isolation) |

So the architecture is a **deterministic frame around stochastic content.**
Every mechanism in this document contains, steers, or persists the
stochasticity — none eliminates it:

- the **gate** bounds *failure modes* (catches structural/catastrophic ones,
  lets quality failures through to the human): machine catches structural,
  human catches quality.
- the **feedback loop** exists *because* output is stochastic — it converts
  "the UI might be wrong" into "the UI is cheap to correct."
- the **manifest** makes stochastic output addressable + durable, so
  corrections *accumulate* on a stable substrate.
- the **kit** + tool descriptions *raise the mean* of the quality
  distribution; the gate/feedback *bound the tails*. Neither pretends to a
  point mass.

The strategic payoff of this honesty: the intelligence-dependent half lives in
the model, the determinism-required half lives in the architecture — so shard
quality **rides the frontier for free**. As the model improves, output improves
with zero architecture change, because we refused to hand-code quality
heuristics that would have frozen us at today's intelligence.

Design heuristic that falls out: never spend effort on content-determinism
(futile, anti-frontier). Spend on rails (grant, grounding, continuity), steering
ergonomics (feedback), defaults (kit), and model context. **Shift the
distribution and bound the tails; never chase the point mass.**

---

## 1. The three-layer model

```
            ┌────────────────────────────────────────────────┐
 truth      │  ENTITY LAYER  (the knowledge graph, logical)  │
            │  theses · sources · records · insights · …     │
            │  one canonical write point per entity + outbox │
            └──────────────────────┬─────────────────────────┘
                                   │ ids (the only currency)
            ┌──────────────────────┴─────────────────────────┐
 interface  │  MANIFEST  (anchors.json, per shard)           │
            │  declares the shard's addressable surface and  │
            │  joins its regions to entities (refs)          │
            └──────────────────────┬─────────────────────────┘
                                   │
            ┌──────────────────────┴─────────────────────────┐
 projection │  SHARD UI  (React code → sandboxed iframe)     │
            │  opaque, disposable, regenerable phenotype     │
            └────────────────────────────────────────────────┘
```

### The entity layer is logical, not a database

"The KG" is **not Neo4j**. It is the *entity abstraction*: anything with a stable
id, an owner, and a lifecycle. What makes something part of the knowledge graph is
satisfying the **entity contract**, not residing in a particular store:

- **Resolvable** — existence can be checked
- **Projectable** — it can be rendered into a view DTO (NodeView)
- **Observable** — its changes emit events

Postgres is where canonical entity records happen to be materialized today. Neo4j
holds the *relationship-optimized projection* of them. Redis is queue/cache — an
echo, never a truth. A future entity kind could canonically live elsewhere, as
long as it honors the contract and the one hard rule:

> **Every entity has exactly one canonical write point, and its writes event
> (through the Postgres outbox, in the same transaction).** Storage-agnostic
> identity does not mean multi-master. Projections never mint ids.

### Everything above the truth is a projection

Neo4j (for traversal), NodeView DTOs (for the bridge), sync frames (for the
client), **and shards (for humans)** are all the same kind of thing: disposable,
rebuildable read models over the entity layer. The "graph" in knowledge graph
names the *shape of the knowledge* (entities and relations), not an engine.

A shard is therefore, precisely: **a projection of the KG with a backchannel** —
a read model that also carries feedback, select-to-graph, and capture flows back
through the same identity layer. Closer to a lens (get/put) than a one-way view.
The database-theory mapping is exact:

| Database concept | Shard equivalent |
|---|---|
| Source of truth | the entity layer |
| View definition | the manifest (`intent` + anchors) |
| The query | `binding` |
| Dependency set | `refs` |
| Materialized view | a baked shard |
| Live view | a bridge shard (`nodes.subscribe`) |
| View invalidation | staleness badging |
| Re-materialization | regeneration from the manifest |

---

## 2. The conservation law

> **If Aladin displays it as knowledge, it must be in the KG.**

Shards are views *of* the graph, so what a view shows must be a subset of what the
graph holds. Anything on screen with no lineage into the entity layer is a leak:
unciteable, unchallengeable, unrefreshable, invisible to every other surface.

The law applies to **knowledge, not presentation**. Three layers of what's on
screen:

1. **Claims, data, sources** — must be entities (or arrive live through refs).
2. **Derivation** — sorting, aggregating, normalizing already-reffed entities.
   That's code, documented by `binding`. No nodes for "sorted descending".
3. **Presentation** — layout, styling, chart-ness. Pure phenotype; the graph
   never knows it, by design.

Granularity test: **"says who?"** — if you could ever point at something and ask
that, it needs identity. The unit of ingestion is the *dataset or claim*, not the
datum (one record entity for a 50-row price series, not 50 nodes).

### Ingest-then-ref (the cold-start answer)

"Make me slides on a topic not in my workspace" is a research task that ends in a
rendering. The agent's flow: **research → capture → ref → render.** It fetches
from anywhere (its own tools are unconstrained), writes findings into the
workspace as entities (sources, records, insights), then builds the shard reffing
what it just created. The graph isn't a precondition for shards — **shards on new
topics are how the graph grows.**

The alternative — baking API data straight into the bundle — produces a research
workspace that develops knowledge outside its own knowledge base: each baked
shard makes the workspace look more populated while the graph stays hollow.

**Engineering consequence:** capture must be ONE cheap MCP call from an authoring
context, or agents will bake instead. Conventions lose to friction every time.

**And capture must be resolve-or-create, not create.** Cheap capture optimizes
for frequency, and frequent agent-captured entities is an entity-resolution
problem: the agent fetches an article already in the graph and captures it
again; ten "make me slides on X" tasks deposit ten near-duplicate records.
Without dedup, the hollow-graph failure is traded for a polluted-graph failure —
made *more* likely by removing friction. v1: exact-key dedup (URL for sources,
external id for records) at the capture call; similarity-based merge suggestions
later. Duplicates also multiply stale badges for the same underlying fact, so
dedup is staleness hygiene, not just tidiness.

**Payoff:** fetched-data staleness becomes entity staleness for free — the record
carries `fetched_at`; a re-fetch updates the record; every shard reffing it
lights up through ordinary machinery that never knew an external API existed.

---

## 3. The manifest — the shard's declared interface

**A manifest is to a shard what an API spec is to a service.** The code is the
implementation: opaque, free to change, nobody else's business. The manifest is
the public interface everything else binds to.

```jsonc
// anchors.json — at the page root, authored via write_file
{
  "version": 1,
  // shard-level key idea, written to the REGENERATION STANDARD:
  // a cold agent reading only this file can rebuild the shard's idea
  "intent": "Track open positions against their theses; surface conviction drift.",
  "anchors": [{
    "id":      "positions:table",     // stable, unique per route; survives refactors
    "kind":    "collection",          // collection|metric|chart|narrative|control
    "route":   "#/",                  // where it renders (hash routing convention)
    "source":  "components/Positions.tsx",
    "binding": "kg.query(type:thesis, status:open) → rows",   // provenance prose
    "refs":    ["t-nvda", "t-amzn", "idx-open-theses"],       // entity ids
    "meaning": "One row per open thesis; key = thesis id;
                clicking a row expands evidence."             // behavior included
  }]
}
// in the JSX:  <table data-anchor="positions:table">  — an ordinary attribute
```

The manifest **describes; it never does.** It has no behavior. All behavior lives
in the consumers that read it.

### Two id spaces, one join table

Anchor ids are identities the **shard** mints (its addressable surface). Refs are
identities the **workspace** owns. Every anchor entry joins one to the other —
which is why feedback, ingestion, staleness, and the bridge grant are all "just
joins."

### Two layers of anchoring

| | Declared layer (canonical) | Mechanical layer (evidence) |
|---|---|---|
| what | `anchors.json` + `data-anchor` | `data-aladin-key` (+ selector/text) |
| cost | agent discipline (gate-enforced) | emitted by kit components as props |
| carries | identity, binding, refs, meaning | data-instance key, instance position |
| survives | any refactor, even regeneration | as long as the component is used |
| role | the join table | instances within regions; clicks outside the declared surface |

**Stamping is done at the component level, not by a build-time transform shim**
(decided 2026-06-12). `Region` emits `data-anchor`; `Collection` emits
`data-aladin-key` from the same key React already requires on lists — both are
ordinary props the kit components spread onto their root element. No
`jsxImportSource` shim, no `JSXDev`, no `react/jsx-dev-runtime` wrapper, no dev
runtime shipped in published. A click resolves to an **evidence bundle**:
nearest declared anchor + instance key + sibling count + CSS selector + visible
text + route; the agent gets the *file* from the manifest's `source` field, not
from the DOM.

> Deprioritized / dropped: auto **`data-aladin-src` (file:line)** stamping. It
> needed the dev transform (the only thing that computes source locations) and
> would have shipped the dev runtime in published — and it was the lowest-value
> piece (redundant with the manifest's `source` for declared regions; line
> numbers drift, and we edit by string-match). It can return later as a
> **draft-only** convenience if it ever earns its place — never as a
> foundation. The React-19-safe "no fiber internals" principle is satisfied
> *more* fully this way: there is no React-runtime coupling at all.

### Genotype / phenotype, and regenerability

**Manifest = genotype; code = disposable phenotype.** The completeness standard
for `intent` and `meaning` is falsifiable: *could a cold agent rebuild the
region's key idea from the manifest alone?* Presentation is deliberately
unspecified — that is the agent's creative latitude and the honest source of
non-determinism.

Because consumers key on anchor ids, **identity survives regeneration**: a shard
rebuilt from its manifest by a different agent — different code, different layout
— keeps every comment, deep link, and graph edge resolving. A service can be
rewritten behind a stable API and no client notices; same property, same reason.

### The verification gate (what makes the manifest honest)

- **Draft build:** schema validation hard; live checks as warnings.
- **Publish:** live checks hard. Rides the existing chromedp preview harness:
  1. every anchor resolves to ≥1 live DOM node on its declared route
  2. anchor ids unique per route
  3. `data-anchor` in DOM but not in manifest → warn (undeclared surface)
  4. every ref exists (canonical store, via services)
  5. collection anchors: repeating children carry keys (soft)
  6. refs whose kind is not observable → warn ("can neither go live nor stale")
  7. **cross-build continuity (hard):** the submitted manifest's anchor-id set is
     diffed against the last *published* manifest's. An id that carries
     dependents (feedback rows, graph edges) and is absent → hard fail
     ("you're about to orphan 3 comments and 1 edge") — UNLESS the manifest
     declares it in a `renames: {old: new}` map, in which case publish
     atomically migrates the dependents (feedback re-keyed, edges re-pointed)
     before the diff runs. Ids with no dependents may vanish freely.
  8. **conservation coverage (warn):** knowledge-dense DOM (clusters of
     numeric/data-shaped content) outside any anchored subtree → warn
     ("undeclared knowledge-looking surface"). Structural, not value-matching —
     literal token⊆NodeView checks would false-fail legitimate derivation
     (computed metrics appear in no NodeView; that's what binding documents).
     Value-level lineage is out of scope; the reproducibility paths are baked
     snapshots and entity-promotion (§5), not a DSL.
- No Chrome binary → schema-only + loud warning. Chrome-present is a deployment
  requirement for real use.

The gate converts "the agent should keep these in sync" into "publish fails if
they aren't." The old fear about canonical layers — unverifiable drift — does not
apply to *binding* canonical: identity and wiring are mechanically checkable in a
way prose content never was. **The manifest is only canonical while the gate is
hard.**

### Verified vs trusted — the two tiers, stated honestly

The gate verifies **identity and wiring**. Several of this document's headline
properties live in the other tier — reported by an intelligent author and kept
honest socially — and the difference must not blur:

| Property | Tier | Mechanism |
|---|---|---|
| Anchors resolve, ids unique, refs exist | **verified** | gate, hard |
| Cross-build anchor continuity (the moat) | **verified** | gate check 7, hard — *promoted; this is the one durability claim that could not stay trusted* |
| Region grounding (output depends on declared refs) | **verified (live)** | differential test via the preview emulator — catches fabrication and unused refs (see §5) |
| Conservation law | **partially verified** | gate check 8, warn — structural coverage only; full conservation needs understanding what pixels mean, which "no AST" deliberately pushed out |
| Binding derivation (the transform *shape*) | **trusted** | the irreducible sliver — unverified inside a verified frame (refs exist + grounding); a wrong derivation misleads provenance only, caught socially, escapable via snapshot or entity-promotion (see §5) |
| Meaning/intent regeneration-completeness | **trusted** | social + kit nudges. No oracle exists: the phenotype is deliberately non-deterministic, so a regeneration can't be diffed against an original to score the prose. "Falsifiable in principle" ≠ verified |

Where a later section claims durability, regenerability, or conservation, read
it against this table.

### The two invariants

1. **Nothing consumes a shard by parsing it.** Every consumer reads the manifest.
   The day a consumer parses the frame, the canonical layer starts rotting.
2. **The gate stays hard at publish.** Discipline converted to mechanics, not hope.

---

## 4. refs — entities, and only entities

`refs` are **workspace entity identifiers**: theses, sources, records, insights —
whatever satisfies the entity contract. Never files, never DOM, never prose,
never `pg:`/`ext:` URIs.

### One set, four consumers

The authoring rule that keeps all consumers coherent: **refs = the set of
entities whose change should invalidate the region** (causal, not topical).

| consumer | reads refs as |
|---|---|
| gate | existence claims to verify |
| ingestion | edges to draw (shard region → entity) |
| bridge | the permission grant (`ids ⊆ refs`) |
| staleness | the invalidation set (entity updated → badge region) |

### Why refs never widen beyond entities

Every consumer is an **identity join**. You can't draw an edge to a SELECT
statement; an external API emits no change events; and — the one with teeth —
granting *reads of your own entities* and granting *network egress* are
categorically different security acts. Refs-as-grant is safe precisely because a
ref can never authorize anything its owner couldn't already read. Typed URIs
(`ext:alphavantage/NVDA`) would smuggle heterogeneous semantics into the one
field every consumer trusts to be homogeneous.

External data gets two honest doors instead: **ingestion** (durable — the high
road; the platform's connectors exist exactly for this) or a future **bridge-v2
external capability** (live — explicit egress scope, host-mediated,
user-visible permission; CSP still `connect-src 'none'`).

### Topic vs data (the NVIDIA question)

A ref grants and fetches an entity's **own data, never its neighborhood**.
A region showing NVIDIA's price chart refs the *price record* and the *article
source* — and the company node only if it renders the company's own fields.
"About NVIDIA" is an edge in the graph (`rec —about→ ent-nvidia`), not a ref.
Topical lookup ("where is NVIDIA visualized?") is a two-hop traversal:
`ent ←about— rec ←ref— region`. **Refs supply the causal hop; the graph's own
edges supply the topical hop.** Keeping refs strictly causal keeps staleness
high-precision — decorative refs would make every badge noise.

### The intensional limit, named honestly

Refs are an explicit list (extensional). "All open theses" or "latest NVIDIA
news" are predicates (intensional) — a new member matches the intension but is
in nobody's list. v1 accepts the gap: the **binding records the intension**, the
refs enumerate the known extension, and the mitigation is a **hub entity** (an
index/feed node that membership changes actually touch — ref the hub, observe
the set). Executable intension, if it ever lands, is an **intensional-refs**
feature (an existing query syntax, deferred) — *not* a binding DSL (§5). Do
**not** make refs "smart" (globs, type filters) in the meantime — a
half-intensional refs field is a worse DSL designed by accident. Also resist
`nodes.related()`
neighborhood queries in the bridge: they'd make grants transitive.

---

## 5. binding — the transform half of `bind`

### What it is

The shard's code is a composition: `code = present ∘ bind`, where
`bind : KG → content` produces the data a region shows and `present : content →
pixels` is the free, unspecified phenotype. The manifest declares `bind`; the
code adds `present`. And `bind` itself splits into selection + transform:

- **refs** is the *selection* — which entities (the extensional snapshot; §4).
- **binding** is the *transform* — what was done to them to produce the value.

So the canonical one-liner:

> **binding describes how the source entities (refs) were transformed into the
> value this region displays** — the source→content step, never the
> content→pixels step.

Each manifest field owns exactly one step of the chain:

| field | question | step |
|---|---|---|
| `refs` | *which* sources | selection |
| `binding.derivation` | *how* they became this value | transform |
| `meaning` | *what* the value signifies / how it behaves | significance |
| presentation (free) | *how* the value became these pixels | phenotype |

Keeping binding at the source→content step (not "how the element was built") is
exactly what lets it **survive regeneration**: a rebuild changes `present` and
preserves `bind`. If binding described the element's construction it would
entangle with presentation and not survive a rebuild — and durability dies.

### The artifact

```jsonc
"binding": {
  "evaluation": "baked",        // baked | live — STRUCTURED, machine-read
  "derivation": "weighted mean of the evidence scores on thesis t-nvda"  // prose
}
```

Only `evaluation` is structured and machine-relevant: it gates live-vs-staleness
behavior, and it makes one gate check possible — `evaluation: live` with an
unobservable ref → hard error (a live region that physically can't update).

### Descriptive, not generative

`derivation` is **written at authoring** (an output — the agent records what it
did) and **read on handoff** (feedback / regeneration / the provenance popover).
The generating agent does not consult it to learn what to build — it gets that
from the task and from `nodes.get`. "Context for the agent" is true only on the
*second pass*, where a later agent reads it as an intent hint.

Critically, prose derivation **does not converge to a value across runs** — it's
an ambiguous spec read by a stochastic interpreter (which evidence? weighted by
what? normalized how?), so independent interpretations diverge even on identical
data. Therefore derivation **explains** a value; it never **reproduces** one.
Determinism lives in the artifact the prose describes, never in the prose:

- **live** → the **code** (a deterministic interpreter; re-runs converge; only
  authoring was stochastic).
- **baked** → the **frozen value + an optional input snapshot**
  (`t-nvda@seq-47, scores [0.6,0.8,0.7] → 72`) — you *read* it, you don't
  re-derive it; the snapshot makes the arithmetic reproducibility-checkable and
  makes a regenerated value diffable (data-moved vs interpreted-differently).
- **promoted** → an **entity** (a derived insight, produced by a service:
  re-runnable, snapshotted, observable).

There is **no stable middle**: prose vague enough to read doesn't converge;
prose precise enough to converge is code written in English — so write code.

### Required iff computed

binding is the transform half, so it's **degenerate/omittable for pass-through
regions** (display an entity's own field — refs + meaning already explain it)
and **required exactly when the region computes** (aggregate, filter, derive —
where the transform is the only thing refs + meaning don't capture). Required in
proportion to how much the region computes rather than displays.

### Unverified, inside a verified frame — and why that's safe

binding's derivation is **not verified** — and that is correct, not a gap,
because it sits between things that *are* verified:

- the **sources** are verified (refs exist — §4).
- the **grounding** is verified for live regions: the gate's differential test
  perturbs each declared ref's projection through the preview emulator and
  confirms the region's output *varies* — proving the value genuinely derives
  from its refs. Fabrication and unused refs are caught here. (For live, the
  bridge grant also makes *under*-declaration impossible: the shard can only
  fetch `ids ⊆ refs`, so it physically cannot render undeclared-entity data.)
- the **value's determinism** is carried by code / snapshot / entity, not prose.

So the one genuinely trusted sliver is the **transform shape** (weighted vs
simple mean). It's low-stakes (a quality bug, not a fabricated number), caught
socially (the popover shows it; read_feedback returns it; a wrong label reads as
wrong to a human), and escapable (snapshot → checkable arithmetic; promote →
service-verified). The worst a wrong derivation does is *mislead about
provenance* — bounded, surfaced, human-judged — never a fake value or a broken
link, because those live in the verified layers.

This is the **vindication** of prose, not a concession: verifying derivation
would cost the no-AST property and buy almost nothing, since every catastrophic
failure is already caught in the verified frame. Verification spent on derivation
would guard the one door behind which nothing structural sits.

### No DSL — ever — for the transform

The pull toward a "binding DSL" conflates two separable things:

- **the transform** (weighted mean, top-N, aggregate) — **never needs a DSL.**
  The shard's JS already runs it, the platform never needs to re-run it, and a
  platform-side transform language would be a second, weaker implementation of
  what JS does. A DSL wouldn't even buy verification — it would verify *that the
  language evaluates*, not that the rendered pixels match (that's the forbidden
  parsing). The mirage of verification just moves the gap.
- **intensional selection** (the open-set problem, "all open theses") — *might*
  eventually want machine-readability, but that's an **evolution of refs**, not
  binding, and should reuse an existing query syntax (never a bespoke one),
  deferred behind hub entities and a bounded `members-of` grant primitive.

And the deeper reason: **anything that outgrows prose should become an entity,
not gain a language.** Want it verified, reused identically, live, or audited?
→ make it a derived insight entity, and the whole entity machinery applies for
free. The DSL urge is a mis-routed "this should be a node" urge. binding stays
the home of cheap, local, throwaway, region-local derivation — and a DSL would
formalize exactly the layer that should stay informal. (This corrects an earlier
"era 2 = executable DSL" framing: there is no DSL trajectory; there is
entity-promotion as the rigor escape valve, and intensional-refs as a separate,
maybe-never refs feature.)

The LLM-author clincher: a DSL's whole historical justification is empowering
*human* authors. Our author is an LLM with deep priors for English, JS, and SQL
and ~zero prior for an invented language — a bespoke DSL is the worst possible
authoring target. The "agent is the compiler" insight (§8) cuts here: our
compiler writes best in languages that already exist. Invent nothing.

---

## 6. Identity mechanics — names, not addresses

### No links between stores

Nothing in Neo4j points at Postgres. No foreign key crosses stores. It is the
**same id string** written in both places; the join happens in code. When
ingestion captures an entity: canonical row + outbox event (one tx) → projection
worker mirrors the node into Neo4j with the same id. The id **is** the pointer.

### Kind rides inside the id

Ids are minted with a kind prefix (`artifact-…`, `rec-…`, `src-…`) by their
owning service, prefix included, at the canonical write — and prefixes are
forever (identity, not naming). Resolution is a small static registry:

```go
var kinds = map[string]EntityService{ "artifact": …, "rec": …, "src": … }

type EntityService interface {
    Exists(ctx, id)   (bool, error)      // the gate's check
    NodeView(ctx, id) (NodeView, error)  // the bridge's fetch (principal-scoped)
    Observable()      bool               // capability: do my writes event?
}
```

No universal entity table: each kind keeps its own table, owned by its own
service — the existing api → service → repo house style extended one level up
with a common interface and a dispatcher. Every consumer (gate, bridge,
staleness) is a **generic loop over this interface**: add a kind, register its
service, every feature supports it with zero per-feature work. Where ids travel
without trustable prefixes (the sync wire), kind is stated explicitly
(`entityKind` + `entityId`) — same information, two encodings.

### Where-metadata is an anti-feature

Do **not** store "this entity's body lives in PG/Redis" on the entity. It's
derivable (so it can drift), consumers would branch on it (storage migrations
become breaking data changes), and no consumer has a correct behavior that
varies by store. The two legitimate neighbors: **body locators** private to the
owning repo (artifact files, shard sources on the data volume — invisible above
the repo) and **capability flags at the kind level** (the registry, derived at
runtime, never persisted per row).

> Asymmetry worth keeping: **fetch is per-kind code; observation is not.** A
> service implements no watching machinery — it promises its writes event
> (outbox, same tx). Delivery (drain → ws → host → shard) is one shared pipeline
> that doesn't know kinds exist. The service owns truth and announcement; the
> platform owns distribution.

---

## 7. Observability → live shards, stale shards

`Observable()` is the hinge for live data. Liveness is **forwarded observation**
— the shard never polls, never connects, never violates `connect-src 'none'`:

```
canonical write → outbox (same tx) → drain → websocket → host
  → host: which open shards subscribed this id? (ids ⊆ manifest refs)
  → postMessage push → SDK → React state → region re-renders
bundle.js untouched. esbuild never runs.
```

**Live and stale are one signal consumed two ways.** Subscribed region → push the
fresh node, pixels update. Baked region → can't update pixels, badge it stale.
Two fallbacks of the same observability chain.

**Rebuilds are for code; subscriptions are for data.** The build loop exists only
for phenotype changes. A live region tracks the graph through ordinary React
re-renders, forever, with zero builds.

### Observability is an authoring-time signal

The agent (or the kit) makes a three-way choice per region, informed by the
capability registry (which must be queryable at authoring time — via
shard_status / manifest validation):

1. **Observable + should track reality → write it live** (`LiveNodes`). The
   default for metrics, tables, "current state" regions.
2. **Observable + deliberately a snapshot → bake it.** "Analysis as of June 11"
   *should* freeze; it still gets the staleness badge; refresh = intentional
   agent re-bake. A legitimate choice, recorded in binding's time clause.
3. **Not observable → baking is the only option**, warned at authoring time so
   the agent says so in `meaning` rather than the user discovering a silently
   frozen chart.

Bounds, honestly: entity-grained (whole-node refresh, not field deltas);
near-real-time (~250ms drain cadence — right for knowledge work, not a stock
ticker); only for observable kinds (hence the gate warning).

---

## 8. The agent is the compiler; the manifest is its type system

No AST, anywhere. The traditional way to get data-bound UI is compiler magic —
AST transforms, reactive compilers — because the author is a human whose intent
must be *inferred*. Our author is an agent: the inference layer is unnecessary.
The agent reads declared semantics (this ref, observable: yes) and **emits the
wiring as ordinary code**. The same bet, paid three times:

- We don't parse code to learn what a shard shows → the agent **declares** it
  (manifest).
- We don't transform code to stamp source locations → we ride the **transform
  contract** the toolchain already honors (anchors/keys are component props, no pass).
- We don't compile reactivity in → the agent **writes** it, informed by declared
  observability.

Everywhere a conventional system puts code analysis, we put a declaration plus
an intelligent author. Declarations are checkable (the gate); intelligence is
steerable (feedback); ASTs are neither.

The gate, accordingly, never verifies "did the code wire reactivity correctly"
(that would be parsing again) — only the observable surface. Whether the agent's
subscription works is settled the way agents settle everything: by *looking*, in
the live preview. Behavioral verification for behavior, declarative verification
for declarations.

### The kit (@aladin/kit), correctly sized

Not required machinery — sugar with a purpose. `Region`, `Collection`,
`LiveNodes`, `HashRouter` make the invariants the path of least resistance
(anchor attrs, keys-as-data-ids, subscription cleanup, the preview fallback),
and a shared vocabulary makes regenerated shards *converge* where freeform React
diverges. Kit taxonomy = manifest `kind` taxonomy. Distributed like react itself:
embedded source, built by the existing vendor pipeline, content-addressed,
import-mapped, Tauri-cached; build meta records the kit hash (a regeneration
input). Grows from observed agent behavior (`declared:null` picks, recurring
hand-rolled patterns). Raw React stays a first-class escape hatch — the kit can
afford to be optional precisely because it isn't load-bearing.

---

## 9. UI stability — the manifest diff is a work order

The counterpart of regenerability: regeneration says the code is *disposable
when needed*; daily life says the code is **capital**. Because every region has
identity plus a source pointer, a manifest change translates into scoped edits:

- added anchor → build one region; everything else untouched
- changed binding → rewire that region (source field says which file; stamps say
  which lines)
- removed anchor → delete one region
- untouched anchors → untouched code, **by construction**

The anchors partition the codebase semantically; edits have natural blast-radius
boundaries. This is where the design parts ways with generative/intent-based UI,
whose failure mode is churn: every tweak regenerates the surface, layout
reshuffles, the user's spatial memory resets. Here the phenotype persists and
accretes; stability is achieved by **not regenerating what didn't change** —
and it's *checkable*, because regions with identity admit per-region screenshot
diffs (untouched regions should be pixel-stable; drift is a flagged ripple).

Economics: **manifest = spec with stable joints; code = capital that compounds;
small intent changes amortize against existing implementation.** Incremental
edits along declared seams daily; rebirth reserved for when you want it — and
even a pixel apocalypse preserves the workspace's relationship to the shard,
because anchors persist.

---

## 10. Positioning — what this is and isn't

**The differentiator:** generated UI as a first-class knowledge object —
addressable, data-bound, provenance-carrying, regenerable — instead of as
output. Artifacts/canvas features have generation but no substrate; notebook
tools have data binding but no agent loop and no graph; notes platforms have
neither at region level. The intersection is empty.

**On "intent-based UI":** the label fits the authoring side — and undersells the
rest. Most of that field is intent-*inferred* and ephemeral (generated per view,
disposable, nothing can depend on it). Shards are **intent-declared and
intent-verified**: intent is written down (manifest), grounded in entities
(refs), and mechanically enforced (the gate). The trap in the label: its
gravitational pull is regenerate-per-view personalization, which would erode
exactly what the moat is — durability (stable anchors, accumulated feedback,
graph edges, staleness history). Because identity lives in the manifest rather
than the code, the architecture could *eventually* afford ephemeral re-rendering
(genotype stable, phenotypes come and go) — a someday-option the identity layer
earns, not the v1 product.

**Differentiator ≠ moat.** The defense is architectural (addressability is a
layer you build *under* everything — cheap on day one, a migration nightmare to
retrofit) plus compounding (every consumer shipped deepens the layer's gravity).
Named dependency: shard value scales with graph population — a thin graph
degrades shards to exactly the opaque artifacts everyone else has.

**The demo the roadmap aims at:** point at a conviction number → provenance
popover → new evidence lands → region badges stale (or just re-renders, if
live) → "revise" → agent reads anchored feedback, patches → user watches the
draft rebuild live → the earlier comment still resolves. Every step is a join
keyed on the same identity layer.

---

## 11. The benefits, compounded

Immediate (this push, M0–M5):
- **Durable select-to-edit** — feedback on `(anchor, key)` survives refactors
- **Real ingestion** — the graph knows what's inside shards, at region level
- **Live build visibility** — status, errors, auto-reload; no more silent iframes
- **Live regions / staleness** — one observability chain, two fallbacks
- **Provenance on hover** — every number can answer "says who, as of when"
- **Agent ergonomics** — edit_file, shard_status (one-call cold resume),
  read_feedback with manifest context, piggyback notices

Enabled (the identity layer's gravity, later):
- **Deep links into regions** — navigate + highlight `shard#anchor=…`
- **Select-to-graph** — a click becomes a real edge (same menu, same anchors)
- **Region transclusion** — addressable + self-describing regions are embeddable
  (live tiles in pages/other shards; the manifest entry is the extraction contract)
- **Regeneration** — rebuild from genotype; consumers don't notice
- **Multi-agent authoring** — regions as work orders; cold takeover via manifest
- **Region-level visual regression** — diffs per anchor, tractable because named
- **Cross-shard consistency** — same entity rendered in N shards, coherence checkable
- **The bridge's permission model, free** — declared bindings = least-privilege
  grant, reviewable before granting

The pattern: every benefit is a **join** (feedback↔code, graph↔pixels,
agent↔region, page↔shard), and all of them key on the same identity layer.
That is the practical meaning of "the manifest is not incidental to the design —
it *is* the design."

---

## 12. Honest limits and risks

- **Intensional refs gap** — explicit lists can't capture predicates; hub
  entities mitigate; an eventual intensional-refs feature (existing query
  syntax) resolves it; don't half-fix refs in the meantime.
- **Granularity is one decision seen twice** — capturing at dataset/claim
  granularity (to avoid node explosion) is exactly what makes staleness fire at
  dataset granularity (any row changes → the whole region flags). The two can
  only be tuned together, or escaped by promoting the finer unit to its own
  entity.
- **Noisy graph ⇄ hollow graph** — conservation guarantees what's on screen is
  in the graph; it says nothing about the graph not filling with low-quality
  duplicates. Resolve-or-create capture (§2) is the guard; without it, cheap
  capture trades hollowness for pollution.
- **The gate needs Chrome at publish** — degrade to schema-only with a loud
  warning; Chrome-present is a deployment requirement for real use.
- **Manifest discipline taxes agents** — kit makes it the default, descriptions
  teach it, the gate fails loudly at publish rather than silently in the data.
- **Thin graph ⇒ thin shards** — the thesis's named dependency.
- **Conservation is only partially mechanical** — the gate's coverage check
  (warn) catches unanchored knowledge-dense surface; full conservation requires
  understanding what pixels mean and stays agent discipline. The §2 law is a
  norm with a tripwire, not an enforced invariant.
- **The transform shape and meaning are trusted prose** — binding's *grounding*
  is verified (live, §5), but the transform *shape* (weighted vs simple mean)
  and `meaning` are not — kept honest socially (read_feedback returns them;
  the popover shows them), escapable via snapshot/entity-promotion. For meaning,
  no oracle is possible.
- **The differentiator is invisible until the loop closes** — M1 and M3 each
  ship a user-visible win on the way; the demo sequence is the validation.

---

## 13. Pointers

- Implementation plan (milestones M0→M5, KIT-1/2, bridge protocol, schemas):
  `~/.claude/plans/shard-authoring-loop.md`
- Self-documenting shard (the architecture + roadmap, as a shard, in the
  workspace): artifact `artifact-35372924-86a5-4f1c-940f-392939d476f7`
  (routes: thesis / arch / security / tooling / anchors / kit / bridge / roadmap)
- Existing code: `backend_v2/internal/docsurface/` (store, build, vendor,
  tokens, preview), `backend_v2/internal/mcp/doc_surface_tools.go`,
  `backend_v2/internal/api/content.go`,
  `aladin_react/src/modules/doc-surface/`,
  `aladin_react/src-tauri/src/vendor_cache.rs`
- Naming: code says `docsurface`/`"app"` until `rename/doc-surface-to-shard`
  merges; "Shard" in user-facing strings only.
