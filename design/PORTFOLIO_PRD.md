# PRD — The Portfolio Surface

> **Audience:** whoever builds this, and whoever builds T2 before it.
> **Date:** 2026-07-31. Revision 1.
> **Status: NOT SCHEDULED.** The engine (T2) and the research bench come first. This is
> written now for one reason only — see §0.
>
> Sections marked **LOCKED** are settled. Sections marked **OPEN** are not.
>
> Companions: `TRADING_PRD.md` (roadmap, D5 execution safety, D6 universe) ·
> `RESEARCH_SURFACE_PRD.md` (the research folder; §5's "view above research folders") ·
> `DESIGN_SPEC.md` (authoritative for styling).

---

## 0. Why write this before building it

Because **it constrains the T2 strategy protocol**, and T2 is next.

A strategy emits a target weight per instrument. Weight *of what*? If it means "of the
whole book," then two armed strategies each emitting `1.0` both claim all your capital
and the engine has no principled way to resolve it. The answer is a portfolio-level
decision that has to be true before the protocol is written, not after.

**The answer (§3): a strategy's weights are weights of its OWN SLEEVE.** The portfolio
decides how big that sleeve is. Getting this backwards is expensive — it's a change to
the process contract, every stored manifest, and every run already recorded.

Everything else here can wait.

---

## 1. What it is  **LOCKED**

Research folders hold the **intent**. Portfolio holds the **truth**. The join between
them is **attribution**.

A research folder cannot hold any of this, for two structural reasons:

1. **Allocation is a cross-strategy fact.** A strategy doesn't know it's 12% of the book,
   and it certainly doesn't know that it and another strategy are both long semis.
   Concentration is only visible from above. Rendered inside a research folder, every
   folder would show a number that is wrong in precisely the way that matters.
2. **Broker truth is singular.** `TRADING_PRD.md` §3 — Alpaca is the source of truth for
   positions; Aladin reconciles and never asserts over it. There is one account. A
   research folder showing "its" positions is already an *attribution* of a single
   account, and attribution can only be computed somewhere that sees every claimant.

This is the surface `RESEARCH_SURFACE_PRD.md` §5 anticipates when it says comparing
strategies is a view *above* research folders, never a folder containing them.

## 2. What it is not  **LOCKED**

- **Not order entry.** No buy/sell, no order tickets. Execution is T5 and lives behind
  its own safety model (D5). This surface reads and reconciles.
- **Not a P&L wall.** `RESEARCH_SURFACE_PRD.md` §2 bans the 40-widget dashboard, and a
  portfolio screen you *watch* is the opposite of the research loop this product exists
  to run. Its job is reconciliation and allocation, not entertainment.
- **Not Markets.** Markets is the world — instruments, quotes, movers, watchlists.
  Portfolio is your book. Different questions; folding either into the other makes both
  worse.
- **Not a second source of truth.** See §3.

---

## 3. The attribution model  **LOCKED** — the expensive part

Three books, only one of which is a fact.

| Book | Where it comes from | Status |
|---|---|---|
| **Intended** | Each armed strategy's target weights × its allocated sleeve | a model |
| **Expected** | Sum of every strategy's intended positions | a model |
| **Actual** | Alpaca | **the fact** |

**A strategy's target weights are weights of its own sleeve, not of the book.** The
portfolio assigns each armed strategy a capital sleeve; the strategy allocates within it.
This is the §0 constraint on T2.

**Attribution is DERIVED, never stored.** The broker reports one AAPL position of N
shares. Any per-strategy split of it is a model, not a fact — so it is computed on read,
proportional to intent, and never written back onto the position. Storing it would create
exactly the second source of truth §2 forbids, and it would silently drift from the
broker.

**The residual is the product.** `actual − expected` is the unattributed bucket: manual
trades, partial fills, failed orders, stale state, your own fat fingers. That difference
is the highest-value thing on this surface and the reason it exists — it is only
interesting when something is wrong, which is exactly when you need it. D5 in
`TRADING_PRD.md` names reconciliation as required before T5; this is where it surfaces.

---

## 4. What's on the surface  **LOCKED**

1. **Positions** — broker truth, unedited, with each row's derived attribution.
2. **Allocation** — two different concentration questions, both needed:
   capital **by strategy**, and exposure **by sector / instrument**. The second is the one
   that hurts, because two uncorrelated-looking strategies can be the same bet.
3. **The strategy roster** — every armed strategy, its sleeve, its state, its recent
   contribution, each row linking back to its research folder. This is §5's view above
   research folders, and the only place "how many of my research folders actually risk
   money" is answerable. Per the trading pivot, that number — promotions — is the honesty
   metric, not run count.
4. **Reconciliation** — expected vs actual, with the residual called out rather than
   buried. Quiet when it agrees.

Nothing else without a reason. An empty surface that tells the truth beats a full one
that doesn't.

## 5. Placement  **LOCKED**

**Its own rail item, beside Markets.** Not a research tab, not a Markets sub-view.

Unlike the research bench (which deliberately took no rail item, `RESEARCH_SURFACE_PRD.md`
§11, because it is per-folder), Portfolio is genuinely singular and workspace-wide — there
is one book. That is what earns the rail entry.

---

## 6. Sequencing

**Blocked behind T2 and the research bench. Do not start early.**

It splits the same way the research container did:

- **Readable today** — positions, account, and allocation come straight from Alpaca. The
  client methods already exist (`internal/market/alpaca/client.go` `GetPositions` /
  `GetAccount`) with an adapter at `internal/app/market_sources.go`; nothing is
  REST-exposed, so it's a thin service + handler away.
- **Waits on the engine** — intended/expected books, attribution, the strategy roster,
  and reconciliation all need strategies that actually emit target weights (T2) and
  execution state (T5).

Building the readable half first is tempting and wrong: with no armed strategies it is a
positions table, which Alpaca already renders better, and it would harden an attribution
model before the thing being attributed exists.

## 7. Open

- **Where the sleeve is set** — on this surface, or on a strategy's Control pane
  (`RESEARCH_SURFACE_PRD.md` §17)? *Lean: here. Sleeves must sum to ≤ 1 across
  strategies, which is a constraint no single strategy can enforce.*
- **Rebalance policy** — does drift between intended and actual auto-correct, prompt, or
  just get reported? Interacts hard with D5 and should be decided with it, not before.
- **Does an unarmed strategy reserve capital?** *Lean: no. Reserved-but-idle capital is a
  fiction that makes every allocation number wrong.*
- **Multi-account** — assume one until there are two. Not a v1 concern.
- **Cash and buying power** — margin makes "allocation" ambiguous. Define against equity
  or against buying power before drawing anything.
