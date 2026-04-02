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

## Phase 1 — Schema & Queue Infrastructure ✅

- ✅ DB migration: `sources`, `snapshots`, `sync_jobs` tables
- ✅ DB migration: `artifacts` updated (source_id, snapshot_id, external_id, version, superseded_by, relevance_score, status)
- ✅ DB migration: `parent_id` on artifacts (tree structure)
- ✅ SQLAlchemy models: Source, Snapshot, SyncJob, Artifact (with tree relationships)
- ✅ `SourceSyncer` ABC (plan + execute)
- ✅ `SyncResult` + `SyncJob` dataclasses
- ✅ `SyncRouter`
- ✅ `JobQueue` with builder pattern
- ✅ Worker loop (SKIP LOCKED dequeue → route → handle result → snapshot completion)
- ✅ Scheduler loop (find due sources → plan → enqueue)
- ✅ Stuck snapshot cleanup (>2h in processing → failed + dead-letter jobs)

---

## Phase 2 — Reddit Live Source ✅

- ✅ `RedditSyncer` — plan() + execute()
- ✅ `ensure_feed` job: upserts top-level feed artifact (no HTTP)
- ✅ `fetch_listing` job: GET /new.json, cursor pagination, stop_at_id, min_score gate
- ✅ `fetch_thread` job: re-fetch active threads, diff detection, supersede logic
- ✅ Score-bracket re-fetch scheduling (≥500 → 30min, ≥100 → 1hr, ≥20 → 2hr)
- ✅ Rate limiting: 10 req/min in-process token bucket
- ✅ Artifact tree: feed → posts (parent_id linking)
- ✅ Seed script: `scripts/seed_reddit_source.py`

---

## Phase 2b — Twitter/X Live Source ✅

- ✅ `TwitterSyncer` — user timeline mode + search/hashtag mode
- ✅ `ensure_feed` job: builds feed artifact for both modes
- ✅ `fetch_timeline` job: user timeline with pagination, user_id resolution
- ✅ `fetch_search` job: recent search with auto-retweet/reply filtering
- ✅ Rate limiting: 100s interval (conservative for Basic tier)
- ✅ Metadata: like_count, retweet_count, reply_count mapped to common score field
- ✅ Seed script: `scripts/seed_twitter_source.py --mode user/search`
- 📋 Wire Twitter Bearer Token credentials and test end-to-end

---

## Phase 2c — Dashboard: Feed Drill-Down ✅

- ✅ `GET /api/artifacts/` returns top-level only (parent_id IS NULL) with childCount
- ✅ `GET /api/artifacts/<id>/children` — paginated, sorted by score
- ✅ ArtifactType extended: `feed`, `post`
- ✅ Feed cards: Live badge, post count
- ✅ Feed drill-down view: ranked post list (score, author, flair, comment count)
- ✅ Three-tier navigation: grid → feed → post focus/chat

---

## Phase 3 — Enrichment Pipeline ✅

Full rewrite of the enrichment pipeline in Go (`backend_v2`). Event-driven blackboard
architecture replaces the old Python polling worker. Postgres is written once at completion.

- ✅ Go worker binary (`backend_v2/cmd/worker/main.go`) owns asynq server, pipeline controller, insight worker
- ✅ Blackboard FSM: Redis holds full artifact state + intermediate data for in-flight entries
- ✅ `pipeline:ingest` asynq task — handoff between syncers and blackboard (unbounded buffer)
- ✅ Event-driven controller: `ready` channel + per-stage channels + one goroutine per stage
- ✅ **FirstPassWorker**: GPT-4o-mini → summary, entities, topics, key_claims, low_confidence_entities
- ✅ **SearchWorker**: resolve low-confidence entities via Tavily (Redis-cached, 7-day TTL)
- ✅ **EmbedWorker**: OpenAI text-embedding-3-small → vector stored in blackboard entry
- ✅ **GraphWorker**: Neo4j promoter — MERGE Artifact/Entity/Topic nodes and MENTIONS/TAGGED_WITH edges
- ✅ Single PG write at `StateComplete` — full artifact (content + enrichment + embedding) in one INSERT
- ✅ `ON CONFLICT (source_id, external_id) DO NOTHING` — dedup via unique partial index
- ✅ Typed error handling: `ErrRateLimit` → asynq retry-after, `ErrTransient` → exponential backoff, `ErrPermanent` → drop
- ✅ Crash recovery: on startup, drain Redis blackboard back into ready channel
- ✅ Signal-based scheduler: `Trigger(sourceID)` for immediate sync on source creation
- ✅ Structured JSON logging with trace IDs (artifact_id, kg_id, source_id, stage) throughout
- ✅ Grafana + Loki + Promtail observability stack in docker-compose
- ✅ Migration: `uq_artifacts_source_external` unique partial index on `(source_id, external_id)`

