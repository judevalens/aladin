# Overnight build log — CTO mode

Autonomous build session while you slept. **Read this first in the morning.**

- **Branch:** `claude/cto-overnight` (off `main` @ `a3a723af`). Separate worktree at
  `.claude/worktrees/cto-overnight`. **Not pushed.** Nothing on `main` or your dev
  data was touched.
- **Operating rules I held myself to:**
  - Only **additive, reversible, well-tested** features — nothing that requires the
    reserved data-model decisions **D1–D6** (those are yours).
  - **No new capture surfaces** (pages/shards are stable, per your pivot).
  - Tested against the **sandbox** stack only (pg :5444); never the dev DB (:8000/:8090).
  - Theme: the stated direction — **insights / ingestion / robustness / DX**.
  - Each feature: plan → implement → test green → commit. Never commit red.

## How to review
Each cycle is one commit on this branch. `git -C .claude/worktrees/cto-overnight log --oneline main..HEAD`.
Run the suite: `cd backend_v2 && TEST_DATABASE_URL=postgres://aladin:password@localhost:5444/aladin go test ./internal/insights/...` (or `make test-go`).
If you like a cycle, it cherry-picks/merges cleanly onto `main`; if not, drop the commit. Each is self-contained.

## Mandate update (you loosened the rules mid-run)
You said: *"you can add new surfaces or work on the data if you can."* So I've widened
scope to include **the data model** and **new surfaces** — but I'm holding these guardrails:
- **Reversible-only.** Data work is **additive** (new tables/columns), never destructive
  migrations or table unification. Anything here can be dropped without data loss.
