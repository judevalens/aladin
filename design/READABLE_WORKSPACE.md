# The Readable Workspace — how content becomes legible to agents

> **Status:** draft — rev 1 (2026-08-26). Not locked; written for owner review after the
> board link-unfurl slice shipped. This is the **master strategy for indexing content as a
> whole** — the substrate under the learning layer and the research bench.
>
> Companions: `STUDY_PRD.md` (the consumer that motivated this) · `TUTOR_PRD.md` §7
> (pointer-over-paraphrase — the trust rule this generalizes) · `INGESTION_PRD.md` (the
> shipped file pipeline this pattern extends) · `RESEARCH_SURFACE_PRD.md` §3 #9 ("all of
> it becomes a context artifact other agents can run over" — the other consumer).

---

## 0. The thesis

**Every Aladin surface that involves an agent reduces to the same requirement: the agent
must be able to read the user's actual objects, with citations.**

- Quiz-me is only honest if it reads *your* excerpts and open questions — "zero invented
  content" (STUDY_PRD §12).
- A study plan is only real if it's drafted from the *actual* outline, links, and boards
  you gathered — not the model's memory of the topic.
- The research bench's promise (§3 #9) is that a strategy folder — hypothesis, runs,
  papers, threads — is something "other agents can run over."
- The trust guarantee everywhere is a **pointer**: "that's derived on p. 94." A pointer
  requires a locator, and a locator requires the content to have been made addressable.

So "index content" is not a search feature. It is the product property that makes the
learning layer (and every future agent surface) possible. Call the property
**readability**: an artifact is *readable* when an agent can get its identity, its
structure, its full text, and a way to point back into it — without a human translating.

The un-glamorous corollary: **readability is a per-kind contract, like syncability.** The
repo already has the "add a synced kind" recipe with named registration points that
silently drop frames when missed. This document defines the same thing for reading — and
the same failure mode: a kind that skips the contract is invisible to every agent surface,
silently.

## 1. The read ladder

Four levels. Every artifact kind must state where it stands on each.

