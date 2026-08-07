import java.time.LocalDate

/** Stands in for Databento: records what it was asked for, so we can prove the cache works. */
private class CountingFetcher : BarFetcher {
    override val source = "fake"
    val calls = mutableListOf<DateRange>()
    override fun fetch(symbol: String, schema: String, range: DateRange): List<BarRow> {
        calls += range
        return generateSequence(range.from) { it.plusDays(1) }
            .takeWhile { !it.isAfter(range.to) }
            .filter { it.dayOfWeek.value <= 5 }                 // weekdays only, like a real tape
            .map { BarRow(it, symbol, 10.0, 11.0, 9.0, 10.5, 1000) }
            .toList()
    }
}

fun main() {
    val f = CountingFetcher()
    openDb().use { conn ->
        conn.createStatement().use { it.execute("DROP TABLE IF EXISTS coverage"); it.execute("DROP TABLE IF EXISTS ohlcv") }
        conn.createCoverageTable(); conn.createOhlcvTable()

        fun ask(label: String, from: String, to: String) {
            f.calls.clear()
            val n = conn.ensureBars(f, "NVDA", "ohlcv-1d", DateRange(LocalDate.parse(from), LocalDate.parse(to)))
            val stored = conn.createStatement().use { st ->
                st.executeQuery("SELECT count(*) FROM ohlcv").use { it.next(); it.getLong(1) }
            }
            println("  ${label.padEnd(34)} fetched=${n.toString().padStart(3)}  " +
                    "vendor calls=${f.calls}  storeTotal=$stored")
        }

        println("read-through, each call asking for a superset of the last:")
        ask("2024-01-01..2024-03-31", "2024-01-01", "2024-03-31")
        ask("same range again", "2024-01-01", "2024-03-31")
        ask("2024-01-01..2024-06-30", "2024-01-01", "2024-06-30")
        ask("2024-09-01..2024-09-30 (disjoint)", "2024-09-01", "2024-09-30")
        ask("2024-01-01..2024-09-30 (fills hole)", "2024-01-01", "2024-09-30")
        ask("whole year again", "2024-01-01", "2024-09-30")

        conn.createStatement().use { st ->
            st.executeQuery("SELECT start_date, end_date, rows FROM coverage ORDER BY start_date").use { rs ->
                println("\n  ledger:")
                while (rs.next()) println("    ${rs.getDate(1)} .. ${rs.getDate(2)}   ${rs.getLong(3)} bars")
            }
        }
    }
}
