# Aladin — Roadmap

## Status legend
- ✅ Done
- 🔨 In progress
- 📋 Planned
- 💭 Future / post-MVP

---

## Phase 0 — Foundation ✅

Core frontend workspace and basic artifact pipeline.

- ✅ React Flow graph canvas (nodes, edges, force layout)
- ✅ Artifact collection panel
- ✅ Dashboard (artifact grid, focus view, chat)
- ✅ Basic Flask backend with Postgres + Neo4j
- ✅ Artifact enrichment (LLM: summary, entities, topics, key_claims)
- ✅ Bluesky single-post ingestion
- ✅ Data model design (Source, Snapshot, Artifact, Node, Edge, Insight, KG)
- ✅ Sync queue design (job schema, builder pattern, syncer interface)

---

## Phase 1 — Schema & Queue Infrastructure 🔨

Lay the foundation everything else depends on.

- 📋 DB migration: `sources` table
- 📋 DB migration: `snapshots` table (with `expected_jobs` / `completed_jobs`)
- 📋 DB migration: `sync_jobs` table + dequeue index
- 📋 DB migration: update `artifacts` (add `source_id`, `snapshot_id`, `external_id`, `version`, `superseded_by`, `relevance_score`, `status`)
- 📋 SQLAlchemy models for Source, Snapshot, SyncJob
- 📋 `SourceSyncer` ABC
- 📋 `SyncResult` + `SyncJob` dataclasses
- 📋 `SyncRouter`
- 📋 `JobQueue` with builder pattern (`JobQueue.builder().add(syncer).build()`)
- 📋 Worker loop (dequeue → route → handle result → snapshot completion check)
- 📋 Scheduler loop (find due sources → plan → enqueue)

---

## Phase 2 — Reddit Live Source 📋

First live source end-to-end.

- 📋 `RedditSyncer` — `plan()` + `execute()`
- 📋 `fetch_listing` job: GET /new.json, cursor management, stop_at_id, min_score gate
- 📋 `fetch_thread` job: re-fetch active threads, diff detection, supersede logic
- 📋 Score-bracket re-fetch scheduling
- 📋 Source config UI (subreddit, min_score, include_comments)
- 📋 Wire into job queue: `JobQueue.builder().add(RedditSyncer()).build()`

---

## Phase 3 — Embedding & Enrichment Pipeline 📋

Wire the existing enrichment into the new artifact model.

- 📋 Embedding worker: picks up `status='pending'` artifacts, generates vector, sets `status='embedded'`
- 📋 Enrichment worker: picks up `status='embedded'`, runs LLM enrichment, sets `status='enriched'`
- 📋 Cold start rule: skip relevance filtering when KG has < 10 nodes
- 📋 Relevance scoring against existing KG artifact embeddings (ANN search)
- 📋 Threshold filtering: below `suggest_threshold` → `dismissed`
- 📋 Snapshot completion detection → enqueue `insight_batch` job

---

## Phase 4 — Insight Pipeline (MVP types) 📋

Batch insight generation for live sources. Immediate for manual input.

- 📋 `InsightPipeline` class: takes snapshot artifacts + KG state
- 📋 `reinforcement` detection: embedding similarity + shared entities → add evidence to edge
- 📋 `extension` detection: similarity + new entities not in existing node → enrich node
- 📋 `bridge` detection: ANN candidates from two disconnected clusters → new edge proposal
- 📋 `convergence` detection: N artifacts from different sources → same node in one batch
- 📋 `proposed_changes` generation per insight type
- 📋 Apply based on `pipeline_autonomy`: auto-promote or ghost + insight record
- 📋 Immediate insight path for `one_shot` sources

---

## Phase 5 — Bluesky Live Source 📋

Second live source. Reuses queue infrastructure from Phase 1-2.

- 📋 `BlueskySyncer` — `plan()` + `execute()`
- 📋 `fetch_feed` job: getFeed polling, opaque cursor
- 📋 `fetch_thread` job: reply detection, deletion handling (NotFound → supersede)
- 📋 Engagement-bracket re-fetch scheduling (like + repost weighting)
- 📋 Source config UI (feed URIs, actor DIDs)

---

## Phase 6 — UI: Insight Feed & Node Neighborhood 📋

Bring the intelligence into the frontend.

- 📋 Insight feed on dashboard: pending insights, accept / dismiss actions
- 📋 Node detail view: evidence trail, confidence over time, connected insights
- 📋 Graph workspace rework: render neighborhood subgraph (10-20 nodes) not full KG
  - "Explore from this node" replaces "view entire graph"
  - Hop depth control (1, 2, 3 hops)
- 📋 Timeline scrubber: scrub KG state by snapshot version
- 📋 Source management UI: add/pause/delete sources, view sync state

---

## Phase 7 — Slack Live Source 💭

Push-based source. Different architecture from poll sources.

- 💭 Slack Events API webhook endpoint
- 💭 Message event handler → artifact creation (bypasses queue)
- 💭 Thread reply fetching on-demand
- 💭 Signing secret verification

---

## Phase 8 — Advanced Insight Types 💭

The expensive reasoning pass. Build after MVP insight types are validated.

- 💭 `contradiction` detection: high similarity + LLM key_claims comparison
- 💭 `obsolescence` detection: temporal language + LLM reasoning
- 💭 Community detection (Leiden algorithm) on KG
- 💭 Community re-summarization on structural graph changes (GraphRAG-inspired)
- 💭 Global query support against community summaries

---

## Phase 9 — Intelligence Feedback Loop 💭

System learns from user behavior.

- 💭 Track accept/dismiss rates per insight type per user
- 💭 Track which dismissed insights were later proven correct (edge invalidated anyway)
- 💭 Adjust insight confidence weights based on user calibration
- 💭 Personalized threshold tuning per source type

---

## Open Design Questions

- **Multi-KG sources**: source feeds multiple KGs — relevance score per KG, autonomy per KG. Deferred to post-MVP (current: one source → one KG).
- **Auth / multi-user**: currently single-user seeded. Needs proper auth before any sharing features.
- **Graph namespace isolation**: each KG is isolated in Neo4j by `kg_id` filter. No cross-KG queries for now.
- **Embedding model**: currently OpenAI `text-embedding-3-small`. Consider local model (nomic-embed) for cost at scale.
