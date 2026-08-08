# research

The bar store for the trading engine. Symbols and a date range in, a matrix out —
fetching only what isn't already held, and never spending without pricing it first.

Kotlin, DuckDB, Databento. No fixture, no scaffolding: the database is derived and
rebuilt by fetching.

## Run

```bash
SMOKE_SYMBOLS=AAPL,MSFT,NVDA ./gradlew -q run -Dmc=FetchKt
```

Credentials come from the environment or `research/.env`:

```
DATABENTO_API_KEY=...
DATABENTO_DATASET=EQUS.SUMMARY        # optional; this is the default
DATABENTO_AUTO_APPROVE_UNDER=0.10     # optional; spend this much without asking
```

## Using it

```kotlin
openDb().use { conn ->
    val fetcher = BudgetedFetcher(DatabentoBatchFetcher(), autoApproveUnder = 0.10)
    val universe = conn.resolveUniverse(symbols, asOf).first

    conn.ensureBars(fetcher, universe, Schema.OHLCV_1D, range)   // fetches only gaps
    val bars = conn.loadMatrix(symbols, range, source = fetcher.source)  // -> BarMatrix
    val df   = conn.loadFrame(symbols, range, source = fetcher.source)   // -> DataFrame
}
```

`BarMatrix` is a flat `DoubleArray` plus shape: `rowMajor` for multik, `colMajor()`
for a per-bar loop, `holes` counting bars the store had no row for.

## What the store guarantees

**Identity is `instrument_id`, never `symbol`.** Tickers get recycled, so a symbol is
a time-scoped attribute and resolution is always *as-of a date*. A symbol that did not
exist then resolves to nothing — a real answer, since pre-IPO and post-delisting are
exactly what survivorship bias hides. Two instruments claiming one ticker on one date
fails loudly rather than tie-breaking.

**Coverage means a range is paid for once.** It cannot be derived from the bars table:
a missing row is ambiguous between weekend, holiday, halt, pre-IPO, post-delisting and
not-fetched. Only a record of what was *requested* distinguishes them. `rows = 0` is a
real answer, so a delisted name is asked about once. Coverage never reaches today,
since the current session's bar is partial until the close.

**`source` is the data SCOPE, not the vendor** — `databento:EQUS.SUMMARY`, not
`databento`. AAPL on 2024-08-01 is 218.36 on 62,500,996 shares consolidated, and
219.65 on 21,277,576 from `XNAS.ITCH`. Neither is inaccurate; each answers a narrower
question, and nothing in the numbers flags which you're holding.

**The matrix is rectangular by construction.** A dense grid is built in SQL by
CROSS JOINing dates × instruments and LEFT JOINing the bars, so holes arrive as
explicit NULLs rather than a ragged result. What a hole becomes is a choice:
`Holes.NAN`, `FORWARD_FILL` or `DROP_DATE`.

**Column labels follow `instrument_id` order**, matching the grid query. Ordering them
alphabetically instead puts every symbol's prices under a different symbol's name.

## Money rails

Coverage is the rail that saves most, because a held range is never fetched, priced or
prompted for. For the rest, `BudgetedFetcher` calls `metadata.get_cost` first, proceeds
below the threshold, asks on the console above it, and refuses past the hard ceiling —
or if the request cannot be priced at all. It **fails closed**: no console means refuse,
so an unattended run can neither spend by assuming consent nor hang for a reply nobody
will type. Set `DATABENTO_INTERACTIVE=1` where stdin works but `System.console()` is null.

## Databento notes

Verified against the live API and the official Python client, which is the authoritative
contract — the published reference would not load.

- `EQUS.SUMMARY` is the only consolidated product on this subscription, and it starts
  **2024-07-01** with `ohlcv-1d`, `definition`, `statistics` only. There is no
  consolidated intraday. Deeper datasets are single-venue (`XNAS.ITCH` from 2018-05) or
  samples (`EQUS.MINI` 4.7% of the tape, `DBEQ.BASIC` 2.5%).
- `timeseries.get_range` is **POST, form-encoded**, max **2,000 symbols** per request.
  No pagination; `limit` is opt-in.
- `pretty_px=true` and `pretty_ts=true` make the server return decimal prices and ISO
  timestamps. Without them prices are int64 × 1e-9 and timestamps nanoseconds, both
  decoded by hand — a wrong scale would silently corrupt every price.
- `stype_in=raw_symbol` or tickers are read as vendor instrument ids;
  `map_symbols=true` or there is no `symbol` column and batched rows can't be attributed.
- `end` is **exclusive**. Row order is **not** stable, so rows are keyed by symbol.
- Batch (`batch.submit_job` → poll `list_jobs` → `list_files` → download) suits bulk;
  streaming suits read-through gaps. Both implement `BarFetcher`, so the store is
  indifferent.
