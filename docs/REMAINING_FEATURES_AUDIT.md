# Remaining PRD Features — Backend/Frontend Audit & Plan

Audit of what each not-yet-built PRD surface needs, grounded in `backend_v2` + the React
app on the `redesign/ide-foundation` branch. Effort tiers: **S** ≤1d · **M** a few days ·
**L** 1–2 wk · **XL** 3 wk+. "Verified" = confirmed in code (file paths given);
"Proposed" = a suggested delta, not yet designed/locked.

**Verification for all backend work:** use the sandbox stack — `make test-db-up` +
`make test-go` (TEST_DATABASE_URL → :5444). Never the dev DB.

## Summary

| Feature | Backend | Frontend | Effort | Suggested order |
|---|---|---|---|---|
| **Delete** (folder/artifact) | ✅ done (`DELETE /api/browser/nodes/{id}`) | wire service→hook→ctx-menu | **S** | 1 |
| **Tracked-topic counts** | small (`Topics()` → counts) | Home/Signals filter chips | **S–M** | 2 |
| **Signals surface** | M (rationale + ranking on top of existing feed) | full Signals view | **M–L** | 3 |
| **Tree signal/unread chips** | M (tree_nodes cols + denorm on ingest) | render chips (sync auto-flows) | **M** | 4 |
| **Home dashboard** | L (brief LLM job + catalysts) | dashboard view | **L** | 5 |
| **Graph read view** | L (Neo4j **not populated** — needs write stage + read API) | node-link view | **L** | 6 |
| **KG: entities/theses/claims + Ask-my-graph** | XL (greenfield model + extraction + LLM) | graph-on-demand panel, ⌘K ask | **XL** | 7 |

---

## 1. Delete — backend done, frontend-only  (S)
**Verified:** `DELETE /api/browser/nodes/{id}` → `Artifacts().DeleteBrowserNode`; also
`DeleteArtifact`; Tauri `db_delete_browser_node` exists. **Gap:** the FE
`services.workspace`/repo doesn't expose delete and the browser context menu has no
Delete item.
**Do:** add `deleteFolder`/`deleteArtifact` to the workspace service + repo (mirror
rename), a `useBrowserPane` handler, and a **Delete (danger)** context-menu item per
`BROWSER_SPEC §5`. Optimistic local removal flows back through sync.

## 2. Tracked-topic counts  (S–M)
**Verified:** `GET /api/feed/topics` → `Feed.Topics()` returns **distinct topic strings
only**; `feed_postgres.go` already filters by topic/source and computes a heuristic
`signalScore` at query time. **Gap:** no counts. **Do:** change `Topics()` to
`GROUP BY topic` with `COUNT(*)` (+ optional unread count); return `{topic,count}[]`.
Feeds the Signals filter sidebar and Home "Tracking" widget.

