# research

The bar store for the trading engine. Symbols and a date range in, a matrix out —
fetching only what isn't already held, and never spending without pricing it first.

Kotlin, DuckDB, Databento. No fixture, no scaffolding: the database is derived and
rebuilt by fetching.

## Run

```bash
SYMBOLS=AAPL,MSFT,NVDA FROM=2024-08-01 TO=2024-09-30 ./gradlew -q run
./gradlew test
```

## Layout

```
aladin/
  Domain.kt          DateRange, Schema, InstrumentType, Instrument, BarRow, Slice
  Env.kt             config from environment or .env
  Main.kt            entry point
  store/
    Db.kt            DuckDB access, the DbType, identifier guards
    BarMatrix.kt     the matrix, and DataFrame -> multik
    Instruments.kt   as-of identity, read-through registry
    Coverage.kt      the ledger — a range is paid for once
    Ohlcv.kt         the bars table and ensureBars
    BarGrid.kt       the dense grid, loadMatrix / loadFrame, hole policies
    BarStore.kt      the Bars interface and BarStore
  vendor/
    Fetcher.kt       BarFetcher, PricedFetcher, SymbologySource, LockedFetcher
    Http.kt          timeouts, retry, Retry-After
    Budget.kt        the cost gate
    Databento.kt     streaming, batch and symbology clients
    DatabentoDecode.kt   pure payload decoding
```

The split that matters: `vendor` knows nothing about the store, `store` depends only on
vendor *interfaces*, and decoding is pure so it can be tested without a key.

Credentials come from the environment or `research/.env`:

```
DATABENTO_API_KEY=...
DATABENTO_DATASET=EQUS.SUMMARY        # optional; this is the default
DATABENTO_AUTO_APPROVE_UNDER=0.10     # optional; spend this much without asking
```

## Using it

```kotlin
BarStore.databento().use { store ->
    val bars = store.bars(listOf("AAPL", "MSFT"), range)   // BarMatrix
    val df   = store.frame(listOf("AAPL", "MSFT"), range)  // DataFrame, for looking
    println(store.held())
}
```

One call. Held ranges come off disk; anything missing is resolved, priced, approved and
fetched on the way. `BarStore.readOnly()` never spends or reaches the network, and the
primary constructor takes any `BarFetcher` — which is how tests run without a key.

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

## Hardening

- **HTTP**: connect and request timeouts, up to 5 retries on 429 and 5xx honouring
  `Retry-After`, and no retry on 4xx — a bad request fails identically every time, so
  retrying only burns quota and delays the error.
- **SQL**: identifiers cannot be bound, so the price field is checked against the real
  column set before interpolation. Instrument ids are `Long`, so they carry no risk.
- **Inputs**: symbols trimmed and de-duplicated, empty requests rejected, backwards
  ranges rejected at construction, a budget whose ceiling sits below its auto-approve
  threshold rejected rather than silently refusing everything.
- **Concurrency**: one lock covers the whole read-through cycle. Locking only the fetch
  is not enough — DuckDB's MVCC rejects the second of two concurrent coverage writes,
  by which point the fetch has been paid for.

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
