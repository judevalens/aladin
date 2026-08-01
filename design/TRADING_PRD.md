# Aladin — Trading Product Plan

> **Status:** living plan, not a locked spec (2026-07-15). Supersedes `PRD.md` as the
> product north star. `PRD.md` and `DATA_MODEL.md` remain accurate about the *substrate*
> (entity/claim layer, ingestion) and stale about the *product* — read them as history.
> Companion docs: `DESIGN_SPEC.md` (tokens/components, unchanged), `BROWSER_SPEC.md`,
> `backend_v2/PIPELINE.md` (ingestion, authoritative).

---

## 0. The thesis

**Aladin is not a backtesting engine. It is the layer around the backtest.**

vectorbt, backtrader, and nautilus_trader already exist and are better than what we'd
write. QuantConnect will hand you a Sharpe ratio today. What none of them hand you is:

> *here's why I believed this, here's what history said, here's what I did,
> here's what happened, here's what I actually learned.*

That's the product. Everything else is a means to it.

### The sharper version: the platform's job is to stop you fooling yourself

A novice with programming skill and a math background does not fail for lack of
compute. They fail in three specific, well-documented ways, and **a platform can make
all three structurally hard instead of merely discouraged**:

| Failure | How a novice hits it | What Aladin does about it |
|---|---|---|
| **Lookahead bias** | Strategy accidentally reads a bar it couldn't have known at decision time. | The strategy contract only ever hands you bars `<= T`. You cannot see the future because the future is not in the argument. |
| **Survivorship bias** | Universe is "symbols that exist today" → the strategy only ever bought companies that survived. | `instruments` retains delisted symbols with validity ranges. Universe resolution is *as-of*, never *as-now*. |
| **Multiple testing** | Run 200 backtests, keep the good one, mistake luck for edge. | **Every backtest run is persisted, forever, attached to its thesis.** The platform can tell you how many trials you actually ran before you fell in love with #187. |

The third one is the differentiator and nobody does it. It needs nothing more exotic
than a table you never delete from, and it is the concrete answer to *"make me a better
trader"* rather than *"make me a faster backtester."*

---

## 1. Who it's for

**You, first. Others, later.** A novice trader with a strong programming and math
background, learning the domain *through* building the harness. Keep the seams clean
enough to open up later (tenancy already exists — everything is `user_id`-scoped), but
do not pay for strangers yet: no onboarding, no education layer, no guardrails for
people whose judgment you can't model.

The corollary: **when in doubt, build the thing that teaches you the domain**, not the
thing that is architecturally satisfying. Those diverge more often here than anywhere
else in this repo.

---

## 2. The core bet: the research log is the product

The thing you cannot pip-install is the record of your own reasoning, tested. Not a
graph — a **spine**, linear and boring, where every arrow is a foreign key:

```
strategies.hypothesis      why I believe this            (prose, authored by you)
  └─ strategy_versions     what "this" actually was      (code hash — reproducibility)
      └─ backtest_runs     what history said             (ALL of them, forever)
          └─ signals       what it saw live
              └─ trades    what I did, and what happened
                  └─ note  what I learned
```

No LLM extraction, no stance inference, no graph traversal. **The value isn't in the
topology — it's in nothing being thrown away.** That's what makes §0's third guarantee
possible, and it's the whole differentiator.

### Why not the claim layer (recorded so it doesn't come back)

An earlier draft of this doc bet on re-aiming the existing `claims` / `claim_edges`
schema: *"a strategy is a claim."* It's out, for three reasons:

1. **The semantics don't match.** `claim_mentions` models *"this text mentions this
   proposition,"* extracted by an LLM from prose. A backtest doesn't *mention* a
   strategy — it *measures* it. Widening `source_kind` was the easy part; the meaning
   was never going to line up.
2. **The best feature in §0 doesn't need it.** Trial counting is a `count(*)` over
   `backtest_runs`. Stances, edges, and traversal add nothing to it.
3. **A graph solves a scale problem we don't have.** You will author tens of
   strategies, not thousands. You can hold them in your head. Graphs earn their
   complexity precisely when you can't.

If a real connection need shows up later — *"these four strategies are all quietly
betting on the same regime"* — build it **then**, against evidence you've actually
seen, instead of designing now for evidence you've imagined.

---

## 3. What Aladin is not

Stated explicitly, because each of these is a plausible-sounding way to lose a year:

- **Not a backtest engine.** We wrap a kernel. We do not write the vectorized math.
- **Not a broker.** Alpaca holds the money and is the source of truth for positions.
- **Not a signal marketplace / social product.** Single player. See §1.
- **Not low-latency.** Swing trading holds for days to weeks. We are not competing on
  microseconds and any design that pretends otherwise is cosplay.
- **Not a generic knowledge platform.** That was the drift. This doc is the correction.

---

## 4. Current reality (the honest baseline)

**There is zero trading code in the repo.** No ticker, no bars, no broker, no
portfolio. (`GET /api/quote` at `internal/api/server.go:375` returns a random
*inspirational* quote. The old `internal/service/signals.go` used "marked to market"
as a metaphor; it was removed with the claim layer, freeing "signal" for its trading
meaning.) The trading side is a from-scratch build.

