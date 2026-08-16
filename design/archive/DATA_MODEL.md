# Aladin Data Model — Master Plan

> **Historical / substrate reference (2026-08-14):** This plan was written for the
> graph-first knowledge-workspace framing. Some entity/relationship ideas remain
> useful substrate, but the active product direction is the trading research
> workspace in [`../../CURRENT_PRODUCT.md`](../../CURRENT_PRODUCT.md). Do not treat this
> as the product roadmap.

> **Status:** living plan, not a locked spec (2026-06-14). The goal is a data model
> that *makes thinking easier* — the foundation the rest of Aladin compounds on.
> Grounded in an audit of the current schema + pipeline (see "Current reality").
> Companion docs: `../SHARD_MODEL.md` (the shard projection model), `../../backend_v2/PIPELINE.md`
> (ingestion, authoritative), `../../docs/archive/PIPELINE_AUDIT.md` (dead-code remediation).

---

## 0. The thesis

**What makes Aladin good is not the capture surfaces (pages, shards) — those are
table stakes. It's a compounding knowledge base: ingest → insight → curate, on a
data model where what you *discover* and what you *author* live in one connected
space.** "Makes thinking easier" = the model does the connecting a human otherwise
does in their head: this source supports that thesis; these three notes are about
the same thing; this feed item contradicts what you wrote last week.

The single biggest obstacle today: **Aladin is two data models in one DB that never
touch.** Fixing that is the spine of this plan.

---

## 1. Current reality (the honest baseline)

Two worlds, same Postgres, **no link between them**:

- **Workspace world (authored):** `artifacts` (pages/notes/shards) → `tree_nodes`
  (browser hierarchy) → `page_documents`/`page_ydoc` (content/CRDT). User-created,
  scoped by `user_id`. No automation.
- **Ingestion world (discovered):** `provider_streams` → `records` (captured from
  HN/Reddit/Bluesky) → LLM enrichment (`summary, entities, topics, key_claims`) →
  `tenant_item_matches` (keyword relevance per `source_subscriptions`) → `insights`
  (pure-SQL topic-trend). Automated, scoped via `knowledge_graphs`.

**The gaps the audit confirmed:**
1. **No bridge.** Zero FKs / shared ids between `artifacts` and `records`/`insights`.
   You cannot promote a record into a page, embed an insight, or link a note to a
   source. Two apps in one database.
2. **Insights are shallow.** One hardcoded SQL trend (3+ topic mentions / 24h). No
   LLM, no hypotheses, no querying. The arXiv 2503.11664 loop is the upgrade path.
3. **Dead lenses.** The `search`/`embed`/`graph` workers are wired but never
   enqueued → **embeddings and Neo4j are unpopulated**; `/api/graph` is a stub.
4. **No curation layer.** Only insight accept/dismiss (`user_status`). No promote,
   merge, or trust.
5. **Single-user.** Everything scoped by `user_id`; no teams/sharing.

`knowledge_graphs` is the one almost-bridge — it scopes subscriptions + insights and
means "what you're learning about" — but it has no link to artifacts.

---

## 2. Principles (the tenets the model must honor)

1. **One logical entity layer.** Every unit of knowledge — authored or ingested —
   is an *entity*: stable id, owner, lifecycle; one canonical write point that
   events through the outbox (per `SHARD_MODEL.md` §1–2). Storage-agnostic; Postgres
   is where it materializes, graph/vector are *projections*.
2. **Connect discovered ↔ authored.** The model's job is the join a human does in
   their head. An authored page can cite a record; a record can support a thesis you
   wrote; an insight can reference both. This is the missing bridge, and it's #1.
3. **Multi-lens access, relational substrate.** Read via structure (SQL), connection
   (graph), similarity (vector) → retrieve relationally (per `SHARD_MODEL.md` §1).
   Graph/vector are discovery indexes over the entity layer, not the substrate.
4. **Provenance + trust tiers.** Every entity carries lineage (where it came from)
   and a trust tier (verified vs. agent-believed). Compounding *amplifies* whatever
   you let in — unverified ingestion without tiers rots the base faster the better
   the loop works.
5. **Curation is first-class.** Promote / merge / dismiss / trust are core
   operations, not an afterthought — the act that turns raw discovery into durable,
   trusted knowledge.
6. **Flows reveal the model; commit incrementally.** Don't design the schema in a
   vacuum. Work ingestion → insights → curation as forcing functions; let the shapes
   they demand define the model; commit each piece before building durable consumers
   (like shard data-wiring, which stays paused until then).

---

## 3. Target model (the sketch to converge on)

A single **entity spine** the two worlds attach to, rather than two hierarchies:

