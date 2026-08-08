/**
 * Ask the store for bars.
 *
 *   SMOKE_SYMBOLS=AAPL,MSFT,NVDA ./gradlew -q run -Dmc=FetchKt
 */

import java.time.LocalDate

fun main() {
    val symbols = (Env["SMOKE_SYMBOLS"] ?: "AAPL,MSFT,NVDA").split(",").map { it.trim() }
    val range = DateRange(
        LocalDate.parse(Env["FETCH_FROM"] ?: "2024-08-01"),
        LocalDate.parse(Env["FETCH_TO"] ?: "2024-09-30"),
    )

    BarStore.databento().use { store ->
        val t0 = System.nanoTime()
        val m = store.bars(symbols, range)
        val secs = (System.nanoTime() - t0) / 1e9

        println("bars($symbols, $range) -> ${m.rows} x ${m.cols}  holes=${m.holes}")
        println("  took ${"%.1f".format(secs)}s   spent \$${"%.4f".format(store.spentUsd)}\n")

        println("  ${"date".padEnd(12)}${m.symbols.joinToString("") { it.padStart(10) }}")
        for (r in 0 until minOf(5, m.rows))
            println("  ${m.dates[r].take(10).padEnd(12)}" +
                    m.symbols.indices.joinToString("") { "%10.2f".format(m.rowMajor[r * m.cols + it]) })

        println("\nheld:")
        println(store.held())
    }
}