**What survives and is worth a lot:**

- **`internal/sync`** — a generic polling engine with cursors, cycles, pagination, and
  dedup (`scheduler.go`, `arbiter.go`, `seen.go`). This is exactly what a market-data
  poller needs. A new source implements the `Syncer` interface at
  `internal/sync/syncer.go:40` and registers in a hardcoded slice at
  `cmd/worker/main.go:252-254`. **No engine changes required.** Months of unglamorous
  work already done.
- **`internal/pipeline`** — asynq orchestration + `reaper.go`, which re-drives stranded
  records. Market-data ingestion needs precisely that reliability frame.
- **db / repo / auth / config** — substrate, unaffected.

**What is knowledge-engine specific:** `claims`, `entities`, `discourse`, `insights`,
`graph`, `pageingest`. **Decision: keep running as-is for now** (see D4) — but nothing
in this plan builds on them, per §2. `sync_v2` is a 7-line stub and can go regardless.

**The migration chain is fine — verified, not assumed (2026-07-15).** An earlier draft
of this doc called it a blocker and made it a T0 gate. That was wrong. Running the real
embedded `db.Migrate` against an empty pgvector container migrates cleanly from scratch:
15 migrations, 34 tables, no error. The gaps (`00007`–`00009`, `00017`) are
**deliberately reserved numbers**, not missing links — goose does not require contiguous
versions, `internal/db/migrate.go:32` explicitly sets `goose.WithAllowMissing()`, and
`pg_trgm`/`vector` come from `00001_baseline`, not from the gap. The `00010` header says
as much: numbered 00010 "deliberately: 00006-00009 are claimed by an uncommitted branch."

**The real (latent, conditional) issue is a merge collision, not a chain break.** The
reserved files exist on other branches: `00007`–`00009` on `claude/suspicious-buck-315848`
(discourse engine), `00017_branding.sql` on `pov-branding`. Both `00012_discourse.sql`
(here) and `00008_entity_discourse.sql` (buck) run `CREATE TABLE public.entity_discourse`
with no `IF NOT EXISTS` — merging buck into a DB that already ran 00012 would fail on
boot. This pivot deprioritizes the discourse engine, so that branch may never merge and
the collision may never fire. **Not a gate; a thing to remember if buck is ever revived.**

**Schema work here can start at 00020 and proceed normally.**

---

## 5. Architecture

```
  L0  Market data      instruments · bars · corporate_actions   [Go + Postgres]
        │                    ▲ Alpaca syncer via sync.Syncer
        ▼
  L1  Strategy         manifest + runtime process               [protocol boundary]
        │              ONE definition ─┬─► backtest
        │                              ├─► live scan
        ▼                              └─► journal attribution
  L2  The log          strategies.hypothesis · strategy_versions
                       backtest_runs · signals · trades · notes
                       [append-only; nothing is ever thrown away]
```

Three layers, not four. There is no reasoning layer — see §2.

### The rule that must not bend

**One strategy definition, three consumers.** A live scan is the strategy applied to
the latest bar. If backtest and live ever run different code, every number downstream
is a lie — this is the single most common failure of homegrown trading stacks, and it
fails silently.

### The strategy boundary is a protocol, not an API

Runtime language is **open** (see D1). That is only survivable if the boundary is a
process contract rather than a language-native interface. A strategy is a process with
a manifest declaring runtime, entrypoint, params, universe, and timeframe.

**Contract shape: batch, not streaming.**

- *Bar-by-bar streaming* matches live scan naturally and works in any language — but
  fights vectorized libraries, which is most of the reason to want Python or Julia.
- *Batch* hands the strategy an array of bars, gets back an array of signals. It runs
  live fine: hand it a trailing window ending at the latest bar, take the last signal.
  Same code, both consumers, rule above preserved.

Batch is also the natural idiom in Python, Julia, *and* Kotlin — so it doesn't quietly
pick a winner while D1 is still open.

### Storage decision: store raw bars, adjust on read

Adjusted prices mutate retroactively every split and dividend. Storing adjusted prices
means **stored history silently changes under you and backtests stop being
reproducible**. Store unadjusted bars plus a `corporate_actions` log; compute
adjustment at read time. Deterministic, replayable, and it makes the adjustment logic
itself testable.

### Identity: `instrument_id`, never `symbol`

Tickers get recycled — a delisted symbol can be reassigned to an unrelated company.
`instrument_id` is stable; `symbol` is a time-scoped attribute of it. This is the same
subtlety that produces survivorship bias, and it is very expensive to retrofit.

---

## 6. Open decisions

