# Ingestion Pipeline — Audit & Streamlining Plan

> **Historical / stale audit (2026-08-14):** This audit predates the current live
> pipeline wiring. The worker now routes enrichment into tenant matching and entity
> resolution, then embedding, with optional low-confidence search and optional Neo4j
> projection. Keep this file as history for why the old dead branch was cleaned up;
> do not use its "dead branch" claims as current truth.

Audit of `backend_v2/internal/pipeline` (+ `internal/sync` ingestion). Goal: **simplicity
(kill dead code), extensibility (clean path for the PRD's new stages), reliability**.
Grounded in code; file paths given. Verify any change against the sandbox: `make test-go`.

## TL;DR
The orchestration design is **good** (generic worker registry + a single `ResultHandler`
that owns routing). The problem is **a whole second pipeline branch that never runs**:
`first_pass → search → embed → graph → SaveComplete` is fully wired but **`TaskFirstPass`
is never enqueued**. Pruning it removes ~4 workers + their routing and resolves most of
the "too many stages to reason about" pain. Two consequences also fall out:
**Neo4j and embeddings are not being populated** (their workers are in the dead branch).

---

## The live flow (what ingestion actually runs)
Entry: `internal/sync/result_handler.go` → `EnqueueGlobalFirstPass` (the **only** entry
enqueued).

```
syncer record → global_first_pass        (LLM enrich: summary/entities/topics/key_claims)
              → [handler] SaveEnrichment  (records.enrichment, staleness-guarded by source_revision)
              → tenant_match              (per active subscription: RecordRelevanceDecider →
                                           writes tenant_item_matches{status,score,reason};
                                           collects InsightTriggers per KG)
              → [handler] enqueueInsightTriggers → insights generator (separate handler)
```
- **Relevance is pluggable** — `tenant_match` holds a `map[key]RecordRelevanceDecider`;
  default `PolicyRelevanceDecider` (`workers/relevance.go`, **live**) writes
  `relevance_reason` (today a policy string — the natural seed for Signals "why this matters").
- Live task types: `global_first_pass`, `tenant_match`. Live results:
  `global_first_pass.done`, `tenant_match.done`.

## The dead branch (wired, never triggered)
`TaskFirstPass` is never enqueued anywhere (verified across `internal/` + `cmd/`), so
nothing downstream of it runs:

| Dead artifact | File / location |
|---|---|
| `first_pass` worker | `workers/first_pass.go` |
| `search` worker | `workers/search.go` |
| `embed` worker | `workers/embed.go` |
| `graph` worker (Neo4j `Entity`/`MENTIONS` promotion) | `workers/graph.go` |
| Handler routing for 5 result types | `handler.go` cases `ResultFirstPassSearchNeeded`/`EmbedReady`/`SearchDone`/`EmbedDone`/`GraphDone` |
| `persist()` + `SaveComplete` path | `handler.go` (the live flow persists via `SaveEnrichment`, not `SaveComplete`) |
| `insights chan<- string` | constructed + written **only** in dead `persist()`; live path uses `insightEnqueuer` |
| `RecordPayload.SearchResolved` / `.Embedding` fields | `payload.go` (only the dead flow sets them) |
| Queue configs `search:5 / embed:3 / graph:5` | `cmd/worker/main.go` |
| `orch.Add(FirstPass/Search/Embed/Graph)` | `cmd/worker/main.go` |

**Consequences:** Neo4j graph is **not populated**, and records are **not embedded**, by
the live pipeline. (Corrects the Graph claim in [`REMAINING_FEATURES_AUDIT.md`](REMAINING_FEATURES_AUDIT.md) §6.)

---

## Architecture assessment (keep this)
- `Orchestrator` (`orchestrator.go`): generic — registers `Worker`s, on each task runs the
  worker and hands the `Result` to the `ResultHandler`. Knows nothing about pipeline shape.
- `ResultHandler`/`FullPipelineHandler` (`handler.go`): owns **all** routing via a switch
  on `Result.Type`. One place to read the whole flow.
- `Enqueuer` (`enqueuer.go`): idempotent per-stage task IDs (`recordID:taskType` →
  duplicate enqueue is a no-op), `MaxRetry=3`, rate-limit-aware `RetryDelay`.
- Error taxonomy (`errors.go`): `ErrRateLimit` (retry after delay) / `ErrTransient`
  (retry w/ backoff) / `ErrPermanent` (drop). Clean and sufficient.

## Extensibility — how a new stage lands (for the PRD features)
Adding a stage touches: a `Worker` impl + a `Result` discriminant + one `case` in the
handler switch + `orch.Add` + a queue concurrency entry. The handler switch is the single
coupling point (good — one file to read the flow; just remember to update it).

For the upcoming features specifically:
- **Signals "why this matters" / ranking** → *no new stage needed*: add an LLM
  `RecordRelevanceDecider` (implements `Decide`) and register it in `tenant_match`; it can
  write a richer `relevance_reason` + score. This is the cleanest extension point the
  current design offers.
- **Claims / graph (KG)** → a new post-enrichment stage (or revive `graph`) folded into
  the **live** flow after `global_first_pass`, writing the typed model.

## Reliability — current state + gaps
Good: idempotent enqueue, bounded retries, rate-limit handling, `source_revision`
staleness guard on `SaveEnrichment`.
Gaps to address while streamlining:
1. **Terminal failures are invisible at the app layer.** When a stage exhausts `MaxRetry`,
   asynq archives the task but the `records.status` stays `pending`/`enriched` — no
   `failed` state, no easy "stuck records" query. (Relates to `make ops-reset-stuck-cycles`,
   which operates at the sync-cycle layer, not per-record.) → add a terminal-failure status
   + a small ops/list query.
2. **Dead queues are still configured** (`search/embed/graph`) — harmless but misleading;
   removed by the prune.
3. **No structured DLQ visibility** in-app (asynq has one; nothing surfaces it).

---

## Recommended plan
**Phase A — Prune the dead branch (simplicity; low risk).**
Delete the dead artifacts in the table above; trim `RecordPayload` to the live fields (or
drop it if `GlobalRecordPayload`/`TenantMatchPayload` fully cover the live flow); remove
the `insights` channel; drop the `search/embed/graph` queue configs + `orch.Add`s. Update
`handler_test.go`/`workers_test.go` (they currently exercise dead result types). Net: one
flow, ~half the workers. Verify `make test-go` green.

**Phase B — Reliability (small, high-value).**
Add a terminal-failure record status set when a stage returns `ErrPermanent` or exhausts
retries, plus a "stuck/failed records" listing for ops. Keep the error taxonomy.

**Phase C — Extensibility hooks (only when those features start).**
Document/confirm the "add a decider" path for Signals rationale; design the KG/graph stage
as a *new live stage* rather than reviving the dead `graph` worker as-is.

> Note on embeddings/graph: pruning **removes dormant scaffolding** for embeddings + Neo4j.
> That capability isn't running today anyway; the KG track ([`REMAINING_FEATURES_AUDIT.md`](REMAINING_FEATURES_AUDIT.md)
> §6–7`) should design a graph/embed stage into the live flow deliberately, not inherit the
> dead one. If you'd rather **not** lose the scaffolding, choose "Consolidate" instead of
> "Prune" (revive embed+graph into the live flow after `global_first_pass`).

## Test coverage to preserve
`handler_test.go` (routing), `tenant_match_test.go`, `relevance_test.go`, `workers_test.go`.
Pruning must update the dead-path assertions in `handler_test`/`workers_test` and keep the
live-path + tenant-match + relevance tests green.