- **Bridge-first, not unify.** I'll connect the two worlds with a relationships layer
  (the master plan's recommended D1), NOT merge `artifacts`+`records` (the contested call).
- **I will NOT make the founder-level, hard-to-reverse calls:** D1 unify-vs-keep (beyond
  the additive bridge), **D6 multi-tenancy**, **D3 revive-vs-replace graph/vector**. Those
  stay yours; I note where I bumped into them.
- **Assumptions documented.** Every judgment call I make is logged below so you can override.
- **Surfaces:** I can build + test the **backend** of a surface; **frontend I scaffold +
  typecheck only** (no Tauri to run in the loop) and flag it for your verification.

## Backlog
**Data / spine (now in scope):**
1. ✅ **The bridge (Phase 1)** — additive `relationships` edge table linking artifacts ↔
   records ↔ insights. **DONE** end-to-end: data (Cycle 2) + service/API/DI (Cycle 3).
2. ⏳ **Promote-to-workspace** — create an artifact from a record/insight + a `derived-from`
   edge (the first real cross-world action; the compounding loop's "capture").
3. ⏳ **Curation surface (backend)** — endpoints to triage insights/records (accept/dismiss/
   promote) over the bridge; frontend scaffold flagged for review.

**Decision-free, additive (original backlog):**
4. ✅ **Insights: `bridge` finder** — entities connecting ≥2 topics (cross-cutting threads).
5. ⏳ Insights: `contradiction` finder — opposing signals on the same entity.
6. ⏳ Pipeline robustness: terminal-failure record status (stuck records queryable) — PIPELINE_AUDIT Phase B.
7. ⏳ Observability: pipeline status counts (records by status, enrichment lag, insight counts).
8. ⏳ DX: tighten the kit component reference in the MCP instructions (exact prop signatures).
9. ⏳ Tests/hardening for the live insight + pipeline paths.

I'll lead with the **bridge (#1)** now that data work is in scope — it's the spine everything
else hangs on. (Backlog evolves as I learn the code; kept current here.)

## Cycle log

### Cycle 1 — Insights: `bridge` finder ✅
- **What:** added `findBridgeInsights` to the insights `Generator` (registered in the
  `GenerateAndStore` finders list, alongside the existing topic-trend finder). It finds
  entities that appear across **≥2 distinct topics** in the KG's relevant records in the
  last 24h — a cross-cutting "bridge" thread that the single-topic trend finder misses.
  Pure SQL over `records.enrichment` (entities × topics), emits the existing `bridge`
  insight type. No schema change, no LLM, no contested decision.
- **Files:** `backend_v2/internal/insights/generator.go` (+finder, +register, +`strings`).
- **Test:** `backend_v2/internal/insights/generator_test.go` — `TestFindBridgeInsights`
  (the package's first test). Seeds the full FK chain (user→kg→stream→subscription→
  records→matches), asserts the shared entity surfaces as a `bridge` with 2 supporting
  records, and that single-topic entities do **not**. **Passes** against the sandbox DB;
  `go vet` clean.
- **Why it's safe:** purely additive read over existing tables; if you dislike it, drop
  the finder from the slice (one line) — no migration, no data change.

### Cycle 2 — The bridge (Phase 1): `relationships` edge layer — DATA ✅
- **What:** the additive connective tissue between the two worlds, the master plan's
  "bridge-first" path (D1) — explicitly **NOT** table unification. A new `relationships`
  table: typed edges (`cites` / `supports` / `contradicts` / `about` / `derived_from`)
  between polymorphic endpoints `(kind ∈ {artifact,record,insight}, id)`. Idempotent
  upsert on the edge key; bidirectional lookup; per-user scoped (FK `user_id`→users,
  ON DELETE CASCADE).
- **Files:** migration `internal/db/migrations/00006_relationships.sql`; port
  `internal/service/relationships.go` (`Relationship` + `RelationshipStore` + valid
  kinds/types); impl `internal/repo/relationship_postgres.go`; test
  `internal/repo/relationship_postgres_test.go` (upsert idempotency, bidirectional
  lookup, scoping leak check, delete). **Green** against sandbox; build + vet clean.
- **Assumptions / judgment calls (override freely):**
  - **Polymorphic edge, no cross-world FK.** Endpoints are `(kind,id)` text pairs because
    artifact/record ids are text and insight ids are uuid — no single FK spans them.
    Endpoint existence is left for the **service layer** to validate on create (next cycle);
    the table itself only guarantees the owner FK. (Alternative you might prefer: separate
    per-pair tables with real FKs — heavier, less flexible. I chose the flexible additive form.)
  - **Edge vocabulary** (`cites/supports/contradicts/about/derived_from`) is a starter set
    from the master plan, enforced by a CHECK + a Go allow-list. Easy to extend.
  - **No outbox/sync wiring yet** — relationships aren't in the client sync stream. Kept
    minimal/additive; sync integration is a later, separate decision.
  - **Reversible:** `DROP TABLE relationships` undoes it entirely; nothing else changed.
  - Did **not** touch D1-unify, D6 multi-tenancy, or D3 graph/vector — the table is scoped
    by `user_id` (matches today's single-user model) without deciding the tenancy model.
- **Next cycle:** `RelationshipService` (principal → userID, endpoint-existence validation,
  BadRequest on bad kind/type) + `/api/relationships` routes + DI wiring + service/API tests.

### Cycle 3 — The bridge (Phase 1): service + API + DI ✅
- **What:** completed the bridge's application layer on top of Cycle 2's data layer.
  `RelationshipService` (interface + unexported impl, per the clean-layering convention)
  resolves the owner from the request principal (`RequirePrincipal`), validates `kind` ∈
  {artifact,record,insight} and `relType` ∈ the allow-list (→ `BadRequest`), then delegates
  to the store. REST: `POST /api/relationships`, `GET /api/relationships?kind=&id=`,
  `DELETE /api/relationships/{id}` — `BadRequest`→400, else 500. Wired through DI
  (`Dependencies.Relationships()`, both `StaticDependencies` and `wiring`, + constructor).
- **Files:** `internal/service/relationships.go` (+service iface/impl), `internal/api/
  relationships.go` (handlers + `registerRelationshipRoutes`), `internal/api/server.go`
  (route registration), `internal/app/wiring.go` (DI), `internal/service/relationships_test.go`
  (service unit test: validation + principal-scoping + missing-principal). Build + vet clean;
  service + repo suites green against sandbox.
- **Assumptions / judgment calls:**
  - **Endpoint-existence validation deferred.** The service validates kind/type/ids but does
    NOT yet verify the referenced artifact/record/insight actually exists (that needs
    cross-store reads — artifacts + records + insights services). Kept the service dependency-
    light for now; a follow-up can inject an existence-checker. Logged so you can prioritize.
  - **No full HTTP integration test.** Covered the logic via the service unit test + the
    Cycle-2 repo integration test; a full server/auth-middleware HTTP roundtrip test is a
    reasonable follow-up but heavier (needs the auth harness). The handlers are thin pass-throughs.
  - Still additive/reversible; no founder-level decisions touched.
- **The bridge is now usable:** an authenticated client can create/list/delete typed edges
  between any artifact/record/insight. This is the connective tissue the whole "connect the
  two worlds" roadmap (DATA_MODEL.md Phase 1) hangs on.
