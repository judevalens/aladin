# research

Kotlin bench for strategy work — the sketchpad ahead of the real engine.
Bars in, target weights out. **Not** an engine: no fills, no costs, no P&L.

## Run

```bash
./gradlew -q run -Dmc=VerifyKt
```

`-Dmc` picks the entry point. Everything lives in the default package, so it's the
class name with a `Kt` suffix.

| entry point | what it does |
|---|---|
| `VerifyKt` | every implementation vs the frozen reference — **run this after any change** |
| `MaCrossKt` | raw per-bar loop, writes `data/weights_kotlin.csv` |
| `MaCrossMultikKt` | multik shifted-sum, writes `data/weights_multik.csv` |
| `EmitCumsumKt` | multik cumsum O(n), writes `data/weights_multik_cumsum.csv` |
| `BenchMaKt` | times each implementation on 5000 bars × 100 symbols |
| `ScaleMaKt` | how each scales with lookback — the case for cumsum |
| `WhySlowKt` | why naive multik is 10× slower (views, not allocation) |
| `TransposeBugKt` | minimal repro: `dot(transpose, vector)` silently returns zeros |
| `ToMultikKt` | DuckDB → multik, plus BLAS matmul vs a hand loop |
| `DfToArrayKt` | column → `DoubleArray`, and why boxing costs memory not speed |
| `ExploreKt` | DataFrame over DuckDB — schema, describe, groupBy, pivot → multik |
| `CoverageProbeKt` | the coverage ledger across every partial/hole/known-empty case |
| `FetchProbeKt` | read-through fetching: only the gaps ever hit the vendor |
| `RaceProbeKt` | concurrent `ensureBars` double-fetches, and the lock that fixes it |

## The golden file

`data/weights_reference.csv` came from the original Python implementation before it
was removed. **Regenerate it only when the strategy's definition deliberately
changes — never to make a failing check pass.** It is the only thing standing between
a numerics regression and a plausible-looking wrong equity curve.

## Two things that will bite you in multik

**Views.** Slices share the base buffer. Writing through one mutates the original, and
in a sweep that shares a bars matrix across runs, one careless strategy corrupts every
other result. Type strategy parameters `MultiArray<Double, D2>` (no `set`) rather than
`D2Array`.

**BLAS on non-contiguous operands.** `mk.linalg.dot(a.transpose(), vector)` returns
**zeros** — no exception, just a `DGEMV` complaint on stderr. `deepCopy()` before any
`mk.linalg.*` call on anything that isn't freshly constructed. See `TransposeBugKt`.

## Performance, measured

multik costs roughly **2.3 ns per element-operation** against a raw `DoubleArray`'s
**0.24 ns** — a fixed ~10× tax. So what matters is *how many array passes* a
formulation makes, not how many adds. Prefer prefix-sum formulations:

```
lookback   per-bar loop   multik cumsum
50               7.7 ms         11.7 ms
100             18.1 ms         10.8 ms
250             54.4 ms         10.5 ms
500            113.3 ms         10.3 ms
```

Cumsum is O(n) regardless of window; the loop is O(n·w). Crossover is around 60.
Caveat: cumsum subtracts large near-equal numbers, so re-run `VerifyKt` when you move
to a much longer series — precision there is empirical, not structural.

## Storage

Bars live in **DuckDB** (`data/research.duckdb`) — embedded, columnar, single file, no
server. Same operational model as SQLite, opposite storage model: columnar and
vectorised rather than row-oriented, so range scans and aggregates are the fast path.

The store is **long** — `(ts, symbol, close)` — because that's the shape real bars
arrive in. `BarStore.loadBars()` pivots once at load and hands back a `BarMatrix` with
both layouts: `rowMajor` for multik, `colMajor()` for the per-bar loop.

No Arrow, no Parquet, no Hadoop, no JNI. Measured: 3.3M rows/sec ingest via DuckDB's
Appender, sub-millisecond range queries, and JDBC extraction at ~274M values/sec —
within 15% of an Arrow export stream, so the plain `ResultSet` is not a bottleneck.

`DataFrame.readSqlQuery(conn, sql)` fails on a `jdbc:duckdb:` URL — auto-detection
only knows the six databases it ships DbTypes for. But `DbType` is an open extension
point, so `BarStore.kt` registers DuckDB properly as `object DuckDb : DbType("duckdb")`
and `Connection.frame(sql)` passes it explicitly. Everything works: `describe()`,
`groupBy`/`aggregate`, schema inference.

Three layers, each doing what it's good at:

- **DuckDB** — storage and heavy aggregation (never materialises the rows)
- **DataFrame** — display, exploration, ad-hoc analysis · `conn.frame(sql)`
- **multik / `DoubleArray`** — the engine's hot path · `df.convertToMultik { }`

## Coverage and fetching

`Coverage.kt` tracks which `(source, symbol, schema, date range)` slices are held, so
a historical range is paid for once. It **cannot** be derived from the bars table — a
missing row is ambiguous between weekend, holiday, halt, pre-IPO, post-delisting, and
not-fetched. Only a record of what was *requested* distinguishes them.

Ranges merge on write, which makes the common question a single `EXISTS`:

```kotlin
conn.isCovered(slice, range)        // do I have all of this?
conn.missingRanges(slice, range)    // what still has to be fetched
conn.ensureBars(fetcher, "NVDA", "ohlcv-1d", range)   // read-through: fetch only gaps
```

`rows = 0` is a real answer — a delisted symbol is recorded as checked-and-empty so it
is never re-requested. Coverage never extends to today, since the current session's
bar is partial until the close.

## Missing

**A verified Databento client.** `DatabentoFetcher` is written but has never made a
real request — no `DATABENTO_API_KEY` configured. Two things to confirm against live
data: the dataset code for consolidated US equities, and that OHLCV prices arrive as
int64 scaled by 1e-9 (`PRICE_SCALE`).

**Known gaps in the store design**, in the order they'd bite:

- `ohlcv.ts` is `DATE`, so it cannot hold 1-min or 5-min bars. Needs `TIMESTAMP`.
- No `UNIQUE (source, symbol, schema, ts)`. Coverage is currently the only thing
  preventing duplicate rows, and duplicates produce plausible wrong numbers rather
  than errors.
- `BarFetcher.fetch` takes one symbol. Databento accepts multi-symbol requests and
  bills by bytes, so a universe backfill should be a few calls, not thousands.
- Concurrent `ensureBars` on the same slice double-fetches (see `RaceProbeKt`).
  A single fetcher behind a lock is enough — DuckDB is single-writer, one JVM.
- Keyed on `symbol`, not `instrument_id`. Tickers get recycled; this is the expensive
  one to retrofit.
- Nothing records raw vs adjusted prices.
