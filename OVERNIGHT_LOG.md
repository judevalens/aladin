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
1. ⏳ **The bridge (Phase 1)** — an additive `relationships` table (typed edges:
   cites / supports / contradicts / about / derived-from) linking artifacts ↔ records ↔
   insights, + repo/service/API + tests. The single highest-leverage move; reversible.
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
