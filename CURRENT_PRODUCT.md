# Aladin Current Product Truth

Status: canonical cleanup baseline as of 2026-08-14.

Aladin is currently a personal trading research workspace. Its job is to make the
research loop truthful: preserve what I believed, exactly what I tested, every run
I performed, what happened live, and what I learned after the fact.

The active product is not a generic knowledge graph, a backtesting engine, a broker,
or a broad AI workspace. Those older ideas may still exist as substrate or history,
but the product direction is the trading research log.

## Active Spine

The core loop is:

```text
hypothesis
  -> strategy version
  -> backtest runs
  -> live scan signals
  -> trades
  -> journal notes
```

The product promise is: stop me fooling myself. In concrete terms, Aladin should make
lookahead bias, survivorship bias, and multiple-testing self-deception harder to miss.

## Build Priorities

1. Market data substrate: instruments, bars, corporate actions, watchlists.
2. Strategy research log: hypotheses, strategy versions, persisted backtest runs.
3. Live scan and signal capture using the same strategy definition as backtests.
4. Journal loop tying trades and lessons back to the original thesis.
5. Execution only after the research and reconciliation loop is trustworthy.

## Current Supporting Systems

These systems are still useful when they serve the active spine:

- React/Tauri shell and local sync.
- Go API, worker, Postgres repositories, realtime outbox.
- Sources, document ingestion, entities, and search as trading research context.
- Copilot as a docked, tool-grounded assistant inside the workspace.
- Shards/doc surfaces when they create concrete research artifacts.

## Parked Product Ideas

These are not the current product direction:

- Generic graph-first knowledge workspace.
- Broad proactive AI insight product.
- Tutor/learning surface.
- Workspace-wide graph exploration as a primary nav destination.
- A standalone "Insights" product surface independent of trading research.

Parked does not mean deleted. It means new work should not expand these surfaces unless
it directly strengthens the trading research loop above.

## Documentation Rule

When docs disagree, trust this file first, then:

- `design/TRADING_PRD.md` for product intent.
- `design/UI_ARCHITECTURE.md` for current frontend conventions.
- `backend_v2/PIPELINE.md` for ingestion architecture after it has been refreshed.
- `README.md` for repo orientation and commands.

Older graph/KG/AI-workspace documents should be treated as historical unless they
explicitly say they are current.
