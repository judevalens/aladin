# Aladin Engineering Roadmap

Strategic anchor: [Aladin Product Vision](../docs/ALADIN_PRODUCT_VISION.md).

## Status legend
- ✅ Done
- 🔨 In progress
- 📋 Planned
- 💭 Future / post-MVP

---

## Current Direction

Aladin is now centered on a Kotlin/Wasm workspace shell with specialist JS surfaces mounted where browser tooling is strongest. Product logic, sync state, repositories, and orchestration stay in Kotlin and Go. React/TypeScript surfaces should remain thin rendering/input adapters.

The current foundation is shifting from a monolithic app surface toward composable workspace subsystems:

- app-level orchestration and shared workspace state
- browser/folder tree navigation
- artifact/work pane with tabs and context rail
- page-specific editor/sync flow
- modular backend artifact slices

This roadmap replaces the older pipeline-first phase ordering with the current architecture-first path.

---

## Phase 1 — Workspace Foundation ✅

Build the main desktop-style workspace shell and split the former god app surface into composed feature slices.

- ✅ Kotlin/Wasm + Compose Multiplatform shell
- ✅ Circuit presenter entrypoint with producer-based decomposition
- ✅ Three-pane workspace direction: app rail, browser pane, artifact/work pane
- ✅ Sidebar producer and UI split
- ✅ Browser producer and UI split
- ✅ Artifact/work pane producer and UI split
- ✅ Page state producer separated from pane orchestration
- ✅ Interface implementation naming standardized with `Impl` suffix
- ✅ Feature package rule established: app subfeatures coordinate through app-root contracts, not sibling imports

### Remaining

- 📋 Continue shrinking `AppPresenter` toward orchestration only
- 📋 Keep pane-specific UI logic inside browser/artifact/page producers
- 📋 Add dependency composition/DI once constructor wiring becomes noisy enough to justify it

---

## Phase 2 — Modular Artifacts ✅

Make artifacts a lightweight shared envelope and move type-specific content/behavior into domain slices.

- ✅ Canonical writable document type is `page`
- ✅ `artifacts` remains the shared metadata/envelope table
- ✅ page markdown moved to `page_documents`
- ✅ page load/save endpoints added under `/api/pages/{id}`
- ✅ file upload/resource endpoint added under `/api/files`
- ✅ backend files are addressable through durable resource URLs
- ✅ browser tree moved toward tree-node driven metadata
- ✅ artifact pane can render by artifact type

### Remaining

- 📋 Keep link, voice, file, and future artifact-specific content out of the artifact envelope
- 📋 Add type-specific viewers/producers as artifact types mature
- 📋 Clean up transitional `artifacts.content` usage after page path is stable
- 📋 Make folder/artifact metadata updates flow through repositories instead of ad hoc refreshes

---

## Phase 3 — Trustworthy Page Editing 🔨

Make markdown pages feel safe: load once, do not reset while typing, autosave predictably, and surface status without extra chrome.

- ✅ Milkdown/Crepe editor mounted through the web widget bridge
- ✅ document updates emitted as structured bridge events
- ✅ Kotlin page syncer owns load/save metadata
- ✅ editor owns live draft text after initial mount
- ✅ page save/load API wired through Kotlin repository
- ✅ page save status moved into the artifact context rail
- ✅ edit/view lock control added
- ✅ backend page revision guard rejects stale saves with `409 Conflict`
- ✅ client sends monotonically increasing page revisions loaded from the backend
- ✅ image/file upload flow routes through Kotlin/backend, not direct React persistence

### Remaining

- 🔨 Stabilize simplified `PageDocumentSyncerImpl` state shape and producer mapping
- 🔨 Restore or refine upload status/retry behavior after syncer simplification
- 🔨 Ensure edit lock consistently disables editor input
- 📋 Add tests around out-of-order save rejection and client conflict handling
- 📋 Consider replacing Crepe if toolbar/drag-handle positioning becomes a product blocker

---

## Phase 4 — Browser and Workspace Metadata 🔨

Make browser/tree metadata reactive and coherent across panes without revision stitches or manual recomposition triggers.

- ✅ inline rename for folders/artifacts
- ✅ rename can start from double-click or context menu
- ✅ app-level focus clear supports outside-click commit
- ✅ browser tree refreshes after local rename

### Remaining

- 🔨 Replace stale tab/title refresh hacks with repository flows
- 🔨 Move folder and artifact metadata into dedicated repositories with in-memory cache
- 📋 Compute breadcrumbs from canonical tree/browser metadata
- 📋 Keep open artifact/tab metadata coherent after rename, move, delete, and realtime updates
- 📋 Decide whether browser folder scope is derived from tree state or a separate navigation model

---

## Phase 5 — Reactive Client Repositories 📋

Use repositories as the client-side cache/store boundary. Producers observe flows and submit intents; repositories own data access, local cache, and update emission.

