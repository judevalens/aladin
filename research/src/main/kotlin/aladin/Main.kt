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

    BarStore.databento().use { store ->
        val df = store.frame(
            listOf("AMZN"),
            DateRange(LocalDate.parse("2024-01-01"), LocalDate.parse("2026-07-30"))
        )
        println(
            df.columns()
        )
    }
}
