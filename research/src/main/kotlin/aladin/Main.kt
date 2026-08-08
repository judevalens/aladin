package aladin

import aladin.store.BarStore
import java.time.LocalDate

/**
 * Ask the store for bars.
 *
 *   SYMBOLS=AAPL,MSFT,NVDA FROM=2024-08-01 TO=2024-09-30 ./gradlew -q run
 *
 * Everything goes through [BarStore.bars] — never a fetcher directly. That is what
 * makes the rails hold: gaps are computed first, so a held range is never fetched,
 * never priced and never prompted for.
 */
fun main() {
    val symbols = (Env["SYMBOLS"] ?: "AAPL,MSFT,NVDA").split(",")
    val range = DateRange(
        LocalDate.parse(Env["FROM"] ?: "2024-08-01"),
        LocalDate.parse(Env["TO"] ?: "2024-09-30"),
    )

    BarStore.databento().use { store ->
        val started = System.nanoTime()
        val m = store.bars(symbols, range)
        val secs = (System.nanoTime() - started) / 1e9

        println("bars($symbols, $range) -> $m")
        println("  ${"%.1f".format(secs)}s   spent \$${"%.4f".format(store.spentUsd)}")
        if (store.spentUsd == 0.0) println("  (nothing missing — no request was priced or made)")

        println()
        println("  ${"date".padEnd(12)}${m.symbols.joinToString("") { it.padStart(10) }}")
        for (r in 0 until minOf(5, m.rows)) {
            println(
                "  ${m.dates[r].take(10).padEnd(12)}" +
                    m.symbols.indices.joinToString("") { "%10.2f".format(m[r, it]) }
            )
        }

        println("\nheld:")
        println(store.held())
    }
}
