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
| `ToMultikKt` | DataFrame → multik, plus BLAS matmul vs a hand loop |

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

## Missing

**Data acquisition.** `data/bars.arrow` is a frozen 647×3 sample; `bench_bars.arrow`
is synthetic. There is no fetch path any more — that's the storage layer being built
next (Parquet store + coverage cache, so historical data is never paid for twice).