- 📋 Split folder and artifact repositories instead of one broad workspace store
- 📋 Keep storage behind interfaces so in-memory storage can later become IndexedDB/SQLite/WASM-backed storage
- 📋 Emit repository flows from actual data updates, not directly from transport events
- 📋 Use simple hand-coded projections per repo instead of building a generic local Firebase framework
- 📋 Establish consistent patterns for single resource, list, breadcrumb, and browser-tree flows

### Design Defaults

- Repositories own local cache and network/API calls.
- Producers should not build resource keys.
- Realtime events are signals to update repository data, not UI state by themselves.
- Lists should preserve requested order and update by id.

---

## Phase 6 — Realtime + Offline Readiness 📋

Add realtime transport and client event processing as the foundation for live metadata updates, offline caching, and future collaboration.

- ✅ initial backend realtime service shape explored with in-memory broker
- 📋 WebSocket connection on client boot
- 📋 subscription messages for workspace metadata streams
- 📋 client event processor with event-id dedupe
- 📋 repository listeners that decide whether a realtime event affects their cached data
- 📋 local storage abstraction for browser persistence
- 📋 offline save queue for page/document mutations
- 📋 cache invalidation policy for uploaded resources and metadata

### Long-Term Transport Direction

- In-memory broker first for local development
- Redis/asynq-backed broker later if distributed fanout is needed
- Subscription keys remain a backend/service detail, not producer/UI logic

---

## Phase 7 — Signals and Source Intelligence 📋

Return to source ingestion and signal generation after the workspace can reliably hold, edit, and react to artifacts.

- 📋 Trending Research Delta v1: one watched subreddit, entity overlap relevance, gap/connection synthesis, in-app inbox
- 📋 reframe source ingestion around `signals`, not raw feed browsing
- 📋 entity extraction at ingestion time for source artifacts and user-authored artifacts
- 📋 trend detection with per-source tuned thresholds before rolling baselines
- 📋 relevance matching through graph/entity overlap against the user's existing artifacts
- 📋 gap and connection analysis as structured LLM comparisons over deterministic graph query results
- 📋 in-app inbox notification surface before push or email delivery
- 📋 connect source artifacts to sections/folders through relevance
- 📋 Daily Brief on Home: what changed, where it matters, suggested next actions
- 📋 Signals area: curated signal cards with evidence and linked artifacts
- 📋 Sources area: operational health, configuration, recent activity
- 📋 Reddit/Bluesky/news/ticker source integrations as reusable live input patterns

### North-Star Loop

The reference workflow for this phase is [Trending Post Gap Analysis](./TRENDING_POST_GAP_ANALYSIS.md): ambient source capture, silent graph/entity structure, proactive synthesis, and a consume surface that explains why the signal was sent.

### Existing Foundation To Reuse

- Go backend pipeline work
- stream-native provider ingestion design
- global source item and enrichment model
- tenant matching model for personalized relevance
- enrichment and graph worker concepts
- queue and scheduler implementation backed by provider streams

---

## Phase 8 — Graph / Context Layer 📋

Make the graph useful as context infrastructure, not as a default visualization-first experience.

- 📋 workspace-wide entity/topic/context graph
- 📋 evidence trails from signals back to artifacts
- 📋 graph-derived related context inside section/artifact workspaces
- 📋 graph explorer as a secondary analysis surface
- 📋 retrieval hooks for future LLM workflows
- 📋 provenance-aware updates when artifacts/signals change

---

## Phase 9 — Packaging + Distribution 💭

Keep web iteration fast, but preserve the path to a desktop app.

- 💭 continue web-first development for iteration speed
- 💭 evaluate Tauri wrapping for native desktop distribution
- 💭 keep Kotlin/Wasm shell and JS specialist surfaces as the app architecture
- 💭 avoid moving business logic into React/Electron/Tauri layers

---

## Phase 10 — Collaboration / CRDT Track 💭

The current markdown + revision model is a stepping stone. Later collaboration should evolve the page path without forcing a UI rewrite.

- 💭 document operation log
- 💭 CRDT-backed page model
- 💭 snapshot compaction
- 💭 migration bridge from markdown snapshots to collaborative document state
- 💭 remote cursors/presence if multi-user editing becomes important
- 💭 conflict-aware offline editing

### Current Bridge

- Page load returns content + revision.
- Editor owns live draft text.
- Kotlin syncer owns save/load metadata.
- Backend rejects stale saves.
- Future CRDT work can replace content/revision with snapshot/ops/clock while preserving the same high-level boundaries.

---

## Open Design Questions

- **Sections vs folders:** product language favors sections, current implementation uses folders/tree nodes. Decide whether sections are a folder kind, artifact kind, or higher-level workspace object.
- **Repository storage:** in-memory first; choose IndexedDB, SQLite/WASM, or another browser-local store when offline work begins.
- **Realtime granularity:** decide how broad workspace subscriptions should be before scaling beyond local development.
- **Page editor ownership:** keep Crepe if issues remain tolerable; rebuild on Milkdown primitives if toolbar/drag-handle control becomes worth the cost.
- **Artifact metadata flow:** decide whether rename/move/delete update open pane state through repository flows, realtime events, or both.
- **CRDT timing:** do not start until single-user autosave, offline queue semantics, and repository boundaries are stable.