---

## Phase 4 — Note-Taking Improvements 📋

Make note creation a first-class experience — the primary manual input path.

- 📋 **Note composer modal**: proper title + body editor (not just clipboard paste)
- 📋 **Markdown rendering** in focus view content panel
- 📋 **Capture-from-context**: "Take note" button on post/feed focus view — opens composer pre-filled with source reference
- 📋 **Selection → note**: highlight text in focus view → "Quote & annotate" creates note with quoted block + annotation
- 📋 **Note-to-artifact linking**: notes can reference other artifacts (stored in metadata)
- 📋 **Voice → note**: audio recordings run through Whisper transcription → text note
- 📋 Note status: `scratch` (quick dump) vs `note` (curated) — grid can filter by status

---

## Phase 5 — Insight Pipeline (MVP types) 📋

Batch insight generation per snapshot. Immediate for manual input.

- 📋 `InsightSyncer` fully implemented (replaces stub)
- 📋 `reinforcement`: embedding similarity + shared entities → add evidence to existing edge
- 📋 `extension`: similarity + new entities → enrich existing node
- 📋 `bridge`: ANN candidates from two disconnected clusters → new edge proposal
- 📋 `convergence`: N artifacts from different sources pointing to same node in one batch
- 📋 `proposed_changes` per insight with confidence score
- 📋 Apply based on `pipeline_autonomy`: `auto_promote` (≥threshold) or `suggest` (ghost node + insight record)
- 📋 Immediate insight path for `one_shot` / manual sources

---

## Phase 6 — UI: Insight Feed & Graph Neighborhood 📋

Bring the intelligence into the frontend.

- 📋 Insight feed on dashboard: pending insights shown as cards — accept / dismiss actions
- 📋 Node detail view: evidence trail, confidence over time, source breakdown
- 📋 Graph workspace rework: render neighborhood subgraph (10–20 nodes), not full KG
  - "Explore from node" → expand 1/2/3 hops
  - Hop depth slider
  - Highlight path between two selected nodes
- 📋 Timeline scrubber: scrub KG state by snapshot version
- 📋 Source management UI: add / pause / delete sources, view sync health

---

## Phase 7 — Bluesky Live Source 📋

- 📋 `BlueskySyncer` — plan() + execute()
- 📋 `fetch_feed` job: AT Protocol getFeed, opaque cursor
- 📋 `fetch_thread` job: reply detection, NotFound → supersede
- 📋 Engagement-bracket re-fetch (like + repost weighting)
- 📋 Seed script: `scripts/seed_bluesky_source.py`

---

## Phase 8 — Slack Live Source 💭

Push-based source — different architecture from poll.

- 💭 Slack Events API webhook endpoint
- 💭 Message/thread event handler → artifact creation (bypasses queue)
- 💭 Signing secret verification
- 💭 Thread reply fetching on-demand

---

## Phase 9 — Advanced Insight Types 💭

Expensive reasoning pass. Build after MVP insight types are validated.

- 💭 `contradiction`: high similarity + LLM key_claims comparison
- 💭 `obsolescence`: temporal language + LLM reasoning
- 💭 Community detection (Leiden) on KG
- 💭 Community re-summarization on structural changes (GraphRAG-inspired)
- 💭 Global query against community summaries

---

## Phase 10 — Intelligence Feedback Loop 💭

System learns from user behavior.

- 💭 Track accept/dismiss rates per insight type
- 💭 Track dismissed insights later proven correct
- 💭 Adjust confidence weights per user calibration
- 💭 Personalized threshold tuning per source type

---

## Open Design Questions

- **Multi-KG sources**: one source → one KG for MVP. Post-MVP: source feeds multiple KGs with per-KG relevance + autonomy.
- **Auth / multi-user**: currently single-user seeded. Needs proper auth before sharing.
- **Graph namespace isolation**: each KG isolated by `kg_id` filter. No cross-KG queries for now.
- **Embedding model**: OpenAI `text-embedding-3-small`. Consider local model (nomic-embed) for cost at scale.
- **Note-to-KG path**: notes should be first-class KG citizens — how does a "scratch" note become a node? Manual promote? Auto on enrichment?
- **Real-time UI**: frontend currently polls. Plan is WebSocket full-duplex — push artifact/insight signals to UI on state change. Pattern: signal-on-write (like outbox but best-effort publish to Redis pub/sub → WS fan-out).
- **Pipeline scaling**: current design is single-process. To scale: replace in-process `ready` channel with asynq queues per stage → stateless workers. At higher scale: Kafka transport, Flink/Spark for stream processing. Syncer and worker interfaces unchanged.