- **Entity** = the common contract (id, kind, owner, lifecycle, provenance, trust).
  Authored artifacts and ingested records are both *kinds* of entity (or both
  project onto one entity spine — see decision D1).
- **Relationships** = the connective tissue: `cites`, `supports`/`contradicts`,
  `about` (entity↔topic), `derived-from`, `mentions`. This is where "thinking
  easier" lives — and what the graph lens indexes.
- **`knowledge_graphs` becomes the real organizing context** — a workspace/topic
  that spans *both* authored and ingested entities, not just subscriptions+insights.
- **Curation promotes** a discovered record into durable knowledge (and dedups /
  merges), moving it up the trust tiers.
- **Insights reference the connected set** (records *and* authored entities), and are
  generated by the text-to-query loop, not a fixed SQL trend.
- **Projections**: graph (relationships/traversal) and vector (similarity) are
  rebuilt from the entity layer — the revived-or-replaced `embed`/`graph` branches.

---

## 4. The decisions that need to be made (these are yours)

| # | Decision | Recommendation |
|---|---|---|
| **D1** | **Unify** artifacts + records into one entity table, or keep them separate with a **relationship/bridge** layer? | **Bridge first** (lower risk): keep the tables, add a relationships table + cross-references; unify later only if the bridge proves the seams. |
| **D2** | What is the canonical **entity spine** — the id space + contract everything joins on? | Define a thin entity registry (id, kind, owner, provenance, trust) that both artifacts and records register into. |
| **D3** | **Graph + vector:** revive the dead `embed`/`graph` branches, or replace them (rebuild as projections of the entity layer)? | Defer until the bridge + insights exist; then **rebuild as projections** (Phase 4), per `../../docs/archive/PIPELINE_AUDIT.md` Phase C — don't resurrect dead scaffold blindly. |
| **D4** | **Provenance + trust** representation (per-entity lineage + tier). | Add provenance + trust columns to the entity spine from the start (cheap; expensive to retrofit). |
| **D5** | **Curation** operations + their state model (promote/merge/dismiss/trust). | Design alongside the insights upgrade — curation is what makes generated insight trustworthy. |
| **D6** | **Multi-tenancy:** stay single-user, or plan the model for teams/sharing now? | Decide *now* even if you don't build it — sharing boundaries are very expensive to retrofit. |

---

## 5. Phased roadmap (flow-driven)

Ordered so each phase exercises the model and de-risks the next. Decisions precede
durable consumers.

- **Phase 0 — Decisions.** Resolve D1–D6 (at least D1, D2, D4, D6). This plan + those
  answers are the gate.
- **Phase 1 — The bridge (highest leverage).** Connect the two worlds: a relationship
  layer + cross-references so authored entities can cite records/insights and vice
  versa; "promote a record into the workspace." This alone makes Aladin *one* thing
  and immediately "makes thinking easier."
- **Phase 2 — Insights upgrade.** Replace the SQL-trend stage with the text-to-query
  loop (hypothesis → query → summarize → verify, arXiv 2503.11664), over the now-
  connected model. `query_sql` (read-only, curated views) is the shared primitive.
- **Phase 3 — Curation layer.** Promote / merge / dismiss / trust as first-class ops;
  the human-in-the-loop that turns discovery + generated insight into trusted base.
- **Phase 4 — Lenses as projections.** Rebuild graph (connection) + vector
  (similarity) as projections of the entity layer (replacing the dead branches),
  feeding discovery for insights + retrieval.
- **Phase 5 — Consumers, natively.** With the model settled, *unpause* the shard
  data-wiring (bridge/refs/query) — shards consume the model instead of being bolted
  on; the compounding loop (`SHARD_MODEL.md` §2 "Compounding") closes.

---

## 6. Sequencing principle

**Decide the spine (Phase 0) → connect the worlds (Phase 1) → let insights +
curation push the model (Phases 2–3) → rebuild the lenses (Phase 4) → let consumers
bind natively (Phase 5).** The model is not a separate up-front project; it
crystallizes out of doing ingestion → insight → curation honestly, with the bridge
as the first and most valuable move.

---

## 7. Pointers
- Current-state audit: this plan's §1 (full table catalog available on request).
- `../../backend_v2/PIPELINE.md` — ingestion flow (authoritative); `../../docs/archive/PIPELINE_AUDIT.md` —
  dead-code remediation (graph/embed/search); `GLOBAL_SOURCE_ITEM_PIPELINE.md` —
  ingestion data model + correctness.
- `SHARD_MODEL.md` — the projection/shard model + the compounding loop that this data
  model ultimately feeds.