| # | Decision | Status / recommendation |
|---|---|---|
| **D1** | **Strategy runtime language** — Python, Julia, Kotlin? | **OPEN, and deliberately deferred.** T1 is Go/Postgres end to end and does not touch it. Settle at T2 by writing the same throwaway strategy in two candidates. *Recommendation: one runtime first (Python — the ecosystem is where the learning is); design the contract so others slot in without engine changes. Three runtimes for a solo novice is 3× harness and 0× edge.* |
| **D2** | **Data vendor** — Alpaca / Tiingo / Polygon? | Alpaca is the default: free data, paper broker, and live broker behind one auth — one integration covers L0 and L4. **Verify free-tier history depth and feed coverage before committing**; if thin, split (Tiingo/Polygon for data, Alpaca for execution). |
| **D3** | **Bar storage** — plain Postgres or TimescaleDB? | **Plain Postgres.** US equities daily bars are ~6k symbols × 252/yr × 20yr ≈ 30M rows; Postgres handles that with a sane PK. Revisit only if intraday resolution lands. |
| **D4** | **The KG code** (`discourse`, `insights`, `graph`) | **Keep running as-is** (user call). Note the cost: the discourse sweep is a goroutine ticker at `cmd/worker/main.go:131` firing every 5 min and burning LLM tokens for a deprioritized product. Cheap to park later — kill the ticker, keep the code. |
| **D5** | **Execution safety model** — kill switch, per-strategy risk limits, reconciliation. | Unresolved, and required *before* T5. Alpaca is the source of truth for positions; we reconcile against it, never trust local state. **Reconciliation surfaces on the Portfolio surface — see `PORTFOLIO_PRD.md` §3.** |
| **D6** | **Universe definition** — how is "what to scan" expressed and resolved as-of a date? | Unresolved. Falls out of L0; must be as-of to preserve §0's survivorship guarantee. |

---

## 7. Roadmap

Ordering is **dependency, not preference**. Backtest, scan, and journal are all
consumers of the bar store; nothing works without L0.

*(There is no T0. An earlier draft gated this roadmap on "fix the migration chain";
that was verified false — see §4. New migrations start at `00020`. Phase numbers are
kept as-is rather than renumbered, since T1–T5 are referenced elsewhere.)*

- **T1 — Market data (L0).** `instruments` (with delisting + symbol validity ranges),
  `bars` (raw), `corporate_actions`; Alpaca syncer via `sync.Syncer`; backfill command
  (pattern: existing `cmd/backfill-lenses`). Go/Postgres only — D1 stays open.
- **T2 — Strategy protocol + backtest (L1/L2).** Manifest, process contract, first
  runtime, `strategies` (incl. `hypothesis`) / `strategy_versions` (code hash —
  reproducibility is the point) / `backtest_runs` / `backtest_trades`. **D1 resolves
  here.** ⚠️ **Constraint from `PORTFOLIO_PRD.md` §3: a strategy's target weights are
  weights of its OWN SLEEVE, not of the whole book.** Settle this in the protocol — after
  the fact it changes the process contract, every stored manifest, and every recorded run.
- **T3 — Live scan → watchlist.** Same process, trailing window, latest bar.
  `scan_runs` / `signals`. Scheduled post-close; daily bars need no intraday infra.
- **T4 — Journal.** `trades` links signal → strategy → outcome, plus a note for what
  you learned. **The loop closes here** — this is where the product becomes the thing
  in §0. Small phase: a table and a form, not an engine.
- **T5 — Execution.** Per-strategy mode (`manual | paper | live`), orders/fills/
  positions, reconciliation, kill switch. **Paper before live, always.** Last because
  you cannot auto-execute a strategy you haven't validated, and every day this code
  doesn't exist is a day it cannot lose money through a bug.

**Why journal is T4 and not T1:** it's the cheapest phase and closes the feedback loop,
but there is nothing to journal until there are trades.

---

## 8. Risks (watch list)

1. **Building the engine instead of trading.** The dominant failure mode for a
   programmer entering this domain: a year of beautiful infrastructure and zero
   learning about markets. Mitigation: §3, and D1's "one runtime first."
2. **Overfitting.** You *will* find edge that isn't there. The platform must make this
   harder, not faster — hence persisted backtest runs and visible trial counts (§0).
3. **Backtest/live divergence.** Mitigated structurally by the one-definition rule, and
   only by that.
4. **Data quality.** Survivorship, adjustments, and lookahead each independently
   invalidate every number downstream. All three are L0 concerns and all three are
   cheaper to get right at T1 than to detect at T4.
5. **Auto-execution bugs cost real money**, unlike every other bug in this repo.
6. **KG drag.** Kept running per D4 — it costs tokens and attention continuously. If it
   isn't earning its keep by T2, park it.

---

## 9. Pointers

- `PRD.md` §2 ("The wedge: trading first") — the original plan already said this. The
  build drifted into the substrate; this doc is the correction. §9's parked item #3
  ("the outcome loop … trading's P&L is ground truth") is now T4.
- `DATA_MODEL.md` — the entity-spine plan. Largely moot: it was solving *"connect the
  authored and discovered worlds"* for a generic KG. This plan doesn't need that bridge.
- `internal/sync/syncer.go:40` — the interface a market-data source implements.
- `cmd/worker/main.go:252-254` — where it registers.
</content>
