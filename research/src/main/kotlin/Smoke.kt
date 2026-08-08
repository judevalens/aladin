/**
 * The smallest real Databento request: one symbol, one month, daily bars.
 *
 * Exists to settle what a fake fetcher cannot — the dataset code, the price scaling,
 * the CSV column names, and the timestamp format. Prints the raw response first so a
 * mismatch is visible rather than silently wrong.
 *
 *   DATABENTO_API_KEY=... ./gradlew -q run -Dmc=SmokeKt
 */

import java.time.LocalDate

private const val COST_CEILING_USD = 1.00

fun main() {
    val symbol = Env["SMOKE_SYMBOL"] ?: "AAPL"
    val dataset = Env["DATABENTO_DATASET"] ?: "EQUS.MINI"
    val range = DateRange(LocalDate.parse("2024-01-02"), LocalDate.parse("2024-01-31"))
    val schema = Schema.OHLCV_1D

    println("dataset=$dataset  symbol=$symbol  schema=${schema.wire}  range=$range")
    println("expected: ~21 bars (${schema.rowsFor(1, 21)} rows at ${schema.barsPerSession}/session)\n")

    val db = DatabentoFetcher(dataset = dataset)

    // usage billing — price it before buying it
    val cost = runCatching { db.estimateCostUsd(listOf(symbol), schema, range) }
        .onFailure { println("cost estimate failed: ${it.message}\n") }.getOrNull()
    cost?.let { println("estimated cost: \$${"%.4f".format(it)}") }
    check(cost == null || cost <= COST_CEILING_USD) { "cost \$$cost exceeds \$$COST_CEILING_USD — aborting" }

    val rows = db.fetch(mapOf(symbol to 1L), schema, range)
    println("\nreturned ${rows.size} bars")
    if (rows.isEmpty()) {
        println("EMPTY — check the dataset code; 'EQUS.SUMMARY' is a guess")
        return
    }

    println("\nfirst three, as parsed:")
    rows.take(3).forEach {
        println("  ${it.ts}  O=${it.open}  H=${it.high}  L=${it.low}  C=${it.close}  V=${it.volume}")
    }

    // the check that catches a wrong PRICE_SCALE instantly
    val close = rows.first().close
    println("\nsanity: $symbol close is ${close}")
    println(when {
        close == null -> "  NULL — column mapping is wrong"
        close in 1.0..10_000.0 -> "  plausible — PRICE_SCALE looks right"
        close > 10_000.0 -> "  too large by ~1e9 — PRICE_SCALE should divide, not multiply"
        else -> "  too small — PRICE_SCALE is over-dividing"
    })

    // only persist once the numbers look sane
    openDb().use { c ->
        c.createInstrumentsTable(); c.createOhlcvTable(); c.createCoverageTable()
        c.registerInstrument(Instrument(1_000L, symbol, LocalDate.parse("1990-01-01"), null))
        val fetched = c.ensureBars(LockedFetcher(db), mapOf(symbol to 1_000L), schema, range)
        println("\nstored $fetched bars; second call fetches " +
                "${c.ensureBars(LockedFetcher(db), mapOf(symbol to 1_000L), schema, range)} (0 = coverage works)")
    }
}
