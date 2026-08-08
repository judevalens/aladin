/**
 * Exercise the batch path end to end against the live API.
 *   ./gradlew -q run -Dmc=BatchProbeKt
 */

import java.time.LocalDate

private const val COST_CEILING_USD = 5.00

fun main() {
    val symbols = (Env["SMOKE_SYMBOLS"] ?: "AAPL,MSFT,NVDA").split(",")
    val range = DateRange(LocalDate.parse("2024-08-01"), LocalDate.parse("2024-09-30"))
    // resolve against whatever the registry already knows, minting ids only for new
    // tickers — registering a second id for an existing symbol is an identity failure
    val ids = openDb().use { c ->
        c.createInstrumentsTable()
        symbols.withIndex().associate { (i, s) ->
            s to (c.resolveInstrument(s, range.from) ?: (9_000L + i).also {
                c.registerInstrument(Instrument(it, s, java.time.LocalDate.parse("1990-01-01"), null))
            })
        }
    }

    // Every fetch goes through the budget gate: priced first, auto-approved under a
    // dime, typed approval above it, hard-refused past the ceiling. Wrapping once here
    // means no call site can forget it.
    val batch = BudgetedFetcher(
        DatabentoBatchFetcher(),
        autoApproveUnder = (Env["DATABENTO_AUTO_APPROVE_UNDER"] ?: "0.10").toDouble(),
        hardCeiling = COST_CEILING_USD,
    )
    println("submitting…")
    val t0 = System.nanoTime()
    val rows = batch.fetch(ids, Schema.OHLCV_1D, range)
    println("  got ${rows.size} rows in ${"%.1f".format((System.nanoTime() - t0) / 1e9)}s")
    rows.take(3).forEach { println("    ${it.ts}  id=${it.instrumentId}  close=${it.close}  vol=${it.volume}") }

    // land them in the store and read back through the strategy-facing API
    openDb().use { c ->
        c.createOhlcvTable(); c.createCoverageTable()
        println("\n  ensureBars via batch: ${c.ensureBars(batch, ids, Schema.OHLCV_1D, range)} bars")
        println("  second call:          ${c.ensureBars(batch, ids, Schema.OHLCV_1D, range)} " +
                "(0 = coverage holds, and nothing was priced or bought)")
        println("  spent this session:   \$${"%.4f".format(batch.spentUsd)}")

        val m = c.loadMatrix(symbols, range, source = batch.source)
        println("\n  BarMatrix ${m.rows} x ${m.cols}  holes=${m.holes}  ${m.symbols}")
        println("  ${"date".padEnd(12)}${m.symbols.joinToString("") { it.padStart(10) }}")
        for (r in 0 until minOf(3, m.rows))
            println("  ${m.dates[r].take(10).padEnd(12)}" +
                    m.symbols.indices.joinToString("") { "%10.2f".format(m.rowMajor[r * m.cols + it]) })
        val aapl = m.symbols.indexOf("AAPL")
        if (aapl >= 0) println("\n  sanity: AAPL first close = ${m.rowMajor[aapl]}  (Alpaca 2024-08-01: 218.36)")
    }
}