## 3. Signals surface  (M–L)  — closer than it looked
**Verified that already exists:** records pipeline (provider streams → `records` →
LLM enrich `summary/entities/topics/key_claims` → **tenant_match** which writes
`tenant_item_matches.relevance_status/relevance_score/relevance_reason`) → search → embed
→ graph → persist. Feed API: list/topics/sources/**save/dismiss/unsave** all present;
`user_status` persisted.
**Key insight:** `tenant_item_matches.relevance_reason` **already exists** — today it's a
policy-match string, but it's the natural home for the PRD's **"why this matters"** card
line. We can surface it now and upgrade its quality later.
**Gap → do (incremental):**
- **v1 (M):** extend `FeedItem` with `rationale` (from `relevance_reason`) + expose the
  existing `signalScore`; build the **Signals view** (ranked cards + "why this matters" +
  topic/source filter sidebar + save/dismiss) against the existing API. Wire the
  placeholder route. Mostly frontend.
- **v1.1 (Proposed, M):** a `signal_rationale` pipeline worker (LLM, per matched
  record×subscription) writing a real rationale + a persisted `curation_score`/label to
  `tenant_item_matches` (new columns). Replaces the heuristic ranking.

## 4. Tree signal/unread chips  (M)
**Verified:** `tree_nodes` columns = `id/parent_id/kind/title/artifact_id/position/seq/
is_deleted/…` — **no counts**. The sync engine sends the whole `tree_nodes` row via
snapshot/outbox frames under a `seq` guard, so **new columns auto-propagate to the Tauri
client** (no sync-engine changes).
**Gap → do (Proposed):** add `tree_nodes.signal_count`/`unread`/`last_activity_at`;
denormalize on record ingest (bump the owning artifact's node + ancestors); include in
`ListAllBrowserNodes` + `BrowserTreeNode` JSON; render the amber chip/dot (data model +
`BrowserTreeRow` already have a slot per `BROWSER_SPEC §3`).

## 5. Home dashboard  (L)
**Verified:** **no** brief/catalyst/tracking backend. Feed `recent` + insights exist.
**Gap → do (Proposed):**
- **Brief:** `home_brief` table + a worker that LLM-summarizes the last-24h top records;
  `GET /api/home/brief`. (Brief-history modal reads prior rows.)
- **Up Next / catalysts:** needs a dated-event source — add `records.event_date` (+
  enrichment to populate it) and `GET /api/home/upcoming`, **or** a manual catalysts
  table. Decide the source before building.
- **Tracking:** reuse #2 (topic counts).
- Frontend: the Home dashboard view (greeting, brief card, feed cards, Up Next/Tracking
  rails) — currently `/home` renders the folder view.

## 6. Graph read view  (L)  — scaffolded but NOT populated
**Corrected (see `PIPELINE_AUDIT.md`):** the Neo4j promoter (`internal/graph/promoter.go`,
`:Record/:Entity/:Topic` + `MENTIONS`/`TAGGED_WITH`) lives in the pipeline **`graph`
worker — which is in the DEAD branch and never runs.** So **Neo4j is currently empty** and
`GET /api/graph` / `/api/graph-explore/full` are stubs (`handleEmptyGraph`).
**Gap → do:** (a) **write side** — fold a graph stage into the *live* flow after
`global_first_pass` so entities/mentions actually get written (or revive the worker
deliberately); (b) **read side** — `GraphService` + `graph_neo4j` Cypher read + real
`/api/graph`; (c) frontend node-link view. Bigger than first stated because the write side
isn't running.

## 7. Knowledge graph (entities/theses/claims) + Ask-my-graph  (XL) — greenfield
**Verified:** **no** Thesis/Claim/stance/conviction anywhere; `key_claims` are extracted
as strings into enrichment JSONB but not modeled or linked. Insights engine exists
(trend/bridge/etc. from Postgres JSONB) but is not the PRD model.
**Gap → do (Proposed, phased):** Postgres `entities`/`theses`/`thesis_entities`/`claims`/
`claim_entities`; pipeline extraction to populate them (LLM stance/claim structuring);
read/write services + API; then **Ask-my-graph** (intent classify → subgraph fetch →
grounded LLM answer with cited nodes) behind ⌘K. This is the PRD's moat and the biggest
lift — plan it as its own multi-phase effort, ideally after the graph **read** view (#6)
proves the visualization + the entity layer.

---

## Cross-cutting notes
- **Sync is free for `tree_nodes` fields:** any column added to `tree_nodes` flows to the
  client via the existing snapshot/outbox frames — no engine changes (verified in
  `internal/service/sync.go` + Tauri `src-tauri/src/sync`).
- **LLM:** an enrichment LLM path already exists (`internal/llm`); new LLM steps (signal
  rationale, brief, ask-graph) should reuse it and keep deterministic fallbacks.
- **Shared types:** each surface needs new `aladin_react/src/shared/api` types +
  a service/repo; follow the existing `feed`/`artifacts` patterns.
- **Recommended path:** ship the **S/M frontend-leaning** wins first (Delete → topic
  counts → Signals v1 → tree chips), which deliver visible product on mostly-existing
  backend, before committing to **Home** and the **XL** knowledge-graph track.

## Open decisions (need product input)
1. **Catalysts source** for "Up Next" — auto-extract `event_date` from records, or a
   manual catalysts/calendar entry? (Blocks Home.)
2. **Signals rationale** — ship v1 surfacing the existing `relevance_reason`, or wait for
   the LLM `signal_rationale` worker?
3. **KG scope/sequencing** — graph **read** view first (quick, over existing Neo4j), or
   go straight at the entities/theses/claims model (the moat, but XL)?