| level | what | contract |
|---|---|---|
| **L1 Identity** | id, kind, title, folder, timestamps | the tree — `get_browser_tree`, `list_artifacts` (done, all kinds) |
| **L2 Rendering** | a cheap, deterministic *text rendering* of the artifact — what it is and what's in it, sized for a glance | per-kind projection, **no LLM in the path**; e.g. the board summary, a document's outline |
| **L3 Full text + locators** | complete content, addressable — every span carries a locator an app can reopen | `(artifactId, locator)` — `page:N` · block id · shape id · `t:SECONDS` (STUDY_PRD's opaque-locator decision, adopted globally) |
| **L4 Retrieval** | the artifact's text is findable from a query across the whole workspace | one federated front door; lexical first, semantic as an upgrade *inside* it |

L2 is the level most kinds are missing and the cheapest to add. L4 is the one everyone
reaches for first and should come last — retrieval over content that isn't rendered and
addressable retrieves garbage you can't cite.

## 2. Current state (verified against code, 2026-08-26)

| kind | L2 rendering | L3 text + locators | L4 retrieval | honest gap |
|---|---|---|---|---|
| **page** | blocks→markdown (MCP get_page) | ✅ block ids | federated (pages+shards via ArtifactRefService) | fine; not chunk-indexed |
| **file/PDF** | outline (chunk tree) | ✅ `artifact_pages`/`_regions`/`_chunks`, page+span locators | per-document FTS only (`search_document`, `artifact_pages.tsv`) — **excluded from federated search** (search.go:170 comment says so) | the one rich pipeline, invisible to global search |
| **board** | ✅ `summarizeBoardContent` — tasks/cards/excerpts/links/doc-windows with cites | shape ids exist; ink **text labels not in the summary** | ❌ not in federated search | summary is good and got links today; ink labels + search missing |
| **shard (app)** | contract.json + catalog (`find_shards`, branch awaiting review) | source files readable | catalog search only | merge the catalog branch; done |
| **link artifact** | title + URL only | ❌ no fetched content | title only via nothing (not federated) | **unfurl now exists shape-level on boards; the artifact kind still stores nothing** — the ingest front-end for URLs (INGESTION D-E fast-follow) is the real answer |
| **voice** | ❌ | ❌ **no transcription path exists** (grep: none) | ❌ | opaque blob; a captured thought no agent can read |
| **video** | — | — (locator `t:S` reserved) | — | future (STUDY S-G) |
| **ink/worksheets** | ❌ (strokes) | shape ids | ❌ | tldraw text shapes are legible rich text — cheap; raw strokes are a research problem, skip |
| **copilot threads** | — | turns exist in Postgres | ❌ | your own reasoning history is unreadable to future turns |
| **entities/records** | ✅ entity context | ✅ | federated (entities section) + KG embeddings (parked layer) | fine |
| **market data** | ✅ typed MCP tools | n/a | n/a | structured, not "content" |

Embeddings today exist **only** in the parked KG tables (`records`/`entities`/`claims`,
pgvector 1536 since baseline). No artifact content has vectors. That is not a scandal —
it's the correct order (see §1) — but M9's "embeddings" milestone should land as part of
this strategy, not beside it.

## 3. Principles (propose to lock)

1. **Every kind ships a projection.** A deterministic `kind → text-with-locators`
   function, maintained next to the kind, exercised by tests. No LLM in the read path —
   projections must be free, instant, and always current. (The board summary is the
   template: shape counts + the texts an agent can act on, cites inline.)
2. **Locators are opaque, universal, and preserved end-to-end.** `(artifactId, locator)`
   is the cite everywhere — wormhole opens it, quiz-me points with it, distill carries it.
   An index result that loses its locator is a paraphrase, and paraphrase is the failure
   mode the tutor rules exist to prevent.
3. **Readability begins at the door.** Content becomes readable when it *lands* (upload,
   paste, save), through the existing async pipeline (asynq + reaper + outbox status) —
   never lazily at query time. Failure is legible and retryable (INGESTION's rule).
4. **One front door for retrieval.** The federated `search` (API + MCP) is the only
   search an agent learns. New kinds join it as providers; semantic search upgrades its
   internals, never spawns a second tool. Per-document search (`search_document`) stays —
   it's scoped reading, not discovery.
5. **The index is a projection of projections — rebuildable, never canonical.** Same
   philosophy as the client replica: one derived table fed by per-kind projections,
   droppable and rebuildable from source at any time. No content lives only in the index.
6. **Freshness rides the spine.** Artifact-changed events (outbox / board projection
   debounce / ydoc→blocks projection) mark index rows stale; a worker re-projects. No
   polling, no manual reindex button as the primary path.
7. **Enrichment is derived and optional.** LLM summaries, entity links, claim extraction
   (the parked KG) are *consumers* of readable content, cached and non-load-bearing.
   Nothing in the read ladder waits on a model.

## 4. The mechanism: `content_index`

One thin table, fed by every kind's projector:

```
content_index (
  artifact_id   text        REFERENCES artifacts ON DELETE CASCADE,
  locator       text,       -- opaque; "" for whole-artifact rows (L2 renderings)
  kind          text,       -- artifact kind, denormalized for filtering
  text          text,       -- the projected span
  tsv           tsvector,   -- lexical, generated
  embedding     vector NULL -- semantic, filled by the async embedder when enabled
  updated_at / source_seq   -- staleness bookkeeping against the spine
)
```

- Pages project per block (locator = block id). Files project per page/chunk (already
  structured — the projector mostly copies). Boards project their summary lines with
  shape-id locators. Links project their fetched/readable content once URL ingestion
  lands, and their unfurl metadata immediately. Voice projects its transcript.
- The federated search's artifact provider queries this table instead of per-kind
  one-offs — which is precisely how "links, notes, files are a follow-up" (search.go's
  own comment) gets closed once instead of four times.
- `embedding` stays NULL until R3; lexical FTS is the floor, and it is already useful.

**Explicitly rejected:** a search sidecar (Elasticsearch/Typesense — a second store to
operate for one user); embedding-first (vectors over unaddressable text produce citations
that can't open); per-surface bespoke readers (the copilot learning N tools for N kinds).

## 5. Milestones

- **R0 — close the free gaps (days).** Board summary includes ink text labels; shard
  catalog branch merges; federated search gains files (title + outline hits from the
  existing tables) and boards (summary text). No schema. Every R0 item is an existing
  asset being plugged in.
- **R1 — the readable contract + `content_index` (the real slice).** The table, the
  per-kind projector interface, spine-driven freshness, pages + files + boards writing
  into it, federated search reading from it. Ships with a **recipe doc** ("add a readable
  kind") mirroring the synced-kind recipe — registration points named, silent-failure
  modes listed.
- **R2 — make gathered sources readable (the learning layer's Gather station).**
  URL ingestion: a link artifact's content fetched and run through the ingest engine
  (INGESTION D-E's fast-follow; reuses the readability + SSRF machinery the unfurl
  service just built). Voice transcription: transcript stored on the artifact, projected.
  After R2, *everything a study gathers* is readable except video.
- **R3 — semantic retrieval (subsumes M9's embeddings item).** Async embedder fills
  `content_index.embedding`; federated search blends lexical + vector behind the same
  tool. Only now — the citations already work, so semantic hits are automatically
  citable.
- **R4 — folder-as-context.** One MCP call renders a folder's L2 projections into a
  single briefing (study folder → materials + positions + board state + open links;
  research folder → RESEARCH §3 #9's context artifact). This is the call the plan object
  (Tutor T1) reads before proposing a syllabus — the moment this strategy pays for the
  learning layer.
- **Video** stays where STUDY_PRD S-G put it, arriving with `t:` locators that this
  system treats opaquely from day one.

## 6. Relation to existing plans

- **M9 ingestion v1** (embeddings, reliability, observability — planned, unstarted):
  its embeddings item becomes R3; its reliability items apply to every projector.
- **You-stream ingestion Y0–Y3**: Y-phases' "ingest the user's pages" becomes the page
  projector plus enrichment consumers; no separate page pipeline.
- **Insight engine / claim layer** (branch): a pure consumer of readable content.
  Nothing here requires reviving it; everything here makes it cheaper if revived.
- **Entity layer**: `artifact_entities` tagging becomes an enrichment pass over
  `content_index` rows rather than bespoke per-kind extraction.

## 7. Non-goals

- Not a knowledge-graph revival — readability is deliberately dumber than the KG and is
  what the KG would consume if it returns.
- Not a RAG product — retrieval serves the copilot's grounding; there is no "chat with
  your docs" surface to build.
- No handwriting recognition for ink strokes in any near milestone.
- No second search tool, no search sidecar process, no LLM-in-read-path — ever, per §3.
