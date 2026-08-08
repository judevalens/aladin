/**
 * Fetch daily bars into the store, then read them back as a matrix.
 *
 *   SMOKE_SYMBOLS=AAPL,MSFT,NVDA ./gradlew -q run -Dmc=FetchKt
 *
 * Everything goes through [Connection.ensureBars] — never a fetcher directly. That is
 * what makes the rails hold: gaps are computed first, so a held range is never fetched,
 * never priced and never prompted for, and only what is genuinely missing reaches the
 * budget gate.
 */

import java.time.LocalDate

private const val HARD_CEILING_USD = 5.00

fun main() {
    val symbols = (Env["SMOKE_SYMBOLS"] ?: "AAPL,MSFT,NVDA").split(",").map { it.trim() }
    val range = DateRange(
        LocalDate.parse(Env["FETCH_FROM"] ?: "2024-08-01"),
        LocalDate.parse(Env["FETCH_TO"] ?: "2024-09-30"),
    )

    val fetcher = BudgetedFetcher(
        DatabentoBatchFetcher(),
        autoApproveUnder = (Env["DATABENTO_AUTO_APPROVE_UNDER"] ?: "0.10").toDouble(),
        hardCeiling = HARD_CEILING_USD,
    )
    println("source=${fetcher.source}  symbols=$symbols  range=$range")

    openDb().use { c ->
        c.createInstrumentsTable(); c.createOhlcvTable(); c.createCoverageTable(); c.createSymbologyTable()

        // Mint an id only for a ticker the registry has never seen. Registering a second
        // id for an existing symbol is an identity failure, not a detail.
        val universe = symbols.withIndex().associate { (i, s) ->
            s to (c.resolveInstrument(s, range.from) ?: (9_000L + i).also {
                c.registerInstrument(Instrument(it, s, LocalDate.parse("1990-01-01"), null))
            })
        }

        val t0 = System.nanoTime()
        val fetched = c.ensureBars(fetcher, universe, Schema.OHLCV_1D, range)
        val secs = (System.nanoTime() - t0) / 1e9
        println("  fetched $fetched bars in ${"%.1f".format(secs)}s   spent \$${"%.4f".format(fetcher.spentUsd)}")
        if (fetched == 0L) println("  (nothing missing — no request was priced or made)")

        val m = c.loadMatrix(symbols, range, source = fetcher.source)
        println("\n  BarMatrix ${m.rows} x ${m.cols}  holes=${m.holes}")
        println("  ${"date".padEnd(12)}${m.symbols.joinToString("") { it.padStart(10) }}")
        for (r in 0 until minOf(5, m.rows))
            println("  ${m.dates[r].take(10).padEnd(12)}" +
                    m.symbols.indices.joinToString("") { "%10.2f".format(m.rowMajor[r * m.cols + it]) })
    }
}
